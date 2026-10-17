package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/pkg/d1"
)

type BeGitTimeScheduleRepository interface {
	GetByGroupID(ctx context.Context, groupID int64) (*model.BeGitTimeSchedule, error)
	Upsert(ctx context.Context, schedule *model.BeGitTimeSchedule) (*model.BeGitTimeSchedule, error)
	ListDue(ctx context.Context, now time.Time) ([]model.BeGitTimeSchedule, error)
	Advance(ctx context.Context, groupID int64, previous, next time.Time) (bool, error)
}

type beGitTimeScheduleRepository struct{ db d1.Client }

func NewBeGitTimeScheduleRepository(db d1.Client) BeGitTimeScheduleRepository {
	return &beGitTimeScheduleRepository{db: db}
}

func parseScheduleTime(value interface{}) *time.Time {
	raw, ok := value.(string)
	if !ok || raw == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			utc := parsed.UTC()
			return &utc
		}
	}
	return nil
}

func parseWeekdays(raw string) []int {
	values := make([]int, 0, 7)
	for _, item := range strings.Split(raw, ",") {
		value, err := strconv.Atoi(strings.TrimSpace(item))
		if err == nil && value >= 1 && value <= 7 {
			values = append(values, value)
		}
	}
	return values
}

func formatWeekdays(values []int) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ",")
}

func scanBeGitTimeSchedule(row map[string]interface{}) *model.BeGitTimeSchedule {
	schedule := &model.BeGitTimeSchedule{}
	if value, ok := row["group_id"].(float64); ok {
		schedule.GroupID = int64(value)
	}
	if value, ok := row["enabled"].(float64); ok {
		schedule.Enabled = value != 0
	}
	if value, ok := row["local_time"].(string); ok {
		schedule.LocalTime = value
	}
	if value, ok := row["timezone"].(string); ok {
		schedule.Timezone = value
	}
	if value, ok := row["weekdays"].(string); ok {
		schedule.Weekdays = parseWeekdays(value)
	}
	schedule.NextRunAt = parseScheduleTime(row["next_run_at"])
	if value, ok := row["updated_by"].(float64); ok {
		schedule.UpdatedBy = int64(value)
	}
	if value := parseScheduleTime(row["created_at"]); value != nil {
		schedule.CreatedAt = *value
	}
	if value := parseScheduleTime(row["updated_at"]); value != nil {
		schedule.UpdatedAt = *value
	}
	return schedule
}

const beGitTimeScheduleSelect = `SELECT group_id, enabled, local_time, timezone, weekdays,
       next_run_at, updated_by, created_at, updated_at FROM begit_time_schedules`

func (r *beGitTimeScheduleRepository) GetByGroupID(ctx context.Context, groupID int64) (*model.BeGitTimeSchedule, error) {
	rows, err := r.db.Query(ctx, beGitTimeScheduleSelect+` WHERE group_id = ? LIMIT 1`, []interface{}{groupID})
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("begit_time_schedule_repository: get failed: %w", err)
	}
	if len(rows) == 0 {
		return nil, ErrNotFound
	}
	return scanBeGitTimeSchedule(rows[0]), nil
}

func (r *beGitTimeScheduleRepository) Upsert(ctx context.Context, schedule *model.BeGitTimeSchedule) (*model.BeGitTimeSchedule, error) {
	var next interface{}
	if schedule.NextRunAt != nil {
		next = schedule.NextRunAt.UTC().Format("2006-01-02 15:04:05")
	}
	_, err := r.db.Exec(ctx, `INSERT INTO begit_time_schedules
       (group_id, enabled, local_time, timezone, weekdays, next_run_at, updated_by)
       VALUES (?, ?, ?, ?, ?, ?, ?)
       ON CONFLICT(group_id) DO UPDATE SET
         enabled = excluded.enabled, local_time = excluded.local_time,
         timezone = excluded.timezone, weekdays = excluded.weekdays,
         next_run_at = excluded.next_run_at, updated_by = excluded.updated_by,
         updated_at = datetime('now')`, []interface{}{
		schedule.GroupID, boolToSQLite(schedule.Enabled), schedule.LocalTime, schedule.Timezone,
		formatWeekdays(schedule.Weekdays), next, schedule.UpdatedBy,
	})
	if err != nil {
		return nil, fmt.Errorf("begit_time_schedule_repository: upsert failed: %w", err)
	}
	return r.GetByGroupID(ctx, schedule.GroupID)
}

func (r *beGitTimeScheduleRepository) ListDue(ctx context.Context, now time.Time) ([]model.BeGitTimeSchedule, error) {
	rows, err := r.db.Query(ctx, beGitTimeScheduleSelect+`
       WHERE enabled = 1 AND next_run_at IS NOT NULL AND next_run_at <= datetime(?)
       ORDER BY next_run_at`, []interface{}{now.UTC().Format("2006-01-02 15:04:05")})
	if errors.Is(err, d1.ErrNotFound) {
		return []model.BeGitTimeSchedule{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("begit_time_schedule_repository: list due failed: %w", err)
	}
	result := make([]model.BeGitTimeSchedule, 0, len(rows))
	for _, row := range rows {
		result = append(result, *scanBeGitTimeSchedule(row))
	}
	return result, nil
}

func (r *beGitTimeScheduleRepository) Advance(ctx context.Context, groupID int64, previous, next time.Time) (bool, error) {
	affected, err := r.db.Exec(ctx, `UPDATE begit_time_schedules
       SET next_run_at = datetime(?), updated_at = datetime('now')
       WHERE group_id = ? AND enabled = 1 AND next_run_at = datetime(?)`, []interface{}{
		next.UTC().Format("2006-01-02 15:04:05"), groupID, previous.UTC().Format("2006-01-02 15:04:05"),
	})
	if err != nil {
		return false, fmt.Errorf("begit_time_schedule_repository: advance failed: %w", err)
	}
	return affected > 0, nil
}
