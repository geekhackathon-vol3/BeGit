package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
	"github.com/irj0927/begit/pkg/fcm"
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
	// ⑦ 通知用の依存（nil 可。未設定なら通知を送らない）
	userRepo     userByIDRepo
	fcmTokenRepo repository.FCMTokenRepository
	fcmClient    fcm.Client
}

// NewCommentService は CommentService を作成する（通知無し。既存配線互換）
func NewCommentService(
	commentRepo repository.CommentRepository,
	postRepo repository.PostRepository,
) CommentService {
	return &commentService{
		commentRepo: commentRepo,
		postRepo:    postRepo,
	}
}

// NewCommentServiceWithNotifications は ⑦ コメント通知付きの CommentService を作成する
func NewCommentServiceWithNotifications(
	commentRepo repository.CommentRepository,
	postRepo repository.PostRepository,
	userRepo userByIDRepo,
	fcmTokenRepo repository.FCMTokenRepository,
	fcmClient fcm.Client,
) CommentService {
	return &commentService{
		commentRepo:  commentRepo,
		postRepo:     postRepo,
		userRepo:     userRepo,
		fcmTokenRepo: fcmTokenRepo,
		fcmClient:    fcmClient,
	}
}

// getPostInGroup は postID がそのグループに属することを確認し、投稿を返す。
func (s *commentService) getPostInGroup(ctx context.Context, groupID, postID int64) (*model.Post, error) {
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("comment_service: getPostInGroup failed: %w", err)
	}
	if post.GroupID != groupID {
		return nil, ErrNotFound
	}
	return post, nil
}

// verifyPostInGroup は postID がそのグループに属することを確認する。
func (s *commentService) verifyPostInGroup(ctx context.Context, groupID, postID int64) error {
	_, err := s.getPostInGroup(ctx, groupID, postID)
	return err
}

// CreateComment はコメントを作成して返す。
func (s *commentService) CreateComment(ctx context.Context, groupID, postID, userID int64, body string) (*model.Comment, error) {
	post, err := s.getPostInGroup(ctx, groupID, postID)
	if err != nil {
		return nil, err
	}
	comment, err := s.commentRepo.Create(ctx, postID, userID, body)
	if err != nil {
		return nil, fmt.Errorf("comment_service: CreateComment failed: %w", err)
	}

	// ⑦ 投稿者本人へ comment 通知（自己抑制・ベストエフォート）
	notifyPostAuthor(ctx, s.userRepo, s.fcmTokenRepo, s.fcmClient, post, userID, BuildComment)

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
