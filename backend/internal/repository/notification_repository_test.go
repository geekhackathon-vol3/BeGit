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

// TestNotificationRepository_Create_Conflict は UNIQUE 制約違反が ErrConstraintViolation を返すことを確認する
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
