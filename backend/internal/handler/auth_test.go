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
