package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/pkg/d1"
)

type RepositoryChangeRepository interface {
	ListAfter(ctx context.Context, groupID, after int64, limit int) ([]model.RepositoryChange, int64, bool, error)
}

type repositoryChangeRepository struct{ db d1.Client }

func NewRepositoryChangeRepository(db d1.Client) RepositoryChangeRepository {
	return &repositoryChangeRepository{db: db}
}

func (r *repositoryChangeRepository) ListAfter(ctx context.Context, groupID, after int64, limit int) ([]model.RepositoryChange, int64, bool, error) {
	if limit < 1 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	bounds, err := r.db.Query(ctx,
		`SELECT COALESCE(MIN(sequence), 0) AS min_sequence, COALESCE(MAX(sequence), 0) AS max_sequence
		 FROM repository_changes WHERE group_id = ?`, []interface{}{groupID})
	if err != nil && !errors.Is(err, d1.ErrNotFound) {
		return nil, after, false, fmt.Errorf("repository_change_repository: bounds failed: %w", err)
	}
	var minSequence, maxSequence int64
	if len(bounds) > 0 {
		if value, ok := bounds[0]["min_sequence"].(float64); ok {
			minSequence = int64(value)
		}
		if value, ok := bounds[0]["max_sequence"].(float64); ok {
			maxSequence = int64(value)
		}
	}
	resetRequired := after > maxSequence || (after > 0 && minSequence > 0 && after < minSequence-1)
	if resetRequired {
		return []model.RepositoryChange{}, maxSequence, true, nil
	}

	rows, err := r.db.Query(ctx,
		`SELECT sequence, group_id, entity_type, entity_id, post_id, operation, changed_at
		 FROM repository_changes WHERE group_id = ? AND sequence > ? ORDER BY sequence ASC LIMIT ?`,
		[]interface{}{groupID, after, limit})
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return []model.RepositoryChange{}, maxSequence, false, nil
		}
		return nil, after, false, fmt.Errorf("repository_change_repository: list failed: %w", err)
	}
	changes := make([]model.RepositoryChange, 0, len(rows))
	for _, row := range rows {
		change := model.RepositoryChange{}
		if value, ok := row["sequence"].(float64); ok {
			change.Sequence = int64(value)
		}
		if value, ok := row["group_id"].(float64); ok {
			change.GroupID = int64(value)
		}
		if value, ok := row["entity_type"].(string); ok {
			change.EntityType = value
		}
		if value, ok := row["entity_id"].(float64); ok {
			change.EntityID = int64(value)
		}
		if value, ok := row["post_id"].(float64); ok {
			id := int64(value)
			change.PostID = &id
		}
		if value, ok := row["operation"].(string); ok {
			change.Operation = value
		}
		if value, ok := row["changed_at"].(string); ok {
			change.ChangedAt, _ = time.Parse(time.RFC3339, value)
			if change.ChangedAt.IsZero() {
				change.ChangedAt, _ = time.Parse("2006-01-02 15:04:05", value)
			}
		}
		changes = append(changes, change)
	}
	cursor := after
	if len(changes) > 0 {
		cursor = changes[len(changes)-1].Sequence
	}
	return changes, cursor, false, nil
}
