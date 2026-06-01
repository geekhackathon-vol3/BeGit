package service

import (
	"context"
	"errors"
	"testing"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
)

// mockReactionRepository はテスト用のリアクションリポジトリモック
type mockReactionRepository struct {
	addFunc          func(ctx context.Context, postID, userID int64, reactionType string) error
	removeFunc       func(ctx context.Context, postID, userID int64, reactionType string) error
	listByPostIDFunc func(ctx context.Context, postID int64) ([]model.Reaction, error)
}

func (m *mockReactionRepository) Add(ctx context.Context, postID, userID int64, reactionType string) error {
	if m.addFunc != nil {
		return m.addFunc(ctx, postID, userID, reactionType)
	}
	return nil
}

func (m *mockReactionRepository) Remove(ctx context.Context, postID, userID int64, reactionType string) error {
	if m.removeFunc != nil {
		return m.removeFunc(ctx, postID, userID, reactionType)
	}
	return nil
}

func (m *mockReactionRepository) ListByPostID(ctx context.Context, postID int64) ([]model.Reaction, error) {
	if m.listByPostIDFunc != nil {
		return m.listByPostIDFunc(ctx, postID)
	}
	return []model.Reaction{}, nil
}

// TestReactionService_AddReaction_Success はリアクション追加後に一覧を返すことを確認する
func TestReactionService_AddReaction_Success(t *testing.T) {
	postRepo := &mockPostRepository{
		getByIDFunc: func(ctx context.Context, postID int64) (*model.Post, error) {
			return &model.Post{ID: postID, GroupID: 1}, nil
		},
	}
	reactionRepo := &mockReactionRepository{
		listByPostIDFunc: func(ctx context.Context, postID int64) ([]model.Reaction, error) {
			return []model.Reaction{{ID: 1, PostID: postID, UserID: 2, ReactionType: "heart"}}, nil
		},
	}

	svc := NewReactionService(reactionRepo, postRepo)
	reactions, err := svc.AddReaction(context.Background(), 1, 10, 2, "heart")
	if err != nil {
		t.Fatalf("AddReaction() failed: %v", err)
	}
	if len(reactions) != 1 {
		t.Fatalf("expected 1 reaction, got %d", len(reactions))
	}
}

// TestReactionService_AddReaction_PostNotInGroup は投稿が別グループの場合に ErrNotFound を返すことを確認する
func TestReactionService_AddReaction_PostNotInGroup(t *testing.T) {
	postRepo := &mockPostRepository{
		getByIDFunc: func(ctx context.Context, postID int64) (*model.Post, error) {
			return &model.Post{ID: postID, GroupID: 999}, nil
		},
	}
	reactionRepo := &mockReactionRepository{}

	svc := NewReactionService(reactionRepo, postRepo)
	_, err := svc.AddReaction(context.Background(), 1, 10, 2, "heart")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// TestReactionService_AddReaction_PostNotFound は投稿が存在しない場合に ErrNotFound を返すことを確認する
func TestReactionService_AddReaction_PostNotFound(t *testing.T) {
	postRepo := &mockPostRepository{
		getByIDFunc: func(ctx context.Context, postID int64) (*model.Post, error) {
			return nil, repository.ErrNotFound
		},
	}
	reactionRepo := &mockReactionRepository{}

	svc := NewReactionService(reactionRepo, postRepo)
	_, err := svc.AddReaction(context.Background(), 1, 10, 2, "heart")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
