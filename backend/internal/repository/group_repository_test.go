package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/irj0927/begit/pkg/d1"
)

// TestGroupRepository_GetByID_NotFound は存在しない group_id に対して ErrNotFound を返すことを確認する
func TestGroupRepository_GetByID_NotFound(t *testing.T) {
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return nil, d1.ErrNotFound
		},
	}

	repo := NewGroupRepository(mock)
	_, err := repo.GetByID(context.Background(), 9999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestGroupRepository_IsMember はメンバーシップ確認が正しく動作することを確認する
func TestGroupRepository_IsMember(t *testing.T) {
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return []map[string]interface{}{
				{"count": float64(1)},
			}, nil
		},
	}

	repo := NewGroupRepository(mock)
	isMember, err := repo.IsMember(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("IsMember() failed: %v", err)
	}
	if !isMember {
		t.Error("expected isMember=true")
	}
}

// TestGroupRepository_IsMember_NotMember は非メンバーに false を返すことを確認する
func TestGroupRepository_IsMember_NotMember(t *testing.T) {
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return []map[string]interface{}{
				{"count": float64(0)},
			}, nil
		},
	}

	repo := NewGroupRepository(mock)
	isMember, err := repo.IsMember(context.Background(), 1, 9999)
	if err != nil {
		t.Fatalf("IsMember() failed: %v", err)
	}
	if isMember {
		t.Error("expected isMember=false")
	}
}

// TestGroupRepository_Create はグループ作成が正常に動作することを確認する
func TestGroupRepository_Create(t *testing.T) {
	createdID := float64(1)
	mock := &mockD1Client{
		execFunc: func(ctx context.Context, sql string, params []interface{}) (int64, error) {
			return 1, nil
		},
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return []map[string]interface{}{
				{
					"id":                   createdID,
					"repo_full_name":        "owner/repo",
					"name":                  "My Team",
					"avatar_url":            "https://example.com/avatar.png",
					"owner_user_id":         float64(1),
					"sprint_duration_days":  float64(7),
					"created_at":            "2026-06-01 00:00:00",
				},
			}, nil
		},
	}

	repo := NewGroupRepository(mock)
	group, err := repo.Create(context.Background(), &groupCreateInput{
		RepoFullName: "owner/repo",
		Name:         "My Team",
		AvatarURL:    "https://example.com/avatar.png",
		OwnerUserID:  1,
	})
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}
	if group.RepoFullName != "owner/repo" {
		t.Errorf("expected RepoFullName=owner/repo, got %s", group.RepoFullName)
	}
}
