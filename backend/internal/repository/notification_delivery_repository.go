package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/irj0927/begit/pkg/d1"
)

// NotificationDeliveryRepository は notification_deliveries テーブルへのアクセスインターフェース。
// Cron 通知（③ challenge_end / ④ sprint_reminder / ⑤ sprint_end / ⑥ sprint_start）の
// 送信済み冪等を UNIQUE(kind, ref_id) で担保する。
type NotificationDeliveryRepository interface {
	// MarkSent は (kind, ref_id) を INSERT する。
	// 既に送信済み（UNIQUE 違反）の場合は alreadySent=true を返し、新規 INSERT の場合は false を返す。
	// 呼び出し側は alreadySent=false のときのみ FCM 送信する（冪等の要）。
	MarkSent(ctx context.Context, kind string, refID int64) (alreadySent bool, err error)
}

// notificationDeliveryRepository は NotificationDeliveryRepository インターフェースの実装
type notificationDeliveryRepository struct {
	db d1.Client
}

// NewNotificationDeliveryRepository は NotificationDeliveryRepository を作成する
func NewNotificationDeliveryRepository(db d1.Client) NotificationDeliveryRepository {
	return &notificationDeliveryRepository{db: db}
}

// MarkSent は (kind, ref_id) を INSERT し、UNIQUE 違反なら送信済みと判定する
func (r *notificationDeliveryRepository) MarkSent(ctx context.Context, kind string, refID int64) (bool, error) {
	_, err := r.db.Exec(ctx,
		`INSERT INTO notification_deliveries (kind, ref_id) VALUES (?, ?)`,
		[]interface{}{kind, refID},
	)
	if err != nil {
		if errors.Is(err, d1.ErrConstraintViolation) {
			// 既に送信済み
			return true, nil
		}
		return false, fmt.Errorf("notification_delivery_repository: MarkSent failed: %w", err)
	}
	return false, nil
}
