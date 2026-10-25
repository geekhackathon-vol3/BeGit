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

// TestPostService_GetDraft_Success は本人の draft を取得できることを確認する
func TestPostService_GetDraft_Success(t *testing.T) {
	postRepo := &mockPostRepository{
		getByIDFunc: func(ctx context.Context, postID int64) (*model.Post, error) {
			return &model.Post{ID: postID, GroupID: 12, UserID: 7, IsDraft: true}, nil
		},
	}
	svc := NewPostService(nil, nil, postRepo, nil, nil, nil)
	post, err := svc.GetDraft(context.Background(), 12, 890, 7)
	if err != nil {
		t.Fatalf("GetDraft() failed: %v", err)
	}
	if post.ID != 890 || !post.IsDraft {
		t.Errorf("unexpected draft: %+v", post)
	}
}

// TestPostService_GetDraft_Forbidden は他人の draft 取得で ErrForbidden を返すことを確認する
func TestPostService_GetDraft_Forbidden(t *testing.T) {
	postRepo := &mockPostRepository{
		getByIDFunc: func(ctx context.Context, postID int64) (*model.Post, error) {
			return &model.Post{ID: postID, GroupID: 12, UserID: 99, IsDraft: true}, nil
		},
	}
	svc := NewPostService(nil, nil, postRepo, nil, nil, nil)
	_, err := svc.GetDraft(context.Background(), 12, 890, 7)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestPostService_GetDraft_ConfirmedOwnReport(t *testing.T) {
	repo := &mockPostRepository{getByIDFunc: func(context.Context, int64) (*model.Post, error) {
		return &model.Post{ID: 890, GroupID: 12, UserID: 7}, nil
	}}
	post, err := NewPostService(nil, nil, repo, nil, nil, nil).GetDraft(context.Background(), 12, 890, 7)
	if err != nil || post.ID != 890 {
		t.Fatalf("confirmed report must remain available for photo attachment: %+v %v", post, err)
	}
}

// TestPostService_ConfirmPost_Idempotent は確定でフィード表示可能になり、再確定が no-op であることを確認する
func TestPostService_ConfirmPost_Idempotent(t *testing.T) {
	confirmCalls := 0
	state := true // is_draft
	postRepo := &mockPostRepository{
		getByIDFunc: func(ctx context.Context, postID int64) (*model.Post, error) {
			return &model.Post{ID: postID, GroupID: 12, UserID: 7, IsDraft: state}, nil
		},
		confirmDraftFunc: func(ctx context.Context, postID int64) error {
			confirmCalls++
			state = false
			return nil
		},
	}
	svc := NewPostService(nil, nil, postRepo, nil, nil, nil)

	if _, err := svc.ConfirmPost(context.Background(), ConfirmPostRequest{}, 12, 890, 7); err != nil {
		t.Fatalf("ConfirmPost() #1 failed: %v", err)
	}
	// 2回目は既に確定済み（is_draft=0）なので短絡し、成功を返す（べき等）。
	confirmed, err := svc.ConfirmPost(context.Background(), ConfirmPostRequest{}, 12, 890, 7)
	if err != nil {
		t.Fatalf("ConfirmPost() #2 (idempotent) failed: %v", err)
	}
	// 確定済みへの再確定は no-op（ConfirmDraft を再呼び出ししない＝サービス層で短絡）。
	if confirmCalls != 1 {
		t.Errorf("expected ConfirmDraft called once (service-level short-circuit on already-confirmed), got %d", confirmCalls)
	}
	if confirmed.IsDraft {
		t.Errorf("expected confirmed post to be non-draft, got is_draft=true")
	}
}

func TestPostService_ListPosts_ShowsConfirmedReportsWithoutOwnReport(t *testing.T) {
	id := int64(10)
	body := "作業しました"
	repo := &mockPostRepository{listByGroupIDFunc: func(context.Context, int64) ([]model.Post, error) {
		return []model.Post{
			{ID: 1, NotificationID: &id, UserID: 2, GroupID: 3, Body: &body},
			{ID: 2, NotificationID: &id, UserID: 4, GroupID: 3, IsDraft: true},
			{ID: 3, NotificationID: &id, UserID: 5, GroupID: 4, Body: &body},
		}, nil
	}}
	posts, err := NewPostService(nil, nil, repo, nil, nil, nil).ListPosts(context.Background(), 3, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 || posts[0].Body == nil || *posts[0].Body != body {
		t.Fatalf("only confirmed group reports must be visible: %+v", posts)
	}
}

func TestPostService_CreatePhotoReport_UsesExistingGitHubPost(t *testing.T) {
	id := int64(10)
	existing := &model.Post{ID: 42, NotificationID: &id, UserID: 1, GroupID: 3}
	repo := &mockPostRepository{getByUserAndNotifFunc: func(context.Context, int64, int64) (*model.Post, error) { return existing, nil }}
	post, err := NewPostService(nil, nil, repo, nil, nil, nil).CreatePost(context.Background(), CreatePostRequest{
		NotificationID: &id, PostType: "memo", ContentSource: "photo",
	}, 3, 1)
	if err != nil || post.ID != 42 {
		t.Fatalf("expected existing report, got %+v, %v", post, err)
	}
}

func TestPostService_CreatePhotoReport_RemainsDraftUntilUpload(t *testing.T) {
	id := int64(10)
	repo := &mockPostRepository{
		getByUserAndNotifFunc: func(context.Context, int64, int64) (*model.Post, error) { return nil, repository.ErrNotFound },
		createDraftFunc: func(_ context.Context, post *model.Post) (*model.Post, error) {
			if !post.IsDraft {
				t.Fatal("photo report must start as draft")
			}
			post.ID = 42
			return post, nil
		},
	}
	post, err := NewPostService(nil, nil, repo, nil, nil, nil).CreatePost(context.Background(), CreatePostRequest{
		NotificationID: &id, PostType: "memo", ContentSource: "photo",
	}, 3, 1)
	if err != nil || post.ID != 42 || !post.IsDraft {
		t.Fatalf("expected photo draft, got %+v, %v", post, err)
	}
}

// TestPostService_CreatePost_GitHubAPIFailed は GitHub API 失敗時に ErrExternalAPI を返すことを確認する
func TestPostService_CreatePost_GitHubAPIFailed(t *testing.T) {
	githubClientFail := &mockGitHubClient{
		getRecentCommitsFunc: func(ctx context.Context, repoFullName, login, accessToken string) (*githubpkg.CommitSummary, error) {
			return nil, ErrExternalAPI
		},
	}

	sprintRepo := &mockSprintRepository{}
	postRepo := &mockPostRepository{}
	groupRepo := &mockGroupRepository{}

	svc := NewPostService(githubClientFail, sprintRepo, postRepo, groupRepo, nil, nil)

	_, err := svc.CreatePost(context.Background(), CreatePostRequest{
		Body:         nil,
		AccessToken:  "valid_token",
		GitHubLogin:  "testuser",
		RepoFullName: "owner/repo",
	}, 1, 1)

	if !errors.Is(err, ErrExternalAPI) {
		t.Errorf("expected ErrExternalAPI when GitHub API fails, got %v", err)
	}
}

func TestPostService_CreatePost_PullRequest(t *testing.T) {
	var created *model.Post
	githubClient := &mockGitHubClient{
		getLatestPullRequestFunc: func(ctx context.Context, repoFullName, login, accessToken string) (*githubpkg.PullRequestSummary, error) {
			return &githubpkg.PullRequestSummary{Number: 42, Title: "Add activity selector", RepoFullName: repoFullName}, nil
		},
	}
	postRepo := &mockPostRepository{createFunc: func(ctx context.Context, post *model.Post) (*model.Post, error) {
		created = post
		post.ID = 1
		return post, nil
	}}
	svc := NewPostService(githubClient, nil, postRepo, nil, nil, nil)

	_, err := svc.CreatePost(context.Background(), CreatePostRequest{
		PostType:     "pull_request",
		AccessToken:  "valid_token",
		GitHubLogin:  "testuser",
		RepoFullName: "owner/repo",
	}, 1, 1)
	if err != nil {
		t.Fatalf("CreatePost() failed: %v", err)
	}
	if created.PostType != "pull_request" || created.LatestCommitMessage == nil || *created.LatestCommitMessage != "PR #42: Add activity selector" {
		t.Errorf("unexpected post: %+v", created)
	}
}

// 同じ BeGit Time! への2回目の報告（UNIQUE(notification_id, user_id) 違反）は 409 にする（今までは 500）
func TestPostService_CreatePost_ConstraintViolationIsConflict(t *testing.T) {
	body := "2回目"
	postRepo := &mockPostRepository{createFunc: func(ctx context.Context, post *model.Post) (*model.Post, error) {
		return nil, repository.ErrConstraintViolation
	}}
	svc := NewPostService(nil, nil, postRepo, nil, nil, nil)

	_, err := svc.CreatePost(context.Background(), CreatePostRequest{
		Body:          &body,
		PostType:      "memo",
		ContentSource: "manual",
	}, 1, 1)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("CreatePost() error = %v, want ErrConflict", err)
	}
}

func TestPostService_CreatePost_MemoSkipsGitHub(t *testing.T) {
	body := "今日は設計を整理します"
	var created *model.Post
	postRepo := &mockPostRepository{createFunc: func(ctx context.Context, post *model.Post) (*model.Post, error) {
		created = post
		post.ID = 1
		return post, nil
	}}
	svc := NewPostService(nil, nil, postRepo, nil, nil, nil)

	_, err := svc.CreatePost(context.Background(), CreatePostRequest{
		Body:         &body,
		PostType:     "memo",
		RepoFullName: "owner/repo",
	}, 1, 1)
	if err != nil {
		t.Fatalf("CreatePost() failed: %v", err)
	}
	if created.PostType != "memo" || created.Body == nil || *created.Body != body {
		t.Errorf("unexpected post: %+v", created)
	}
}

func TestPostService_CreatePost_RejectsInvalidTypeAndEmptyMemo(t *testing.T) {
	svc := NewPostService(nil, nil, &mockPostRepository{}, nil, nil, nil)
	for _, req := range []CreatePostRequest{
		{PostType: "issue", RepoFullName: "owner/repo"},
		{PostType: "memo", RepoFullName: "owner/repo"},
	} {
		if _, err := svc.CreatePost(context.Background(), req, 1, 1); !errors.Is(err, ErrValidation) {
			t.Errorf("expected ErrValidation for %+v, got %v", req, err)
		}
	}
}

func TestPostService_CreatePost_SelectedCommit(t *testing.T) {
	sha := "abc123"
	var created *model.Post
	githubClient := &mockGitHubClient{
		getCommitFunc: func(ctx context.Context, repoFullName, gotSHA, accessToken string) (*githubpkg.Commit, error) {
			if gotSHA != sha {
				t.Fatalf("expected SHA %q, got %q", sha, gotSHA)
			}
			return &githubpkg.Commit{
				SHA: sha, Message: "feat: selected commit", AuthorLogin: "TestUser", Additions: 12, Deletions: 3,
			}, nil
		},
	}
	postRepo := &mockPostRepository{createFunc: func(ctx context.Context, post *model.Post) (*model.Post, error) {
		created = post
		post.ID = 1
		return post, nil
	}}
	svc := NewPostService(githubClient, nil, postRepo, nil, nil, nil)

	_, err := svc.CreatePost(context.Background(), CreatePostRequest{
		PostType: "commit", ContentSource: "github", CommitSHA: &sha,
		AccessToken: "token", GitHubLogin: "testuser", RepoFullName: "owner/repo",
	}, 1, 1)
	if err != nil {
		t.Fatalf("CreatePost() failed: %v", err)
	}
	if created.LatestCommitMessage == nil || *created.LatestCommitMessage != "feat: selected commit" || created.CommitCount != 1 {
		t.Fatalf("unexpected post: %+v", created)
	}
}

func TestPostService_CreatePost_SelectedPullRequest(t *testing.T) {
	number := 87
	var created *model.Post
	githubClient := &mockGitHubClient{
		getPullRequestFunc: func(ctx context.Context, repoFullName string, gotNumber int, accessToken string) (*githubpkg.PullRequest, error) {
			return &githubpkg.PullRequest{Number: gotNumber, Title: "Add GitHub picker", AuthorLogin: "testuser"}, nil
		},
	}
	postRepo := &mockPostRepository{createFunc: func(ctx context.Context, post *model.Post) (*model.Post, error) {
		created = post
		post.ID = 1
		return post, nil
	}}
	svc := NewPostService(githubClient, nil, postRepo, nil, nil, nil)

	_, err := svc.CreatePost(context.Background(), CreatePostRequest{
		PostType: "pull_request", ContentSource: "github", PullRequestNumber: &number,
		AccessToken: "token", GitHubLogin: "testuser", RepoFullName: "owner/repo",
	}, 1, 1)
	if err != nil {
		t.Fatalf("CreatePost() failed: %v", err)
	}
	if created.LatestCommitMessage == nil || *created.LatestCommitMessage != "PR #87: Add GitHub picker" {
		t.Fatalf("unexpected post: %+v", created)
	}
}

func TestPostService_CreatePost_ManualSkipsGitHub(t *testing.T) {
	body := "今日はここまで進めました"
	var created *model.Post
	postRepo := &mockPostRepository{createFunc: func(ctx context.Context, post *model.Post) (*model.Post, error) {
		created = post
		post.ID = 1
		return post, nil
	}}
	svc := NewPostService(nil, nil, postRepo, nil, nil, nil)

	_, err := svc.CreatePost(context.Background(), CreatePostRequest{
		Body: &body, PostType: "commit", ContentSource: "manual", RepoFullName: "owner/repo",
	}, 1, 1)
	if err != nil {
		t.Fatalf("CreatePost() failed: %v", err)
	}
	if created.Body == nil || *created.Body != body || created.LatestCommitMessage != nil {
		t.Fatalf("unexpected post: %+v", created)
	}
}

func TestPostService_ConfirmPost_RejectsPhotoDraftWithoutUpload(t *testing.T) {
	id := int64(9)
	repo := &mockPostRepository{getByIDFunc: func(context.Context, int64) (*model.Post, error) {
		return &model.Post{ID: 42, GroupID: 3, UserID: 1, NotificationID: &id, PostType: "memo", IsDraft: true}, nil
	}}
	_, err := NewPostService(nil, nil, repo, nil, nil, nil).ConfirmPost(context.Background(), ConfirmPostRequest{}, 3, 42, 1)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("photo draft confirmed without upload: %v", err)
	}
}

func TestPostService_PhotoReportAcceptedLateWithinSprint(t *testing.T) {
	id := int64(9)
	notif := &mockNotificationRepository{getByIDFunc: func(context.Context, int64) (*model.Notification, error) {
		return &model.Notification{ID: id, SprintID: 7, SentAt: time.Now().Add(-2 * time.Hour)}, nil
	}}
	sprint := &mockSprintRepository{getByIDFunc: func(context.Context, int64) (*model.Sprint, error) {
		return &model.Sprint{ID: 7, GroupID: 3, EndsAt: time.Now().Add(time.Hour)}, nil
	}}
	repo := &mockPostRepository{createDraftFunc: func(_ context.Context, post *model.Post) (*model.Post, error) {
		post.ID = 42
		return post, nil
	}}
	post, err := NewPostService(nil, sprint, repo, nil, nil, nil, notif).CreatePost(context.Background(), CreatePostRequest{
		NotificationID: &id, PostType: "memo", ContentSource: "photo",
	}, 3, 1)
	if err != nil || !post.IsDraft {
		t.Fatalf("late photo report must be uploadable: %+v %v", post, err)
	}
}
