package service

import (
	"context"
	"errors"
	"testing"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
	githubpkg "github.com/irj0927/begit/pkg/github"
)

// TestGroupService_SyncMembers_Success は登録済みコラボレーターが追加され最新一覧を返すことを確認する
func TestGroupService_SyncMembers_Success(t *testing.T) {
	var batchIDs []int64
	githubClient := &mockGitHubClient{
		getCollaboratorsFunc: func(ctx context.Context, repoFullName, accessToken string) ([]githubpkg.User, error) {
			return []githubpkg.User{
				{Login: "alice"},
				{Login: "bob"},
				{Login: "unregistered"},
				{Login: ""},
			}, nil
		},
	}
	groupRepo := &mockGroupRepository{
		getByIDFunc: func(ctx context.Context, groupID int64) (*model.Group, error) {
			return &model.Group{ID: groupID, RepoFullName: "alice/repo"}, nil
		},
		batchAddMembersFunc: func(ctx context.Context, groupID int64, userIDs []int64, role string) error {
			batchIDs = userIDs
			return nil
		},
		getMembersFunc: func(ctx context.Context, groupID int64) ([]model.GroupMember, error) {
			return []model.GroupMember{
				{UserID: 1, Login: "alice", Role: "owner"},
				{UserID: 2, Login: "bob", Role: "member"},
			}, nil
		},
	}
	userRepo := &mockUserRepository{
		getByGitHubLoginFunc: func(ctx context.Context, login string) (*model.User, error) {
			switch login {
			case "alice":
				return &model.User{ID: 1, GitHubLogin: "alice"}, nil
			case "bob":
				return &model.User{ID: 2, GitHubLogin: "bob"}, nil
			default:
				return nil, repository.ErrNotFound
			}
		},
	}

	svc := NewGroupService(GroupServiceConfig{}, githubClient, groupRepo, userRepo)
	members, err := svc.SyncMembers(context.Background(), 1, "token")
	if err != nil {
		t.Fatalf("SyncMembers() failed: %v", err)
	}
	if len(batchIDs) != 2 {
		t.Errorf("expected 2 registered members added, got %v", batchIDs)
	}
	if len(members) != 2 {
		t.Errorf("expected 2 members in result, got %d", len(members))
	}
}

// TestGroupService_SyncMembers_GroupNotFound はグループ未存在で ErrNotFound を返すことを確認する
func TestGroupService_SyncMembers_GroupNotFound(t *testing.T) {
	groupRepo := &mockGroupRepository{
		getByIDFunc: func(ctx context.Context, groupID int64) (*model.Group, error) {
			return nil, repository.ErrNotFound
		},
	}
	svc := NewGroupService(GroupServiceConfig{}, &mockGitHubClient{}, groupRepo, &mockUserRepository{})

	_, err := svc.SyncMembers(context.Background(), 1, "token")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
