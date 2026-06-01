package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/irj0927/begit/internal/service"
)

// mockWebhookService はテスト用の Webhook サービスモック
type mockWebhookService struct {
	processWebhookFunc func(ctx context.Context, req service.WebhookRequest) error
}

func (m *mockWebhookService) ProcessWebhook(ctx context.Context, req service.WebhookRequest) error {
	if m.processWebhookFunc != nil {
		return m.processWebhookFunc(ctx, req)
	}
	return nil
}

// mockWebhookRepository はテスト用の Webhook リポジトリモック
type mockWebhookRepository struct {
	insertDeliveryFunc func(ctx context.Context, deliveryID, eventType string) (bool, error)
}

func (m *mockWebhookRepository) InsertDelivery(ctx context.Context, deliveryID, eventType string) (bool, error) {
	if m.insertDeliveryFunc != nil {
		return m.insertDeliveryFunc(ctx, deliveryID, eventType)
	}
	return false, nil
}

// computeHMAC は HMAC-SHA256 署名を計算する
func computeHMAC(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// TestWebhookHandler_ValidSignature は正しい HMAC で 200 を返すことを確認する
func TestWebhookHandler_ValidSignature(t *testing.T) {
	secret := "webhook_secret"
	payload := []byte(`{"repository":{"full_name":"owner/repo"}}`)

	webhookSvc := &mockWebhookService{}
	webhookRepo := &mockWebhookRepository{}

	handler := NewWebhookHandler(webhookSvc, webhookRepo, secret)

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", bytes.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", computeHMAC(secret, payload))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-GitHub-Delivery", "delivery-uuid-123")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d. body: %s", rr.Code, rr.Body.String())
	}
}

// TestWebhookHandler_InvalidSignature は誤った HMAC で 403 を返すことを確認する
func TestWebhookHandler_InvalidSignature(t *testing.T) {
	secret := "webhook_secret"
	payload := []byte(`{"repository":{"full_name":"owner/repo"}}`)

	webhookSvc := &mockWebhookService{}
	webhookRepo := &mockWebhookRepository{}

	handler := NewWebhookHandler(webhookSvc, webhookRepo, secret)

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", bytes.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", "sha256=invalidsignature")
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-GitHub-Delivery", "delivery-uuid-456")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", rr.Code)
	}
}

// TestWebhookHandler_Idempotent は同じ delivery_id で2回送ると2回目も 200 を返すことを確認する
func TestWebhookHandler_Idempotent(t *testing.T) {
	secret := "webhook_secret"
	payload := []byte(`{"repository":{"full_name":"owner/repo"}}`)
	callCount := 0

	webhookSvc := &mockWebhookService{
		processWebhookFunc: func(ctx context.Context, req service.WebhookRequest) error {
			callCount++
			return nil
		},
	}

	insertCount := 0
	webhookRepo := &mockWebhookRepository{
		insertDeliveryFunc: func(ctx context.Context, deliveryID, eventType string) (bool, error) {
			insertCount++
			if insertCount >= 2 {
				return true, nil // 重複
			}
			return false, nil
		},
	}

	handler := NewWebhookHandler(webhookSvc, webhookRepo, secret)

	sig := computeHMAC(secret, payload)

	// 1回目
	req1 := httptest.NewRequest(http.MethodPost, "/webhook/github", bytes.NewReader(payload))
	req1.Header.Set("X-Hub-Signature-256", sig)
	req1.Header.Set("X-GitHub-Event", "push")
	req1.Header.Set("X-GitHub-Delivery", "same-delivery-id")
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("1st request: expected 200, got %d", rr1.Code)
	}

	// 2回目（同じ delivery_id）
	req2 := httptest.NewRequest(http.MethodPost, "/webhook/github", bytes.NewReader(payload))
	req2.Header.Set("X-Hub-Signature-256", sig)
	req2.Header.Set("X-GitHub-Event", "push")
	req2.Header.Set("X-GitHub-Delivery", "same-delivery-id")
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Errorf("2nd request (duplicate): expected 200, got %d", rr2.Code)
	}

	if callCount != 1 {
		t.Errorf("expected ProcessWebhook to be called 1 time, got %d", callCount)
	}
}
