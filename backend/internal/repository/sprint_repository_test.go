package repository

import (
	"context"
	"testing"
	"time"

	"github.com/irj0927/begit/pkg/d1"
)

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
