package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/irj0927/begit/internal/model"
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

// TestPostService_GetDraft_NotDraft は確定済み投稿の draft 取得で ErrNotFound を返すことを確認する
func TestPostService_GetDraft_NotDraft(t *testing.T) {
	postRepo := &mockPostRepository{
		getByIDFunc: func(ctx context.Context, postID int64) (*model.Post, error) {
			return &model.Post{ID: postID, GroupID: 12, UserID: 7, IsDraft: false}, nil
		},
	}
	svc := NewPostService(nil, nil, postRepo, nil, nil, nil)
	_, err := svc.GetDraft(context.Background(), 12, 890, 7)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for non-draft, got %v", err)
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

// TestPostService_ListPosts_Blurred は同じ通知回へ未投稿の場合に他メンバーの sensitive フィールドが nil で返ることを確認する
func TestPostService_ListPosts_Blurred(t *testing.T) {
	requestUserID := int64(1)
	otherUserID := int64(2)
	notificationID := int64(10)

	body := "コードを書きました"
	repo := "owner/repo"
	msg := "Fix bug"

	postRepo := &mockPostRepository{
		listByGroupIDFunc: func(ctx context.Context, groupID int64) ([]model.Post, error) {
			return []model.Post{
				{
					ID:                  1,
					NotificationID:      &notificationID,
					UserID:              otherUserID,
					GroupID:             1,
					PostType:            "commit",
					Body:                &body,
					RepoFullName:        &repo,
					LatestCommitMessage: &msg,
					CommitCount:         3,
					CreatedAt:           time.Now(),
				},
			}, nil
		},
	}
	groupRepo := &mockGroupRepository{
		getMembersFunc: func(ctx context.Context, groupID int64) ([]model.GroupMember, error) {
			return []model.GroupMember{
				{UserID: otherUserID, Login: "other", AvatarURL: "https://example.com/2.png"},
			}, nil
		},
	}

	svc := NewPostService(nil, nil, postRepo, groupRepo, nil, nil)

	posts, err := svc.ListPosts(context.Background(), 1, requestUserID)
	if err != nil {
		t.Fatalf("ListPosts() failed: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("expected 1 post, got %d", len(posts))
	}

	// ぼかし制御: リクエストユーザーが未投稿の場合、他メンバーの sensitive フィールドは nil
	post := posts[0]
	if !post.Blurred {
		t.Error("expected Blurred=true for other member's post when requester has not posted")
	}
	if post.Body != nil {
		t.Errorf("expected Body=nil (blurred), got %v", post.Body)
	}
	if post.RepoFullName != nil {
		t.Errorf("expected RepoFullName=nil (blurred), got %v", post.RepoFullName)
	}
	if post.LatestCommitMessage != nil {
		t.Errorf("expected LatestCommitMessage=nil (blurred), got %v", post.LatestCommitMessage)
	}
}

// TestPostService_ListPosts_NotBlurred は同じ通知回へ投稿済みの場合に全フィールドが公開されることを確認する
func TestPostService_ListPosts_NotBlurred(t *testing.T) {
	requestUserID := int64(1)
	otherUserID := int64(2)
	notificationID := int64(10)

	body := "コードを書きました"
	repo := "owner/repo"
	msg := "Fix bug"

	postRepo := &mockPostRepository{
		listByGroupIDFunc: func(ctx context.Context, groupID int64) ([]model.Post, error) {
			return []model.Post{
				{
					ID:                  1,
					NotificationID:      &notificationID,
					UserID:              otherUserID,
					GroupID:             1,
					PostType:            "commit",
					Body:                &body,
					RepoFullName:        &repo,
					LatestCommitMessage: &msg,
					CommitCount:         3,
					CreatedAt:           time.Now(),
				},
				{
					ID:             2,
					NotificationID: &notificationID,
					UserID:         requestUserID,
					GroupID:        1,
					PostType:       "memo",
					CreatedAt:      time.Now(),
				},
			}, nil
		},
	}
	groupRepo := &mockGroupRepository{
		getMembersFunc: func(ctx context.Context, groupID int64) ([]model.GroupMember, error) {
			return []model.GroupMember{
				{UserID: otherUserID, Login: "other", AvatarURL: "https://example.com/2.png"},
			}, nil
		},
	}

	svc := NewPostService(nil, nil, postRepo, groupRepo, nil, nil)

	posts, err := svc.ListPosts(context.Background(), 1, requestUserID)
	if err != nil {
		t.Fatalf("ListPosts() failed: %v", err)
	}

	post := posts[0]
	if post.Blurred {
		t.Error("expected Blurred=false for other member's post when requester has posted")
	}
	if post.Body == nil || *post.Body != "コードを書きました" {
		t.Errorf("expected Body=コードを書きました (not blurred), got %v", post.Body)
	}
}

// TestPostService_ListPosts_OwnPost は自分自身の投稿がぼかされないことを確認する
func TestPostService_ListPosts_OwnPost(t *testing.T) {
	requestUserID := int64(1)

	body := "自分のコード"
	repo := "owner/repo"
	msg := "My commit"

	postRepo := &mockPostRepository{
		listByGroupIDFunc: func(ctx context.Context, groupID int64) ([]model.Post, error) {
			return []model.Post{
				{
					ID:                  1,
					UserID:              requestUserID, // 自分の投稿
					GroupID:             1,
					PostType:            "commit",
					Body:                &body,
					RepoFullName:        &repo,
					LatestCommitMessage: &msg,
					CreatedAt:           time.Now(),
				},
			}, nil
		},
	}
	groupRepo := &mockGroupRepository{
		getMembersFunc: func(ctx context.Context, groupID int64) ([]model.GroupMember, error) {
			return []model.GroupMember{
				{UserID: requestUserID, Login: "self", AvatarURL: "https://example.com/1.png"},
			}, nil
		},
	}

	svc := NewPostService(nil, nil, postRepo, groupRepo, nil, nil)

	posts, err := svc.ListPosts(context.Background(), 1, requestUserID)
	if err != nil {
		t.Fatalf("ListPosts() failed: %v", err)
	}

	post := posts[0]
	if post.Blurred {
		t.Error("own post should never be blurred")
	}
	if post.Body == nil || *post.Body != "自分のコード" {
		t.Errorf("expected own post Body to be visible, got %v", post.Body)
	}
}

// TestPostService_ListPosts_PreNotificationPostStaysVisible は通知に紐づかない既存投稿を再ロックしないことを確認する。
func TestPostService_ListPosts_PreNotificationPostStaysVisible(t *testing.T) {
	body := "以前から表示されていた投稿"
	postRepo := &mockPostRepository{
		listByGroupIDFunc: func(ctx context.Context, groupID int64) ([]model.Post, error) {
			return []model.Post{{
				ID:        1,
				UserID:    2,
				GroupID:   groupID,
				PostType:  "memo",
				Body:      &body,
				CreatedAt: time.Now(),
			}}, nil
		},
	}

	svc := NewPostService(nil, nil, postRepo, nil, nil, nil)
	posts, err := svc.ListPosts(context.Background(), 1, 1)
	if err != nil {
		t.Fatalf("ListPosts() failed: %v", err)
	}
	if len(posts) != 1 || posts[0].Blurred {
		t.Fatalf("notification以前の投稿は表示されるべき: %+v", posts)
	}
	if posts[0].Body == nil || *posts[0].Body != body {
		t.Errorf("expected historical post body to remain visible, got %v", posts[0].Body)
	}
}

// TestPostService_ListPosts_MissedDoesNotUnlock は missed レコードでは同じ通知回を解除しないことを確認する。
func TestPostService_ListPosts_MissedDoesNotUnlock(t *testing.T) {
	notificationID := int64(10)
	status := "missed"
	postRepo := &mockPostRepository{
		listByGroupIDFunc: func(ctx context.Context, groupID int64) ([]model.Post, error) {
			return []model.Post{
				{
					ID:             1,
					NotificationID: &notificationID,
					UserID:         2,
					GroupID:        groupID,
					PostType:       "commit",
				},
				{
					ID:             2,
					NotificationID: &notificationID,
					UserID:         1,
					GroupID:        groupID,
					PostType:       "memo",
					Status:         &status,
				},
			}, nil
		},
	}

	svc := NewPostService(nil, nil, postRepo, nil, nil, nil)
	posts, err := svc.ListPosts(context.Background(), 1, 1)
	if err != nil {
		t.Fatalf("ListPosts() failed: %v", err)
	}
	if len(posts) != 2 || !posts[0].Blurred {
		t.Fatalf("missed post must not unlock notification: %+v", posts)
	}
}

// TestPostService_ListPosts_NewerPostUnlocksPastNotifications は、新しい通知で自分が
// 投稿すると、それ以前の通知でロックされていた投稿も表示されることを確認する。
func TestPostService_ListPosts_NewerPostUnlocksPastNotifications(t *testing.T) {
	previousNotificationID := int64(10)
	currentNotificationID := int64(11)
	body := "前回の投稿"

	postRepo := &mockPostRepository{
		listByGroupIDFunc: func(ctx context.Context, groupID int64) ([]model.Post, error) {
			return []model.Post{
				{
					ID:             1,
					NotificationID: &previousNotificationID,
					UserID:         2,
					GroupID:        groupID,
					PostType:       "memo",
					Body:           &body,
				},
				{
					ID:             2,
					NotificationID: &currentNotificationID,
					UserID:         1,
					GroupID:        groupID,
					PostType:       "memo",
				},
			}, nil
		},
	}

	svc := NewPostService(nil, nil, postRepo, nil, nil, nil)
	posts, err := svc.ListPosts(context.Background(), 1, 1)
	if err != nil {
		t.Fatalf("ListPosts() failed: %v", err)
	}
	if len(posts) != 2 || posts[0].Blurred {
		t.Fatalf("newer self post should unlock previous notification posts: %+v", posts)
	}
	if posts[0].Body == nil || *posts[0].Body != body {
		t.Errorf("expected previous post body to be visible, got %v", posts[0].Body)
	}
}

// TestPostService_ListPosts_NewNotificationStaysLocked は、最後の自分の投稿より新しい
// 通知に紐づく投稿は、自分がその通知で投稿するまで隠れることを確認する。
func TestPostService_ListPosts_NewNotificationStaysLocked(t *testing.T) {
	previousNotificationID := int64(10)
	currentNotificationID := int64(11)

	postRepo := &mockPostRepository{
		listByGroupIDFunc: func(ctx context.Context, groupID int64) ([]model.Post, error) {
			return []model.Post{
				{
					ID:             1,
					NotificationID: &currentNotificationID,
					UserID:         2,
					GroupID:        groupID,
					PostType:       "memo",
				},
				{
					ID:             2,
					NotificationID: &previousNotificationID,
					UserID:         1,
					GroupID:        groupID,
					PostType:       "memo",
				},
			}, nil
		},
	}

	svc := NewPostService(nil, nil, postRepo, nil, nil, nil)
	posts, err := svc.ListPosts(context.Background(), 1, 1)
	if err != nil {
		t.Fatalf("ListPosts() failed: %v", err)
	}
	if len(posts) != 2 || !posts[0].Blurred {
		t.Fatalf("post from newer notification should remain locked: %+v", posts)
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
