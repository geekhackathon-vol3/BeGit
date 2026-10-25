package handler

import (
	"crypto/subtle"

	"github.com/gin-gonic/gin"
)

// validQueueSecret は Worker の queue() が付ける X-Notification-Queue-Secret を定数時間で照合する。
// Queue の内部エンドポイント（外部通知の配信 / FCM の通知）で共通に使う。secret が未設定なら常に拒否する。
func validQueueSecret(c *gin.Context, secret string) bool {
	provided := c.GetHeader("X-Notification-Queue-Secret")
	return secret != "" && len(provided) == len(secret) && subtle.ConstantTimeCompare([]byte(provided), []byte(secret)) == 1
}
