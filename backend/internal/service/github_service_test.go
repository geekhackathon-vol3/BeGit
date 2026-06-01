package service

import (
	"context"
	"errors"
	"testing"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
	githubpkg "github.com/irj0927/begit/pkg/github"
)

// TestGitHubService_ListRepos_Success はリポジトリ一覧を返すことを確認する
func TestGitHubService_ListRepos_Success(t *testing.T) {
	gh := &mockGitHubClient{
		listUserReposFunc: func(ctx context.Context, accessToken string) ([]githubpkg.Repo, error) {
			return []githubpkg.Repo{{FullName: "alice/repo", CanPush: true}}, nil
		},
	}
	svc := NewGitHubService(gh, &mockGroupRepository{})

	repos, err := svc.ListRepos(context.Background(), "token")
	if err != nil {
		t.Fatalf("ListRepos() failed: %v", err)
	}
	if len(repos) != 1 || repos[0].FullName != "alice/repo" {
		t.Errorf("unexpected repos: %+v", repos)
	}
}

// TestGitHubService_ListGroupCommits_Success はグループのコミット一覧を返すことを確認する
func TestGitHubService_ListGroupCommits_Success(t *testing.T) {
	var capturedRepo string
	gh := &mockGitHubClient{
		listCommitsFunc: func(ctx context.Context, repoFullName, accessToken string, opts githubpkg.CommitListOptions) ([]githubpkg.Commit, error) {
			capturedRepo = repoFullName
			return []githubpkg.Commit{{SHA: "abc"}}, nil
		},
	}
	groupRepo := &mockGroupRepository{
		getByIDFunc: func(ctx context.Context, groupID int64) (*model.Group, error) {
			return &model.Group{ID: groupID, RepoFullName: "alice/repo"}, nil
		},
	}
	svc := NewGitHubService(gh, groupRepo)

	commits, err := svc.ListGroupCommits(context.Background(), 1, "token", githubpkg.CommitListOptions{})
	if err != nil {
		t.Fatalf("ListGroupCommits() failed: %v", err)
	}
	if len(commits) != 1 || commits[0].SHA != "abc" {
		t.Errorf("unexpected commits: %+v", commits)
	}
	if capturedRepo != "alice/repo" {
		t.Errorf("expected repo alice/repo, got %q", capturedRepo)
	}
}

// TestGitHubService_ListGroupCommits_GroupNotFound はグループが存在しない場合に ErrNotFound を返すことを確認する
func TestGitHubService_ListGroupCommits_GroupNotFound(t *testing.T) {
	groupRepo := &mockGroupRepository{
		getByIDFunc: func(ctx context.Context, groupID int64) (*model.Group, error) {
			return nil, repository.ErrNotFound
		},
	}
	svc := NewGitHubService(&mockGitHubClient{}, groupRepo)

	_, err := svc.ListGroupCommits(context.Background(), 1, "token", githubpkg.CommitListOptions{})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
