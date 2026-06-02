package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/pkg/d1"
)

// PhotoRepository は photos テーブルへのアクセスインターフェース
type PhotoRepository interface {
	Create(ctx context.Context, photo *model.Photo) (*model.Photo, error)
	ListByPostID(ctx context.Context, postID int64) ([]model.Photo, error)
	// ListByPostIDs は複数投稿の写真をまとめて取得し post_id ごとにグルーピングする（フィードの N+1 回避）
	ListByPostIDs(ctx context.Context, postIDs []int64) (map[int64][]model.Photo, error)
	Delete(ctx context.Context, photoID int64) error
}

// photoRepository は PhotoRepository インターフェースの実装
type photoRepository struct {
	db d1.Client
}

// NewPhotoRepository は PhotoRepository を作成する
func NewPhotoRepository(db d1.Client) PhotoRepository {
	return &photoRepository{db: db}
}

// scanPhoto は D1 クエリ結果を model.Photo に変換する
func scanPhoto(row map[string]interface{}) *model.Photo {
	p := &model.Photo{}
	if v, ok := row["id"].(float64); ok {
		p.ID = int64(v)
	}
	if v, ok := row["post_id"].(float64); ok {
		p.PostID = int64(v)
	}
	if v, ok := row["r2_key"].(string); ok {
		p.R2Key = v
	}
	if v, ok := row["photo_type"].(string); ok {
		p.PhotoType = v
	}
	if v, ok := row["created_at"].(string); ok {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			t, _ = time.Parse("2006-01-02 15:04:05", v)
		}
		p.CreatedAt = t
	}
	return p
}

// Create は photos テーブルにレコードを挿入し、作成後の行を返す
func (r *photoRepository) Create(ctx context.Context, photo *model.Photo) (*model.Photo, error) {
	rows, err := r.db.Query(ctx,
		`INSERT INTO photos (post_id, r2_key, photo_type)
		 VALUES (?, ?, ?)
		 RETURNING id, post_id, r2_key, photo_type, created_at`,
		[]interface{}{photo.PostID, photo.R2Key, photo.PhotoType},
	)
	if err != nil {
		if errors.Is(err, d1.ErrConstraintViolation) {
			return nil, ErrConstraintViolation
		}
		return nil, fmt.Errorf("photo_repository: Create failed: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("photo_repository: Create returned no rows")
	}
	return scanPhoto(rows[0]), nil
}

// ListByPostID は指定投稿の写真を id 昇順で取得する
func (r *photoRepository) ListByPostID(ctx context.Context, postID int64) ([]model.Photo, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, post_id, r2_key, photo_type, created_at
		 FROM photos WHERE post_id = ? ORDER BY id ASC`,
		[]interface{}{postID},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return []model.Photo{}, nil
		}
		return nil, fmt.Errorf("photo_repository: ListByPostID failed: %w", err)
	}

	photos := make([]model.Photo, 0, len(rows))
	for _, row := range rows {
		photos = append(photos, *scanPhoto(row))
	}
	return photos, nil
}

// ListByPostIDs は複数投稿の写真をまとめて取得し post_id でグルーピングして返す
func (r *photoRepository) ListByPostIDs(ctx context.Context, postIDs []int64) (map[int64][]model.Photo, error) {
	result := make(map[int64][]model.Photo)
	if len(postIDs) == 0 {
		return result, nil
	}

	placeholders := make([]string, len(postIDs))
	params := make([]interface{}, len(postIDs))
	for i, id := range postIDs {
		placeholders[i] = "?"
		params[i] = id
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, post_id, r2_key, photo_type, created_at
		 FROM photos WHERE post_id IN (`+strings.Join(placeholders, ", ")+`) ORDER BY id ASC`,
		params,
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return result, nil
		}
		return nil, fmt.Errorf("photo_repository: ListByPostIDs failed: %w", err)
	}

	for _, row := range rows {
		p := scanPhoto(row)
		result[p.PostID] = append(result[p.PostID], *p)
	}
	return result, nil
}

// Delete は指定された photo_id のレコードを削除する
func (r *photoRepository) Delete(ctx context.Context, photoID int64) error {
	_, err := r.db.Query(ctx,
		`DELETE FROM photos WHERE id = ?`,
		[]interface{}{photoID},
	)
	if err != nil {
		return fmt.Errorf("photo_repository: Delete failed: %w", err)
	}
	return nil
}
