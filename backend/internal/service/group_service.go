package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
	githubpkg "github.com/irj0927/begit/pkg/github"
)

// CreateGroupRequest はグループ作成リクエスト
type CreateGroupRequest struct {
	RepoFullName string
	Name         string
	AccessToken  string
}

// GroupDetail はグループ詳細（メンバー一覧付き）
type GroupDetail struct {
	model.Group
	Members []model.GroupMember
}

// GroupServiceConfig は GroupService の設定
type GroupServiceConfig struct {
	AppBaseURL          string
	GitHubWebhookSecret string
}

// GroupService はグループ管理サービスインターフェース
type GroupService interface {
	ListGroups(ctx context.Context, userID int64) ([]model.Group, error)
	CreateGroup(ctx context.Context, req CreateGroupRequest, userID int64) (*model.Group, error)
	GetGroup(ctx context.Context, groupID, userID int64) (*GroupDetail, error)
	SyncMembers(ctx context.Context, groupID int64, accessToken string) ([]model.GroupMember, error)
}

// groupService は GroupService インターフェースの実装
type groupService struct {
	config       GroupServiceConfig
	githubClient githubpkg.Client
	groupRepo    repository.GroupRepository
	userRepo     repository.UserRepository
}

// NewGroupService は GroupService を作成する
func NewGroupService(
	config GroupServiceConfig,
	githubClient githubpkg.Client,
	groupRepo repository.GroupRepository,
	userRepo repository.UserRepository,
) GroupService {
	return &groupService{
		config:       config,
		githubClient: githubClient,
		groupRepo:    groupRepo,
		userRepo:     userRepo,
	}
}

// isHookAlreadyExistsError は Webhook が既に存在するエラーかどうかを判定する
func isHookAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return contains(errMsg, "hook already exists") || contains(errMsg, "Hook already exists")
}

// contains は文字列に部分文字列が含まれるかチェック
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexString(s, substr) >= 0)
}

// indexString は部分文字列の位置を返す
func indexString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// ListGroups はユーザーが所属するグループ一覧を返す
func (s *groupService) ListGroups(ctx context.Context, userID int64) ([]model.Group, error) {
	groups, err := s.groupRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("group_service: ListGroups failed: %w", err)
	}
	return groups, nil
}

// CreateGroup はリポジトリ情報取得 → Webhook 登録 → グループ作成 → オーナー追加 → コラボレーター自動追加の順に処理する。
// Webhook 登録を先に行うことで、登録失敗時にグループが作成されないことを保証する。
func (s *groupService) CreateGroup(ctx context.Context, req CreateGroupRequest, userID int64) (*model.Group, error) {
	// Step 1: リポジトリ情報（avatar_url）を取得
	repoInfo, err := s.githubClient.GetRepoInfo(ctx, req.RepoFullName, req.AccessToken)
	if err != nil {
		if errors.Is(err, githubpkg.ErrUnauthorized) {
			return nil, ErrUnauthorized
		}
		return nil, fmt.Errorf("%w: get repo info failed: %v", ErrExternalAPI, err)
	}

	// Step 2: GitHub Webhook を登録（失敗時はグループを作成しない）
	// "hook already exists" エラーは非致命的として扱う
	webhookURL := s.config.AppBaseURL + "/webhook/github"
	if err := s.githubClient.RegisterWebhook(ctx, req.RepoFullName, req.AccessToken, webhookURL, s.config.GitHubWebhookSecret); err != nil {
		if !isHookAlreadyExistsError(err) {
			return nil, fmt.Errorf("%w: webhook registration failed: %v", ErrExternalAPI, err)
		}
	}

	// Step 3: グループを D1 に作成
	group, err := s.groupRepo.Create(ctx, &repository.GroupCreateInput{
		RepoFullName: req.RepoFullName,
		Name:         req.Name,
		AvatarURL:    repoInfo.AvatarURL,
		OwnerUserID:  userID,
	})
	if err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return nil, ErrConflict
		}
		return nil, fmt.Errorf("group_service: CreateGroup failed: %w", err)
	}

	// Step 4: 作成者を owner ロールで group_members に追加
	if err := s.groupRepo.AddMember(ctx, group.ID, userID, "owner"); err != nil {
		return nil, fmt.Errorf("group_service: AddMember (owner) failed: %w", err)
	}

	// Step 5: GitHub コラボレーターを取得して BeGit 登録済みユーザーを自動追加
	collaborators, err := s.githubClient.GetCollaborators(ctx, req.RepoFullName, req.AccessToken)
	if err == nil {
		var memberIDs []int64
		for _, collab := range collaborators {
			if collab.Login == "" {
				continue
			}
			u, err := s.userRepo.GetByGitHubLogin(ctx, collab.Login)
			if err != nil {
				continue // BeGit 未登録ユーザーはスキップ
			}
			if u.ID != userID { // オーナー自身は除外
				memberIDs = append(memberIDs, u.ID)
			}
		}
		if len(memberIDs) > 0 {
			_ = s.groupRepo.BatchAddMembers(ctx, group.ID, memberIDs, "member") // エラーは無視（ベストエフォート）
		}
	}

	return group, nil
}

// SyncMembers は GitHub コラボレーターを取得し、BeGit 登録済みユーザーを
// group_members に追加（加算的 upsert）して、更新後のメンバー一覧を返す。
//
// 方針: 加算のみ。GitHub から外れたコラボレーターは自動削除しない
// （オーナーの誤削除や履歴の喪失を避けるため）。既存メンバーの role は
// AddMember の INSERT OR IGNORE により維持される（owner が member に降格しない）。
func (s *groupService) SyncMembers(ctx context.Context, groupID int64, accessToken string) ([]model.GroupMember, error) {
	group, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("group_service: SyncMembers GetByID failed: %w", err)
	}

	collaborators, err := s.githubClient.GetCollaborators(ctx, group.RepoFullName, accessToken)
	if err != nil {
		if errors.Is(err, githubpkg.ErrUnauthorized) {
			return nil, ErrUnauthorized
		}
		return nil, fmt.Errorf("%w: get collaborators failed: %v", ErrExternalAPI, err)
	}

	var memberIDs []int64
	for _, collab := range collaborators {
		if collab.Login == "" {
			continue
		}
		u, err := s.userRepo.GetByGitHubLogin(ctx, collab.Login)
		if err != nil {
			continue // BeGit 未登録ユーザーはスキップ
		}
		memberIDs = append(memberIDs, u.ID)
	}
	if len(memberIDs) > 0 {
		if err := s.groupRepo.BatchAddMembers(ctx, groupID, memberIDs, "member"); err != nil {
			return nil, fmt.Errorf("group_service: SyncMembers BatchAddMembers failed: %w", err)
		}
	}

	members, err := s.groupRepo.GetMembers(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("group_service: SyncMembers GetMembers failed: %w", err)
	}
	return members, nil
}

// GetGroup はグループ詳細とメンバー一覧を返す
func (s *groupService) GetGroup(ctx context.Context, groupID, userID int64) (*GroupDetail, error) {
	group, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("group_service: GetGroup failed: %w", err)
	}

	// メンバーシップ確認
	isMember, err := s.groupRepo.IsMember(ctx, groupID, userID)
	if err != nil {
		return nil, fmt.Errorf("group_service: IsMember check failed: %w", err)
	}
	if !isMember {
		return nil, ErrForbidden
	}

	members, err := s.groupRepo.GetMembers(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("group_service: GetMembers failed: %w", err)
	}

	return &GroupDetail{
		Group:   *group,
		Members: members,
	}, nil
}
