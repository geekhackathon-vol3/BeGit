package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/pkg/d1"
)

// PostRepository は posts テーブルへのアクセスインターフェース
type PostRepository interface {
	Create(ctx context.Context, post *model.Post) (*model.Post, error)
	ListByGroupID(ctx context.Context, groupID int64) ([]model.Post, error)
	HasPostedInSprint(ctx context.Context, userID, sprintID int64) (bool, error)
	GetByUserAndNotification(ctx context.Context, userID, notifID int64) (*model.Post, error)
}

// postRepository は PostRepository インターフェースの実装
type postRepository struct {
	db d1.Client
}

// NewPostRepository は PostRepository を作成する
func NewPostRepository(db d1.Client) PostRepository {
	return &postRepository{db: db}
}

// scanPost は D1 クエリ結果を model.Post に変換する
func scanPost(row map[string]interface{}) (*model.Post, error) {
	p := &model.Post{}

	if v, ok := row["id"].(float64); ok {
		p.ID = int64(v)
	}
	if v, ok := row["notification_id"].(float64); ok {
		id := int64(v)
		p.NotificationID = &id
	}
	if v, ok := row["user_id"].(float64); ok {
		p.UserID = int64(v)
	}
	if v, ok := row["group_id"].(float64); ok {
		p.GroupID = int64(v)
	}
	if v, ok := row["post_type"].(string); ok {
		p.PostType = v
	}
	if v, ok := row["body"].(string); ok {
		p.Body = &v
	}
	if v, ok := row["repo_full_name"].(string); ok {
		p.RepoFullName = &v
	}
	if v, ok := row["branch_name"].(string); ok {
		p.BranchName = &v
	}
	if v, ok := row["commit_count"].(float64); ok {
		p.CommitCount = int(v)
	}
	if v, ok := row["additions"].(float64); ok {
		p.Additions = int(v)
	}
	if v, ok := row["deletions"].(float64); ok {
		p.Deletions = int(v)
	}
	if v, ok := row["latest_commit_message"].(string); ok {
		p.LatestCommitMessage = &v
	}
	if v, ok := row["status"].(string); ok {
		p.Status = &v
	}
	if v, ok := row["created_at"].(string); ok {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			t, _ = time.Parse("2006-01-02 15:04:05", v)
		}
		p.CreatedAt = t
	}

	return p, nil
}

// Create は posts テーブルにレコードを挿入する
func (r *postRepository) Create(ctx context.Context, post *model.Post) (*model.Post, error) {
	var notifID interface{} = nil
	if post.NotificationID != nil {
		notifID = *post.NotificationID
	}

	_, err := r.db.Exec(ctx,
		`INSERT INTO posts (notification_id, user_id, group_id, post_type, body, repo_full_name, commit_count, additions, deletions, latest_commit_message)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		[]interface{}{
			notifID, post.UserID, post.GroupID, post.PostType,
			post.Body, post.RepoFullName,
			post.CommitCount, post.Additions, post.Deletions,
			post.LatestCommitMessage,
		},
	)
	if err != nil {
		if errors.Is(err, d1.ErrConstraintViolation) {
			return nil, ErrConstraintViolation
		}
		return nil, fmt.Errorf("post_repository: Create failed: %w", err)
	}

	// 作成後に取得
	rows, err := r.db.Query(ctx,
		`SELECT id, notification_id, user_id, group_id, post_type, body, repo_full_name, branch_name, commit_count, additions, deletions, latest_commit_message, status, created_at
		 FROM posts WHERE user_id = ? AND group_id = ? ORDER BY id DESC LIMIT 1`,
		[]interface{}{post.UserID, post.GroupID},
	)
	if err != nil {
		return nil, fmt.Errorf("post_repository: Create fetch after insert failed: %w", err)
	}

	return scanPost(rows[0])
}

// ListByGroupID はグループのフィード一覧を投稿日時の降順で取得する
func (r *postRepository) ListByGroupID(ctx context.Context, groupID int64) ([]model.Post, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, notification_id, user_id, group_id, post_type, body, repo_full_name, branch_name, commit_count, additions, deletions, latest_commit_message, status, created_at
		 FROM posts WHERE group_id = ? ORDER BY created_at DESC`,
		[]interface{}{groupID},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return []model.Post{}, nil
		}
		return nil, fmt.Errorf("post_repository: ListByGroupID failed: %w", err)
	}

	posts := make([]model.Post, 0, len(rows))
	for _, row := range rows {
		p, err := scanPost(row)
		if err != nil {
			return nil, err
		}
		posts = append(posts, *p)
	}
	return posts, nil
}

// HasPostedInSprint は userID が sprintID のスプリントで投稿済みかどうかを確認する
func (r *postRepository) HasPostedInSprint(ctx context.Context, userID, sprintID int64) (bool, error) {
	rows, err := r.db.Query(ctx,
		`SELECT COUNT(*) as count
		 FROM posts p
		 INNER JOIN notifications n ON p.notification_id = n.id
		 WHERE p.user_id = ? AND n.sprint_id = ?`,
		[]interface{}{userID, sprintID},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("post_repository: HasPostedInSprint failed: %w", err)
	}

	if len(rows) == 0 {
		return false, nil
	}

	count, _ := rows[0]["count"].(float64)
	return count > 0, nil
}

// GetByUserAndNotification はユーザーと通知 ID で投稿を取得する
func (r *postRepository) GetByUserAndNotification(ctx context.Context, userID, notifID int64) (*model.Post, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, notification_id, user_id, group_id, post_type, body, repo_full_name, branch_name, commit_count, additions, deletions, latest_commit_message, status, created_at
		 FROM posts WHERE user_id = ? AND notification_id = ? LIMIT 1`,
		[]interface{}{userID, notifID},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("post_repository: GetByUserAndNotification failed: %w", err)
	}

	return scanPost(rows[0])
}
