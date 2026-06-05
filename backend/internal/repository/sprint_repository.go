package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/pkg/d1"
)

// SprintRepository は sprints テーブルへのアクセスインターフェース
type SprintRepository interface {
	// GetOrCreateCurrentSprint は現在アクティブなスプリントを取得するか、存在しなければ作成する
	GetOrCreateCurrentSprint(ctx context.Context, groupID int64, durationDays int) (*model.Sprint, error)
	// GetCurrentSprint は現在アクティブなスプリントを取得する。なければ ErrNotFound を返す
	GetCurrentSprint(ctx context.Context, groupID int64) (*model.Sprint, error)
	// GetByID は sprint ID でスプリントを取得する
	GetByID(ctx context.Context, sprintID int64) (*model.Sprint, error)
	// ListReminderDue は ends_at の3日前に到達した（かつ未終了の）スプリントを返す（④ sprint_reminder 用）。
	ListReminderDue(ctx context.Context) ([]model.Sprint, error)
	// ListEnded は ends_at に到達した（終了済み）スプリントを返す（⑤ sprint_end 用）。
	ListEnded(ctx context.Context) ([]model.Sprint, error)
	// ListActive は現在アクティブなスプリントを返す（⑥ sprint_start 候補。冪等は deliveries で担保）。
	ListActive(ctx context.Context) ([]model.Sprint, error)
}

// sprintRepository は SprintRepository インターフェースの実装
type sprintRepository struct {
	db d1.Client
}

// NewSprintRepository は SprintRepository を作成する
func NewSprintRepository(db d1.Client) SprintRepository {
	return &sprintRepository{db: db}
}

// scanSprint は D1 クエリ結果を model.Sprint に変換する
func scanSprint(row map[string]interface{}) (*model.Sprint, error) {
	sprint := &model.Sprint{}

	if v, ok := row["id"].(float64); ok {
		sprint.ID = int64(v)
	}
	if v, ok := row["group_id"].(float64); ok {
		sprint.GroupID = int64(v)
	}
	if v, ok := row["index_num"].(float64); ok {
		sprint.IndexNum = int(v)
	}
	if v, ok := row["started_at"].(string); ok {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			t, _ = time.Parse("2006-01-02 15:04:05", v)
		}
		sprint.StartedAt = t
	}
	if v, ok := row["ends_at"].(string); ok {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			t, _ = time.Parse("2006-01-02 15:04:05", v)
		}
		sprint.EndsAt = t
	}

	return sprint, nil
}

// GetCurrentSprint は現在アクティブなスプリントを取得する
func (r *sprintRepository) GetCurrentSprint(ctx context.Context, groupID int64) (*model.Sprint, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, group_id, index_num, started_at, ends_at
		 FROM sprints
		 WHERE group_id = ? AND started_at <= datetime('now') AND ends_at > datetime('now')
		 LIMIT 1`,
		[]interface{}{groupID},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("sprint_repository: GetCurrentSprint failed: %w", err)
	}

	return scanSprint(rows[0])
}

// GetByID は sprint ID でスプリントを取得する
func (r *sprintRepository) GetByID(ctx context.Context, sprintID int64) (*model.Sprint, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, group_id, index_num, started_at, ends_at
		 FROM sprints
		 WHERE id = ?
		 LIMIT 1`,
		[]interface{}{sprintID},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("sprint_repository: GetByID failed: %w", err)
	}

	return scanSprint(rows[0])
}

// scanSprints は複数行をスライスへ変換する（Cron 走査クエリ共通）
func scanSprints(rows []map[string]interface{}) ([]model.Sprint, error) {
	sprints := make([]model.Sprint, 0, len(rows))
	for _, row := range rows {
		s, err := scanSprint(row)
		if err != nil {
			return nil, err
		}
		sprints = append(sprints, *s)
	}
	return sprints, nil
}

// ListReminderDue は ends_at の3日前に到達した（かつ未終了の）スプリントを返す。
// 境界: datetime(ends_at, '-3 days') <= now() AND ends_at > now()
func (r *sprintRepository) ListReminderDue(ctx context.Context) ([]model.Sprint, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, group_id, index_num, started_at, ends_at
		 FROM sprints
		 WHERE datetime(ends_at, '-3 days') <= datetime('now') AND ends_at > datetime('now')`,
		[]interface{}{},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return []model.Sprint{}, nil
		}
		return nil, fmt.Errorf("sprint_repository: ListReminderDue failed: %w", err)
	}
	return scanSprints(rows)
}

// ListEnded は ends_at に到達した（終了済み）スプリントを返す。
// 境界: ends_at <= now()
func (r *sprintRepository) ListEnded(ctx context.Context) ([]model.Sprint, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, group_id, index_num, started_at, ends_at
		 FROM sprints
		 WHERE ends_at <= datetime('now')`,
		[]interface{}{},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return []model.Sprint{}, nil
		}
		return nil, fmt.Errorf("sprint_repository: ListEnded failed: %w", err)
	}
	return scanSprints(rows)
}

// ListActive は現在アクティブなスプリントを返す（⑥ sprint_start 候補）。
// 境界: started_at <= now() AND ends_at > now()
func (r *sprintRepository) ListActive(ctx context.Context) ([]model.Sprint, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, group_id, index_num, started_at, ends_at
		 FROM sprints
		 WHERE started_at <= datetime('now') AND ends_at > datetime('now')`,
		[]interface{}{},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return []model.Sprint{}, nil
		}
		return nil, fmt.Errorf("sprint_repository: ListActive failed: %w", err)
	}
	return scanSprints(rows)
}

// GetOrCreateCurrentSprint は現在アクティブなスプリントを取得するか、存在しなければ作成する
func (r *sprintRepository) GetOrCreateCurrentSprint(ctx context.Context, groupID int64, durationDays int) (*model.Sprint, error) {
	// まず既存のアクティブスプリントを検索
	sprint, err := r.GetCurrentSprint(ctx, groupID)
	if err == nil {
		return sprint, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	// スプリントが存在しない場合は新規作成
	now := time.Now().UTC()
	endsAt := now.AddDate(0, 0, durationDays)

	_, err = r.db.Exec(ctx,
		`INSERT INTO sprints (group_id, started_at, ends_at)
		 VALUES (?, datetime(?), datetime(?))`,
		[]interface{}{groupID, now.Format("2006-01-02 15:04:05"), endsAt.Format("2006-01-02 15:04:05")},
	)
	if err != nil {
		// UNIQUE 制約違反（並行作成）の場合は既存を取得
		if errors.Is(err, d1.ErrConstraintViolation) {
			return r.GetCurrentSprint(ctx, groupID)
		}
		return nil, fmt.Errorf("sprint_repository: GetOrCreateCurrentSprint insert failed: %w", err)
	}

	// 作成後に取得
	return r.GetCurrentSprint(ctx, groupID)
}
