package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/pkg/d1"
)

type NotificationDeliveryJobRepository interface {
	Create(ctx context.Context, eventKey string, channelID int64, eventType, payloadJSON string) (jobID int64, created bool, err error)
	GetByID(ctx context.Context, jobID int64) (*model.NotificationDeliveryJob, error)
	MarkAttempt(ctx context.Context, jobID int64, message string) error
	MarkDelivered(ctx context.Context, jobID int64) error
	MarkFailed(ctx context.Context, jobID int64, message string) error
}

type notificationDeliveryJobRepository struct{ db d1.Client }

func NewNotificationDeliveryJobRepository(db d1.Client) NotificationDeliveryJobRepository {
	return &notificationDeliveryJobRepository{db: db}
}

func (r *notificationDeliveryJobRepository) Create(ctx context.Context, eventKey string, channelID int64, eventType, payloadJSON string) (int64, bool, error) {
	_, err := r.db.Exec(ctx, `INSERT INTO notification_delivery_jobs
		(event_key, channel_id, event_type, payload_json) VALUES (?, ?, ?, ?)`, []interface{}{eventKey, channelID, eventType, payloadJSON})
	created := true
	if errors.Is(err, d1.ErrConstraintViolation) {
		created = false
	} else if err != nil {
		return 0, false, fmt.Errorf("notification_delivery_job_repository: create failed: %w", err)
	}
	rows, err := r.db.Query(ctx, `SELECT id FROM notification_delivery_jobs WHERE event_key = ? AND channel_id = ? LIMIT 1`, []interface{}{eventKey, channelID})
	if err != nil {
		return 0, false, fmt.Errorf("notification_delivery_job_repository: lookup failed: %w", err)
	}
	id, _ := rows[0]["id"].(float64)
	return int64(id), created, nil
}

func (r *notificationDeliveryJobRepository) GetByID(ctx context.Context, jobID int64) (*model.NotificationDeliveryJob, error) {
	rows, err := r.db.Query(ctx, `SELECT id, event_key, channel_id, event_type, payload_json, status, attempt_count, COALESCE(last_error, '') AS last_error
		FROM notification_delivery_jobs WHERE id = ? LIMIT 1`, []interface{}{jobID})
	if errors.Is(err, d1.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("notification_delivery_job_repository: get failed: %w", err)
	}
	row := rows[0]
	job := &model.NotificationDeliveryJob{}
	if v, ok := row["id"].(float64); ok {
		job.ID = int64(v)
	}
	if v, ok := row["event_key"].(string); ok {
		job.EventKey = v
	}
	if v, ok := row["channel_id"].(float64); ok {
		job.ChannelID = int64(v)
	}
	if v, ok := row["event_type"].(string); ok {
		job.EventType = v
	}
	if v, ok := row["payload_json"].(string); ok {
		job.PayloadJSON = v
	}
	if v, ok := row["status"].(string); ok {
		job.Status = v
	}
	if v, ok := row["attempt_count"].(float64); ok {
		job.AttemptCount = int(v)
	}
	if v, ok := row["last_error"].(string); ok {
		job.LastError = v
	}
	return job, nil
}

func (r *notificationDeliveryJobRepository) MarkAttempt(ctx context.Context, jobID int64, message string) error {
	_, err := r.db.Exec(ctx, `UPDATE notification_delivery_jobs SET attempt_count = attempt_count + 1, last_error = ? WHERE id = ?`, []interface{}{message, jobID})
	return err
}

func (r *notificationDeliveryJobRepository) MarkDelivered(ctx context.Context, jobID int64) error {
	_, err := r.db.Exec(ctx, `UPDATE notification_delivery_jobs SET status = 'delivered', delivered_at = datetime('now'), last_error = NULL WHERE id = ?`, []interface{}{jobID})
	return err
}

func (r *notificationDeliveryJobRepository) MarkFailed(ctx context.Context, jobID int64, message string) error {
	_, err := r.db.Exec(ctx, `UPDATE notification_delivery_jobs SET status = 'failed', last_error = ? WHERE id = ?`, []interface{}{message, jobID})
	return err
}
