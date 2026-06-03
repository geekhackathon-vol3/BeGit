package handler

import (
	"crypto/subtle"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/internal/service"
)

// CronHandler は内部 Cron エンドポイント（POST /internal/cron）のハンドラ。
// Workers scheduled() 経由でのみ到達し、X-Cron-Secret 一致時のみ受理する。
type CronHandler struct {
	cronService service.CronService
	cronSecret  string
}

// NewCronHandler は CronHandler を作成する
func NewCronHandler(cronService service.CronService, cronSecret string) *CronHandler {
	return &CronHandler{cronService: cronService, cronSecret: cronSecret}
}

// Run は kind に応じて時刻起点通知を発火する。
//
//	@Summary		内部 Cron 起動（サーバ間）
//	@Description	X-Cron-Secret 一致時のみ受理し kind(minutely|daily) を cron_service へ振り分ける
//	@Tags			internal
//	@Produce		json
//	@Param			kind			query		string	true	"minutely | daily"
//	@Param			X-Cron-Secret	header		string	true	"Cron 起動シークレット"
//	@Success		200				{object}	map[string]string
//	@Failure		400				{object}	ErrorResponse
//	@Failure		403				{object}	ErrorResponse
//	@Failure		500				{object}	ErrorResponse
//	@Router			/internal/cron [post]
func (h *CronHandler) Run(c *gin.Context) {
	// X-Cron-Secret を定数時間比較で検証（secret 未設定または不一致は 403）
	provided := c.GetHeader("X-Cron-Secret")
	if h.cronSecret == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(h.cronSecret)) != 1 {
		respondError(c, http.StatusForbidden, "forbidden")
		return
	}

	kind := c.Query("kind")

	if err := h.cronService.RunCron(c.Request.Context(), kind); err != nil {
		if errors.Is(err, service.ErrInvalidCronKind) {
			respondError(c, http.StatusBadRequest, "invalid kind")
			return
		}
		respondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
