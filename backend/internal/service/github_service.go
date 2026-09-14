package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/irj0927/begit/internal/repository"
	githubpkg "github.com/irj0927/begit/pkg/github"
)

// GitHubService は GitHub REST API をプロキシするサービスインターフェース
type GitHubService interface {
	// ListRepos は認証ユーザーがアクセスできる（push/admin 権限のある）リポジトリ一覧を返す。
	ListRepos(ctx context.Context, accessToken string) ([]githubpkg.Repo, error)
	// ListInstallationRepos はGitHub App Installationに許可された一覧を返す。
	ListInstallationRepos(ctx context.Context, installationID int64) ([]githubpkg.Repo, error)
	// ListGroupCommits はグループに紐づくリポジトリのコミット一覧を返す。
	ListGroupCommits(ctx context.Context, groupID int64, accessToken string, opts githubpkg.CommitListOptions) ([]githubpkg.Commit, error)
}

// gitHubService は GitHubService インターフェースの実装
type gitHubService struct {
	config                  GitHubServiceConfig
	githubClient            githubpkg.Client
	installationTokenClient githubpkg.AppInstallationTokenClient
	groupRepo               repository.GroupRepository
}

// GitHubServiceConfig はGitHub App方式の一覧取得設定。
type GitHubServiceConfig struct {
	GitHubAppID         string
	GitHubAppPrivateKey string
}

// NewGitHubService は GitHubService を作成する
func NewGitHubService(
	githubClient githubpkg.Client,
	groupRepo repository.GroupRepository,
) GitHubService {
	return &gitHubService{
		githubClient: githubClient,
		groupRepo:    groupRepo,
	}
}

// NewGitHubServiceWithAppToken はOAuth一覧に加えてApp Installation一覧を有効化する。
func NewGitHubServiceWithAppToken(
	config GitHubServiceConfig,
	githubClient githubpkg.Client,
	installationTokenClient githubpkg.AppInstallationTokenClient,
	groupRepo repository.GroupRepository,
) GitHubService {
	return &gitHubService{
		config:                  config,
		githubClient:            githubClient,
		installationTokenClient: installationTokenClient,
		groupRepo:               groupRepo,
	}
}

// ListRepos は認証ユーザーのリポジトリ一覧を返す。
func (s *gitHubService) ListRepos(ctx context.Context, accessToken string) ([]githubpkg.Repo, error) {
	if s.githubClient == nil {
		return nil, fmt.Errorf("%w: github client not configured", ErrExternalAPI)
	}

	repos, err := s.githubClient.ListUserRepos(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to list repos: %v", ErrExternalAPI, err)
	}
	return repos, nil
}

func (s *gitHubService) ListInstallationRepos(ctx context.Context, installationID int64) ([]githubpkg.Repo, error) {
	if installationID <= 0 || s.installationTokenClient == nil || s.config.GitHubAppID == "" || s.config.GitHubAppPrivateKey == "" {
		return nil, fmt.Errorf("%w: GitHub App installation is not configured", ErrExternalAPI)
	}
	repoClient, ok := s.githubClient.(githubpkg.InstallationRepositoriesClient)
	if !ok {
		return nil, fmt.Errorf("%w: GitHub client does not support installation repositories", ErrExternalAPI)
	}

	token, err := s.installationTokenClient.CreateInstallationAccessToken(
		ctx,
		s.config.GitHubAppID,
		s.config.GitHubAppPrivateKey,
		installationID,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create installation token: %v", ErrExternalAPI, err)
	}
	repos, err := repoClient.ListInstallationRepos(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to list installation repos: %v", ErrExternalAPI, err)
	}
	return repos, nil
}

// ListGroupCommits はグループの repo_full_name を解決し、コミット一覧を返す。
func (s *gitHubService) ListGroupCommits(ctx context.Context, groupID int64, accessToken string, opts githubpkg.CommitListOptions) ([]githubpkg.Commit, error) {
	if s.githubClient == nil {
		return nil, fmt.Errorf("%w: github client not configured", ErrExternalAPI)
	}

	group, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("github_service: ListGroupCommits failed: %w", err)
	}

	commits, err := s.githubClient.ListCommits(ctx, group.RepoFullName, accessToken, opts)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to list commits: %v", ErrExternalAPI, err)
	}
	return commits, nil
}
