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
