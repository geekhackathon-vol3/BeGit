package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/service"
)

// mockAuthService はテスト用の認証サービスモック
type mockAuthService struct {
	exchangeCodeFunc func(ctx context.Context, code string) (*service.AuthResult, error)
	getUserFunc      func(ctx context.Context, userID int64) (*model.User, error)
}

func (m *mockAuthService) ExchangeCode(ctx context.Context, code string) (*service.AuthResult, error) {
	if m.exchangeCodeFunc != nil {
		return m.exchangeCodeFunc(ctx, code)
	}
	return &service.AuthResult{
		User:  model.User{ID: 1, GitHubLogin: "testuser"},
		Token: "valid_token",
	}, nil
}

func (m *mockAuthService) GetUser(ctx context.Context, userID int64) (*model.User, error) {
	if m.getUserFunc != nil {
		return m.getUserFunc(ctx, userID)
	}
	return &model.User{ID: userID, GitHubLogin: "testuser"}, nil
}

// newAuthRouter は /auth/github を登録したテスト用 gin エンジンを作る
func newAuthRouter(svc service.AuthService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/auth/github", NewAuthHandler(svc).GitHub)
	return r
}

// TestAuthHandler_ExchangeCode は有効コードで 200 と token フィールドを返すことを確認する
func TestAuthHandler_ExchangeCode(t *testing.T) {
	authSvc := &mockAuthService{
		exchangeCodeFunc: func(ctx context.Context, code string) (*service.AuthResult, error) {
			return &service.AuthResult{
				User:  model.User{ID: 1, GitHubLogin: "testuser", AvatarURL: "https://example.com/avatar.png"},
				Token: "github_access_token",
			}, nil
		},
	}

	body, _ := json.Marshal(map[string]string{"code": "valid_auth_code"})
	req := httptest.NewRequest(http.MethodPost, "/auth/github", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	newAuthRouter(authSvc).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d. body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if _, ok := resp["token"]; !ok {
		t.Error("expected 'token' field in response")
	}
	if resp["token"] != "github_access_token" {
		t.Errorf("expected token=github_access_token, got %v", resp["token"])
	}
}

// TestAuthHandler_InvalidCode は無効コードで 401 を返すことを確認する
func TestAuthHandler_InvalidCode(t *testing.T) {
	authSvc := &mockAuthService{
		exchangeCodeFunc: func(ctx context.Context, code string) (*service.AuthResult, error) {
			return nil, service.ErrUnauthorized
		},
	}

	body, _ := json.Marshal(map[string]string{"code": "invalid_code"})
	req := httptest.NewRequest(http.MethodPost, "/auth/github", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	newAuthRouter(authSvc).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

// TestAuthHandler_MissingCode は code フィールド欠損で 422 を返すことを確認する
func TestAuthHandler_MissingCode(t *testing.T) {
	authSvc := &mockAuthService{}

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPost, "/auth/github", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	newAuthRouter(authSvc).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected status 422, got %d", rr.Code)
	}
}

// TestAuthHandler_ContentTypeJSON は全レスポンスに JSON Content-Type が付与されることを確認する
func TestAuthHandler_ContentTypeJSON(t *testing.T) {
	authSvc := &mockAuthService{}

	body, _ := json.Marshal(map[string]string{"code": "test_code"})
	req := httptest.NewRequest(http.MethodPost, "/auth/github", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	newAuthRouter(authSvc).ServeHTTP(rr, req)

	contentType := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}
}

// newMeRouter は GET /me を登録したテスト用 gin エンジンを作る。
// bearerAuth の代わりに userID を直接コンテキストへ注入するミドルウェアを挟む
// （userID 注入後のハンドラ挙動のみを検証するため）。
func newMeRouter(svc service.AuthService, userID int64, authed bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/me", func(c *gin.Context) {
		if authed {
			c.Set(ctxUserID, userID)
		}
		c.Next()
	}, NewAuthHandler(svc).Me)
	return r
}

// TestAuthHandler_Me は認証済みユーザーの情報を 200 で返すことを確認する
func TestAuthHandler_Me(t *testing.T) {
	authSvc := &mockAuthService{
		getUserFunc: func(ctx context.Context, userID int64) (*model.User, error) {
			return &model.User{
				ID:          userID,
				GitHubLogin: "octocat",
				GitHubName:  "The Octocat",
				AvatarURL:   "https://example.com/octocat.png",
			}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	rr := httptest.NewRecorder()
	newMeRouter(authSvc, 7, true).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	var got UserJSON
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got.ID != 7 || got.Login != "octocat" || got.Name != "The Octocat" {
		t.Errorf("unexpected user: %+v", got)
	}
}

// TestAuthHandler_Me_Unauthorized は userID 未注入時に 401 を返すことを確認する
func TestAuthHandler_Me_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	rr := httptest.NewRecorder()
	newMeRouter(&mockAuthService{}, 0, false).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}
