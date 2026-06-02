package repository

import (
	"context"
	"testing"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/pkg/d1"
)

// TestPhotoRepository_Create は INSERT 後に作成された写真行を返すことを確認する
func TestPhotoRepository_Create(t *testing.T) {
	mock := &mockD1Client{
		execFunc: func(ctx context.Context, sql string, params []interface{}) (int64, error) {
			return 1, nil
		},
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return []map[string]interface{}{
				{"id": float64(10), "post_id": float64(42), "r2_key": "posts/42/main.jpg", "photo_type": "main"},
			}, nil
		},
	}

	repo := NewPhotoRepository(mock)
	got, err := repo.Create(context.Background(), &model.Photo{
		PostID:    42,
		R2Key:     "posts/42/main.jpg",
		PhotoType: "main",
	})
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}
	if got.ID != 10 || got.PostID != 42 || got.R2Key != "posts/42/main.jpg" || got.PhotoType != "main" {
		t.Errorf("unexpected photo: %+v", got)
	}
}

// TestPhotoRepository_ListByPostID は投稿の写真一覧を返すことを確認する
func TestPhotoRepository_ListByPostID(t *testing.T) {
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return []map[string]interface{}{
				{"id": float64(1), "post_id": float64(42), "r2_key": "posts/42/main.jpg", "photo_type": "main"},
				{"id": float64(2), "post_id": float64(42), "r2_key": "posts/42/front.jpg", "photo_type": "front"},
			}, nil
		},
	}

	repo := NewPhotoRepository(mock)
	photos, err := repo.ListByPostID(context.Background(), 42)
	if err != nil {
		t.Fatalf("ListByPostID() failed: %v", err)
	}
	if len(photos) != 2 {
		t.Fatalf("expected 2 photos, got %d", len(photos))
	}
}

// TestPhotoRepository_ListByPostID_Empty は写真がない場合に空スライスを返すことを確認する
func TestPhotoRepository_ListByPostID_Empty(t *testing.T) {
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return nil, d1.ErrNotFound
		},
	}

	repo := NewPhotoRepository(mock)
	photos, err := repo.ListByPostID(context.Background(), 42)
	if err != nil {
		t.Fatalf("ListByPostID() should not fail when empty: %v", err)
	}
	if len(photos) != 0 {
		t.Errorf("expected 0 photos, got %d", len(photos))
	}
}

// TestPhotoRepository_ListByPostIDs は複数投稿の写真を post_id でグルーピングして返すことを確認する
func TestPhotoRepository_ListByPostIDs(t *testing.T) {
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return []map[string]interface{}{
				{"id": float64(1), "post_id": float64(42), "r2_key": "posts/42/main.jpg", "photo_type": "main"},
				{"id": float64(2), "post_id": float64(42), "r2_key": "posts/42/front.jpg", "photo_type": "front"},
				{"id": float64(3), "post_id": float64(43), "r2_key": "posts/43/main.jpg", "photo_type": "main"},
			}, nil
		},
	}

	repo := NewPhotoRepository(mock)
	grouped, err := repo.ListByPostIDs(context.Background(), []int64{42, 43})
	if err != nil {
		t.Fatalf("ListByPostIDs() failed: %v", err)
	}
	if len(grouped[42]) != 2 {
		t.Errorf("expected 2 photos for post 42, got %d", len(grouped[42]))
	}
	if len(grouped[43]) != 1 {
		t.Errorf("expected 1 photo for post 43, got %d", len(grouped[43]))
	}
}

// TestPhotoRepository_ListByPostIDs_Empty は空入力で空マップを返すことを確認する
func TestPhotoRepository_ListByPostIDs_Empty(t *testing.T) {
	repo := NewPhotoRepository(&mockD1Client{})
	grouped, err := repo.ListByPostIDs(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListByPostIDs() failed: %v", err)
	}
	if len(grouped) != 0 {
		t.Errorf("expected empty map, got %d entries", len(grouped))
	}
}
