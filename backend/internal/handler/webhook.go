package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/internal/repository"
	"github.com/irj0927/begit/internal/service"
)

// WebhookHandler は GitHub Webhook 受信エンドポイントのハンドラ
type WebhookHandler struct {
	webhookService service.WebhookService
	webhookRepo    repository.WebhookRepository
	secret         string
}

// NewWebhookHandler は WebhookHandler を作成する
func NewWebhookHandler(
	webhookService service.WebhookService,
	webhookRepo repository.WebhookRepository,
	secret string,
) *WebhookHandler {
	return &WebhookHandler{
		webhookService: webhookService,
		webhookRepo:    webhookRepo,
		secret:         secret,
	}
}

// Receive は GitHub Webhook を受信する。
//
//	@Summary		GitHub Webhook 受信（サーバ間）
//	@Description	X-Hub-Signature-256 で HMAC 検証し、X-GitHub-Delivery で冪等性を担保する
//	@Tags			webhook
//	@Accept			json
//	@Produce		json
//	@Param			X-Hub-Signature-256	header		string	true	"HMAC-SHA256 署名"
//	@Param			X-GitHub-Event		header		string	true	"イベント種別"
//	@Param			X-GitHub-Delivery	header		string	true	"配信 ID（冪等性キー）"
//	@Success		200					{object}	map[string]string
//	@Failure		400					{object}	ErrorResponse
//	@Failure		403					{object}	ErrorResponse
//	@Failure		500					{object}	ErrorResponse
//	@Router			/webhook/github [post]
func (h *WebhookHandler) Receive(c *gin.Context) {
	// ペイロードを読み込む
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		respondError(c, http.StatusBadRequest, "failed to read request body")
		return
	}

	// X-Hub-Signature-256 で HMAC 検証
	sig := c.GetHeader("X-Hub-Signature-256")
	if !h.verifySignature(payload, sig) {
		respondError(c, http.StatusForbidden, "invalid signature")
		return
	}

	// X-GitHub-Delivery で冪等性確認
	deliveryID := c.GetHeader("X-GitHub-Delivery")
	eventType := c.GetHeader("X-GitHub-Event")

	isDuplicate, err := h.webhookRepo.InsertDelivery(c.Request.Context(), deliveryID, eventType)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if isDuplicate {
		// 重複配信 → 200 OK で即座に返す
		c.JSON(http.StatusOK, map[string]string{"status": "duplicate"})
		return
	}

	// イベント処理を WebhookService に委譲
	if err := h.webhookService.ProcessWebhook(c.Request.Context(), service.WebhookRequest{
		DeliveryID: deliveryID,
		EventType:  eventType,
		Payload:    payload,
	}); err != nil {
		respondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// verifySignature は X-Hub-Signature-256 ヘッダーを検証する
func (h *WebhookHandler) verifySignature(payload []byte, sig string) bool {
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
