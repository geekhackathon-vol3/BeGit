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
