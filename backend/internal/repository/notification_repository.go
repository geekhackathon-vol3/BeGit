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
	// GetActiveInSprint は指定スプリントで現在進行中の最新通知を返す。
	GetActiveInSprint(ctx context.Context, sprintID int64, now time.Time) (*model.Notification, error)
	// Stop は通知発行者本人の通知を停止する。
	Stop(ctx context.Context, notifID, userID int64) error
	// GetLatestInSprintBefore は同一スプリント内・指定時刻以前(sent_at <= before)で最新の通知を返す。
	// ② Nice Work! の anchor 特定に使用する。該当が無ければ ErrNotFound。
	GetLatestInSprintBefore(ctx context.Context, sprintID int64, before time.Time) (*model.Notification, error)
	// HasActiveInSprint は同一スプリント内にアクティブ通知（未中断 かつ sent_at + 1h > now()）が存在するかを返す。
	// ① BeGit Time! の時間的非共存判定に使用する。
	HasActiveInSprint(ctx context.Context, sprintID int64) (bool, error)
	// GetActiveByGroup はグループ内で進行中（未中断 かつ sent_at + 1h > now()）の通知を返す。
	// スプリントを経由せず sprints.group_id で引くため、スプリント最終1時間に発行された通知も
	// スプリント切替後に取りこぼさない。複数あれば最新（sent_at DESC）。該当が無ければ ErrNotFound。
	GetActiveByGroup(ctx context.Context, groupID int64) (*model.Notification, error)
	// EndIfActive は通知が進行中（未中断 かつ sent_at + 1h > now()）の場合のみ ended_at = now() を記録する。
	// 更新できたら true。既に中断済み・1時間経過済み・存在しない場合は false（エラーにしない）。
	EndIfActive(ctx context.Context, notifID int64) (bool, error)
	// CreateIfNoActive は同一スプリント内にアクティブ通知が無い場合のみ INSERT する（原子的）。
	// allowMultiplePerSprint が false のときは、同一スプリントで同一ユーザーが発行済みの場合も拒否する（1スプリント1人1回）。
	// いずれかの条件で拒否した場合は ErrConstraintViolation を返す。
	CreateIfNoActive(ctx context.Context, notif *model.Notification, allowMultiplePerSprint bool) (*model.Notification, error)
	// ListChallengeEndDue は締め切り（sent_at + 1h、または途中中断なら ended_at）に到達した通知を返す
	// （③ challenge_end の対象抽出）。
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

// parseNotificationTime は D1 の日時文字列（RFC3339 または SQLite の datetime('now') 形式、UTC）を time.Time に変換する
func parseNotificationTime(v string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, v)
	if err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02 15:04:05", v)
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
		t, err := parseNotificationTime(v)
		if err != nil {
			return nil, fmt.Errorf("failed to parse sent_at: %w", err)
		}
		n.SentAt = t
	}
	if v, ok := row["stopped_at"].(string); ok && v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			t, err = time.Parse("2006-01-02 15:04:05", v)
		}
		if err == nil {
			n.StoppedAt = &t
		}
	}
	// ended_at は NULL（未中断）が既定。非 NULL のときだけ設定する
	if v, ok := row["ended_at"].(string); ok && v != "" {
		t, err := parseNotificationTime(v)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ended_at: %w", err)
		}
		n.EndedAt = &t
	}
	return n, nil
}

// Create は notifications テーブルにレコードを挿入する（発行ルールの判定はしない）。
// D1 の制約違反時は ErrConstraintViolation を返す
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
		`SELECT id, sprint_id, sent_by, message, sent_at, stopped_at, ended_at
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
		`SELECT id, sprint_id, sent_by, message, sent_at, stopped_at, ended_at
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

// HasActiveInSprint は同一スプリント内にアクティブ通知（未中断 かつ sent_at + 1h > now()）が存在するかを返す
func (r *notificationRepository) HasActiveInSprint(ctx context.Context, sprintID int64) (bool, error) {
	rows, err := r.db.Query(ctx,
		`SELECT COUNT(*) as count
		 FROM notifications
		 WHERE sprint_id = ? AND stopped_at IS NULL AND ended_at IS NULL
		 AND datetime(sent_at, '+1 hour') > datetime('now')`,
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
// allowMultiplePerSprint が false のときは「1スプリント1人1回」も同じ INSERT の条件で保証する
// （DB に UNIQUE(sprint_id, sent_by) は持たない。0006 で撤去）。
// 条件に合わず INSERT が 0 行となった場合は ErrConstraintViolation を返す。
func (r *notificationRepository) CreateIfNoActive(ctx context.Context, notif *model.Notification, allowMultiplePerSprint bool) (*model.Notification, error) {
	message := notif.Message
	if message == "" {
		message = "今、なに作ってる？"
	}

	// 「アクティブ」= 未中断（stopped_at / ended_at がともに NULL）かつ発行から1時間以内。
	query := `INSERT INTO notifications (sprint_id, sent_by, message)
		 SELECT ?, ?, ?
		 WHERE NOT EXISTS (
		   SELECT 1 FROM notifications
		   WHERE sprint_id = ? AND stopped_at IS NULL AND ended_at IS NULL
		     AND datetime(sent_at, '+1 hour') > datetime('now')
		 )`
	params := []interface{}{notif.SprintID, notif.SentBy, message, notif.SprintID}
	if !allowMultiplePerSprint {
		query += `
		 AND NOT EXISTS (
		   SELECT 1 FROM notifications
		   WHERE sprint_id = ? AND sent_by = ?
		 )`
		params = append(params, notif.SprintID, notif.SentBy)
	}

	// INSERT with conditional WHERE NOT EXISTS to ensure atomicity
	rowsAffected, err := r.db.Exec(ctx, query, params)
	if err != nil {
		if errors.Is(err, d1.ErrConstraintViolation) {
			return nil, ErrConstraintViolation
		}
		return nil, fmt.Errorf("notification_repository: CreateIfNoActive failed: %w", err)
	}

	// Check if the INSERT succeeded (affected rows should be 1)
	if rowsAffected == 0 {
		// INSERT was blocked by WHERE NOT EXISTS (active notification exists, or already sent in this sprint)
		return nil, ErrConstraintViolation
	}

	// Fetch the created record
	rows, err := r.db.Query(ctx,
		`SELECT id, sprint_id, sent_by, message, sent_at, stopped_at, ended_at
		 FROM notifications
		 WHERE sprint_id = ? AND sent_by = ?
		 ORDER BY id DESC LIMIT 1`,
		[]interface{}{notif.SprintID, notif.SentBy},
	)
	if err != nil {
		return nil, fmt.Errorf("notification_repository: CreateIfNoActive fetch after insert failed: %w", err)
	}

	return scanNotification(rows[0])
}

// ListChallengeEndDue は締め切りに到達した通知を返す（③ challenge_end 対象）。
// 締め切りは sent_at + 1h、または発行者が途中中断した場合は ended_at（記録された時点で到達済み）。
// 既に challenge_end として送信済み（notification_deliveries に記録済み）の通知は除外する。
func (r *notificationRepository) ListChallengeEndDue(ctx context.Context) ([]model.Notification, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, sprint_id, sent_by, message, sent_at, stopped_at, ended_at
		 FROM notifications
		 WHERE (datetime(sent_at, '+1 hour') <= datetime('now') OR stopped_at IS NOT NULL OR ended_at IS NOT NULL)
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
		`SELECT id, sprint_id, sent_by, message, sent_at, stopped_at, ended_at
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

// GetActiveByGroup はグループ内で進行中（未中断 かつ sent_at + 1h > now()）の通知を返す。
// sprints.group_id で引くため現在のスプリントを経由しない（スプリント切替直後の取りこぼし防止）。
func (r *notificationRepository) GetActiveByGroup(ctx context.Context, groupID int64) (*model.Notification, error) {
	rows, err := r.db.Query(ctx,
		`SELECT n.id, n.sprint_id, n.sent_by, n.message, n.sent_at, n.stopped_at, n.ended_at
		 FROM notifications n
		 INNER JOIN sprints s ON s.id = n.sprint_id
		 WHERE s.group_id = ? AND n.stopped_at IS NULL AND n.ended_at IS NULL
		   AND datetime(n.sent_at, '+1 hour') > datetime('now')
		 ORDER BY n.sent_at DESC, n.id DESC
		 LIMIT 1`,
		[]interface{}{groupID},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("notification_repository: GetActiveByGroup failed: %w", err)
	}
	if len(rows) == 0 {
		return nil, ErrNotFound
	}

	return scanNotification(rows[0])
}

// EndIfActive は通知が進行中の場合のみ ended_at = now() を記録する（条件付き UPDATE で原子的）。
// 更新行が 0 なら false（既に中断済み・1時間経過済み・存在しない）。
func (r *notificationRepository) EndIfActive(ctx context.Context, notifID int64) (bool, error) {
	rowsAffected, err := r.db.Exec(ctx,
		`UPDATE notifications
		 SET ended_at = datetime('now')
		 WHERE id = ? AND stopped_at IS NULL AND ended_at IS NULL AND datetime(sent_at, '+1 hour') > datetime('now')`,
		[]interface{}{notifID},
	)
	if err != nil {
		return false, fmt.Errorf("notification_repository: EndIfActive failed: %w", err)
	}
	return rowsAffected > 0, nil
}

// GetByID は notifID で通知を取得する
func (r *notificationRepository) GetByID(ctx context.Context, notifID int64) (*model.Notification, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, sprint_id, sent_by, message, sent_at, stopped_at, ended_at
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

// GetActiveInSprint は送信から1時間以内の最新通知を返す。
func (r *notificationRepository) GetActiveInSprint(ctx context.Context, sprintID int64, now time.Time) (*model.Notification, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, sprint_id, sent_by, message, sent_at, stopped_at, ended_at
		 FROM notifications
		 WHERE sprint_id = ? AND stopped_at IS NULL AND ended_at IS NULL
		 AND datetime(sent_at, '+1 hour') > datetime(?)
		 ORDER BY sent_at DESC, id DESC LIMIT 1`,
		[]interface{}{sprintID, now.UTC().Format("2006-01-02 15:04:05")},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("notification_repository: GetActiveInSprint failed: %w", err)
	}
	return scanNotification(rows[0])
}

// Stop は通知発行者本人の通知を停止する。
func (r *notificationRepository) Stop(ctx context.Context, notifID, userID int64) error {
	rowsAffected, err := r.db.Exec(ctx,
		`UPDATE notifications
		 SET stopped_at = datetime('now')
		 WHERE id = ? AND sent_by = ? AND stopped_at IS NULL AND ended_at IS NULL`,
		[]interface{}{notifID, userID},
	)
	if err != nil {
		return fmt.Errorf("notification_repository: Stop failed: %w", err)
	}
	if rowsAffected == 0 {
		return ErrConstraintViolation
	}
	return nil
}
