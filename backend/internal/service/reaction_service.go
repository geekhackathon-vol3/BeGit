package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
)

// ReactionService はリアクションサービスインターフェース
type ReactionService interface {
	// AddReaction はリアクションを追加し、更新後の一覧を返す。
	AddReaction(ctx context.Context, groupID, postID, userID int64, reactionType string) ([]model.Reaction, error)
	// RemoveReaction はリアクションを削除し、更新後の一覧を返す。
	RemoveReaction(ctx context.Context, groupID, postID, userID int64, reactionType string) ([]model.Reaction, error)
	// ListReactions は投稿のリアクション一覧を返す。
	ListReactions(ctx context.Context, groupID, postID int64) ([]model.Reaction, error)
}

// reactionService は ReactionService インターフェースの実装
type reactionService struct {
	reactionRepo repository.ReactionRepository
	postRepo     repository.PostRepository
}

// NewReactionService は ReactionService を作成する
func NewReactionService(
	reactionRepo repository.ReactionRepository,
	postRepo repository.PostRepository,
) ReactionService {
	return &reactionService{
		reactionRepo: reactionRepo,
		postRepo:     postRepo,
	}
}

// verifyPostInGroup は postID がそのグループに属することを確認する。
// 属さない / 存在しない場合は ErrNotFound を返す。
func (s *reactionService) verifyPostInGroup(ctx context.Context, groupID, postID int64) error {
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("reaction_service: verifyPostInGroup failed: %w", err)
	}
	if post.GroupID != groupID {
		return ErrNotFound
	}
	return nil
}

// AddReaction はリアクションを追加し、更新後の一覧を返す。
func (s *reactionService) AddReaction(ctx context.Context, groupID, postID, userID int64, reactionType string) ([]model.Reaction, error) {
	if err := s.verifyPostInGroup(ctx, groupID, postID); err != nil {
		return nil, err
	}
	if err := s.reactionRepo.Add(ctx, postID, userID, reactionType); err != nil {
		return nil, fmt.Errorf("reaction_service: AddReaction failed: %w", err)
	}
	return s.reactionRepo.ListByPostID(ctx, postID)
}

// RemoveReaction はリアクションを削除し、更新後の一覧を返す。
func (s *reactionService) RemoveReaction(ctx context.Context, groupID, postID, userID int64, reactionType string) ([]model.Reaction, error) {
	if err := s.verifyPostInGroup(ctx, groupID, postID); err != nil {
		return nil, err
	}
	if err := s.reactionRepo.Remove(ctx, postID, userID, reactionType); err != nil {
		return nil, fmt.Errorf("reaction_service: RemoveReaction failed: %w", err)
	}
	return s.reactionRepo.ListByPostID(ctx, postID)
}

// ListReactions は投稿のリアクション一覧を返す。
func (s *reactionService) ListReactions(ctx context.Context, groupID, postID int64) ([]model.Reaction, error) {
	if err := s.verifyPostInGroup(ctx, groupID, postID); err != nil {
		return nil, err
	}
	return s.reactionRepo.ListByPostID(ctx, postID)
}
