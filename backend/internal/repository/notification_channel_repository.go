package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/pkg/d1"
)

type NotificationChannelRepository interface {
	ListByGroup(ctx context.Context, groupID int64) ([]model.NotificationChannel, error)
	ListEnabledForEvent(ctx context.Context, groupID int64, eventType string) ([]model.NotificationChannel, error)
	GetByID(ctx context.Context, channelID int64) (*model.NotificationChannel, error)
	Create(ctx context.Context, channel *model.NotificationChannel) (*model.NotificationChannel, error)
	Update(ctx context.Context, channel *model.NotificationChannel) (*model.NotificationChannel, error)
	Delete(ctx context.Context, channelID int64) error
}

type notificationChannelRepository struct{ db d1.Client }

func NewNotificationChannelRepository(db d1.Client) NotificationChannelRepository {
	return &notificationChannelRepository{db: db}
}

func scanNotificationChannel(row map[string]interface{}) model.NotificationChannel {
	c := model.NotificationChannel{}
	if v, ok := row["id"].(float64); ok {
		c.ID = int64(v)
	}
	if v, ok := row["group_id"].(float64); ok {
		c.GroupID = int64(v)
	}
	if v, ok := row["platform"].(string); ok {
		c.Platform = v
	}
	if v, ok := row["display_name"].(string); ok {
		c.DisplayName = v
	}
	if v, ok := row["encrypted_webhook_url"].(string); ok {
		c.EncryptedWebhookURL = v
	}
	if v, ok := row["enabled"].(float64); ok {
		c.Enabled = v != 0
	}
	if v, ok := row["created_by"].(float64); ok {
		c.CreatedBy = int64(v)
	}
	if v, ok := row["event_type"].(string); ok && v != "" {
		c.EventTypes = []string{v}
	}
	if v, ok := row["created_at"].(string); ok {
		c.CreatedAt = parseSQLiteTime(v)
	}
	if v, ok := row["updated_at"].(string); ok {
		c.UpdatedAt = parseSQLiteTime(v)
	}
	return c
}

func parseSQLiteTime(value string) time.Time {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t
	}
	t, _ := time.Parse("2006-01-02 15:04:05", value)
	return t.UTC()
}

const notificationChannelSelect = `
	SELECT c.id, c.group_id, c.platform, c.display_name, c.encrypted_webhook_url,
	       c.enabled, c.created_by, c.created_at, c.updated_at, s.event_type
	FROM notification_channels c
	LEFT JOIN notification_channel_subscriptions s ON s.channel_id = c.id`

func channelsFromRows(rows []map[string]interface{}) []model.NotificationChannel {
	ordered := make([]model.NotificationChannel, 0)
	positions := make(map[int64]int)
	for _, row := range rows {
		item := scanNotificationChannel(row)
		if idx, exists := positions[item.ID]; exists {
			if len(item.EventTypes) > 0 {
				ordered[idx].EventTypes = append(ordered[idx].EventTypes, item.EventTypes[0])
			}
			continue
		}
		positions[item.ID] = len(ordered)
		ordered = append(ordered, item)
	}
	return ordered
}

func (r *notificationChannelRepository) ListByGroup(ctx context.Context, groupID int64) ([]model.NotificationChannel, error) {
	rows, err := r.db.Query(ctx, notificationChannelSelect+` WHERE c.group_id = ? ORDER BY c.created_at, c.id, s.event_type`, []interface{}{groupID})
	if errors.Is(err, d1.ErrNotFound) {
		return []model.NotificationChannel{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("notification_channel_repository: list failed: %w", err)
	}
	return channelsFromRows(rows), nil
}

func (r *notificationChannelRepository) ListEnabledForEvent(ctx context.Context, groupID int64, eventType string) ([]model.NotificationChannel, error) {
	rows, err := r.db.Query(ctx, notificationChannelSelect+`
		WHERE c.group_id = ? AND c.enabled = 1
		  AND EXISTS (SELECT 1 FROM notification_channel_subscriptions x WHERE x.channel_id = c.id AND x.event_type = ?)
		ORDER BY c.id, s.event_type`, []interface{}{groupID, eventType})
	if errors.Is(err, d1.ErrNotFound) {
		return []model.NotificationChannel{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("notification_channel_repository: list enabled failed: %w", err)
	}
	return channelsFromRows(rows), nil
}

func (r *notificationChannelRepository) GetByID(ctx context.Context, channelID int64) (*model.NotificationChannel, error) {
	rows, err := r.db.Query(ctx, notificationChannelSelect+` WHERE c.id = ? ORDER BY s.event_type`, []interface{}{channelID})
	if errors.Is(err, d1.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("notification_channel_repository: get failed: %w", err)
	}
	items := channelsFromRows(rows)
	if len(items) == 0 {
		return nil, ErrNotFound
	}
	return &items[0], nil
}

func (r *notificationChannelRepository) Create(ctx context.Context, channel *model.NotificationChannel) (*model.NotificationChannel, error) {
	_, err := r.db.Exec(ctx, `INSERT INTO notification_channels
		(group_id, platform, display_name, encrypted_webhook_url, enabled, created_by)
		VALUES (?, ?, ?, ?, ?, ?)`, []interface{}{channel.GroupID, channel.Platform, channel.DisplayName, channel.EncryptedWebhookURL, boolToSQLite(channel.Enabled), channel.CreatedBy})
	if errors.Is(err, d1.ErrConstraintViolation) {
		return nil, ErrConflict
	}
	if err != nil {
		return nil, fmt.Errorf("notification_channel_repository: create failed: %w", err)
	}

	rows, err := r.db.Query(ctx, `SELECT id FROM notification_channels
		WHERE group_id = ? AND platform = ? AND display_name = ? LIMIT 1`, []interface{}{channel.GroupID, channel.Platform, channel.DisplayName})
	if err != nil {
		return nil, fmt.Errorf("notification_channel_repository: lookup created channel failed: %w", err)
	}
	id, _ := rows[0]["id"].(float64)
	channel.ID = int64(id)
	if err := r.replaceSubscriptions(ctx, channel.ID, channel.EventTypes); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, channel.ID)
}

func (r *notificationChannelRepository) Update(ctx context.Context, channel *model.NotificationChannel) (*model.NotificationChannel, error) {
	rows, err := r.db.Exec(ctx, `UPDATE notification_channels
		SET display_name = ?, encrypted_webhook_url = ?, enabled = ?, updated_at = datetime('now')
		WHERE id = ? AND group_id = ?`, []interface{}{channel.DisplayName, channel.EncryptedWebhookURL, boolToSQLite(channel.Enabled), channel.ID, channel.GroupID})
	if errors.Is(err, d1.ErrConstraintViolation) {
		return nil, ErrConflict
	}
	if err != nil {
		return nil, fmt.Errorf("notification_channel_repository: update failed: %w", err)
	}
	if rows == 0 {
		return nil, ErrNotFound
	}
	if err := r.replaceSubscriptions(ctx, channel.ID, channel.EventTypes); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, channel.ID)
}

func (r *notificationChannelRepository) replaceSubscriptions(ctx context.Context, channelID int64, eventTypes []string) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM notification_channel_subscriptions WHERE channel_id = ?`, []interface{}{channelID}); err != nil {
		return fmt.Errorf("notification_channel_repository: delete subscriptions failed: %w", err)
	}
	for _, eventType := range eventTypes {
		if _, err := r.db.Exec(ctx, `INSERT INTO notification_channel_subscriptions (channel_id, event_type) VALUES (?, ?)`, []interface{}{channelID, eventType}); err != nil {
			return fmt.Errorf("notification_channel_repository: insert subscription failed: %w", err)
		}
	}
	return nil
}

func (r *notificationChannelRepository) Delete(ctx context.Context, channelID int64) error {
	rows, err := r.db.Exec(ctx, `DELETE FROM notification_channels WHERE id = ?`, []interface{}{channelID})
	if err != nil {
		return fmt.Errorf("notification_channel_repository: delete failed: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
