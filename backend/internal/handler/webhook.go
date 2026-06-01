package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/irj0927/begit/internal/repository"
	"github.com/irj0927/begit/internal/service"
)

// webhookHandler は WebhookHandler の実装
type webhookHandler struct {
	webhookService service.WebhookService
	webhookRepo    repository.WebhookRepository
	secret         string
}

// NewWebhookHandler は WebhookHandler を作成する
func NewWebhookHandler(
	webhookService service.WebhookService,
	webhookRepo repository.WebhookRepository,
	secret string,
) http.Handler {
	return &webhookHandler{
		webhookService: webhookService,
		webhookRepo:    webhookRepo,
		secret:         secret,
	}
}

// ServeHTTP は POST /webhook/github を処理する
func (h *webhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// ペイロードを読み込む
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	// X-Hub-Signature-256 で HMAC 検証
	sig := r.Header.Get("X-Hub-Signature-256")
	if !h.verifySignature(payload, sig) {
		writeError(w, http.StatusForbidden, "invalid signature")
		return
	}

	// X-GitHub-Delivery で冪等性確認
	deliveryID := r.Header.Get("X-GitHub-Delivery")
	eventType := r.Header.Get("X-GitHub-Event")

	isDuplicate, err := h.webhookRepo.InsertDelivery(r.Context(), deliveryID, eventType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if isDuplicate {
		// 重複配信 → 200 OK で即座に返す
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "duplicate"})
		return
	}

	// イベント処理を WebhookService に委譲
	if err := h.webhookService.ProcessWebhook(r.Context(), service.WebhookRequest{
		DeliveryID: deliveryID,
		EventType:  eventType,
		Payload:    payload,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// verifySignature は X-Hub-Signature-256 ヘッダーを検証する
func (h *webhookHandler) verifySignature(payload []byte, sig string) bool {
	if sig == "" {
		return false
	}

	const prefix = "sha256="
	if !strings.HasPrefix(sig, prefix) {
		return false
	}

	expected := sig[len(prefix):]

	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(payload)
	actual := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(actual))
}
