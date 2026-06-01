package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/irj0927/begit/pkg/d1"
)

// WebhookRepository は github_webhook_deliveries テーブルへのアクセスインターフェース
type WebhookRepository interface {
	// InsertDelivery は delivery_id を github_webhook_deliveries テーブルに INSERT する。
	// UNIQUE 制約違反（重複）の場合は isDuplicate=true, err=nil を返す。
	InsertDelivery(ctx context.Context, deliveryID, eventType string) (isDuplicate bool, err error)
}

// webhookRepository は WebhookRepository インターフェースの実装
type webhookRepository struct {
	db d1.Client
}

// NewWebhookRepository は WebhookRepository を作成する
func NewWebhookRepository(db d1.Client) WebhookRepository {
	return &webhookRepository{db: db}
}

// InsertDelivery は delivery_id を INSERT する
func (r *webhookRepository) InsertDelivery(ctx context.Context, deliveryID, eventType string) (bool, error) {
	_, err := r.db.Exec(ctx,
		`INSERT INTO github_webhook_deliveries (delivery_id, event_type) VALUES (?, ?)`,
		[]interface{}{deliveryID, eventType},
	)
	if err != nil {
		if errors.Is(err, d1.ErrConstraintViolation) {
			// 重複の場合は isDuplicate=true, err=nil を返す
			return true, nil
		}
		return false, fmt.Errorf("webhook_repository: InsertDelivery failed: %w", err)
	}
	return false, nil
}
