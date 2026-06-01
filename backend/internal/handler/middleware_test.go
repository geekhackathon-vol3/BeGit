package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
)

// mockUserRepoForHandler はハンドラーテスト用のユーザーリポジトリモック
type mockUserRepoForHandler struct {
	getByEncryptedTokenFunc func(ctx context.Context, encryptedToken string) (*model.User, error)
	upsertUserFunc          func(ctx context.Context, user *model.User) (*model.User, error)
	getByGitHubLoginFunc    func(ctx context.Context, login string) (*model.User, error)
}

func (m *mockUserRepoForHandler) GetByEncryptedToken(ctx context.Context, encryptedToken string) (*model.User, error) {
	if m.getByEncryptedTokenFunc != nil {
		return m.getByEncryptedTokenFunc(ctx, encryptedToken)
	}
	return nil, errors.New("not found")
}

func (m *mockUserRepoForHandler) UpsertUser(ctx context.Context, user *model.User) (*model.User, error) {
	if m.upsertUserFunc != nil {
		return m.upsertUserFunc(ctx, user)
	}
	return user, nil
}

func (m *mockUserRepoForHandler) GetByGitHubLogin(ctx context.Context, login string) (*model.User, error) {
	if m.getByGitHubLoginFunc != nil {
		return m.getByGitHubLoginFunc(ctx, login)
	}
	return nil, errors.New("not found")
}

// mockEncryptorForHandler はハンドラーテスト用の暗号化モック
type mockEncryptorForHandler struct {
	encryptFunc func(plaintext string) (string, error)
	decryptFunc func(ciphertext string) (string, error)
}

func (m *mockEncryptorForHandler) Encrypt(plaintext string) (string, error) {
	if m.encryptFunc != nil {
		return m.encryptFunc(plaintext)
	}
	return "encrypted:" + plaintext, nil
}

func (m *mockEncryptorForHandler) Decrypt(ciphertext string) (string, error) {
	if m.decryptFunc != nil {
		return m.decryptFunc(ciphertext)
	}
	return "decrypted", nil
}

// TestBearerAuthMiddleware_ValidToken は有効なトークンで userID がコンテキストに注入されることを確認する
func TestBearerAuthMiddleware_ValidToken(t *testing.T) {
	userRepo := &mockUserRepoForHandler{
		getByEncryptedTokenFunc: func(ctx context.Context, encryptedToken string) (*model.User, error) {
			return &model.User{ID: 42, GitHubLogin: "testuser", CreatedAt: time.Now()}, nil
		},
	}
	crypto := &mockEncryptorForHandler{}

	handlerCalled := false
	var capturedUserID int64

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/groups", BearerAuth(userRepo, crypto), func(c *gin.Context) {
		handlerCalled = true
		if id, ok := userIDFromContext(c); ok {
			capturedUserID = id
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/groups", nil)
	req.Header.Set("Authorization", "Bearer valid_token_abc123")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	if !handlerCalled {
		t.Error("expected inner handler to be called")
	}
	if capturedUserID != 42 {
		t.Errorf("expected userID=42, got %d", capturedUserID)
	}
}

// TestBearerAuthMiddleware_InvalidToken は無効なトークンで 401 を返すことを確認する
func TestBearerAuthMiddleware_InvalidToken(t *testing.T) {
	userRepo := &mockUserRepoForHandler{
		getByEncryptedTokenFunc: func(ctx context.Context, encryptedToken string) (*model.User, error) {
			return nil, repository.ErrNotFound
		},
	}
	crypto := &mockEncryptorForHandler{}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/groups", BearerAuth(userRepo, crypto), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/groups", nil)
	req.Header.Set("Authorization", "Bearer invalid_token")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

// TestBearerAuthMiddleware_MissingHeader は Authorization ヘッダー欠損で 401 を返すことを確認する
func TestBearerAuthMiddleware_MissingHeader(t *testing.T) {
	userRepo := &mockUserRepoForHandler{}
	crypto := &mockEncryptorForHandler{}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/groups", BearerAuth(userRepo, crypto), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/groups", nil)
	// Authorization ヘッダーなし
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for missing header, got %d", rr.Code)
	}
}
