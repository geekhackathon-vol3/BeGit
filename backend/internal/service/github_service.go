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
	// ListGroupCommits はグループに紐づくリポジトリのコミット一覧を返す。
	ListGroupCommits(ctx context.Context, groupID int64, accessToken string, opts githubpkg.CommitListOptions) ([]githubpkg.Commit, error)
}

// gitHubService は GitHubService インターフェースの実装
type gitHubService struct {
	githubClient githubpkg.Client
	groupRepo    repository.GroupRepository
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
