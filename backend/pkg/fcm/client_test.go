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

// TestSendToTokens_WithData は data 付き送信で FCM リクエストボディに data が含まれることを検証する
func TestSendToTokens_WithData(t *testing.T) {
	var captured map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &fcmClient{
		httpClient:  server.Client(),
		fcmEndpoint: server.URL + "/v1/projects/{project_id}/messages:send",
		projectID:   "test-project",
		accessToken: "test_token",
	}

	data := map[string]string{"type": "nice_work", "group_id": "12", "draft_post_id": "890"}
	err := client.SendToTokensWithData(context.Background(), []string{"token1"}, Notification{Title: "Nice Work!", Body: "x"}, data)
	if err != nil {
		t.Fatalf("SendToTokensWithData() failed: %v", err)
	}

	msg, ok := captured["message"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected message object, got: %v", captured)
	}
	gotData, ok := msg["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data object in message, got: %v", msg)
	}
	if gotData["type"] != "nice_work" || gotData["group_id"] != "12" || gotData["draft_post_id"] != "890" {
		t.Errorf("data fields mismatch: %v", gotData)
	}
	// notification も併存していること
	notif, ok := msg["notification"].(map[string]interface{})
	if !ok || notif["title"] != "Nice Work!" {
		t.Errorf("expected notification title=Nice Work!, got: %v", msg["notification"])
	}
}

// TestSendToTokens_NoDataOmitsDataField は data が空の場合に従来どおり notification のみ送信することを検証する
func TestSendToTokens_NoDataOmitsDataField(t *testing.T) {
	var captured map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &fcmClient{
		httpClient:  server.Client(),
		fcmEndpoint: server.URL + "/v1/projects/{project_id}/messages:send",
		projectID:   "test-project",
		accessToken: "test_token",
	}

	err := client.SendToTokens(context.Background(), []string{"token1"}, Notification{Title: "BeGit Time!", Body: "x"})
	if err != nil {
		t.Fatalf("SendToTokens() failed: %v", err)
	}

	msg, _ := captured["message"].(map[string]interface{})
	if _, exists := msg["data"]; exists {
		t.Errorf("expected no data field when data is empty, got: %v", msg["data"])
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
