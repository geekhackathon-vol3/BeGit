package repository

import (
	"context"
	"testing"

	"github.com/irj0927/begit/pkg/d1"
)

// TestReactionRepository_ListByPostID はリアクション一覧を返すことを確認する
func TestReactionRepository_ListByPostID(t *testing.T) {
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return []map[string]interface{}{
				{"id": float64(1), "post_id": float64(10), "user_id": float64(2), "reaction_type": "heart", "login": "alice", "avatar_url": "a.png"},
				{"id": float64(2), "post_id": float64(10), "user_id": float64(3), "reaction_type": "lgtm", "login": "bob", "avatar_url": "b.png"},
			}, nil
		},
	}

	repo := NewReactionRepository(mock)
	reactions, err := repo.ListByPostID(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListByPostID() failed: %v", err)
	}
	if len(reactions) != 2 {
		t.Fatalf("expected 2 reactions, got %d", len(reactions))
	}
	if reactions[0].ReactionType != "heart" || reactions[0].Login != "alice" {
		t.Errorf("unexpected first reaction: %+v", reactions[0])
	}
}

// TestReactionRepository_ListByPostID_Empty は空の場合に空スライスを返すことを確認する
func TestReactionRepository_ListByPostID_Empty(t *testing.T) {
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return nil, d1.ErrNotFound
		},
	}

	repo := NewReactionRepository(mock)
	reactions, err := repo.ListByPostID(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListByPostID() should not fail when empty, got: %v", err)
	}
	if len(reactions) != 0 {
		t.Errorf("expected 0 reactions, got %d", len(reactions))
	}
}

// TestReactionRepository_Add は INSERT OR IGNORE を実行することを確認する
func TestReactionRepository_Add(t *testing.T) {
	called := false
	mock := &mockD1Client{
		execFunc: func(ctx context.Context, sql string, params []interface{}) (int64, error) {
			called = true
			if len(params) != 3 {
				t.Errorf("expected 3 params, got %d", len(params))
			}
			return 1, nil
		},
	}

	repo := NewReactionRepository(mock)
	if err := repo.Add(context.Background(), 10, 2, "heart"); err != nil {
		t.Fatalf("Add() failed: %v", err)
	}
	if !called {
		t.Error("expected Exec to be called")
	}
}

// TestReactionRepository_Remove は DELETE を実行することを確認する
func TestReactionRepository_Remove(t *testing.T) {
	called := false
	mock := &mockD1Client{
		execFunc: func(ctx context.Context, sql string, params []interface{}) (int64, error) {
			called = true
			return 1, nil
		},
	}

	repo := NewReactionRepository(mock)
	if err := repo.Remove(context.Background(), 10, 2, "heart"); err != nil {
		t.Fatalf("Remove() failed: %v", err)
	}
	if !called {
		t.Error("expected Exec to be called")
	}
}
