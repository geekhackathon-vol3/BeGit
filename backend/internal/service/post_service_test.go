package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/irj0927/begit/internal/model"
	githubpkg "github.com/irj0927/begit/pkg/github"
)

// TestPostService_ListPosts_Blurred はリクエストユーザーが未投稿の場合に他メンバーの sensitive フィールドが nil で返ることを確認する
func TestPostService_ListPosts_Blurred(t *testing.T) {
	requestUserID := int64(1)
	otherUserID := int64(2)

	body := "コードを書きました"
	repo := "owner/repo"
	msg := "Fix bug"

	sprintRepo := &mockSprintRepository{}
	postRepo := &mockPostRepository{
		hasPostedInSprintFunc: func(ctx context.Context, userID, sprintID int64) (bool, error) {
			return false, nil // リクエストユーザーは未投稿
		},
		listByGroupIDFunc: func(ctx context.Context, groupID int64) ([]model.Post, error) {
			return []model.Post{
				{
					ID:                  1,
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

	svc := NewPostService(nil, sprintRepo, postRepo, groupRepo, nil, nil)

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

// TestPostService_ListPosts_NotBlurred はリクエストユーザーが投稿済みの場合に全フィールドが公開されることを確認する
func TestPostService_ListPosts_NotBlurred(t *testing.T) {
	requestUserID := int64(1)
	otherUserID := int64(2)

	body := "コードを書きました"
	repo := "owner/repo"
	msg := "Fix bug"

	sprintRepo := &mockSprintRepository{}
	postRepo := &mockPostRepository{
		hasPostedInSprintFunc: func(ctx context.Context, userID, sprintID int64) (bool, error) {
			return true, nil // リクエストユーザーは投稿済み
		},
		listByGroupIDFunc: func(ctx context.Context, groupID int64) ([]model.Post, error) {
			return []model.Post{
				{
					ID:                  1,
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

	svc := NewPostService(nil, sprintRepo, postRepo, groupRepo, nil, nil)

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

	sprintRepo := &mockSprintRepository{}
	postRepo := &mockPostRepository{
		hasPostedInSprintFunc: func(ctx context.Context, userID, sprintID int64) (bool, error) {
			return false, nil // 未投稿（だが自分の投稿は常に公開）
		},
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

	svc := NewPostService(nil, sprintRepo, postRepo, groupRepo, nil, nil)

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
