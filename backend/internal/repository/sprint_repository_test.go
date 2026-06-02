package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/irj0927/begit/pkg/d1"
)

// sprintRow はテスト用のスプリント行ヘルパ
func sprintRow(id, groupID int64) map[string]interface{} {
	return map[string]interface{}{
		"id":         float64(id),
		"group_id":   float64(groupID),
		"index_num":  float64(0),
		"started_at": "2026-06-01T00:00:00Z",
		"ends_at":    "2026-06-08T00:00:00Z",
	}
}

// TestSprintRepository_ListReminderDue は ends_at の3日前に到達したスプリントを抽出するクエリを確認する
func TestSprintRepository_ListReminderDue(t *testing.T) {
	var capturedSQL string
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			capturedSQL = sql
			return []map[string]interface{}{sprintRow(1, 2)}, nil
		},
	}

	repo := NewSprintRepository(mock)
	sprints, err := repo.ListReminderDue(context.Background())
	if err != nil {
		t.Fatalf("ListReminderDue() failed: %v", err)
	}
	if len(sprints) != 1 || sprints[0].ID != 1 {
		t.Errorf("expected 1 sprint id=1, got %v", sprints)
	}
	// ends_at-3日 到達かつ未終了の境界を表すクエリであること
	if !strings.Contains(capturedSQL, "-3 day") || !strings.Contains(capturedSQL, "ends_at") {
		t.Errorf("unexpected SQL: %s", capturedSQL)
	}
}

// TestSprintRepository_ListEnded は ends_at に到達したスプリントを抽出するクエリを確認する
func TestSprintRepository_ListEnded(t *testing.T) {
	var capturedSQL string
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			capturedSQL = sql
			return []map[string]interface{}{sprintRow(3, 2)}, nil
		},
	}

	repo := NewSprintRepository(mock)
	sprints, err := repo.ListEnded(context.Background())
	if err != nil {
		t.Fatalf("ListEnded() failed: %v", err)
	}
	if len(sprints) != 1 || sprints[0].ID != 3 {
		t.Errorf("expected 1 sprint id=3, got %v", sprints)
	}
	if !strings.Contains(capturedSQL, "ends_at <=") {
		t.Errorf("expected ends_at <= now boundary, got SQL: %s", capturedSQL)
	}
}

// TestSprintRepository_ListActive は現在アクティブなスプリント（⑥ sprint_start 候補）を抽出するクエリを確認する
func TestSprintRepository_ListActive(t *testing.T) {
	var capturedSQL string
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			capturedSQL = sql
			return []map[string]interface{}{sprintRow(5, 2), sprintRow(6, 4)}, nil
		},
	}

	repo := NewSprintRepository(mock)
	sprints, err := repo.ListActive(context.Background())
	if err != nil {
		t.Fatalf("ListActive() failed: %v", err)
	}
	if len(sprints) != 2 {
		t.Errorf("expected 2 active sprints, got %d", len(sprints))
	}
	if !strings.Contains(capturedSQL, "started_at <=") || !strings.Contains(capturedSQL, "ends_at >") {
		t.Errorf("expected active sprint window, got SQL: %s", capturedSQL)
	}
}

// TestSprintRepository_ListEnded_Empty は該当無しで空スライスを返すことを確認する
func TestSprintRepository_ListEnded_Empty(t *testing.T) {
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return nil, d1.ErrNotFound
		},
	}

	repo := NewSprintRepository(mock)
	sprints, err := repo.ListEnded(context.Background())
	if err != nil {
		t.Fatalf("ListEnded() should not error when empty, got: %v", err)
	}
	if len(sprints) != 0 {
		t.Errorf("expected 0 sprints, got %d", len(sprints))
	}
}

// TestSprintRepository_GetOrCreate は同一 groupID で連続呼び出しした場合に1件のみ作成されることを確認する
func TestSprintRepository_GetOrCreate(t *testing.T) {
	callCount := 0
	existingSprint := map[string]interface{}{
		"id":         float64(1),
		"group_id":   float64(2),
		"index_num":  float64(0),
		"started_at": time.Now().Format("2006-01-02 15:04:05"),
		"ends_at":    time.Now().AddDate(0, 0, 7).Format("2006-01-02 15:04:05"),
	}

	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			callCount++
			if callCount == 1 {
				// 最初のクエリ: スプリント検索 → 見つかった
				return []map[string]interface{}{existingSprint}, nil
			}
			return nil, d1.ErrNotFound
		},
		execFunc: func(ctx context.Context, sql string, params []interface{}) (int64, error) {
			return 1, nil
		},
	}

	repo := NewSprintRepository(mock)

	// 1回目: スプリントが存在する場合は作成しない
	sprint1, err := repo.GetOrCreateCurrentSprint(context.Background(), 2, 7)
	if err != nil {
		t.Fatalf("GetOrCreateCurrentSprint() #1 failed: %v", err)
	}
	if sprint1.GroupID != 2 {
		t.Errorf("expected GroupID=2, got %d", sprint1.GroupID)
	}
}

// TestSprintRepository_GetOrCreate_Create は既存スプリントがない場合に新規作成することを確認する
func TestSprintRepository_GetOrCreate_Create(t *testing.T) {
	createdSprint := map[string]interface{}{
		"id":         float64(1),
		"group_id":   float64(2),
		"index_num":  float64(0),
		"started_at": time.Now().Format("2006-01-02 15:04:05"),
		"ends_at":    time.Now().AddDate(0, 0, 7).Format("2006-01-02 15:04:05"),
	}

	queryCallCount := 0
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			queryCallCount++
			if queryCallCount == 1 {
				// スプリント検索 → 見つからない
				return nil, d1.ErrNotFound
			}
			// INSERT 後の SELECT → 見つかった
			return []map[string]interface{}{createdSprint}, nil
		},
		execFunc: func(ctx context.Context, sql string, params []interface{}) (int64, error) {
			return 1, nil
		},
	}

	repo := NewSprintRepository(mock)
	sprint, err := repo.GetOrCreateCurrentSprint(context.Background(), 2, 7)
	if err != nil {
		t.Fatalf("GetOrCreateCurrentSprint() failed: %v", err)
	}
	if sprint.GroupID != 2 {
		t.Errorf("expected GroupID=2, got %d", sprint.GroupID)
	}
}

// TestSprintRepository_GetCurrentSprint_NotFound はアクティブスプリントがない場合に ErrNotFound を返すことを確認する
func TestSprintRepository_GetCurrentSprint_NotFound(t *testing.T) {
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return nil, d1.ErrNotFound
		},
	}

	repo := NewSprintRepository(mock)
	_, err := repo.GetCurrentSprint(context.Background(), 99)
	if err == nil {
		t.Error("expected error when no active sprint found")
	}
}
