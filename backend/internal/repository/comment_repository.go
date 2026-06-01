package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/pkg/d1"
)

// CommentRepository は comments テーブルへのアクセスインターフェース
type CommentRepository interface {
	// Create はコメントを作成し、users JOIN 付きで返す。
	Create(ctx context.Context, postID, userID int64, body string) (*model.Comment, error)
	// ListByPostID は投稿のコメント一覧を created_at 昇順・users JOIN 付きで取得する。
	ListByPostID(ctx context.Context, postID int64) ([]model.Comment, error)
	// GetByID は commentID でコメントを取得する。
	GetByID(ctx context.Context, commentID int64) (*model.Comment, error)
	// Delete は commentID のコメントを削除する。
	Delete(ctx context.Context, commentID int64) error
}

// commentRepository は CommentRepository インターフェースの実装
type commentRepository struct {
	db d1.Client
}

// NewCommentRepository は CommentRepository を作成する
func NewCommentRepository(db d1.Client) CommentRepository {
	return &commentRepository{db: db}
}

// scanComment は D1 クエリ結果を model.Comment に変換する
func scanComment(row map[string]interface{}) model.Comment {
	c := model.Comment{}
	if v, ok := row["id"].(float64); ok {
		c.ID = int64(v)
	}
	if v, ok := row["post_id"].(float64); ok {
		c.PostID = int64(v)
	}
	if v, ok := row["user_id"].(float64); ok {
		c.UserID = int64(v)
	}
	if v, ok := row["body"].(string); ok {
		c.Body = v
	}
	if v, ok := row["created_at"].(string); ok {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			t, _ = time.Parse("2006-01-02 15:04:05", v)
		}
		c.CreatedAt = t
	}
	if v, ok := row["login"].(string); ok {
		c.Login = v
	}
	if v, ok := row["avatar_url"].(string); ok {
		c.AvatarURL = v
	}
	return c
}

// Create はコメントを作成し、作成後に users JOIN 付きで取得して返す。
func (r *commentRepository) Create(ctx context.Context, postID, userID int64, body string) (*model.Comment, error) {
	_, err := r.db.Exec(ctx,
		`INSERT INTO comments (post_id, user_id, body) VALUES (?, ?, ?)`,
		[]interface{}{postID, userID, body},
	)
	if err != nil {
		if errors.Is(err, d1.ErrConstraintViolation) {
			return nil, ErrConstraintViolation
		}
		return nil, fmt.Errorf("comment_repository: Create failed: %w", err)
	}

	rows, err := r.db.Query(ctx,
		`SELECT c.id, c.post_id, c.user_id, c.body, c.created_at, u.github_login as login, u.avatar_url
		 FROM comments c
		 INNER JOIN users u ON c.user_id = u.id
		 WHERE c.post_id = ? AND c.user_id = ? ORDER BY c.id DESC LIMIT 1`,
		[]interface{}{postID, userID},
	)
	if err != nil {
		return nil, fmt.Errorf("comment_repository: Create fetch after insert failed: %w", err)
	}
	if len(rows) == 0 {
		return nil, ErrNotFound
	}
	comment := scanComment(rows[0])
	return &comment, nil
}

// ListByPostID は投稿のコメント一覧を created_at 昇順で取得する。
func (r *commentRepository) ListByPostID(ctx context.Context, postID int64) ([]model.Comment, error) {
	rows, err := r.db.Query(ctx,
		`SELECT c.id, c.post_id, c.user_id, c.body, c.created_at, u.github_login as login, u.avatar_url
		 FROM comments c
		 INNER JOIN users u ON c.user_id = u.id
		 WHERE c.post_id = ?
		 ORDER BY c.created_at ASC, c.id ASC`,
		[]interface{}{postID},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return []model.Comment{}, nil
		}
		return nil, fmt.Errorf("comment_repository: ListByPostID failed: %w", err)
	}

	comments := make([]model.Comment, 0, len(rows))
	for _, row := range rows {
		comments = append(comments, scanComment(row))
	}
	return comments, nil
}

// GetByID は commentID でコメントを取得する。
func (r *commentRepository) GetByID(ctx context.Context, commentID int64) (*model.Comment, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, post_id, user_id, body, created_at FROM comments WHERE id = ? LIMIT 1`,
		[]interface{}{commentID},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("comment_repository: GetByID failed: %w", err)
	}
	if len(rows) == 0 {
		return nil, ErrNotFound
	}
	comment := scanComment(rows[0])
	return &comment, nil
}

// Delete は commentID のコメントを削除する。
func (r *commentRepository) Delete(ctx context.Context, commentID int64) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM comments WHERE id = ?`,
		[]interface{}{commentID},
	)
	if err != nil {
		return fmt.Errorf("comment_repository: Delete failed: %w", err)
	}
	return nil
}
