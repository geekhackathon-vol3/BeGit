package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/pkg/d1"
)

// TestPostRepository_ListByGroupID_ExcludesDraft はフィード一覧クエリが is_draft = 0 で絞ることを確認する
func TestPostRepository_ListByGroupID_ExcludesDraft(t *testing.T) {
	var capturedSQL string
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			capturedSQL = sql
			return []map[string]interface{}{}, nil
		},
	}

	repo := NewPostRepository(mock)
	if _, err := repo.ListByGroupID(context.Background(), 1); err != nil {
		t.Fatalf("ListByGroupID() failed: %v", err)
	}
	if !strings.Contains(capturedSQL, "is_draft = 0") {
		t.Errorf("expected ListByGroupID to filter is_draft = 0, got SQL: %s", capturedSQL)
	}
}

// TestPostRepository_CreateDraft は is_draft=1 と notification_id を指定して draft を作成することを確認する
func TestPostRepository_CreateDraft(t *testing.T) {
	var capturedSQL string
	var capturedParams []interface{}
	mock := &mockD1Client{
		execFunc: func(ctx context.Context, sql string, params []interface{}) (int64, error) {
			capturedSQL = sql
			capturedParams = params
			return 1, nil
		},
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return []map[string]interface{}{
				{"id": float64(890), "notification_id": float64(345), "user_id": float64(10), "group_id": float64(12), "post_type": "commit", "is_draft": float64(1), "created_at": "2026-06-02T10:00:00Z"},
			}, nil
		},
	}

	repo := NewPostRepository(mock)
	notifID := int64(345)
	branch := "main"
	created, err := repo.CreateDraft(context.Background(), &model.Post{
		NotificationID: &notifID,
		UserID:         10,
		GroupID:        12,
		PostType:       "commit",
		BranchName:     &branch,
		CommitCount:    3,
	})
	if err != nil {
		t.Fatalf("CreateDraft() failed: %v", err)
	}
	if created.ID != 890 {
		t.Errorf("expected created draft id=890, got %d", created.ID)
	}
	if !strings.Contains(capturedSQL, "is_draft") || !strings.Contains(capturedSQL, "1)") {
		t.Errorf("expected INSERT to set is_draft=1, got SQL: %s", capturedSQL)
	}
	if !strings.Contains(capturedSQL, "branch_name") {
		t.Errorf("expected INSERT to persist branch_name, got SQL: %s", capturedSQL)
	}
	// branch_name がパラメータに含まれること（draft プレフィル用）
	foundBranch := false
	for _, p := range capturedParams {
		if v, ok := p.(*string); ok && v != nil && *v == "main" {
			foundBranch = true
		}
	}
	if !foundBranch {
		t.Errorf("expected branch_name param, got: %v", capturedParams)
	}
}

// TestPostRepository_CreateDraft_Conflict は UNIQUE(notification_id,user_id) 違反で ErrConstraintViolation を返すことを確認する
func TestPostRepository_CreateDraft_Conflict(t *testing.T) {
	mock := &mockD1Client{
		execFunc: func(ctx context.Context, sql string, params []interface{}) (int64, error) {
			return 0, d1.ErrConstraintViolation
		},
	}

	repo := NewPostRepository(mock)
	notifID := int64(345)
	_, err := repo.CreateDraft(context.Background(), &model.Post{NotificationID: &notifID, UserID: 10, GroupID: 12, PostType: "commit"})
	if err != ErrConstraintViolation {
		t.Errorf("expected ErrConstraintViolation, got %v", err)
	}
}

// TestPostRepository_ConfirmDraft は is_draft=0 へ更新する UPDATE を発行することを確認する
func TestPostRepository_ConfirmDraft(t *testing.T) {
	var capturedSQL string
	mock := &mockD1Client{
		execFunc: func(ctx context.Context, sql string, params []interface{}) (int64, error) {
			capturedSQL = sql
			return 1, nil
		},
	}

	repo := NewPostRepository(mock)
	if err := repo.ConfirmDraft(context.Background(), 890); err != nil {
		t.Fatalf("ConfirmDraft() failed: %v", err)
	}
	if !strings.Contains(capturedSQL, "is_draft = 0") || !strings.Contains(capturedSQL, "UPDATE") {
		t.Errorf("expected UPDATE setting is_draft = 0, got SQL: %s", capturedSQL)
	}
}

// TestPostRepository_ConfirmDraft_Idempotent は既に確定済み(0行更新)でもエラーにしないことを確認する
func TestPostRepository_ConfirmDraft_Idempotent(t *testing.T) {
	mock := &mockD1Client{
		execFunc: func(ctx context.Context, sql string, params []interface{}) (int64, error) {
			return 0, nil // 既に is_draft=0 なら影響行0
		},
	}

	repo := NewPostRepository(mock)
	if err := repo.ConfirmDraft(context.Background(), 890); err != nil {
		t.Fatalf("ConfirmDraft() should be idempotent, got: %v", err)
	}
}

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
