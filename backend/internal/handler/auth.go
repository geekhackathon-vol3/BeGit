package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/internal/repository"
	"github.com/irj0927/begit/internal/service"
)

// AuthRequest は POST /auth/github のリクエストボディ
type AuthRequest struct {
	Code string `json:"code" example:"abcdef123456"`
}

// AuthResponse は POST /auth/github のレスポンス
type AuthResponse struct {
	User  UserJSON `json:"user"`
	Token string   `json:"token"`
}

// UserJSON は JSON レスポンス用のユーザー型
type UserJSON struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
	Name      string `json:"name"`
}

// AuthHandler は認証エンドポイントのハンドラ
type AuthHandler struct {
	authService service.AuthService
}

// NewAuthHandler は AuthHandler を作成する
func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// GitHub は GitHub OAuth ログインを処理する
//
//	@Summary		GitHub OAuth ログイン
//	@Description	GitHub OAuth 認可コードをアクセストークンへ交換し、ユーザーを作成/更新してトークンを発行する
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		AuthRequest	true	"GitHub 認可コード"
//	@Success		200		{object}	AuthResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		422		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/auth/github [post]
func (h *AuthHandler) GitHub(c *gin.Context) {
	var req AuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Code == "" {
		respondError(c, http.StatusUnprocessableEntity, "code: required")
		return
	}

	result, err := h.authService.ExchangeCode(c.Request.Context(), req.Code)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorized) {
			respondError(c, http.StatusUnauthorized, "unauthorized")
			return
		}
		log.Printf("auth github exchange failed: %v", err)
		respondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusOK, AuthResponse{
		User: UserJSON{
			ID:        result.User.ID,
			Login:     result.User.GitHubLogin,
			AvatarURL: result.User.AvatarURL,
			Name:      result.User.GitHubName,
		},
		Token: result.Token,
	})
}

// Me は現在ログイン中のユーザー情報を返す
//
//	@Summary		現在のユーザー情報
//	@Description	Bearer トークンで認証されたユーザー自身の情報を返す
//	@Tags			auth
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	UserJSON
//	@Failure		401	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.authService.GetUser(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(c, http.StatusNotFound, "not found")
			return
		}
		log.Printf("get me failed: %v", err)
		respondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusOK, UserJSON{
		ID:        user.ID,
		Login:     user.GitHubLogin,
		AvatarURL: user.AvatarURL,
		Name:      user.GitHubName,
	})
}
