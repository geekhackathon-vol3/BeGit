package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
	"github.com/irj0927/begit/pkg/r2"
)

// MaxPhotoBytes は 1 枚あたりのアップロード上限サイズ（10MB）
const MaxPhotoBytes = 10 * 1024 * 1024

// allowedPhotoContentTypes は許可する画像 Content-Type と R2 キーの拡張子の対応
var allowedPhotoContentTypes = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/heic": "heic",
}

// UploadFile は 1 枚分のアップロードファイル
type UploadFile struct {
	ContentType string
	Data        []byte
}

// UploadPhotosRequest は写真アップロードリクエスト。
// Main（背面）は必須、Front（前面）は任意。
type UploadPhotosRequest struct {
	Main  *UploadFile
	Front *UploadFile
}

// PhotoService は写真アップロードサービスインターフェース
type PhotoService interface {
	UploadPhotos(ctx context.Context, req UploadPhotosRequest, groupID, postID, userID int64) ([]model.Photo, error)
}

// photoService は PhotoService インターフェースの実装
type photoService struct {
	r2Client  r2.Client
	photoRepo repository.PhotoRepository
	postRepo  repository.PostRepository
}

// NewPhotoService は PhotoService を作成する
func NewPhotoService(
	r2Client r2.Client,
	photoRepo repository.PhotoRepository,
	postRepo repository.PostRepository,
) PhotoService {
	return &photoService{
		r2Client:  r2Client,
		photoRepo: photoRepo,
		postRepo:  postRepo,
	}
}

// UploadPhotos は投稿の所有権を検証し、main/front を R2 に保存して photos テーブルに紐付ける
func (s *photoService) UploadPhotos(ctx context.Context, req UploadPhotosRequest, groupID, postID, userID int64) ([]model.Photo, error) {
	if s.r2Client == nil {
		return nil, fmt.Errorf("%w: r2 client not configured", ErrExternalAPI)
	}

	// main は必須
	if req.Main == nil {
		return nil, fmt.Errorf("%w: main photo is required", ErrValidation)
	}

	// 投稿の存在と所有権を検証する
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("photo_service: GetByID failed: %w", err)
	}
	if post.GroupID != groupID {
		return nil, ErrNotFound
	}
	if post.UserID != userID {
		return nil, ErrForbidden
	}

	// アップロード対象を順序付きで組み立てる（main → front）
	type item struct {
		photoType string
		file      *UploadFile
	}
	items := []item{{photoType: "main", file: req.Main}}
	if req.Front != nil {
		items = append(items, item{photoType: "front", file: req.Front})
	}

	// First validate all items before any external writes
	type validatedItem struct {
		photoType string
		file      *UploadFile
		ext       string
		key       string
	}
	validated := make([]validatedItem, 0, len(items))
	for _, it := range items {
		ext, err := validatePhoto(it.file)
		if err != nil {
			return nil, err
		}
		key := fmt.Sprintf("posts/%d/%s.%s", postID, it.photoType, ext)
		validated = append(validated, validatedItem{
			photoType: it.photoType,
			file:      it.file,
			ext:       ext,
			key:       key,
		})
	}

	// Now perform uploads and DB creates, rolling back on failure
	photos := make([]model.Photo, 0, len(validated))
	uploadedKeys := make([]string, 0, len(validated))
	createdIDs := make([]int64, 0, len(validated))

	for _, v := range validated {
		// Upload to R2
		if err := s.r2Client.PutObject(ctx, v.key, v.file.ContentType, v.file.Data); err != nil {
			// Rollback: delete already uploaded R2 objects
			for _, k := range uploadedKeys {
				_ = s.r2Client.DeleteObject(ctx, k)
			}
			return nil, fmt.Errorf("%w: r2 put failed: %v", ErrExternalAPI, err)
		}
		uploadedKeys = append(uploadedKeys, v.key)

		// Create DB record
		created, err := s.photoRepo.Create(ctx, &model.Photo{
			PostID:    postID,
			R2Key:     v.key,
			PhotoType: v.photoType,
		})
		if err != nil {
			// Rollback: delete uploaded R2 objects
			for _, k := range uploadedKeys {
				_ = s.r2Client.DeleteObject(ctx, k)
			}
			// Note: DB rollback would require transaction support or manual cleanup
			// For now we rely on application-level retry logic
			return nil, fmt.Errorf("photo_service: Create failed: %w", err)
		}
		createdIDs = append(createdIDs, created.ID)
		photos = append(photos, *created)
	}

	return photos, nil
}

// validatePhoto は Content-Type とサイズを検証し、R2 キー用の拡張子を返す
func validatePhoto(f *UploadFile) (string, error) {
	if len(f.Data) == 0 {
		return "", fmt.Errorf("%w: empty file", ErrValidation)
	}
	if len(f.Data) > MaxPhotoBytes {
		return "", fmt.Errorf("%w: file too large (max %d bytes)", ErrValidation, MaxPhotoBytes)
	}
	ext, ok := allowedPhotoContentTypes[f.ContentType]
	if !ok {
		return "", fmt.Errorf("%w: unsupported content type %q", ErrValidation, f.ContentType)
	}
	return ext, nil
}
