package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/pkg/d1"
)

// ReactionRepository は reactions テーブルへのアクセスインターフェース
type ReactionRepository interface {
	// Add はリアクションを追加する。UNIQUE(post_id, user_id, reaction_type) により冪等。
	Add(ctx context.Context, postID, userID int64, reactionType string) error
	// Remove はリアクションを削除する（トグル用）。
	Remove(ctx context.Context, postID, userID int64, reactionType string) error
	// ListByPostID は投稿のリアクション一覧を users JOIN 付きで取得する。
	ListByPostID(ctx context.Context, postID int64) ([]model.Reaction, error)
}

// reactionRepository は ReactionRepository インターフェースの実装
type reactionRepository struct {
	db d1.Client
}

// NewReactionRepository は ReactionRepository を作成する
func NewReactionRepository(db d1.Client) ReactionRepository {
	return &reactionRepository{db: db}
}

// scanReaction は D1 クエリ結果を model.Reaction に変換する
func scanReaction(row map[string]interface{}) model.Reaction {
	r := model.Reaction{}
	if v, ok := row["id"].(float64); ok {
		r.ID = int64(v)
	}
	if v, ok := row["post_id"].(float64); ok {
		r.PostID = int64(v)
	}
	if v, ok := row["user_id"].(float64); ok {
		r.UserID = int64(v)
	}
	if v, ok := row["reaction_type"].(string); ok {
		r.ReactionType = v
	}
	if v, ok := row["login"].(string); ok {
		r.Login = v
	}
	if v, ok := row["avatar_url"].(string); ok {
		r.AvatarURL = v
	}
	return r
}

// Add はリアクションを追加する。INSERT OR IGNORE で UNIQUE 制約を冪等に扱う。
func (r *reactionRepository) Add(ctx context.Context, postID, userID int64, reactionType string) error {
	_, err := r.db.Exec(ctx,
		`INSERT OR IGNORE INTO reactions (post_id, user_id, reaction_type) VALUES (?, ?, ?)`,
		[]interface{}{postID, userID, reactionType},
	)
	if err != nil {
		return fmt.Errorf("reaction_repository: Add failed: %w", err)
	}
	return nil
}

// Remove はリアクションを削除する。
func (r *reactionRepository) Remove(ctx context.Context, postID, userID int64, reactionType string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM reactions WHERE post_id = ? AND user_id = ? AND reaction_type = ?`,
		[]interface{}{postID, userID, reactionType},
	)
	if err != nil {
		return fmt.Errorf("reaction_repository: Remove failed: %w", err)
	}
	return nil
}

// ListByPostID は投稿のリアクション一覧を取得する。
func (r *reactionRepository) ListByPostID(ctx context.Context, postID int64) ([]model.Reaction, error) {
	rows, err := r.db.Query(ctx,
		`SELECT rx.id, rx.post_id, rx.user_id, rx.reaction_type, u.github_login as login, u.avatar_url
		 FROM reactions rx
		 INNER JOIN users u ON rx.user_id = u.id
		 WHERE rx.post_id = ?
		 ORDER BY rx.id ASC`,
		[]interface{}{postID},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return []model.Reaction{}, nil
		}
		return nil, fmt.Errorf("reaction_repository: ListByPostID failed: %w", err)
	}

	reactions := make([]model.Reaction, 0, len(rows))
	for _, row := range rows {
		reactions = append(reactions, scanReaction(row))
	}
	return reactions, nil
}
