package handler

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/service"
)

// mockPhotoService はテスト用の PhotoService モック
type mockPhotoService struct {
	uploadFunc func(ctx context.Context, req service.UploadPhotosRequest, groupID, postID, userID int64) ([]model.Photo, error)
}

func (m *mockPhotoService) UploadPhotos(ctx context.Context, req service.UploadPhotosRequest, groupID, postID, userID int64) ([]model.Photo, error) {
	if m.uploadFunc != nil {
		return m.uploadFunc(ctx, req, groupID, postID, userID)
	}
	return []model.Photo{{ID: 1, PostID: postID, R2Key: "posts/1/main.jpg", PhotoType: "main"}}, nil
}

// stubSigner は presigned URL を固定で返すモック
type stubSigner struct{}

func (s *stubSigner) PresignGetURL(key string, ttl time.Duration) (string, error) {
	return "https://r2.example/" + key, nil
}

// newPhotoRouter は userID を注入した上で /groups/:id/posts/:postId/photos を登録する
func newPhotoRouter(svc service.PhotoService, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/groups/:id/posts/:postId/photos", func(c *gin.Context) {
		if userID != 0 {
			c.Set(ctxUserID, userID)
		}
		NewPhotoHandler(svc, &stubSigner{}).Upload(c)
	})
	return r
}

// buildMultipart は main/front のファイルフィールドを持つ multipart リクエストボディを作る
func buildMultipart(t *testing.T, fields map[string]struct {
	contentType string
	data        []byte
}) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	for name, f := range fields {
		h := make(map[string][]string)
		h["Content-Disposition"] = []string{`form-data; name="` + name + `"; filename="` + name + `.jpg"`}
		h["Content-Type"] = []string{f.contentType}
		part, err := w.CreatePart(h)
		if err != nil {
			t.Fatalf("CreatePart: %v", err)
		}
		part.Write(f.data)
	}
	w.Close()
	return body, w.FormDataContentType()
}

// TestPhotoHandler_Upload_Success は main+front アップロードで 201 と presigned URL を返すことを確認する
func TestPhotoHandler_Upload_Success(t *testing.T) {
	svc := &mockPhotoService{
		uploadFunc: func(ctx context.Context, req service.UploadPhotosRequest, groupID, postID, userID int64) ([]model.Photo, error) {
			if req.Main == nil {
				t.Error("expected main file")
			}
			return []model.Photo{
				{ID: 1, PostID: postID, R2Key: "posts/42/main.jpg", PhotoType: "main"},
				{ID: 2, PostID: postID, R2Key: "posts/42/front.jpg", PhotoType: "front"},
			}, nil
		},
	}

	body, ct := buildMultipart(t, map[string]struct {
		contentType string
		data        []byte
	}{
		"main":  {"image/jpeg", []byte("main-bytes")},
		"front": {"image/jpeg", []byte("front-bytes")},
	})

	req := httptest.NewRequest(http.MethodPost, "/groups/5/posts/42/photos", body)
	req.Header.Set("Content-Type", ct)
	rr := httptest.NewRecorder()

	newPhotoRouter(svc, 1).ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("https://r2.example/posts/42/main.jpg")) {
		t.Errorf("expected presigned URL in response, got %s", rr.Body.String())
	}
}

// TestPhotoHandler_Upload_Unauthorized は userID 未注入で 401 を返すことを確認する
func TestPhotoHandler_Upload_Unauthorized(t *testing.T) {
	body, ct := buildMultipart(t, map[string]struct {
		contentType string
		data        []byte
	}{"main": {"image/jpeg", []byte("x")}})

	req := httptest.NewRequest(http.MethodPost, "/groups/5/posts/42/photos", body)
	req.Header.Set("Content-Type", ct)
	rr := httptest.NewRecorder()

	newPhotoRouter(&mockPhotoService{}, 0).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

// TestPhotoHandler_Upload_Forbidden は service が ErrForbidden を返すと 403 になることを確認する
func TestPhotoHandler_Upload_Forbidden(t *testing.T) {
	svc := &mockPhotoService{
		uploadFunc: func(ctx context.Context, req service.UploadPhotosRequest, groupID, postID, userID int64) ([]model.Photo, error) {
			return nil, service.ErrForbidden
		},
	}
	body, ct := buildMultipart(t, map[string]struct {
		contentType string
		data        []byte
	}{"main": {"image/jpeg", []byte("x")}})

	req := httptest.NewRequest(http.MethodPost, "/groups/5/posts/42/photos", body)
	req.Header.Set("Content-Type", ct)
	rr := httptest.NewRecorder()

	newPhotoRouter(svc, 1).ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// TestPhotoHandler_Upload_ValidationError は service が ErrValidation を返すと 422 になることを確認する
func TestPhotoHandler_Upload_ValidationError(t *testing.T) {
	svc := &mockPhotoService{
		uploadFunc: func(ctx context.Context, req service.UploadPhotosRequest, groupID, postID, userID int64) ([]model.Photo, error) {
			return nil, service.ErrValidation
		},
	}
	body, ct := buildMultipart(t, map[string]struct {
		contentType string
		data        []byte
	}{"main": {"application/pdf", []byte("x")}})

	req := httptest.NewRequest(http.MethodPost, "/groups/5/posts/42/photos", body)
	req.Header.Set("Content-Type", ct)
	rr := httptest.NewRecorder()

	newPhotoRouter(svc, 1).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}
