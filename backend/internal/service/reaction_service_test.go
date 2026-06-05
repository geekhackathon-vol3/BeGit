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

// mockUserByID は userByIDRepo を満たすテスト用モック
type mockUserByID struct {
	getByIDFunc func(ctx context.Context, id int64) (*model.User, error)
}

func (m *mockUserByID) GetByID(ctx context.Context, id int64) (*model.User, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return &model.User{ID: id, GitHubLogin: "actor"}, nil
}

// TestReaction_NotifiesAuthor_OnOtherUser は他者操作で投稿者へ reaction を送ることを確認する
func TestReaction_NotifiesAuthor_OnOtherUser(t *testing.T) {
	postRepo := &mockPostRepository{
		getByIDFunc: func(ctx context.Context, postID int64) (*model.Post, error) {
			return &model.Post{ID: postID, GroupID: 12, UserID: 99}, nil // 投稿者=99
		},
	}
	reactionRepo := &mockReactionRepository{}
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

	svc := NewReactionServiceWithNotifications(reactionRepo, postRepo, userRepo, ft, fc)
	// actor=2（投稿者99 と異なる）
	if _, err := svc.AddReaction(context.Background(), 12, 890, 2, "heart"); err != nil {
		t.Fatalf("AddReaction() failed: %v", err)
	}
	if len(fc.withDataCalls) != 1 {
		t.Fatalf("expected 1 reaction notification, got %d", len(fc.withDataCalls))
	}
	c := fc.withDataCalls[0]
	if c.data["type"] != "reaction" || c.data["post_id"] != "890" || c.data["actor_login"] != "octocat" || c.tokens[0] != "author-tok" {
		t.Errorf("unexpected reaction data/tokens: %v %v", c.data, c.tokens)
	}
}

// TestReaction_SelfSuppression は自己操作で送信しないことを確認する
func TestReaction_SelfSuppression(t *testing.T) {
	postRepo := &mockPostRepository{
		getByIDFunc: func(ctx context.Context, postID int64) (*model.Post, error) {
			return &model.Post{ID: postID, GroupID: 12, UserID: 5}, nil
		},
	}
	fc := &fakeFCMClient{}
	svc := NewReactionServiceWithNotifications(&mockReactionRepository{}, postRepo, &mockUserByID{}, &mockFCMTokenRepository{}, fc)
	// actor=5 == 投稿者5
	if _, err := svc.AddReaction(context.Background(), 12, 890, 5, "heart"); err != nil {
		t.Fatalf("AddReaction() failed: %v", err)
	}
	if len(fc.withDataCalls) != 0 {
		t.Error("expected no self-notification")
	}
}

// TestReaction_FCMFailure_DoesNotFail は FCM 失敗でもリアクション登録が成功することを確認する
func TestReaction_FCMFailure_DoesNotFail(t *testing.T) {
	postRepo := &mockPostRepository{
		getByIDFunc: func(ctx context.Context, postID int64) (*model.Post, error) {
			return &model.Post{ID: postID, GroupID: 12, UserID: 99}, nil
		},
	}
	ft := &mockFCMTokenRepository{
		getTokensByUserIDFunc: func(ctx context.Context, userID int64) ([]string, error) {
			return []string{"t"}, nil
		},
	}
	fc := &failingFCMClient{}
	svc := NewReactionServiceWithNotifications(&mockReactionRepository{}, postRepo, &mockUserByID{}, ft, fc)
	if _, err := svc.AddReaction(context.Background(), 12, 890, 2, "heart"); err != nil {
		t.Fatalf("AddReaction() should succeed even if FCM fails, got: %v", err)
	}
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
