package service

import (
	"context"
	"errors"
	"testing"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
)

// mockCommentRepository はテスト用のコメントリポジトリモック
type mockCommentRepository struct {
	createFunc       func(ctx context.Context, postID, userID int64, body string) (*model.Comment, error)
	listByPostIDFunc func(ctx context.Context, postID int64) ([]model.Comment, error)
	getByIDFunc      func(ctx context.Context, commentID int64) (*model.Comment, error)
	deleteFunc       func(ctx context.Context, commentID int64) error
}

func (m *mockCommentRepository) Create(ctx context.Context, postID, userID int64, body string) (*model.Comment, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, postID, userID, body)
	}
	return &model.Comment{ID: 1, PostID: postID, UserID: userID, Body: body}, nil
}

func (m *mockCommentRepository) ListByPostID(ctx context.Context, postID int64) ([]model.Comment, error) {
	if m.listByPostIDFunc != nil {
		return m.listByPostIDFunc(ctx, postID)
	}
	return []model.Comment{}, nil
}

func (m *mockCommentRepository) GetByID(ctx context.Context, commentID int64) (*model.Comment, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, commentID)
	}
	return nil, repository.ErrNotFound
}

func (m *mockCommentRepository) Delete(ctx context.Context, commentID int64) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, commentID)
	}
	return nil
}

// TestComment_NotifiesAuthor_OnOtherUser は他者操作で投稿者へ comment を送ることを確認する
func TestComment_NotifiesAuthor_OnOtherUser(t *testing.T) {
	postRepo := &mockPostRepository{
		getByIDFunc: func(ctx context.Context, postID int64) (*model.Post, error) {
			return &model.Post{ID: postID, GroupID: 12, UserID: 99}, nil
		},
	}
	userRepo := &mockUserByID{
		getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
			return &model.User{ID: id, GitHubLogin: "octocat"}, nil
		},
	}
	ft := &mockFCMTokenRepository{
		getTokensByUserIDFunc: func(ctx context.Context, userID int64) ([]string, error) {
			return []string{"author-tok"}, nil
		},
	}
	fc := &fakeFCMClient{}

	svc := NewCommentServiceWithNotifications(&mockCommentRepository{}, postRepo, userRepo, ft, fc)
	if _, err := svc.CreateComment(context.Background(), 12, 890, 2, "nice"); err != nil {
		t.Fatalf("CreateComment() failed: %v", err)
	}
	if len(fc.withDataCalls) != 1 {
		t.Fatalf("expected 1 comment notification, got %d", len(fc.withDataCalls))
	}
	if fc.withDataCalls[0].data["type"] != "comment" || fc.withDataCalls[0].data["actor_login"] != "octocat" {
		t.Errorf("unexpected comment data: %v", fc.withDataCalls[0].data)
	}
}

// TestComment_SelfSuppression は自己コメントで送信しないことを確認する
func TestComment_SelfSuppression(t *testing.T) {
	postRepo := &mockPostRepository{
		getByIDFunc: func(ctx context.Context, postID int64) (*model.Post, error) {
			return &model.Post{ID: postID, GroupID: 12, UserID: 5}, nil
		},
	}
	fc := &fakeFCMClient{}
	svc := NewCommentServiceWithNotifications(&mockCommentRepository{}, postRepo, &mockUserByID{}, &mockFCMTokenRepository{}, fc)
	if _, err := svc.CreateComment(context.Background(), 12, 890, 5, "self"); err != nil {
		t.Fatalf("CreateComment() failed: %v", err)
	}
	if len(fc.withDataCalls) != 0 {
		t.Error("expected no self-notification")
	}
}

func postInGroupRepo(groupID int64) *mockPostRepository {
	return &mockPostRepository{
		getByIDFunc: func(ctx context.Context, postID int64) (*model.Post, error) {
			return &model.Post{ID: postID, GroupID: groupID}, nil
		},
	}
}

// TestCommentService_CreateComment_Success はコメント作成が成功することを確認する
func TestCommentService_CreateComment_Success(t *testing.T) {
	svc := NewCommentService(&mockCommentRepository{}, postInGroupRepo(1))
	comment, err := svc.CreateComment(context.Background(), 1, 10, 2, "hello")
	if err != nil {
		t.Fatalf("CreateComment() failed: %v", err)
	}
	if comment.Body != "hello" {
		t.Errorf("unexpected body: %q", comment.Body)
	}
}

// TestCommentService_DeleteComment_Forbidden は他人のコメント削除で ErrForbidden を返すことを確認する
func TestCommentService_DeleteComment_Forbidden(t *testing.T) {
	commentRepo := &mockCommentRepository{
		getByIDFunc: func(ctx context.Context, commentID int64) (*model.Comment, error) {
			return &model.Comment{ID: commentID, PostID: 10, UserID: 99}, nil
		},
	}
	svc := NewCommentService(commentRepo, postInGroupRepo(1))
	err := svc.DeleteComment(context.Background(), 1, 10, 5, 2)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

// TestCommentService_DeleteComment_Success は本人のコメント削除が成功することを確認する
func TestCommentService_DeleteComment_Success(t *testing.T) {
	deleted := false
	commentRepo := &mockCommentRepository{
		getByIDFunc: func(ctx context.Context, commentID int64) (*model.Comment, error) {
			return &model.Comment{ID: commentID, PostID: 10, UserID: 2}, nil
		},
		deleteFunc: func(ctx context.Context, commentID int64) error {
			deleted = true
			return nil
		},
	}
	svc := NewCommentService(commentRepo, postInGroupRepo(1))
	if err := svc.DeleteComment(context.Background(), 1, 10, 5, 2); err != nil {
		t.Fatalf("DeleteComment() failed: %v", err)
	}
	if !deleted {
		t.Error("expected Delete to be called")
	}
}

// TestCommentService_ListComments_PostNotInGroup は別グループの投稿で ErrNotFound を返すことを確認する
func TestCommentService_ListComments_PostNotInGroup(t *testing.T) {
	svc := NewCommentService(&mockCommentRepository{}, postInGroupRepo(999))
	_, err := svc.ListComments(context.Background(), 1, 10)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// TestComment_FCMFailure_DoesNotFail は FCM 失敗でもコメント登録が成功することを確認する（ベストエフォート + ログ）
func TestComment_FCMFailure_DoesNotFail(t *testing.T) {
	postRepo := &mockPostRepository{
		getByIDFunc: func(ctx context.Context, postID int64) (*model.Post, error) {
			return &model.Post{ID: postID, GroupID: 12, UserID: 99}, nil
		},
	}
	userRepo := &mockUserByID{
		getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
			return &model.User{ID: id, GitHubLogin: "octocat"}, nil
		},
	}
	ft := &mockFCMTokenRepository{
		getTokensByUserIDFunc: func(ctx context.Context, userID int64) ([]string, error) {
			return []string{"author-tok"}, nil
		},
	}
	svc := NewCommentServiceWithNotifications(&mockCommentRepository{}, postRepo, userRepo, ft, &failingFCMClient{})
	if _, err := svc.CreateComment(context.Background(), 12, 890, 2, "nice"); err != nil {
		t.Fatalf("CreateComment() should succeed even if FCM fails, got: %v", err)
	}
}
