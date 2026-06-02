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
	// GetLatestInSprintBefore は同一スプリント内・指定時刻以前(sent_at <= before)で最新の通知を返す。
	// ② Nice Work! の anchor 特定に使用する。該当が無ければ ErrNotFound。
	GetLatestInSprintBefore(ctx context.Context, sprintID int64, before time.Time) (*model.Notification, error)
	// HasActiveInSprint は同一スプリント内に sent_at + 1h > now() を満たすアクティブ通知が存在するかを返す。
	// ① BeGit Time! の時間的非共存判定に使用する。
	HasActiveInSprint(ctx context.Context, sprintID int64) (bool, error)
	// CreateIfNoActive は同一スプリント内にアクティブ通知が無い場合のみ INSERT する（原子的）。
	// アクティブ通知が既に存在する場合は ErrConstraintViolation を返す（時間的非共存保証）。
	CreateIfNoActive(ctx context.Context, notif *model.Notification) (*model.Notification, error)
	// ListChallengeEndDue は sent_at + 1h <= now() に到達した通知を返す（③ challenge_end の対象抽出）。
	ListChallengeEndDue(ctx context.Context) ([]model.Notification, error)
	// ListBySprintID は指定スプリントの全通知を返す（⑤ サマリ算出用）。
	ListBySprintID(ctx context.Context, sprintID int64) ([]model.Notification, error)
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
		message = "今、なに作ってる？"
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

// GetLatestInSprintBefore は同一スプリント内・指定時刻以前で最新の通知を返す
func (r *notificationRepository) GetLatestInSprintBefore(ctx context.Context, sprintID int64, before time.Time) (*model.Notification, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, sprint_id, sent_by, message, sent_at
		 FROM notifications
		 WHERE sprint_id = ? AND sent_at <= datetime(?)
		 ORDER BY sent_at DESC, id DESC
		 LIMIT 1`,
		[]interface{}{sprintID, before.UTC().Format("2006-01-02 15:04:05")},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("notification_repository: GetLatestInSprintBefore failed: %w", err)
	}

	return scanNotification(rows[0])
}

// HasActiveInSprint は同一スプリント内に sent_at + 1h > now() のアクティブ通知が存在するかを返す
func (r *notificationRepository) HasActiveInSprint(ctx context.Context, sprintID int64) (bool, error) {
	rows, err := r.db.Query(ctx,
		`SELECT COUNT(*) as count
		 FROM notifications
		 WHERE sprint_id = ? AND datetime(sent_at, '+1 hour') > datetime('now')`,
		[]interface{}{sprintID},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("notification_repository: HasActiveInSprint failed: %w", err)
	}

	if len(rows) == 0 {
		return false, nil
	}
	count, _ := rows[0]["count"].(float64)
	return count > 0, nil
}

// CreateIfNoActive は同一スプリント内にアクティブ通知が無い場合のみ INSERT する（原子的）。
// INSERT ... WHERE NOT EXISTS で時間的非共存を原子的に保証する。
// アクティブ通知が既に存在する場合は INSERT が 0 行となり ErrConstraintViolation を返す。
func (r *notificationRepository) CreateIfNoActive(ctx context.Context, notif *model.Notification) (*model.Notification, error) {
	message := notif.Message
	if message == "" {
		message = "今、なに作ってる？"
	}

	// INSERT with conditional WHERE NOT EXISTS to ensure atomicity
	result, err := r.db.Exec(ctx,
		`INSERT INTO notifications (sprint_id, sent_by, message)
		 SELECT ?, ?, ?
		 WHERE NOT EXISTS (
		   SELECT 1 FROM notifications
		   WHERE sprint_id = ? AND datetime(sent_at, '+1 hour') > datetime('now')
		 )
		 AND NOT EXISTS (
		   SELECT 1 FROM notifications
		   WHERE sprint_id = ? AND sent_by = ?
		 )`,
		[]interface{}{notif.SprintID, notif.SentBy, message, notif.SprintID, notif.SprintID, notif.SentBy},
	)
	if err != nil {
		if errors.Is(err, d1.ErrConstraintViolation) {
			return nil, ErrConstraintViolation
		}
		return nil, fmt.Errorf("notification_repository: CreateIfNoActive failed: %w", err)
	}

	// Check if the INSERT succeeded (affected rows should be 1)
	// D1 Exec doesn't return affected rows, so we need to fetch the created record
	rows, err := r.db.Query(ctx,
		`SELECT id, sprint_id, sent_by, message, sent_at
		 FROM notifications
		 WHERE sprint_id = ? AND sent_by = ?
		 ORDER BY id DESC LIMIT 1`,
		[]interface{}{notif.SprintID, notif.SentBy},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			// INSERT was blocked by WHERE NOT EXISTS (active notification exists or UNIQUE violation)
			return nil, ErrConstraintViolation
		}
		return nil, fmt.Errorf("notification_repository: CreateIfNoActive fetch after insert failed: %w", err)
	}

	if len(rows) == 0 {
		return nil, ErrConstraintViolation
	}

	_ = result // suppress unused variable warning
	return scanNotification(rows[0])
}

// ListChallengeEndDue は sent_at + 1h <= now() に到達した通知を返す（③ challenge_end 対象）。
// 既に challenge_end として送信済み（notification_deliveries に記録済み）の通知は除外する。
func (r *notificationRepository) ListChallengeEndDue(ctx context.Context) ([]model.Notification, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, sprint_id, sent_by, message, sent_at
		 FROM notifications
		 WHERE datetime(sent_at, '+1 hour') <= datetime('now')
		 AND NOT EXISTS (
		   SELECT 1 FROM notification_deliveries
		   WHERE kind = 'challenge_end' AND ref_id = notifications.id
		 )`,
		[]interface{}{},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return []model.Notification{}, nil
		}
		return nil, fmt.Errorf("notification_repository: ListChallengeEndDue failed: %w", err)
	}
	return scanNotifications(rows)
}

// ListBySprintID は指定スプリントの全通知を返す
func (r *notificationRepository) ListBySprintID(ctx context.Context, sprintID int64) ([]model.Notification, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, sprint_id, sent_by, message, sent_at
		 FROM notifications WHERE sprint_id = ?`,
		[]interface{}{sprintID},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return []model.Notification{}, nil
		}
		return nil, fmt.Errorf("notification_repository: ListBySprintID failed: %w", err)
	}
	return scanNotifications(rows)
}

// scanNotifications は複数行をスライスへ変換する
func scanNotifications(rows []map[string]interface{}) ([]model.Notification, error) {
	out := make([]model.Notification, 0, len(rows))
	for _, row := range rows {
		n, err := scanNotification(row)
		if err != nil {
			return nil, err
		}
		out = append(out, *n)
	}
	return out, nil
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
