package service

import (
	"context"
	"fmt"

	"github.com/irj0927/begit/internal/repository"
)

// FCMTokenService は FCM トークン管理サービスインターフェース
type FCMTokenService interface {
	UpsertFCMToken(ctx context.Context, userID int64, token string) error
	// DeleteFCMTokens はユーザーの FCM トークンを全削除する（ログアウト用）
	DeleteFCMTokens(ctx context.Context, userID int64) error
}

// fcmTokenService は FCMTokenService インターフェースの実装
type fcmTokenService struct {
	fcmTokenRepo repository.FCMTokenRepository
}

// NewFCMTokenService は FCMTokenService を作成する
func NewFCMTokenService(fcmTokenRepo repository.FCMTokenRepository) FCMTokenService {
	return &fcmTokenService{fcmTokenRepo: fcmTokenRepo}
}

// UpsertFCMToken は FCM トークンを UPSERT する
func (s *fcmTokenService) UpsertFCMToken(ctx context.Context, userID int64, token string) error {
	if err := s.fcmTokenRepo.Upsert(ctx, userID, token); err != nil {
		return fmt.Errorf("fcm_token_service: UpsertFCMToken failed: %w", err)
	}
	return nil
}

// DeleteFCMTokens はユーザーの FCM トークンを全削除する
func (s *fcmTokenService) DeleteFCMTokens(ctx context.Context, userID int64) error {
	if err := s.fcmTokenRepo.DeleteByUserID(ctx, userID); err != nil {
		return fmt.Errorf("fcm_token_service: DeleteFCMTokens failed: %w", err)
	}
	return nil
}
