package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/internal/service"
)

// pushMessageMaxBytes は Queue のメッセージの本文の上限。ID が数個だけなので小さくてよい
const pushMessageMaxBytes = 4 * 1024

// pushMessage は Worker が NOTIFICATION_QUEUE に積む FCM 通知のメッセージ
// （src/api/types.ts の NotificationQueueMessage のうち type のあるもの）。
type pushMessage struct {
	Type string `json:"type"`
	// GroupID は全ての type で必須
	GroupID int64 `json:"group_id"`
	// begit_time
	NotificationID int64 `json:"notification_id"`
	SprintID       int64 `json:"sprint_id"`
	// reaction
	PostID       int64 `json:"post_id"`
	ActorUserID  int64 `json:"actor_user_id"`
	AuthorUserID int64 `json:"author_user_id"`
}

// PushNotificationHandler は Queue から届いた FCM 通知を送る内部エンドポイント（#134）。
// Worker の queue() からだけ呼ぶ（/internal/* は Worker が外部から遮断する）。
type PushNotificationHandler struct {
	service service.PushNotificationService
	secret  string
}

func NewPushNotificationHandler(service service.PushNotificationService, secret string) *PushNotificationHandler {
	return &PushNotificationHandler{service: service, secret: secret}
}

// Send は POST /internal/push-notifications。
// 応答は外部通知の配信（NotificationDeliveryHandler.Deliver）と同じ扱いにする：
// 2xx は完了、4xx は再送しても変わらないので捨てる、5xx は Queue が再送する。
func (h *PushNotificationHandler) Send(c *gin.Context) {
	if !validQueueSecret(c, h.secret) {
		respondError(c, http.StatusForbidden, "forbidden")
		return
	}

	var msg pushMessage
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, pushMessageMaxBytes))
	if err := decoder.Decode(&msg); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	var err error
	switch msg.Type {
	case "begit_time":
		if !allPositive(msg.GroupID, msg.NotificationID, msg.SprintID) {
			respondError(c, http.StatusBadRequest, "invalid begit_time message")
			return
		}
		err = h.service.SendBeGitTime(c.Request.Context(), msg.GroupID, msg.NotificationID, msg.SprintID)
	case "reaction":
		if !allPositive(msg.GroupID, msg.PostID, msg.ActorUserID, msg.AuthorUserID) {
			respondError(c, http.StatusBadRequest, "invalid reaction message")
			return
		}
		err = h.service.SendReaction(c.Request.Context(), msg.GroupID, msg.PostID, msg.ActorUserID, msg.AuthorUserID)
	default:
		respondError(c, http.StatusBadRequest, "unsupported message type")
		return
	}

	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			respondError(c, http.StatusNotFound, "recipient not found")
			return
		}
		if errors.Is(err, service.ErrPushUndeliverable) {
			respondError(c, http.StatusUnprocessableEntity, "push undeliverable")
			return
		}
		// Queue consumer は 5xx のみ retry する。
		respondError(c, http.StatusServiceUnavailable, "push failed")
		return
	}
	c.Status(http.StatusNoContent)
}

func allPositive(ids ...int64) bool {
	for _, id := range ids {
		if id <= 0 {
			return false
		}
	}
	return true
}
