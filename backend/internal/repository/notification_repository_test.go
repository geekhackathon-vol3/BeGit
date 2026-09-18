package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/pkg/d1"
)

// mustTime は RFC3339 文字列を time.Time にパースする（テストヘルパ）
func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

// contains は s が substr を含むか（テストヘルパ）
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// TestNotificationRepository_Create_Conflict は D1 の制約違反が ErrConstraintViolation に変換されることを確認する
func TestNotificationRepository_Create_Conflict(t *testing.T) {
	mock := &mockD1Client{
		execFunc: func(ctx context.Context, sql string, params []interface{}) (int64, error) {
			return 0, d1.ErrConstraintViolation
		},
	}

	repo := NewNotificationRepository(mock)
	_, err := repo.Create(context.Background(), &model.Notification{
		SprintID: 1,
		SentBy:   2,
		Message:  "test",
	})
	if !errors.Is(err, ErrConstraintViolation) {
		t.Errorf("expected ErrConstraintViolation, got %v", err)
	}
}

// TestNotificationRepository_CreateIfNoActive_OncePerSprintCondition は allowMultiplePerSprint に応じて
// 「1スプリント1人1回」の条件（sent_by の NOT EXISTS）を付け外しすることを確認する
func TestNotificationRepository_CreateIfNoActive_OncePerSprintCondition(t *testing.T) {
	cases := []struct {
		name          string
		allowMultiple bool
		wantSenderSQL bool
		wantParams    int
	}{
		{name: "既定は1人1回を判定する", allowMultiple: false, wantSenderSQL: true, wantParams: 6},
		{name: "許可時は1人1回を判定しない", allowMultiple: true, wantSenderSQL: false, wantParams: 4},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotSQL string
			var gotParams []interface{}
			mock := &mockD1Client{
				execFunc: func(ctx context.Context, sql string, params []interface{}) (int64, error) {
					gotSQL, gotParams = sql, params
					return 1, nil
				},
				queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
					return []map[string]interface{}{{"id": float64(1), "sprint_id": float64(7), "sent_by": float64(2), "message": "m", "sent_at": "2026-09-16 09:00:00"}}, nil
				},
			}

			repo := NewNotificationRepository(mock)
			if _, err := repo.CreateIfNoActive(context.Background(), &model.Notification{SprintID: 7, SentBy: 2}, tc.allowMultiple); err != nil {
				t.Fatalf("CreateIfNoActive() failed: %v", err)
			}
			if !strings.Contains(gotSQL, "+1 hour") {
				t.Errorf("active-challenge condition must always be present, sql=%s", gotSQL)
			}
			if got := strings.Contains(gotSQL, "sent_by = ?"); got != tc.wantSenderSQL {
				t.Errorf("sender condition present=%v, want %v", got, tc.wantSenderSQL)
			}
			if len(gotParams) != tc.wantParams {
				t.Errorf("params len=%d, want %d", len(gotParams), tc.wantParams)
			}
		})
	}
}

// TestNotificationRepository_CreateIfNoActive_Blocked は条件に合わず 0 行のとき ErrConstraintViolation を返すことを確認する
func TestNotificationRepository_CreateIfNoActive_Blocked(t *testing.T) {
	mock := &mockD1Client{
		execFunc: func(ctx context.Context, sql string, params []interface{}) (int64, error) {
			return 0, nil
		},
	}

	repo := NewNotificationRepository(mock)
	_, err := repo.CreateIfNoActive(context.Background(), &model.Notification{SprintID: 7, SentBy: 2}, true)
	if !errors.Is(err, ErrConstraintViolation) {
		t.Errorf("expected ErrConstraintViolation, got %v", err)
	}
}

// TestNotificationRepository_GetLatestInSprintBefore は時刻以前で最新の通知を返すことを確認する
func TestNotificationRepository_GetLatestInSprintBefore(t *testing.T) {
	var capturedSQL string
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			capturedSQL = sql
			return []map[string]interface{}{
				{
					"id":        float64(42),
					"sprint_id": float64(7),
					"sent_by":   float64(10),
					"message":   "今なに作ってる？",
					"sent_at":   "2026-06-02T10:00:00Z",
				},
			}, nil
		},
	}

	repo := NewNotificationRepository(mock)
	notif, err := repo.GetLatestInSprintBefore(context.Background(), 7, mustTime("2026-06-02T10:30:00Z"))
	if err != nil {
		t.Fatalf("GetLatestInSprintBefore() failed: %v", err)
	}
	if notif.ID != 42 {
		t.Errorf("expected notif id=42, got %d", notif.ID)
	}
	// 時刻以前 (sent_at <= ?) かつ降順最新 を表すクエリであることをゆるく確認
	if !contains(capturedSQL, "sprint_id") || !contains(capturedSQL, "sent_at") {
		t.Errorf("unexpected SQL: %s", capturedSQL)
	}
}

// TestNotificationRepository_GetLatestInSprintBefore_None は anchor 無しで ErrNotFound を返すことを確認する
func TestNotificationRepository_GetLatestInSprintBefore_None(t *testing.T) {
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return nil, d1.ErrNotFound
		},
	}

	repo := NewNotificationRepository(mock)
	_, err := repo.GetLatestInSprintBefore(context.Background(), 7, mustTime("2026-06-02T10:30:00Z"))
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestNotificationRepository_HasActiveInSprint_True はアクティブ通知が存在する場合に true を返すことを確認する
func TestNotificationRepository_HasActiveInSprint_True(t *testing.T) {
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return []map[string]interface{}{{"count": float64(1)}}, nil
		},
	}

	repo := NewNotificationRepository(mock)
	active, err := repo.HasActiveInSprint(context.Background(), 7)
	if err != nil {
		t.Fatalf("HasActiveInSprint() failed: %v", err)
	}
	if !active {
		t.Error("expected active=true")
	}
}

// TestNotificationRepository_HasActiveInSprint_False は境界（ちょうど +1h 経過）でアクティブ無しと判定することを確認する
func TestNotificationRepository_HasActiveInSprint_False(t *testing.T) {
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return []map[string]interface{}{{"count": float64(0)}}, nil
		},
	}

	repo := NewNotificationRepository(mock)
	active, err := repo.HasActiveInSprint(context.Background(), 7)
	if err != nil {
		t.Fatalf("HasActiveInSprint() failed: %v", err)
	}
	if active {
		t.Error("expected active=false at +1h boundary")
	}
}

// TestNotificationRepository_ListChallengeEndDue は sent_at+1h 到達の通知を返すクエリを確認する
func TestNotificationRepository_ListChallengeEndDue(t *testing.T) {
	var capturedSQL string
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			capturedSQL = sql
			return []map[string]interface{}{
				{"id": float64(345), "sprint_id": float64(7), "sent_by": float64(1), "message": "m", "sent_at": "2026-06-02T08:00:00Z"},
			}, nil
		},
	}
	repo := NewNotificationRepository(mock)
	notifs, err := repo.ListChallengeEndDue(context.Background())
	if err != nil {
		t.Fatalf("ListChallengeEndDue() failed: %v", err)
	}
	if len(notifs) != 1 || notifs[0].ID != 345 {
		t.Errorf("expected 1 notif id=345, got %v", notifs)
	}
	if !contains(capturedSQL, "+1 hour") {
		t.Errorf("expected +1 hour boundary in SQL: %s", capturedSQL)
	}
}

// TestPostRepository_CreateMissed_Conflict は既投稿で UNIQUE 違反を返すことを確認する（skip 用）
func TestPostRepository_CreateMissed_Conflict(t *testing.T) {
	mock := &mockD1Client{
		execFunc: func(ctx context.Context, sql string, params []interface{}) (int64, error) {
			return 0, d1.ErrConstraintViolation
		},
	}
	repo := NewPostRepository(mock)
	err := repo.CreateMissed(context.Background(), 100, 1, 12)
	if err != ErrConstraintViolation {
		t.Errorf("expected ErrConstraintViolation, got %v", err)
	}
}

// TestWebhookRepository_InsertDelivery_Duplicate は同じ delivery_id で2回呼んだとき isDuplicate=true が返ることを確認する
func TestWebhookRepository_InsertDelivery_Duplicate(t *testing.T) {
	callCount := 0
	mock := &mockD1Client{
		execFunc: func(ctx context.Context, sql string, params []interface{}) (int64, error) {
			callCount++
			if callCount >= 2 {
				return 0, d1.ErrConstraintViolation
			}
			return 1, nil
		},
	}

	repo := NewWebhookRepository(mock)

	// 1回目: 正常
	isDuplicate1, err1 := repo.InsertDelivery(context.Background(), "delivery-uuid-123", "push")
	if err1 != nil {
		t.Fatalf("InsertDelivery() #1 failed: %v", err1)
	}
	if isDuplicate1 {
		t.Error("expected isDuplicate=false for first call")
	}

	// 2回目: UNIQUE 制約違反 → isDuplicate=true, err=nil
	isDuplicate2, err2 := repo.InsertDelivery(context.Background(), "delivery-uuid-123", "push")
	if err2 != nil {
		t.Fatalf("InsertDelivery() #2 should not return error, got: %v", err2)
	}
	if !isDuplicate2 {
		t.Error("expected isDuplicate=true for second call with same delivery_id")
	}
}

// TestNotificationRepository_ActivePredicateExcludesEnded は「進行中」判定（HasActiveInSprint / CreateIfNoActive）が
// 途中中断済み（ended_at 非 NULL）の通知を除外する SQL になっていることを確認する。
// これが無いと「中断 → 即再発行」が 409 のままになる。
func TestNotificationRepository_ActivePredicateExcludesEnded(t *testing.T) {
	var querySQL, execSQL string
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			querySQL = sql
			return []map[string]interface{}{{"count": float64(0)}}, nil
		},
		execFunc: func(ctx context.Context, sql string, params []interface{}) (int64, error) {
			execSQL = sql
			return 0, nil
		},
	}
	repo := NewNotificationRepository(mock)

	if _, err := repo.HasActiveInSprint(context.Background(), 7); err != nil {
		t.Fatalf("HasActiveInSprint() failed: %v", err)
	}
	if !contains(querySQL, "ended_at IS NULL") || !contains(querySQL, "+1 hour") {
		t.Errorf("HasActiveInSprint SQL should exclude ended notifications: %s", querySQL)
	}

	_, _ = repo.CreateIfNoActive(context.Background(), &model.Notification{SprintID: 7, SentBy: 1}, true)
	if !contains(execSQL, "ended_at IS NULL") || !contains(execSQL, "+1 hour") {
		t.Errorf("CreateIfNoActive SQL should exclude ended notifications: %s", execSQL)
	}
}

// TestNotificationRepository_ListChallengeEndDue_IncludesEnded は途中中断済み（ended_at 非 NULL）の通知も
// ③ challenge_end の対象として抽出されること（SQL に OR ended_at IS NOT NULL がある）を確認する
func TestNotificationRepository_ListChallengeEndDue_IncludesEnded(t *testing.T) {
	var capturedSQL string
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			capturedSQL = sql
			return []map[string]interface{}{
				{"id": float64(8), "sprint_id": float64(5), "sent_by": float64(23), "message": "m",
					"sent_at": "2026-09-18 01:25:46", "ended_at": "2026-09-18 01:40:00"},
			}, nil
		},
	}
	repo := NewNotificationRepository(mock)
	notifs, err := repo.ListChallengeEndDue(context.Background())
	if err != nil {
		t.Fatalf("ListChallengeEndDue() failed: %v", err)
	}
	if !contains(capturedSQL, "ended_at IS NOT NULL") {
		t.Errorf("expected ended_at condition in SQL: %s", capturedSQL)
	}
	if len(notifs) != 1 || notifs[0].EndedAt == nil {
		t.Fatalf("expected 1 notif with EndedAt, got %+v", notifs)
	}
	if got := notifs[0].EndedAt.Format("2006-01-02 15:04:05"); got != "2026-09-18 01:40:00" {
		t.Errorf("unexpected EndedAt: %s", got)
	}
}

// TestNotificationRepository_ScanNotification_EndedAtNull は ended_at が NULL/欠落のとき EndedAt が nil のままになることを確認する
func TestNotificationRepository_ScanNotification_EndedAtNull(t *testing.T) {
	for name, row := range map[string]map[string]interface{}{
		"nil":     {"id": float64(1), "sprint_id": float64(1), "sent_by": float64(1), "sent_at": "2026-09-18T01:25:46Z", "ended_at": nil},
		"missing": {"id": float64(1), "sprint_id": float64(1), "sent_by": float64(1), "sent_at": "2026-09-18T01:25:46Z"},
	} {
		n, err := scanNotification(row)
		if err != nil {
			t.Fatalf("[%s] scanNotification() failed: %v", name, err)
		}
		if n.EndedAt != nil {
			t.Errorf("[%s] expected EndedAt nil, got %v", name, n.EndedAt)
		}
	}
}

// TestNotificationRepository_GetActiveByGroup はグループ単位（sprints JOIN）で進行中の通知を1件返すことを確認する
func TestNotificationRepository_GetActiveByGroup(t *testing.T) {
	var capturedSQL string
	var capturedParams []interface{}
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			capturedSQL = sql
			capturedParams = params
			return []map[string]interface{}{
				{"id": float64(8), "sprint_id": float64(5), "sent_by": float64(23), "message": "m", "sent_at": "2026-09-18 01:25:46"},
			}, nil
		},
	}
	repo := NewNotificationRepository(mock)
	n, err := repo.GetActiveByGroup(context.Background(), 12)
	if err != nil {
		t.Fatalf("GetActiveByGroup() failed: %v", err)
	}
	if n.ID != 8 || n.SprintID != 5 || n.SentBy != 23 || n.EndedAt != nil {
		t.Errorf("unexpected notification: %+v", n)
	}
	for _, want := range []string{"JOIN sprints", "group_id = ?", "ended_at IS NULL", "+1 hour", "ORDER BY n.sent_at DESC", "LIMIT 1"} {
		if !contains(capturedSQL, want) {
			t.Errorf("expected %q in SQL: %s", want, capturedSQL)
		}
	}
	if len(capturedParams) != 1 || capturedParams[0] != int64(12) {
		t.Errorf("expected params [12], got %v", capturedParams)
	}
}

// TestNotificationRepository_GetActiveByGroup_None は進行中が無ければ ErrNotFound を返すことを確認する
func TestNotificationRepository_GetActiveByGroup_None(t *testing.T) {
	for name, mock := range map[string]*mockD1Client{
		"d1 not found": {queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return nil, d1.ErrNotFound
		}},
		"empty rows": {queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return []map[string]interface{}{}, nil
		}},
	} {
		repo := NewNotificationRepository(mock)
		_, err := repo.GetActiveByGroup(context.Background(), 12)
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("[%s] expected ErrNotFound, got %v", name, err)
		}
	}
}

// TestNotificationRepository_EndIfActive は条件付き UPDATE の結果（更新行数）を bool に変換することを確認する
func TestNotificationRepository_EndIfActive(t *testing.T) {
	for name, tc := range map[string]struct {
		rows int64
		want bool
	}{
		"ended":      {rows: 1, want: true},
		"not active": {rows: 0, want: false},
	} {
		var capturedSQL string
		var capturedParams []interface{}
		mock := &mockD1Client{
			execFunc: func(ctx context.Context, sql string, params []interface{}) (int64, error) {
				capturedSQL = sql
				capturedParams = params
				return tc.rows, nil
			},
		}
		repo := NewNotificationRepository(mock)
		got, err := repo.EndIfActive(context.Background(), 8)
		if err != nil {
			t.Fatalf("[%s] EndIfActive() failed: %v", name, err)
		}
		if got != tc.want {
			t.Errorf("[%s] expected %v, got %v", name, tc.want, got)
		}
		for _, want := range []string{"UPDATE notifications", "SET ended_at = datetime('now')", "ended_at IS NULL", "+1 hour"} {
			if !contains(capturedSQL, want) {
				t.Errorf("[%s] expected %q in SQL: %s", name, want, capturedSQL)
			}
		}
		if len(capturedParams) != 1 || capturedParams[0] != int64(8) {
			t.Errorf("[%s] expected params [8], got %v", name, capturedParams)
		}
	}
}
