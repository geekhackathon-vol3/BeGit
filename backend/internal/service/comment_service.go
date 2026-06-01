package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
)

// CommentService はコメントサービスインターフェース
type CommentService interface {
	// CreateComment はコメントを作成して返す。
	CreateComment(ctx context.Context, groupID, postID, userID int64, body string) (*model.Comment, error)
	// ListComments は投稿のコメント一覧を返す。
	ListComments(ctx context.Context, groupID, postID int64) ([]model.Comment, error)
	// DeleteComment はコメントを削除する（本人のみ）。
	DeleteComment(ctx context.Context, groupID, postID, commentID, userID int64) error
}

// commentService は CommentService インターフェースの実装
type commentService struct {
	commentRepo repository.CommentRepository
	postRepo    repository.PostRepository
}

// NewCommentService は CommentService を作成する
func NewCommentService(
	commentRepo repository.CommentRepository,
	postRepo repository.PostRepository,
) CommentService {
	return &commentService{
		commentRepo: commentRepo,
		postRepo:    postRepo,
	}
}

// verifyPostInGroup は postID がそのグループに属することを確認する。
func (s *commentService) verifyPostInGroup(ctx context.Context, groupID, postID int64) error {
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("comment_service: verifyPostInGroup failed: %w", err)
	}
	if post.GroupID != groupID {
		return ErrNotFound
	}
	return nil
}

// CreateComment はコメントを作成して返す。
func (s *commentService) CreateComment(ctx context.Context, groupID, postID, userID int64, body string) (*model.Comment, error) {
	if err := s.verifyPostInGroup(ctx, groupID, postID); err != nil {
		return nil, err
	}
	comment, err := s.commentRepo.Create(ctx, postID, userID, body)
	if err != nil {
		return nil, fmt.Errorf("comment_service: CreateComment failed: %w", err)
	}
	return comment, nil
}

// ListComments は投稿のコメント一覧を返す。
func (s *commentService) ListComments(ctx context.Context, groupID, postID int64) ([]model.Comment, error) {
	if err := s.verifyPostInGroup(ctx, groupID, postID); err != nil {
		return nil, err
	}
	return s.commentRepo.ListByPostID(ctx, postID)
}

// DeleteComment はコメントを削除する。本人以外は ErrForbidden を返す。
func (s *commentService) DeleteComment(ctx context.Context, groupID, postID, commentID, userID int64) error {
	if err := s.verifyPostInGroup(ctx, groupID, postID); err != nil {
		return err
	}

	comment, err := s.commentRepo.GetByID(ctx, commentID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("comment_service: DeleteComment failed: %w", err)
	}
	if comment.PostID != postID {
		return ErrNotFound
	}
	if comment.UserID != userID {
		return ErrForbidden
	}

	if err := s.commentRepo.Delete(ctx, commentID); err != nil {
		return fmt.Errorf("comment_service: DeleteComment failed: %w", err)
	}
	return nil
}
