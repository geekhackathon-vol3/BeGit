package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/internal/service"
)

// NotificationJSON は通知レスポンス型
type NotificationJSON struct {
	ID       int64  `json:"id"`
	SprintID int64  `json:"sprint_id"`
	SentAt   string `json:"sent_at"`
}

// NotificationStatusJSON は通知ステータスレスポンス型
type NotificationStatusJSON struct {
	NotificationID int64              `json:"notification_id"`
	Members        []MemberStatusJSON `json:"members"`
}

// MemberStatusJSON はメンバーごとのステータス
type MemberStatusJSON struct {
	UserID    int64  `json:"user_id"`
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
	Status    string `json:"status" example:"On Time"` // "On Time" | "Late" | "Missed"
}

// NotificationHandler は通知エンドポイントのハンドラ
type NotificationHandler struct {
	notificationService service.NotificationService
}

// NewNotificationHandler は NotificationHandler を作成する
func NewNotificationHandler(notificationService service.NotificationService) *NotificationHandler {
	return &NotificationHandler{notificationService: notificationService}
}

// Send は BeGit Time 通知を発行する。
//
//	@Summary		BeGit Time 通知発行
//	@Description	1 スプリント 1 人 1 回まで
//	@Tags			notifications
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"グループ ID"
//	@Success		201	{object}	NotificationJSON
//	@Failure		401	{object}	ErrorResponse
//	@Failure		409	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/groups/{id}/notifications [post]
func (h *NotificationHandler) Send(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid group id")
		return
	}

	notif, err := h.notificationService.SendNotification(c.Request.Context(), groupID, userID)
	if err != nil {
		if errors.Is(err, service.ErrConflict) {
			respondError(c, http.StatusConflict, "conflict: already sent notification in this sprint")
			return
		}
		respondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusCreated, NotificationJSON{
		ID:       notif.ID,
		SprintID: notif.SprintID,
		SentAt:   notif.SentAt.UTC().Format(time.RFC3339),
	})
}

// GetStatus は通知の達成ステータスを返す。
//
//	@Summary		通知の達成ステータス（On Time / Late / Missed）
//	@Tags			notifications
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"グループ ID"
//	@Param			nid	path		int	true	"通知 ID"
//	@Success		200	{object}	NotificationStatusJSON
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/groups/{id}/notifications/{nid} [get]
func (h *NotificationHandler) GetStatus(c *gin.Context) {
	if _, ok := userIDFromContext(c); !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid group id")
		return
	}

	notifID, err := strconv.ParseInt(c.Param("nid"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid notification id")
		return
	}

	status, err := h.notificationService.GetNotificationStatus(c.Request.Context(), notifID, groupID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			respondError(c, http.StatusNotFound, "not found")
			return
		}
		respondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	members := make([]MemberStatusJSON, 0, len(status.Members))
	for _, m := range status.Members {
		members = append(members, MemberStatusJSON{
			UserID:    m.UserID,
			Login:     m.Login,
			AvatarURL: m.AvatarURL,
			Status:    m.Status,
		})
	}

	c.JSON(http.StatusOK, NotificationStatusJSON{
		NotificationID: status.NotificationID,
		Members:        members,
	})
}
