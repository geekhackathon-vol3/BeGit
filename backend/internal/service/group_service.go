package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
	githubpkg "github.com/irj0927/begit/pkg/github"
)

// CreateGroupRequest はグループ作成リクエスト
type CreateGroupRequest struct {
	RepoFullName   string
	Name           string
	InstallationID int64
	ReadOnly       bool
	// AccessToken は旧OAuth方式との後方互換用。新規クライアントはInstallationIDを指定する。
	AccessToken string
}

// GroupDetail はグループ詳細（メンバー一覧付き）
type GroupDetail struct {
	model.Group
	Members []model.GroupMember
	MyRole  string
}

// GroupServiceConfig は GroupService の設定
type GroupServiceConfig struct {
	AppBaseURL          string
	GitHubWebhookSecret string
	GitHubAppID         string
	GitHubAppPrivateKey string
}

// GroupService はグループ管理サービスインターフェース
type GroupService interface {
	ListGroups(ctx context.Context, userID int64) ([]GroupDetail, error)
	CreateGroup(ctx context.Context, req CreateGroupRequest, userID int64) (*model.Group, error)
	LeaveGroup(ctx context.Context, groupID, userID int64) error
	GetGroup(ctx context.Context, groupID, userID int64) (*GroupDetail, error)
	SyncMembers(ctx context.Context, groupID int64, accessToken string) ([]model.GroupMember, error)
	UpdateMemberRole(ctx context.Context, groupID, requesterUserID, targetUserID int64, role string) error
}

// groupService は GroupService インターフェースの実装
type groupService struct {
	config                  GroupServiceConfig
	githubClient            githubpkg.Client
	installationTokenClient githubpkg.AppInstallationTokenClient
	groupRepo               repository.GroupRepository
	userRepo                repository.UserRepository
}

// NewGroupService は GroupService を作成する
func NewGroupService(
	config GroupServiceConfig,
	githubClient githubpkg.Client,
	groupRepo repository.GroupRepository,
	userRepo repository.UserRepository,
) GroupService {
	return newGroupService(config, githubClient, nil, groupRepo, userRepo)
}

// NewGroupServiceWithAppToken はGitHub AppのInstallation Tokenを使うグループサービスを作成する。
func NewGroupServiceWithAppToken(
	config GroupServiceConfig,
	githubClient githubpkg.Client,
	installationTokenClient githubpkg.AppInstallationTokenClient,
	groupRepo repository.GroupRepository,
	userRepo repository.UserRepository,
) GroupService {
	return newGroupService(config, githubClient, installationTokenClient, groupRepo, userRepo)
}

func newGroupService(
	config GroupServiceConfig,
	githubClient githubpkg.Client,
	installationTokenClient githubpkg.AppInstallationTokenClient,
	groupRepo repository.GroupRepository,
	userRepo repository.UserRepository,
) GroupService {
	return &groupService{
		config:                  config,
		githubClient:            githubClient,
		installationTokenClient: installationTokenClient,
		groupRepo:               groupRepo,
		userRepo:                userRepo,
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
func (s *groupService) ListGroups(ctx context.Context, userID int64) ([]GroupDetail, error) {
	groups, err := s.groupRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("group_service: ListGroups failed: %w", err)
	}

	details := make([]GroupDetail, 0, len(groups))
	for _, group := range groups {
		members, err := s.groupRepo.GetMembers(ctx, group.ID)
		if err != nil {
			return nil, fmt.Errorf("group_service: ListGroups GetMembers failed: %w", err)
		}
		details = append(details, GroupDetail{
			Group:   group,
			Members: members,
			MyRole:  roleForUser(members, userID),
		})
	}

	return details, nil
}

// CreateGroup はリポジトリ情報取得 → Webhook 登録 → グループ作成 → オーナー追加 → コラボレーター自動追加の順に処理する。
// Webhook 登録を先に行うことで、登録失敗時にグループが作成されないことを保証する。
func (s *groupService) CreateGroup(ctx context.Context, req CreateGroupRequest, userID int64) (*model.Group, error) {
	repoFullName, err := normalizeRepoFullName(req.RepoFullName)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid repository", ErrValidation)
	}
	req.RepoFullName = repoFullName
	if req.ReadOnly && req.InstallationID > 0 {
		return nil, fmt.Errorf("%w: read-only repositories cannot use a GitHub App installation", ErrValidation)
	}

	accessToken, err := s.resolveGitHubAccessToken(ctx, req)
	if err != nil {
		return nil, err
	}

	// Step 1: リポジトリ情報（avatar_url）を取得
	repoInfo, err := s.githubClient.GetRepoInfo(ctx, req.RepoFullName, accessToken)
	if err != nil {
		if errors.Is(err, githubpkg.ErrUnauthorized) {
			return nil, ErrUnauthorized
		}
		if errors.Is(err, githubpkg.ErrForbidden) || errors.Is(err, githubpkg.ErrNotFound) {
			return nil, ErrForbidden
		}
		return nil, fmt.Errorf("%w: get repo info failed: %v", ErrExternalAPI, err)
	}
	if req.ReadOnly && repoInfo.Private {
		return nil, fmt.Errorf("%w: read-only mode is available only for public repositories", ErrForbidden)
	}
	// GitHub App未接続かつpush権限のない公開リポジトリは、クライアント指定に
	// 関係なく公開閲覧モードに固定する。read_only=falseの自己申告で昇格させない。
	effectiveReadOnly := req.ReadOnly || (req.InstallationID <= 0 && !repoInfo.CanPush)
	if effectiveReadOnly && repoInfo.Private {
		return nil, fmt.Errorf("%w: read-only mode is available only for public repositories", ErrForbidden)
	}
	req.RepoFullName = repoInfo.FullName
	if strings.TrimSpace(req.Name) == "" || strings.EqualFold(strings.TrimSpace(req.Name), repoFullName) {
		req.Name = repoInfo.FullName
	}

	if !effectiveReadOnly {
		// Step 2: GitHub Webhook を登録（失敗時はグループを作成しない）
		// "hook already exists" と 403 Forbidden（admin 権限不足）は非致命的として扱う
		webhookURL := s.config.AppBaseURL + "/webhook/github"
		if err := s.githubClient.RegisterWebhook(ctx, req.RepoFullName, accessToken, webhookURL, s.config.GitHubWebhookSecret); err != nil {
			if !isHookAlreadyExistsError(err) && !errors.Is(err, githubpkg.ErrForbidden) {
				return nil, fmt.Errorf("%w: webhook registration failed: %v", ErrExternalAPI, err)
			}
		}
	}

	// Step 3: グループを D1 に作成
	group, err := s.groupRepo.Create(ctx, &repository.GroupCreateInput{
		RepoFullName: req.RepoFullName,
		Name:         req.Name,
		AvatarURL:    repoInfo.AvatarURL,
		ReadOnly:     effectiveReadOnly,
		OwnerUserID:  userID,
	})
	if err != nil {
		if errors.Is(err, repository.ErrConflict) {
			existingGroup, getErr := s.groupRepo.GetByRepoFullName(ctx, req.RepoFullName)
			if getErr != nil {
				return nil, fmt.Errorf("group_service: GetByRepoFullName after conflict failed: %w", getErr)
			}
			joinRole := model.GroupRoleViewer
			if req.InstallationID > 0 || repoInfo.CanPush {
				joinRole = model.GroupRolePoster
			}
			if addErr := s.groupRepo.AddMember(ctx, existingGroup.ID, userID, joinRole); addErr != nil {
				return nil, fmt.Errorf("group_service: AddMember after conflict failed: %w", addErr)
			}
			return existingGroup, nil
		}
		return nil, fmt.Errorf("group_service: CreateGroup failed: %w", err)
	}

	// Step 4: 作成者を owner ロールで group_members に追加
	if err := s.groupRepo.AddMember(ctx, group.ID, userID, model.GroupRoleOwner); err != nil {
		return nil, fmt.Errorf("group_service: AddMember (owner) failed: %w", err)
	}

	// Step 5: GitHub コラボレーターを取得して BeGit 登録済みユーザーを自動追加。
	// 表示専用リポジトリはコラボレーター情報も権限対象のため取得しない。
	if effectiveReadOnly {
		return group, nil
	}
	collaborators, err := s.githubClient.GetCollaborators(ctx, req.RepoFullName, accessToken)
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
			_ = s.groupRepo.BatchAddMembers(ctx, group.ID, memberIDs, model.GroupRolePoster) // エラーは無視（ベストエフォート）
		}
	}

	return group, nil
}

func normalizeRepoFullName(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if strings.HasPrefix(trimmed, "https://") || strings.HasPrefix(trimmed, "http://") {
		parsed, err := url.Parse(trimmed)
		if err != nil || (!strings.EqualFold(parsed.Hostname(), "github.com") && !strings.EqualFold(parsed.Hostname(), "www.github.com")) {
			return "", ErrValidation
		}
		trimmed = strings.Trim(parsed.Path, "/")
	}
	trimmed = strings.TrimSuffix(strings.TrimSuffix(trimmed, "/"), ".git")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.ContainsAny(trimmed, "?# \t\n") {
		return "", ErrValidation
	}
	return parts[0] + "/" + parts[1], nil
}

func roleForUser(members []model.GroupMember, userID int64) string {
	for _, member := range members {
		if member.UserID == userID {
			return model.NormalizeGroupRole(member.Role)
		}
	}
	return ""
}

// LeaveGroup はログイン中ユーザーをグループから外し、HOMEの一覧から永続的に削除する。
// GitHub上のリポジトリや、他ユーザーのグループ参加状態は変更しない。
func (s *groupService) LeaveGroup(ctx context.Context, groupID, userID int64) error {
	if err := s.groupRepo.RemoveMember(ctx, groupID, userID); err != nil {
		return fmt.Errorf("group_service: LeaveGroup failed: %w", err)
	}
	return nil
}

// resolveGitHubAccessToken はInstallation IDが指定された場合にApp方式へ切り替える。
// 旧OAuthクライアントとの段階移行中はAccessTokenへのフォールバックを許可する。
func (s *groupService) resolveGitHubAccessToken(ctx context.Context, req CreateGroupRequest) (string, error) {
	if req.InstallationID <= 0 {
		if req.AccessToken == "" {
			return "", ErrUnauthorized
		}
		return req.AccessToken, nil
	}
	if s.installationTokenClient == nil || s.config.GitHubAppID == "" || s.config.GitHubAppPrivateKey == "" {
		return "", fmt.Errorf("%w: GitHub App token client is not configured", ErrExternalAPI)
	}

	token, err := s.installationTokenClient.CreateInstallationAccessToken(
		ctx,
		s.config.GitHubAppID,
		s.config.GitHubAppPrivateKey,
		req.InstallationID,
	)
	if err != nil {
		if errors.Is(err, githubpkg.ErrUnauthorized) {
			return "", ErrUnauthorized
		}
		if errors.Is(err, githubpkg.ErrForbidden) || errors.Is(err, githubpkg.ErrNotFound) {
			return "", ErrForbidden
		}
		return "", fmt.Errorf("%w: failed to create installation token: %v", ErrExternalAPI, err)
	}
	if token == "" {
		return "", fmt.Errorf("%w: empty installation token", ErrExternalAPI)
	}
	return token, nil
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
	if group.ReadOnly {
		members, membersErr := s.groupRepo.GetMembers(ctx, groupID)
		if membersErr != nil {
			return nil, fmt.Errorf("group_service: SyncMembers GetMembers failed: %w", membersErr)
		}
		return members, nil
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
		if err := s.groupRepo.BatchAddMembers(ctx, groupID, memberIDs, model.GroupRolePoster); err != nil {
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
		MyRole:  roleForUser(members, userID),
	}, nil
}

// UpdateMemberRole はownerがメンバーを表示専用または投稿可能へ変更する。
func (s *groupService) UpdateMemberRole(ctx context.Context, groupID, requesterUserID, targetUserID int64, role string) error {
	requester, err := s.groupRepo.GetMember(ctx, groupID, requesterUserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrForbidden
		}
		return fmt.Errorf("group_service: UpdateMemberRole requester lookup failed: %w", err)
	}
	if model.NormalizeGroupRole(requester.Role) != model.GroupRoleOwner {
		return ErrForbidden
	}
	if role != model.GroupRoleViewer && role != model.GroupRolePoster {
		return fmt.Errorf("%w: role must be viewer or poster", ErrValidation)
	}
	target, err := s.groupRepo.GetMember(ctx, groupID, targetUserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("group_service: UpdateMemberRole target lookup failed: %w", err)
	}
	if model.NormalizeGroupRole(target.Role) == model.GroupRoleOwner {
		return fmt.Errorf("%w: owner role cannot be changed", ErrValidation)
	}
	if err := s.groupRepo.UpdateMemberRole(ctx, groupID, targetUserID, role); err != nil {
		return fmt.Errorf("group_service: UpdateMemberRole failed: %w", err)
	}
	return nil
}
