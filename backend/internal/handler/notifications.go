package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

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
	Status    string `json:"status"` // "On Time" | "Late" | "Missed"
}

// notificationHandler は NotificationHandler の実装
type notificationHandler struct {
	notificationService service.NotificationService
}

// NewNotificationHandler は NotificationHandler を作成する
func NewNotificationHandler(notificationService service.NotificationService) http.Handler {
	return &notificationHandler{notificationService: notificationService}
}

// ServeHTTP は /groups/:id/notifications と /groups/:id/notifications/:nid を処理する
func (h *notificationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID, ok := r.Context().Value(UserIDKey).(int64)
	if !ok || userID == 0 {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	groupIDStr := r.PathValue("id")
	groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid group id")
		return
	}

	nidStr := r.PathValue("nid")

	if nidStr == "" {
		// POST /groups/:id/notifications
		switch r.Method {
		case http.MethodPost:
			h.sendNotification(w, r, groupID, userID)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// GET /groups/:id/notifications/:nid
	notifID, err := strconv.ParseInt(nidStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid notification id")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getNotificationStatus(w, r, groupID, notifID)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// sendNotification は POST /groups/:id/notifications を処理する
func (h *notificationHandler) sendNotification(w http.ResponseWriter, r *http.Request, groupID, userID int64) {
	notif, err := h.notificationService.SendNotification(r.Context(), groupID, userID)
	if err != nil {
		if errors.Is(err, service.ErrConflict) {
			writeError(w, http.StatusConflict, "conflict: already sent notification in this sprint")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(NotificationJSON{
		ID:       notif.ID,
		SprintID: notif.SprintID,
		SentAt:   notif.SentAt.Format("2006-01-02T15:04:05Z"),
	})
}

// getNotificationStatus は GET /groups/:id/notifications/:nid を処理する
func (h *notificationHandler) getNotificationStatus(w http.ResponseWriter, r *http.Request, groupID, notifID int64) {
	status, err := h.notificationService.GetNotificationStatus(r.Context(), notifID, groupID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
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

	json.NewEncoder(w).Encode(NotificationStatusJSON{
		NotificationID: status.NotificationID,
		Members:        members,
	})
}
