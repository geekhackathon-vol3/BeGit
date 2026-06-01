package repository

import (
	"context"
	"testing"

	"github.com/irj0927/begit/pkg/d1"
)

// TestCommentRepository_ListByPostID はコメント一覧を返すことを確認する
func TestCommentRepository_ListByPostID(t *testing.T) {
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return []map[string]interface{}{
				{"id": float64(1), "post_id": float64(10), "user_id": float64(2), "body": "nice", "created_at": "2026-06-01 10:00:00", "login": "alice", "avatar_url": "a.png"},
			}, nil
		},
	}

	repo := NewCommentRepository(mock)
	comments, err := repo.ListByPostID(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListByPostID() failed: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	if comments[0].Body != "nice" || comments[0].Login != "alice" {
		t.Errorf("unexpected comment: %+v", comments[0])
	}
}

// TestCommentRepository_ListByPostID_Empty は空の場合に空スライスを返すことを確認する
func TestCommentRepository_ListByPostID_Empty(t *testing.T) {
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return nil, d1.ErrNotFound
		},
	}

	repo := NewCommentRepository(mock)
	comments, err := repo.ListByPostID(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListByPostID() should not fail when empty, got: %v", err)
	}
	if len(comments) != 0 {
		t.Errorf("expected 0 comments, got %d", len(comments))
	}
}

// TestCommentRepository_GetByID_NotFound は存在しない場合に ErrNotFound を返すことを確認する
func TestCommentRepository_GetByID_NotFound(t *testing.T) {
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return []map[string]interface{}{}, nil
		},
	}

	repo := NewCommentRepository(mock)
	_, err := repo.GetByID(context.Background(), 999)
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// TestCommentRepository_Delete は DELETE を実行することを確認する
func TestCommentRepository_Delete(t *testing.T) {
	called := false
	mock := &mockD1Client{
		execFunc: func(ctx context.Context, sql string, params []interface{}) (int64, error) {
			called = true
			return 1, nil
		},
	}

	repo := NewCommentRepository(mock)
	if err := repo.Delete(context.Background(), 1); err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}
	if !called {
		t.Error("expected Exec to be called")
	}
}
