package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
	githubpkg "github.com/irj0927/begit/pkg/github"
)

// mockGroupRepository はテスト用のグループリポジトリモック
type mockGroupRepository struct {
	listByUserIDFunc      func(ctx context.Context, userID int64) ([]model.Group, error)
	createFunc            func(ctx context.Context, input *repository.GroupCreateInput) (*model.Group, error)
	getByIDFunc           func(ctx context.Context, groupID int64) (*model.Group, error)
	getByRepoFullNameFunc func(ctx context.Context, repoFullName string) (*model.Group, error)
	addMemberFunc         func(ctx context.Context, groupID, userID int64, role string) error
	batchAddMembersFunc   func(ctx context.Context, groupID int64, userIDs []int64, role string) error
	isMemberFunc          func(ctx context.Context, groupID, userID int64) (bool, error)
	getMembersFunc        func(ctx context.Context, groupID int64) ([]model.GroupMember, error)
}

func (m *mockGroupRepository) ListByUserID(ctx context.Context, userID int64) ([]model.Group, error) {
	if m.listByUserIDFunc != nil {
		return m.listByUserIDFunc(ctx, userID)
	}
	return []model.Group{}, nil
}

func (m *mockGroupRepository) Create(ctx context.Context, input *repository.GroupCreateInput) (*model.Group, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, input)
	}
	return &model.Group{ID: 1, RepoFullName: input.RepoFullName, Name: input.Name, CreatedAt: time.Now()}, nil
}

func (m *mockGroupRepository) GetByID(ctx context.Context, groupID int64) (*model.Group, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, groupID)
	}
	return nil, errors.New("not found")
}

func (m *mockGroupRepository) GetByRepoFullName(ctx context.Context, repoFullName string) (*model.Group, error) {
	if m.getByRepoFullNameFunc != nil {
		return m.getByRepoFullNameFunc(ctx, repoFullName)
	}
	return nil, errors.New("not found")
}

func (m *mockGroupRepository) AddMember(ctx context.Context, groupID, userID int64, role string) error {
	if m.addMemberFunc != nil {
		return m.addMemberFunc(ctx, groupID, userID, role)
	}
	return nil
}

func (m *mockGroupRepository) BatchAddMembers(ctx context.Context, groupID int64, userIDs []int64, role string) error {
	if m.batchAddMembersFunc != nil {
		return m.batchAddMembersFunc(ctx, groupID, userIDs, role)
	}
	return nil
}

func (m *mockGroupRepository) IsMember(ctx context.Context, groupID, userID int64) (bool, error) {
	if m.isMemberFunc != nil {
		return m.isMemberFunc(ctx, groupID, userID)
	}
	return true, nil
}

func (m *mockGroupRepository) GetMembers(ctx context.Context, groupID int64) ([]model.GroupMember, error) {
	if m.getMembersFunc != nil {
		return m.getMembersFunc(ctx, groupID)
	}
	return []model.GroupMember{}, nil
}

// TestGroupService_CreateGroup_WebhookSuccess は Webhook 登録成功後のみグループが作成されることを確認する
func TestGroupService_CreateGroup_WebhookSuccess(t *testing.T) {
	webhookRegistered := false
	groupCreated := false

	githubClient := &mockGitHubClient{
		registerWebhookFunc: func(ctx context.Context, repoFullName, accessToken, webhookURL, secret string) error {
			webhookRegistered = true
			return nil
		},
	}
	groupRepo := &mockGroupRepository{
		createFunc: func(ctx context.Context, input *repository.GroupCreateInput) (*model.Group, error) {
			groupCreated = true
			return &model.Group{ID: 1, RepoFullName: input.RepoFullName, Name: input.Name}, nil
		},
	}
	userRepo := &mockUserRepository{
		getByGitHubLoginFunc: func(ctx context.Context, login string) (*model.User, error) {
			return nil, errors.New("not found")
		},
	}

	svc := NewGroupService(GroupServiceConfig{
		AppBaseURL:          "https://example.com",
		GitHubWebhookSecret: "webhook_secret",
	}, githubClient, groupRepo, userRepo)

	req := CreateGroupRequest{
		RepoFullName: "owner/repo",
		Name:         "My Team",
		AccessToken:  "github_token",
	}

	group, err := svc.CreateGroup(context.Background(), req, 1)
	if err != nil {
		t.Fatalf("CreateGroup() failed: %v", err)
	}
	if !webhookRegistered {
		t.Error("expected webhook to be registered before group creation")
	}
	if !groupCreated {
		t.Error("expected group to be created after webhook registration")
	}
	if group.RepoFullName != "owner/repo" {
		t.Errorf("expected RepoFullName=owner/repo, got %s", group.RepoFullName)
	}
}

// TestGroupService_CreateGroup_WebhookFailed は Webhook 登録失敗時にグループが作成されないことを確認する
func TestGroupService_CreateGroup_WebhookFailed(t *testing.T) {
	groupCreated := false

	githubClient := &mockGitHubClient{
		registerWebhookFunc: func(ctx context.Context, repoFullName, accessToken, webhookURL, secret string) error {
			return githubpkg.ErrExternalAPI
		},
	}
	groupRepo := &mockGroupRepository{
		createFunc: func(ctx context.Context, input *repository.GroupCreateInput) (*model.Group, error) {
			groupCreated = true
			return nil, nil
		},
	}
	userRepo := &mockUserRepository{}

	svc := NewGroupService(GroupServiceConfig{
		AppBaseURL:          "https://example.com",
		GitHubWebhookSecret: "webhook_secret",
	}, githubClient, groupRepo, userRepo)

	_, err := svc.CreateGroup(context.Background(), CreateGroupRequest{
		RepoFullName: "owner/repo",
		Name:         "My Team",
		AccessToken:  "github_token",
	}, 1)

	if !errors.Is(err, ErrExternalAPI) {
		t.Errorf("expected ErrExternalAPI, got %v", err)
	}
	if groupCreated {
		t.Error("group should not be created when webhook registration fails")
	}
}

func TestGroupService_CreateGroup_ExistingGroupAddsMember(t *testing.T) {
	addedMember := false

	githubClient := &mockGitHubClient{}
	groupRepo := &mockGroupRepository{
		createFunc: func(ctx context.Context, input *repository.GroupCreateInput) (*model.Group, error) {
			return nil, repository.ErrConflict
		},
		getByRepoFullNameFunc: func(ctx context.Context, repoFullName string) (*model.Group, error) {
			if repoFullName != "owner/repo" {
				t.Fatalf("expected repo full name owner/repo, got %s", repoFullName)
			}
			return &model.Group{ID: 42, RepoFullName: repoFullName, Name: "Existing Repo"}, nil
		},
		addMemberFunc: func(ctx context.Context, groupID, userID int64, role string) error {
			if groupID != 42 || userID != 7 || role != "member" {
				t.Fatalf("unexpected AddMember args: groupID=%d userID=%d role=%s", groupID, userID, role)
			}
			addedMember = true
			return nil
		},
	}
	userRepo := &mockUserRepository{}

	svc := NewGroupService(GroupServiceConfig{
		AppBaseURL:          "https://example.com",
		GitHubWebhookSecret: "webhook_secret",
	}, githubClient, groupRepo, userRepo)

	group, err := svc.CreateGroup(context.Background(), CreateGroupRequest{
		RepoFullName: "owner/repo",
		Name:         "My Team",
		AccessToken:  "github_token",
	}, 7)

	if err != nil {
		t.Fatalf("CreateGroup() failed: %v", err)
	}
	if !addedMember {
		t.Error("expected existing group member to be added")
	}
	if group.ID != 42 {
		t.Errorf("expected existing group ID 42, got %d", group.ID)
	}
}
