package service

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
	"github.com/irj0927/begit/pkg/fcm"
)

// userByIDRepo は ID からユーザーを引く最小インターフェース（repository.UserRepository が満たす）。
// ⑦ の actor_login 取得に使用する。
type userByIDRepo interface {
	GetByID(ctx context.Context, id int64) (*model.User, error)
}

// notifyPostAuthor は ⑦ ソーシャル通知（reaction/comment）を投稿者本人へ送る共通ヘルパ。
// 自己操作（actor == 投稿者）は送信しない（自己抑制、Req5.3）。
// FCM 失敗・依存未設定は本処理を失敗させない（ベストエフォート、Req6.4）。
func notifyPostAuthor(
	ctx context.Context,
	userRepo userByIDRepo,
	fcmTokenRepo repository.FCMTokenRepository,
	fcmClient fcm.Client,
	post *model.Post,
	actorID int64,
	buildPayload func(groupID, postID int64, actorLogin string) Payload,
) {
	if userRepo == nil || fcmTokenRepo == nil || fcmClient == nil || post == nil {
		return
	}
	// 自己抑制
	if actorID == post.UserID {
		return
	}

	actor, err := userRepo.GetByID(ctx, actorID)
	if err != nil {
		log.Printf("social: GetByID actor %d failed: %v", actorID, err)
		return
	}

	tokens, err := fcmTokenRepo.GetTokensByUserID(ctx, post.UserID)
	if err != nil {
		log.Printf("social: GetTokensByUserID author %d failed: %v", post.UserID, err)
		return
	}
	if len(tokens) == 0 {
		return
	}

	payload := buildPayload(post.GroupID, post.ID, actor.GitHubLogin)
	_ = fcmClient.SendToTokensWithData(ctx, tokens, payload.Notification, payload.Data)
}

// allowedReactionTypes は許可されたリアクションタイプのセット
var allowedReactionTypes = map[string]bool{
	"heart":     true,
	"thumbsup":  true,
	"celebrate": true,
	"fire":      true,
	"rocket":    true,
}

// ErrInvalidReactionType は無効なリアクションタイプが指定された場合に返す
var ErrInvalidReactionType = errors.New("invalid reaction type")

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
	// ⑦ 通知用の依存（nil 可。未設定なら通知を送らない）
	userRepo     userByIDRepo
	fcmTokenRepo repository.FCMTokenRepository
	fcmClient    fcm.Client
}

// NewReactionService は ReactionService を作成する（通知無し。既存配線互換）
func NewReactionService(
	reactionRepo repository.ReactionRepository,
	postRepo repository.PostRepository,
) ReactionService {
	return &reactionService{
		reactionRepo: reactionRepo,
		postRepo:     postRepo,
	}
}

// NewReactionServiceWithNotifications は ⑦ リアクション通知付きの ReactionService を作成する
func NewReactionServiceWithNotifications(
	reactionRepo repository.ReactionRepository,
	postRepo repository.PostRepository,
	userRepo userByIDRepo,
	fcmTokenRepo repository.FCMTokenRepository,
	fcmClient fcm.Client,
) ReactionService {
	return &reactionService{
		reactionRepo: reactionRepo,
		postRepo:     postRepo,
		userRepo:     userRepo,
		fcmTokenRepo: fcmTokenRepo,
		fcmClient:    fcmClient,
	}
}

// getPostInGroup は postID がそのグループに属することを確認し、投稿を返す。
// 属さない / 存在しない場合は ErrNotFound を返す。
func (s *reactionService) getPostInGroup(ctx context.Context, groupID, postID int64) (*model.Post, error) {
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("reaction_service: getPostInGroup failed: %w", err)
	}
	if post.GroupID != groupID {
		return nil, ErrNotFound
	}
	return post, nil
}

// verifyPostInGroup は postID がそのグループに属することを確認する。
func (s *reactionService) verifyPostInGroup(ctx context.Context, groupID, postID int64) error {
	_, err := s.getPostInGroup(ctx, groupID, postID)
	return err
}

// AddReaction はリアクションを追加し、更新後の一覧を返す。
func (s *reactionService) AddReaction(ctx context.Context, groupID, postID, userID int64, reactionType string) ([]model.Reaction, error) {
	if !allowedReactionTypes[reactionType] {
		return nil, ErrInvalidReactionType
	}
	post, err := s.getPostInGroup(ctx, groupID, postID)
	if err != nil {
		return nil, err
	}
	if err := s.reactionRepo.Add(ctx, postID, userID, reactionType); err != nil {
		return nil, fmt.Errorf("reaction_service: AddReaction failed: %w", err)
	}

	// ⑦ 投稿者本人へ reaction 通知（自己抑制・ベストエフォート）
	notifyPostAuthor(ctx, s.userRepo, s.fcmTokenRepo, s.fcmClient, post, userID, BuildReaction)

	return s.reactionRepo.ListByPostID(ctx, postID)
}

// RemoveReaction はリアクションを削除し、更新後の一覧を返す。
func (s *reactionService) RemoveReaction(ctx context.Context, groupID, postID, userID int64, reactionType string) ([]model.Reaction, error) {
	if !allowedReactionTypes[reactionType] {
		return nil, ErrInvalidReactionType
	}
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
