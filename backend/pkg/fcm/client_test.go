package fcm

import (
	"context"
	"encoding/json"
	"errors"
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

// TestSendToTokens_PermanentFailure は FCM の 400 / 404 が ErrPermanent になり、403（権限不足でも返る）や、
// 一時的な失敗が1つでも混ざった場合は ErrPermanent にならないことを確認する（Queue の再送の判定に使う）
func TestSendToTokens_PermanentFailure(t *testing.T) {
	cases := []struct {
		name      string
		statuses  map[string]int
		permanent bool
	}{
		{"404 unregistered", map[string]int{"a": http.StatusNotFound}, true},
		{"400 invalid", map[string]int{"a": http.StatusBadRequest, "b": http.StatusNotFound}, true},
		{"403", map[string]int{"a": http.StatusForbidden}, false},
		{"403 and 404", map[string]int{"a": http.StatusForbidden, "b": http.StatusNotFound}, false},
		{"401", map[string]int{"a": http.StatusUnauthorized}, false},
		{"500", map[string]int{"a": http.StatusInternalServerError}, false},
		{"429", map[string]int{"a": http.StatusTooManyRequests}, false},
		{"404 and 503", map[string]int{"a": http.StatusNotFound, "b": http.StatusServiceUnavailable}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req fcmMessageRequest
				_ = json.NewDecoder(r.Body).Decode(&req)
				w.WriteHeader(tc.statuses[req.Message.Token])
			}))
			defer server.Close()
			client := &fcmClient{httpClient: server.Client(), fcmEndpoint: server.URL, projectID: "p", accessToken: "t"}

			tokens := make([]string, 0, len(tc.statuses))
			for token := range tc.statuses {
				tokens = append(tokens, token)
			}
			err := client.SendToTokens(context.Background(), tokens, Notification{Title: "x"})
			if err == nil {
				t.Fatal("expected an error")
			}
			if got := errors.Is(err, ErrPermanent); got != tc.permanent {
				t.Errorf("errors.Is(err, ErrPermanent) = %v, want %v (err=%v)", got, tc.permanent, err)
			}
		})
	}
}
