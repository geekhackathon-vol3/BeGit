package handler

import "github.com/gin-gonic/gin"

// コンテキストキー。BearerAuth ミドルウェアが gin.Context に格納し、
// 各ハンドラが取り出して利用する。
const (
	ctxUserID      = "userID"
	ctxAccessToken = "accessToken"
)

// ErrorResponse は全エンドポイント共通のエラーレスポンス型。
type ErrorResponse struct {
	Error string `json:"error"`
}

// respondError は JSON エラーレスポンスを書き込み、後続ハンドラを中断する。
// ミドルウェア・ハンドラのどちらから呼んでも安全。
func respondError(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, ErrorResponse{Error: message})
}

// userIDFromContext は BearerAuth が注入した userID を取り出す。
// 未注入または 0 の場合は ok=false。
func userIDFromContext(c *gin.Context) (int64, bool) {
	v, exists := c.Get(ctxUserID)
	if !exists {
		return 0, false
	}
	id, ok := v.(int64)
	if !ok || id == 0 {
		return 0, false
	}
	return id, true
}

// accessTokenFromContext は BearerAuth が注入したアクセストークンを取り出す。
func accessTokenFromContext(c *gin.Context) string {
	return c.GetString(ctxAccessToken)
}
