package handler

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
	"github.com/irj0927/begit/internal/service"
)

type NotificationChannelJSON struct {
	ID          int64    `json:"id"`
	Platform    string   `json:"platform"`
	DisplayName string   `json:"display_name"`
	Enabled     bool     `json:"enabled"`
	Connected   bool     `json:"connected"`
	EventTypes  []string `json:"event_types"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type NotificationChannelListResponse struct {
	Channels []NotificationChannelJSON `json:"channels"`
}

type CreateNotificationChannelRequest struct {
	Platform    string   `json:"platform"`
	DisplayName string   `json:"display_name"`
	WebhookURL  string   `json:"webhook_url"`
	EventTypes  []string `json:"event_types"`
}

type UpdateNotificationChannelRequest struct {
	DisplayName *string   `json:"display_name"`
	WebhookURL  *string   `json:"webhook_url"`
	Enabled     *bool     `json:"enabled"`
	EventTypes  *[]string `json:"event_types"`
}

type NotificationChannelHandler struct {
	service service.NotificationChannelService
}

func NewNotificationChannelHandler(service service.NotificationChannelService) *NotificationChannelHandler {
	return &NotificationChannelHandler{service: service}
}

func channelJSON(channel model.NotificationChannel) NotificationChannelJSON {
	return NotificationChannelJSON{
		ID: channel.ID, Platform: channel.Platform, DisplayName: channel.DisplayName,
		Enabled: channel.Enabled, Connected: channel.EncryptedWebhookURL != "", EventTypes: channel.EventTypes,
		CreatedAt: formatOptionalTime(channel.CreatedAt), UpdatedAt: formatOptionalTime(channel.UpdatedAt),
	}
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func parseChannelRoute(c *gin.Context) (groupID, channelID int64, ok bool) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || groupID <= 0 {
		respondError(c, http.StatusBadRequest, "invalid group id")
		return 0, 0, false
	}
	if raw := c.Param("channelId"); raw != "" {
		channelID, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || channelID <= 0 {
			respondError(c, http.StatusBadRequest, "invalid channel id")
			return 0, 0, false
		}
	}
	return groupID, channelID, true
}

// List はオーナー向け通知チャネル一覧を返す。Webhook URL は返さない。
//
//	@Summary		外部通知チャネル一覧
//	@Tags			notification-channels
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"グループ ID"
//	@Success		200	{object}	NotificationChannelListResponse
//	@Failure		403	{object}	ErrorResponse
//	@Router			/groups/{id}/notification-channels [get]
func (h *NotificationChannelHandler) List(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	groupID, _, ok := parseChannelRoute(c)
	if !ok {
		return
	}
	channels, err := h.service.List(c.Request.Context(), groupID, userID)
	if err != nil {
		respondChannelError(c, err)
		return
	}
	result := make([]NotificationChannelJSON, 0, len(channels))
	for _, channel := range channels {
		result = append(result, channelJSON(channel))
	}
	c.JSON(http.StatusOK, NotificationChannelListResponse{Channels: result})
}

// Create は Slack / Discord Incoming Webhook を暗号化して接続する。
//
//	@Summary		外部通知チャネル作成
//	@Tags			notification-channels
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int							true	"グループ ID"
//	@Param			request	body		CreateNotificationChannelRequest	true	"通知先"
//	@Success		201		{object}	NotificationChannelJSON
//	@Failure		422		{object}	ErrorResponse
//	@Router			/groups/{id}/notification-channels [post]
func (h *NotificationChannelHandler) Create(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	groupID, _, ok := parseChannelRoute(c)
	if !ok {
		return
	}
	var req CreateNotificationChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}
	channel, err := h.service.Create(c.Request.Context(), groupID, userID, service.CreateNotificationChannelInput{
		Platform: req.Platform, DisplayName: req.DisplayName, WebhookURL: req.WebhookURL, EventTypes: req.EventTypes,
	})
	if err != nil {
		respondChannelError(c, err)
		return
	}
	c.JSON(http.StatusCreated, channelJSON(*channel))
}

// Update は表示名・Webhook・有効状態・購読イベントを更新する。
//
//	@Summary		外部通知チャネル更新
//	@Tags			notification-channels
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id			path		int							true	"グループ ID"
//	@Param			channelId	path		int							true	"チャネル ID"
//	@Param			request		body		UpdateNotificationChannelRequest	true	"更新内容"
//	@Success		200			{object}	NotificationChannelJSON
//	@Router			/groups/{id}/notification-channels/{channelId} [patch]
func (h *NotificationChannelHandler) Update(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	groupID, channelID, ok := parseChannelRoute(c)
	if !ok {
		return
	}
	var req UpdateNotificationChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}
	channel, err := h.service.Update(c.Request.Context(), groupID, channelID, userID, service.UpdateNotificationChannelInput{
		DisplayName: req.DisplayName, WebhookURL: req.WebhookURL, Enabled: req.Enabled, EventTypes: req.EventTypes,
	})
	if err != nil {
		respondChannelError(c, err)
		return
	}
	c.JSON(http.StatusOK, channelJSON(*channel))
}

// Delete は外部通知チャネルを削除する。
//
//	@Summary	外部通知チャネル削除
//	@Tags		notification-channels
//	@Security	BearerAuth
//	@Param		id			path	int	true	"グループ ID"
//	@Param		channelId	path	int	true	"チャネル ID"
//	@Success	204
//	@Router		/groups/{id}/notification-channels/{channelId} [delete]
func (h *NotificationChannelHandler) Delete(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	groupID, channelID, ok := parseChannelRoute(c)
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), groupID, channelID, userID); err != nil {
		respondChannelError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Test は保存済みWebhookへテスト通知を送る。
//
//	@Summary	外部通知チャネルのテスト送信
//	@Tags		notification-channels
//	@Security	BearerAuth
//	@Param		id			path	int	true	"グループ ID"
//	@Param		channelId	path	int	true	"チャネル ID"
//	@Success	204
//	@Failure	502	{object}	ErrorResponse
//	@Router		/groups/{id}/notification-channels/{channelId}/test [post]
func (h *NotificationChannelHandler) Test(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	groupID, channelID, ok := parseChannelRoute(c)
	if !ok {
		return
	}
	if err := h.service.Test(c.Request.Context(), groupID, channelID, userID); err != nil {
		if errors.Is(err, service.ErrValidation) {
			respondError(c, http.StatusUnprocessableEntity, err.Error())
			return
		}
		// URL や外部サービスの応答は漏らさず、設定画面には再確認を促す。
		if !errors.Is(err, service.ErrForbidden) && !errors.Is(err, service.ErrNotFound) {
			respondError(c, http.StatusBadGateway, "test notification failed")
			return
		}
		respondChannelError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func respondChannelError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrForbidden):
		respondError(c, http.StatusForbidden, "only the repository owner can manage notification channels")
	case errors.Is(err, service.ErrNotFound), errors.Is(err, repository.ErrNotFound):
		respondError(c, http.StatusNotFound, "notification channel not found")
	case errors.Is(err, service.ErrValidation):
		respondError(c, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, repository.ErrConflict):
		respondError(c, http.StatusConflict, "a notification channel with this name already exists")
	default:
		respondError(c, http.StatusInternalServerError, "internal server error")
	}
}

type NotificationDeliveryHandler struct {
	service service.ExternalNotificationDeliveryService
	secret  string
}

func NewNotificationDeliveryHandler(service service.ExternalNotificationDeliveryService, secret string) *NotificationDeliveryHandler {
	return &NotificationDeliveryHandler{service: service, secret: secret}
}

func (h *NotificationDeliveryHandler) Deliver(c *gin.Context) {
	provided := c.GetHeader("X-Notification-Queue-Secret")
	if h.secret == "" || len(provided) != len(h.secret) || subtle.ConstantTimeCompare([]byte(provided), []byte(h.secret)) != 1 {
		respondError(c, http.StatusForbidden, "forbidden")
		return
	}
	jobID, err := strconv.ParseInt(c.Param("jobId"), 10, 64)
	if err != nil || jobID <= 0 {
		respondError(c, http.StatusBadRequest, "invalid job id")
		return
	}
	if err := h.service.Deliver(c.Request.Context(), jobID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(c, http.StatusNotFound, "job not found")
			return
		}
		// Queue consumer は 5xx のみ retry する。
		respondError(c, http.StatusServiceUnavailable, "delivery failed")
		return
	}
	c.Status(http.StatusNoContent)
}
