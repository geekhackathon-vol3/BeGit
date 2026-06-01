package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/irj0927/begit/pkg/d1"
)

// FCMTokenRepository は fcm_tokens テーブルへのアクセスインターフェ��ス
type FCMTokenRepository interface {
	// Upsert は FCM トークンを INSERT OR REPLACE する
	Upsert(ctx context.Context, userID int64, token string) error
	// GetTokensByGroupID はグループ内の全メンバーの FCM トークン一覧を取得する
	GetTokensByGroupID(ctx context.Context, groupID int64) ([]string, error)
	// DeleteByUserID はユーザーの FCM トークンを全て削除する（ログアウト用）
	DeleteByUserID(ctx context.Context, userID int64) error
}

// fcmTokenRepository は FCMTokenRepository インターフェースの実装
type fcmTokenRepository struct {
	db d1.Client
}

// NewFCMTokenRepository は FCMTokenRepository を作成する
func NewFCMTokenRepository(db d1.Client) FCMTokenRepository {
	return &fcmTokenRepository{db: db}
}

// Upsert は FCM トークンを INSERT OR REPLACE する
func (r *fcmTokenRepository) Upsert(ctx context.Context, userID int64, token string) error {
	_, err := r.db.Exec(ctx,
		`INSERT OR REPLACE INTO fcm_tokens (user_id, token, updated_at)
		 VALUES (?, ?, datetime('now'))`,
		[]interface{}{userID, token},
	)
	if err != nil {
		return fmt.Errorf("fcm_token_repository: Upsert failed: %w", err)
	}
	return nil
}

// DeleteByUserID はユーザーの FCM トークンを全て削除する
func (r *fcmTokenRepository) DeleteByUserID(ctx context.Context, userID int64) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM fcm_tokens WHERE user_id = ?`,
		[]interface{}{userID},
	)
	if err != nil {
		return fmt.Errorf("fcm_token_repository: DeleteByUserID failed: %w", err)
	}
	return nil
}

// GetTokensByGroupID はグループ内の全メンバーの FCM トークン一覧を取得する
func (r *fcmTokenRepository) GetTokensByGroupID(ctx context.Context, groupID int64) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT ft.token
		 FROM fcm_tokens ft
		 INNER JOIN group_members gm ON ft.user_id = gm.user_id
		 WHERE gm.group_id = ?`,
		[]interface{}{groupID},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("fcm_token_repository: GetTokensByGroupID failed: %w", err)
	}

	tokens := make([]string, 0, len(rows))
	for _, row := range rows {
		if token, ok := row["token"].(string); ok {
			tokens = append(tokens, token)
		}
	}
	return tokens, nil
}
