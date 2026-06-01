package repository

import (
	"context"
	"testing"

	"github.com/irj0927/begit/pkg/d1"
)

// TestPostRepository_HasPostedInSprint は posts テーブルの存在確認クエリで true/false を返すことを確認する
func TestPostRepository_HasPostedInSprint_True(t *testing.T) {
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return []map[string]interface{}{
				{"count": float64(1)},
			}, nil
		},
	}

	repo := NewPostRepository(mock)
	hasPosted, err := repo.HasPostedInSprint(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("HasPostedInSprint() failed: %v", err)
	}
	if !hasPosted {
		t.Error("expected hasPosted=true")
	}
}

// TestPostRepository_HasPostedInSprint_False は未投稿の場合に false を返すことを確認する
func TestPostRepository_HasPostedInSprint_False(t *testing.T) {
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return []map[string]interface{}{
				{"count": float64(0)},
			}, nil
		},
	}

	repo := NewPostRepository(mock)
	hasPosted, err := repo.HasPostedInSprint(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("HasPostedInSprint() failed: %v", err)
	}
	if hasPosted {
		t.Error("expected hasPosted=false")
	}
}

// TestFCMTokenRepository_GetTokensByGroupID はグループ内の全 FCM トークンを返すことを確認する
func TestFCMTokenRepository_GetTokensByGroupID(t *testing.T) {
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return []map[string]interface{}{
				{"token": "token1"},
				{"token": "token2"},
			}, nil
		},
	}

	repo := NewFCMTokenRepository(mock)
	tokens, err := repo.GetTokensByGroupID(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetTokensByGroupID() failed: %v", err)
	}
	if len(tokens) != 2 {
		t.Errorf("expected 2 tokens, got %d", len(tokens))
	}
}

// TestFCMTokenRepository_GetTokensByGroupID_Empty は空の場合に空スライスを返すことを確認する
func TestFCMTokenRepository_GetTokensByGroupID_Empty(t *testing.T) {
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return nil, d1.ErrNotFound
		},
	}

	repo := NewFCMTokenRepository(mock)
	tokens, err := repo.GetTokensByGroupID(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetTokensByGroupID() should not fail when empty, got: %v", err)
	}
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(tokens))
	}
}
