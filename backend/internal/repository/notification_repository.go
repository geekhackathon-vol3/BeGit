package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/pkg/d1"
)

// NotificationRepository は notifications テーブルへのアクセスインターフェース
type NotificationRepository interface {
	Create(ctx context.Context, notif *model.Notification) (*model.Notification, error)
	GetByID(ctx context.Context, notifID int64) (*model.Notification, error)
}

// notificationRepository は NotificationRepository インターフェースの実装
type notificationRepository struct {
	db d1.Client
}

// NewNotificationRepository は NotificationRepository を作成する
func NewNotificationRepository(db d1.Client) NotificationRepository {
	return &notificationRepository{db: db}
}

// scanNotification は D1 クエリ結果を model.Notification に変換する
func scanNotification(row map[string]interface{}) (*model.Notification, error) {
	n := &model.Notification{}
	if v, ok := row["id"].(float64); ok {
		n.ID = int64(v)
	}
	if v, ok := row["sprint_id"].(float64); ok {
		n.SprintID = int64(v)
	}
	if v, ok := row["sent_by"].(float64); ok {
		n.SentBy = int64(v)
	}
	if v, ok := row["message"].(string); ok {
		n.Message = v
	}
	if v, ok := row["sent_at"].(string); ok {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			t, err = time.Parse("2006-01-02 15:04:05", v)
			if err != nil {
				return nil, fmt.Errorf("failed to parse sent_at: %w", err)
			}
		}
		n.SentAt = t
	}
	return n, nil
}

// Create は notifications テーブルにレコードを挿入する
// UNIQUE(sprint_id, sent_by) 違反時は ErrConstraintViolation を返す
func (r *notificationRepository) Create(ctx context.Context, notif *model.Notification) (*model.Notification, error) {
	message := notif.Message
	if message == "" {
		message = "今なに作ってる？"
	}

	_, err := r.db.Exec(ctx,
		`INSERT INTO notifications (sprint_id, sent_by, message) VALUES (?, ?, ?)`,
		[]interface{}{notif.SprintID, notif.SentBy, message},
	)
	if err != nil {
		if errors.Is(err, d1.ErrConstraintViolation) {
			return nil, ErrConstraintViolation
		}
		return nil, fmt.Errorf("notification_repository: Create failed: %w", err)
	}

	// 作成されたレコードを取得して返す
	rows, err := r.db.Query(ctx,
		`SELECT id, sprint_id, sent_by, message, sent_at
		 FROM notifications
		 WHERE sprint_id = ? AND sent_by = ?
		 ORDER BY id DESC LIMIT 1`,
		[]interface{}{notif.SprintID, notif.SentBy},
	)
	if err != nil {
		return nil, fmt.Errorf("notification_repository: Create fetch after insert failed: %w", err)
	}

	return scanNotification(rows[0])
}

// GetByID は notifID で通知を取得する
func (r *notificationRepository) GetByID(ctx context.Context, notifID int64) (*model.Notification, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, sprint_id, sent_by, message, sent_at
		 FROM notifications WHERE id = ? LIMIT 1`,
		[]interface{}{notifID},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("notification_repository: GetByID failed: %w", err)
	}

	return scanNotification(rows[0])
}
