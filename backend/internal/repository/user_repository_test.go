package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/pkg/d1"
)

// mockD1Client はテスト用のD1クライアントモック
type mockD1Client struct {
	queryFunc func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error)
	execFunc  func(ctx context.Context, sql string, params []interface{}) (int64, error)
}

func (m *mockD1Client) Query(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
	if m.queryFunc != nil {
		return m.queryFunc(ctx, sql, params)
	}
	return nil, d1.ErrNotFound
}

func (m *mockD1Client) Exec(ctx context.Context, sql string, params []interface{}) (int64, error) {
	if m.execFunc != nil {
		return m.execFunc(ctx, sql, params)
	}
	return 1, nil
}

// TestGetByEncryptedToken は正しい encrypted_access_token に対してユーザーを返すことを確認する
func TestGetByEncryptedToken(t *testing.T) {
	expectedUser := map[string]interface{}{
		"id":                     float64(1),
		"github_id":              float64(12345),
		"github_login":           "testuser",
		"github_name":            "Test User",
		"avatar_url":             "https://example.com/avatar.png",
		"encrypted_access_token": "nonce:encrypted",
		"created_at":             time.Now().Format(time.RFC3339),
	}

	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return []map[string]interface{}{expectedUser}, nil
		},
	}

	repo := NewUserRepository(mock)
	user, err := repo.GetByEncryptedToken(context.Background(), "nonce:encrypted")
	if err != nil {
		t.Fatalf("GetByEncryptedToken() failed: %v", err)
	}
	if user.GitHubLogin != "testuser" {
		t.Errorf("expected login=testuser, got %s", user.GitHubLogin)
	}
}

// TestGetByEncryptedToken_NotFound は見つからない場合に ErrNotFound を返すことを確認する
func TestGetByEncryptedToken_NotFound(t *testing.T) {
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return nil, d1.ErrNotFound
		},
	}

	repo := NewUserRepository(mock)
	_, err := repo.GetByEncryptedToken(context.Background(), "unknown_token")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestUpsertUser は新規ユーザー作成・既存ユーザー更新の両方を処理することを確認する
func TestUpsertUser(t *testing.T) {
	mock := &mockD1Client{
		execFunc: func(ctx context.Context, sql string, params []interface{}) (int64, error) {
			return 1, nil
		},
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return []map[string]interface{}{
				{
					"id":                     float64(1),
					"github_id":              float64(12345),
					"github_login":           "testuser",
					"github_name":            "Test User",
					"avatar_url":             "https://example.com/avatar.png",
					"encrypted_access_token": "nonce:encrypted",
					"created_at":             time.Now().Format(time.RFC3339),
				},
			}, nil
		},
	}

	repo := NewUserRepository(mock)
	user := &model.User{
		GitHubID:             12345,
		GitHubLogin:          "testuser",
		GitHubName:           "Test User",
		AvatarURL:            "https://example.com/avatar.png",
		EncryptedAccessToken: "nonce:encrypted",
	}

	result, err := repo.UpsertUser(context.Background(), user)
	if err != nil {
		t.Fatalf("UpsertUser() failed: %v", err)
	}
	if result.GitHubLogin != "testuser" {
		t.Errorf("expected login=testuser, got %s", result.GitHubLogin)
	}
}
