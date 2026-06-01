package fcm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestSendToTokens_EmptyTokens はトークンリストが空の場合に API 呼び出しをスキップすることを確認する
func TestSendToTokens_EmptyTokens(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
	}))
	defer server.Close()

	client := &fcmClient{
		httpClient:  server.Client(),
		fcmEndpoint: server.URL,
		projectID:   "test-project",
		accessToken: "test_token",
	}

	err := client.SendToTokens(context.Background(), []string{}, Notification{
		Title: "Test",
		Body:  "Test message",
	})
	if err != nil {
		t.Fatalf("SendToTokens() with empty tokens should not fail, got: %v", err)
	}
	if callCount != 0 {
		t.Errorf("expected 0 API calls, got %d", callCount)
	}
}

// TestSendToTokens_Success は FCM 送信が正常完了することを確認する
func TestSendToTokens_Success(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"name": "projects/test-project/messages/fake_message_id",
		})
	}))
	defer server.Close()

	client := &fcmClient{
		httpClient:  server.Client(),
		fcmEndpoint: server.URL + "/v1/projects/{project_id}/messages:send",
		projectID:   "test-project",
		accessToken: "test_token",
	}

	tokens := []string{"token1", "token2", "token3"}
	err := client.SendToTokens(context.Background(), tokens, Notification{
		Title: "BeGit Time!",
		Body:  "今なに作ってる？",
	})
	if err != nil {
		t.Fatalf("SendToTokens() failed: %v", err)
	}
	if callCount != len(tokens) {
		t.Errorf("expected %d API calls, got %d", len(tokens), callCount)
	}
}
