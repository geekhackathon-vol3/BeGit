package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestExchangeCode はコード交換が正しく access_token を返すことを確認する
func TestExchangeCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": "github_access_token_abc123",
			"token_type":   "bearer",
		})
	}))
	defer server.Close()

	client := &githubClient{
		httpClient:    server.Client(),
		oauthEndpoint: server.URL,
		apiEndpoint:   server.URL,
	}

	token, err := client.ExchangeCode(context.Background(), "client_id", "client_secret", "auth_code")
	if err != nil {
		t.Fatalf("ExchangeCode() failed: %v", err)
	}
	if token != "github_access_token_abc123" {
		t.Errorf("expected token=github_access_token_abc123, got %s", token)
	}
}

// TestExchangeCode_InvalidCode は無効コードで ErrUnauthorized が返ることを確認する
func TestExchangeCode_InvalidCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"error":             "bad_verification_code",
			"error_description": "The code passed is incorrect or expired.",
		})
	}))
	defer server.Close()

	client := &githubClient{
		httpClient:    server.Client(),
		oauthEndpoint: server.URL,
		apiEndpoint:   server.URL,
	}

	_, err := client.ExchangeCode(context.Background(), "client_id", "client_secret", "invalid_code")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

// TestGetUser はユーザー情報を正しく返すことを確認する
func TestGetUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":         float64(12345),
			"login":      "testuser",
			"avatar_url": "https://example.com/avatar.png",
			"name":       "Test User",
		})
	}))
	defer server.Close()

	client := &githubClient{
		httpClient:    server.Client(),
		oauthEndpoint: server.URL,
		apiEndpoint:   server.URL,
	}

	user, err := client.GetUser(context.Background(), "test_token")
	if err != nil {
		t.Fatalf("GetUser() failed: %v", err)
	}
	if user.Login != "testuser" {
		t.Errorf("expected login=testuser, got %s", user.Login)
	}
	if user.ID != 12345 {
		t.Errorf("expected id=12345, got %d", user.ID)
	}
}

// TestGetUser_Unauthorized は 401 で ErrUnauthorized が返ることを確認する
func TestGetUser_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"message": "Unauthorized"})
	}))
	defer server.Close()

	client := &githubClient{
		httpClient:    server.Client(),
		oauthEndpoint: server.URL,
		apiEndpoint:   server.URL,
	}

	_, err := client.GetUser(context.Background(), "bad_token")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

// TestGetRepoInfo はリポジトリ情報を正しく返すことを確認する
func TestGetRepoInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"full_name": "owner/repo",
			"owner": map[string]interface{}{
				"avatar_url": "https://example.com/owner-avatar.png",
			},
		})
	}))
	defer server.Close()

	client := &githubClient{
		httpClient:    server.Client(),
		oauthEndpoint: server.URL,
		apiEndpoint:   server.URL,
	}

	info, err := client.GetRepoInfo(context.Background(), "owner/repo", "test_token")
	if err != nil {
		t.Fatalf("GetRepoInfo() failed: %v", err)
	}
	if info.FullName != "owner/repo" {
		t.Errorf("expected full_name=owner/repo, got %s", info.FullName)
	}
	if info.AvatarURL != "https://example.com/owner-avatar.png" {
		t.Errorf("expected avatar_url, got %s", info.AvatarURL)
	}
}

// TestRegisterWebhook は Webhook 登録が正常完了することを確認する
func TestRegisterWebhook(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": float64(1),
		})
	}))
	defer server.Close()

	client := &githubClient{
		httpClient:    server.Client(),
		oauthEndpoint: server.URL,
		apiEndpoint:   server.URL,
	}

	err := client.RegisterWebhook(context.Background(), "owner/repo", "test_token", "https://example.com/webhook/github", "webhook_secret")
	if err != nil {
		t.Fatalf("RegisterWebhook() failed: %v", err)
	}
}

// TestGetRecentCommits はコミット情報を正しく返すことを確認する
func TestGetRecentCommits(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Query().Get("author") != "" {
			// commits list endpoint
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"sha": "abc123"},
				{"sha": "def456"},
			})
		} else {
			// commit detail endpoint
			json.NewEncoder(w).Encode(map[string]interface{}{
				"sha": "abc123",
				"commit": map[string]interface{}{
					"message": "Initial commit",
				},
				"stats": map[string]interface{}{
					"additions": float64(100),
					"deletions": float64(50),
				},
			})
		}
	}))
	defer server.Close()

	client := &githubClient{
		httpClient:    server.Client(),
		oauthEndpoint: server.URL,
		apiEndpoint:   server.URL,
	}

	summary, err := client.GetRecentCommits(context.Background(), "owner/repo", "testuser", "test_token")
	if err != nil {
		t.Fatalf("GetRecentCommits() failed: %v", err)
	}
	if summary.CommitCount != 2 {
		t.Errorf("expected CommitCount=2, got %d", summary.CommitCount)
	}
	if summary.Additions != 100 {
		t.Errorf("expected Additions=100, got %d", summary.Additions)
	}
	if summary.LatestCommitMessage != "Initial commit" {
		t.Errorf("expected message=Initial commit, got %s", summary.LatestCommitMessage)
	}
}
