package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/internal/service"
)

// UpdateFCMTokenRequest は PUT /me/fcm-token のリクエストボディ
type UpdateFCMTokenRequest struct {
	FCMToken string `json:"fcm_token" example:"fcm-device-token"`
}

// FCMTokenHandler は FCM トークン登録エンドポイントのハンドラ
type FCMTokenHandler struct {
	fcmTokenService service.FCMTokenService
}

// NewFCMTokenHandler は FCMTokenHandler を作成する
func NewFCMTokenHandler(fcmTokenService service.FCMTokenService) *FCMTokenHandler {
	return &FCMTokenHandler{fcmTokenService: fcmTokenService}
}

// Upsert は FCM トークンを登録/更新する。
//
//	@Summary		FCM トークン登録 / 更新
//	@Tags			me
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		UpdateFCMTokenRequest	true	"FCM トークン"
//	@Success		200		{object}	map[string]string
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/me/fcm-token [put]
func (h *FCMTokenHandler) Upsert(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req UpdateFCMTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.FCMToken == "" {
		respondError(c, http.StatusBadRequest, "fcm_token: required")
		return
	}

	if err := h.fcmTokenService.UpsertFCMToken(c.Request.Context(), userID, req.FCMToken); err != nil {
		respondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusOK, map[string]string{})
}
