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

// ListGroups はユーザーが所属するグループ一覧を返す
func (s *groupService) ListGroups(ctx context.Context, userID int64) ([]model.Group, error) {
	groups, err := s.groupRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("group_service: ListGroups failed: %w", err)
	}
	return groups, nil
}

// CreateGroup は Webhook 先行登録 → グループ作成 → オーナー追加 → コラボレーター自動追加の順に処理する
func (s *groupService) CreateGroup(ctx context.Context, req CreateGroupRequest, userID int64) (*model.Group, error) {
	// Step 1: リポジトリ情報（avatar_url）を取得
	repoInfo, err := s.githubClient.GetRepoInfo(ctx, req.RepoFullName, req.AccessToken)
	if err != nil {
		if errors.Is(err, githubpkg.ErrUnauthorized) {
			return nil, ErrUnauthorized
		}
		return nil, fmt.Errorf("%w: get repo info failed: %v", ErrExternalAPI, err)
	}

	// Step 2: GitHub Webhook を先行登録（失敗したら D1 INSERT しない）
	webhookURL := s.config.AppBaseURL + "/webhook/github"
	if err := s.githubClient.RegisterWebhook(ctx, req.RepoFullName, req.AccessToken, webhookURL, s.config.GitHubWebhookSecret); err != nil {
		return nil, fmt.Errorf("%w: webhook registration failed: %v", ErrExternalAPI, err)
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
