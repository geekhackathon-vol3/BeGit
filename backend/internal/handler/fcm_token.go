package handler

import (
	"log"
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
	authService     service.AuthService
}

// NewFCMTokenHandler は FCMTokenHandler を作成する
func NewFCMTokenHandler(fcmTokenService service.FCMTokenService, authService service.AuthService) *FCMTokenHandler {
	return &FCMTokenHandler{fcmTokenService: fcmTokenService, authService: authService}
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

// Logout はログアウト処理を行う。
//
// GitHub OAuth トークンを失効させ（次回ログイン時にフル認証画面を表示させるため）、
// FCM トークンを削除して Push 通知が継続しないようにする。
// トークン失効が失敗しても FCM 削除とレスポンスは続行する（ベストエフォート）。
//
//	@Summary		ログアウト
//	@Description	GitHub OAuth トークンを失効させ FCM トークンを削除する。
//	@Tags			auth
//	@Produce		json
//	@Security		BearerAuth
//	@Success		204
//	@Failure		401	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/auth/logout [post]
func (h *FCMTokenHandler) Logout(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	// GitHub OAuthトークンを失効させる（失敗しても続行）
	if token := accessTokenFromContext(c); token != "" {
		if err := h.authService.RevokeToken(c.Request.Context(), token); err != nil {
			log.Printf("logout: failed to revoke GitHub token for user %d: %v", userID, err)
		}
	}

	if err := h.fcmTokenService.DeleteFCMTokens(c.Request.Context(), userID); err != nil {
		respondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.Status(http.StatusNoContent)
}
