package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/irj0927/begit/internal/service"
)

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

// authHandler は AuthHandler の実装
type authHandler struct {
	authService service.AuthService
}

// NewAuthHandler は AuthHandler を作成する
func NewAuthHandler(authService service.AuthService) http.Handler {
	return &authHandler{authService: authService}
}

// ServeHTTP は POST /auth/github を処理する
func (h *authHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Code == "" {
		writeError(w, http.StatusUnprocessableEntity, "code: required")
		return
	}

	result, err := h.authService.ExchangeCode(r.Context(), req.Code)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorized) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := AuthResponse{
		User: UserJSON{
			ID:        result.User.ID,
			Login:     result.User.GitHubLogin,
			AvatarURL: result.User.AvatarURL,
			Name:      result.User.GitHubName,
		},
		Token: result.Token,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
