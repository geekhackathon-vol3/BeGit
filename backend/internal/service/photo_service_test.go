package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/irj0927/begit/internal/model"
)

// mockR2Client はテスト用の R2 クライアントモック
type mockR2Client struct {
	putObjectFunc    func(ctx context.Context, key, contentType string, body []byte) error
	deleteObjectFunc func(ctx context.Context, key string) error
}

func (m *mockR2Client) PutObject(ctx context.Context, key, contentType string, body []byte) error {
	if m.putObjectFunc != nil {
		return m.putObjectFunc(ctx, key, contentType, body)
	}
	return nil
}

func (m *mockR2Client) PresignGetURL(key string, ttl time.Duration) (string, error) {
	return "https://r2.example/" + key, nil
}

func (m *mockR2Client) DeleteObject(ctx context.Context, key string) error {
	if m.deleteObjectFunc != nil {
		return m.deleteObjectFunc(ctx, key)
	}
	return nil
}

// mockPhotoRepository はテスト用の写真リポジトリモック
type mockPhotoRepository struct {
	createFunc        func(ctx context.Context, photo *model.Photo) (*model.Photo, error)
	listByPostIDFunc  func(ctx context.Context, postID int64) ([]model.Photo, error)
	listByPostIDsFunc func(ctx context.Context, postIDs []int64) (map[int64][]model.Photo, error)
}

func (m *mockPhotoRepository) Create(ctx context.Context, photo *model.Photo) (*model.Photo, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, photo)
	}
	photo.ID = 1
	return photo, nil
}

func (m *mockPhotoRepository) ListByPostID(ctx context.Context, postID int64) ([]model.Photo, error) {
	if m.listByPostIDFunc != nil {
		return m.listByPostIDFunc(ctx, postID)
	}
	return []model.Photo{}, nil
}

func (m *mockPhotoRepository) ListByPostIDs(ctx context.Context, postIDs []int64) (map[int64][]model.Photo, error) {
	if m.listByPostIDsFunc != nil {
		return m.listByPostIDsFunc(ctx, postIDs)
	}
	return map[int64][]model.Photo{}, nil
}

func jpegFile() *UploadFile {
	return &UploadFile{ContentType: "image/jpeg", Data: []byte("fake-jpeg-bytes")}
}

// TestPhotoService_UploadPhotos_Success は main+front を保存し photos を返すことを確認する
func TestPhotoService_UploadPhotos_Success(t *testing.T) {
	postRepo := &mockPostRepository{
		getByIDFunc: func(ctx context.Context, postID int64) (*model.Post, error) {
			return &model.Post{ID: postID, UserID: 1, GroupID: 5}, nil
		},
	}
	var putKeys []string
	r2c := &mockR2Client{
		putObjectFunc: func(ctx context.Context, key, contentType string, body []byte) error {
			putKeys = append(putKeys, key)
			return nil
		},
	}
	var nextID int64
	photoRepo := &mockPhotoRepository{
		createFunc: func(ctx context.Context, photo *model.Photo) (*model.Photo, error) {
			nextID++
			photo.ID = nextID
			return photo, nil
		},
	}

	svc := NewPhotoService(r2c, photoRepo, postRepo)
	photos, err := svc.UploadPhotos(context.Background(), UploadPhotosRequest{
		Main:  jpegFile(),
		Front: jpegFile(),
	}, 5, 42, 1)
	if err != nil {
		t.Fatalf("UploadPhotos() failed: %v", err)
	}
	if len(photos) != 2 {
		t.Fatalf("expected 2 photos, got %d", len(photos))
	}
	if photos[0].PhotoType != "main" || photos[1].PhotoType != "front" {
		t.Errorf("unexpected photo types: %+v", photos)
	}
	wantKeys := []string{"posts/42/main.jpg", "posts/42/front.jpg"}
	for i, k := range wantKeys {
		if putKeys[i] != k {
			t.Errorf("put key[%d]=%s, want %s", i, putKeys[i], k)
		}
	}
}

// TestPhotoService_UploadPhotos_MainOnly は front 省略でも main のみ保存することを確認する
func TestPhotoService_UploadPhotos_MainOnly(t *testing.T) {
	postRepo := &mockPostRepository{
		getByIDFunc: func(ctx context.Context, postID int64) (*model.Post, error) {
			return &model.Post{ID: postID, UserID: 1, GroupID: 5}, nil
		},
	}
	svc := NewPhotoService(&mockR2Client{}, &mockPhotoRepository{}, postRepo)
	photos, err := svc.UploadPhotos(context.Background(), UploadPhotosRequest{Main: jpegFile()}, 5, 42, 1)
	if err != nil {
		t.Fatalf("UploadPhotos() failed: %v", err)
	}
	if len(photos) != 1 || photos[0].PhotoType != "main" {
		t.Errorf("expected 1 main photo, got %+v", photos)
	}
}

// TestPhotoService_UploadPhotos_MissingMain は main 未指定で ErrValidation を返すことを確認する
func TestPhotoService_UploadPhotos_MissingMain(t *testing.T) {
	svc := NewPhotoService(&mockR2Client{}, &mockPhotoRepository{}, &mockPostRepository{})
	_, err := svc.UploadPhotos(context.Background(), UploadPhotosRequest{}, 5, 42, 1)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

// TestPhotoService_UploadPhotos_NotOwner は他人の投稿への紐付けで ErrForbidden を返すことを確認する
func TestPhotoService_UploadPhotos_NotOwner(t *testing.T) {
	postRepo := &mockPostRepository{
		getByIDFunc: func(ctx context.Context, postID int64) (*model.Post, error) {
			return &model.Post{ID: postID, UserID: 999, GroupID: 5}, nil
		},
	}
	svc := NewPhotoService(&mockR2Client{}, &mockPhotoRepository{}, postRepo)
	_, err := svc.UploadPhotos(context.Background(), UploadPhotosRequest{Main: jpegFile()}, 5, 42, 1)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

// TestPhotoService_UploadPhotos_WrongGroup は別グループの投稿で ErrNotFound を返すことを確認する
func TestPhotoService_UploadPhotos_WrongGroup(t *testing.T) {
	postRepo := &mockPostRepository{
		getByIDFunc: func(ctx context.Context, postID int64) (*model.Post, error) {
			return &model.Post{ID: postID, UserID: 1, GroupID: 99}, nil
		},
	}
	svc := NewPhotoService(&mockR2Client{}, &mockPhotoRepository{}, postRepo)
	_, err := svc.UploadPhotos(context.Background(), UploadPhotosRequest{Main: jpegFile()}, 5, 42, 1)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestPhotoService_UploadPhotos_BadContentType は不正な Content-Type で ErrValidation を返すことを確認する
func TestPhotoService_UploadPhotos_BadContentType(t *testing.T) {
	postRepo := &mockPostRepository{
		getByIDFunc: func(ctx context.Context, postID int64) (*model.Post, error) {
			return &model.Post{ID: postID, UserID: 1, GroupID: 5}, nil
		},
	}
	svc := NewPhotoService(&mockR2Client{}, &mockPhotoRepository{}, postRepo)
	_, err := svc.UploadPhotos(context.Background(), UploadPhotosRequest{
		Main: &UploadFile{ContentType: "application/pdf", Data: []byte("x")},
	}, 5, 42, 1)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

// TestPhotoService_UploadPhotos_R2Failure は R2 PUT 失敗で ErrExternalAPI を返すことを確認する
func TestPhotoService_UploadPhotos_R2Failure(t *testing.T) {
	postRepo := &mockPostRepository{
		getByIDFunc: func(ctx context.Context, postID int64) (*model.Post, error) {
			return &model.Post{ID: postID, UserID: 1, GroupID: 5}, nil
		},
	}
	r2c := &mockR2Client{
		putObjectFunc: func(ctx context.Context, key, contentType string, body []byte) error {
			return errors.New("network error")
		},
	}
	svc := NewPhotoService(r2c, &mockPhotoRepository{}, postRepo)
	_, err := svc.UploadPhotos(context.Background(), UploadPhotosRequest{Main: jpegFile()}, 5, 42, 1)
	if !errors.Is(err, ErrExternalAPI) {
		t.Errorf("expected ErrExternalAPI, got %v", err)
	}
}
