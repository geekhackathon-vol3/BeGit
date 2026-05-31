package handler

import (
	"encoding/json"
	"net/http"

	"github.com/irj0927/begit/internal/service"
)

// fcmTokenHandler は FCMTokenHandler の実装
type fcmTokenHandler struct {
	fcmTokenService service.FCMTokenService
}

// NewFCMTokenHandler は FCMTokenHandler を作成する
func NewFCMTokenHandler(fcmTokenService service.FCMTokenService) http.Handler {
	return &fcmTokenHandler{fcmTokenService: fcmTokenService}
}

// ServeHTTP は PUT /me/fcm-token を処理する
func (h *fcmTokenHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, ok := r.Context().Value(UserIDKey).(int64)
	if !ok || userID == 0 {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		FCMToken string `json:"fcm_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.FCMToken == "" {
		writeError(w, http.StatusBadRequest, "fcm_token: required")
		return
	}

	if err := h.fcmTokenService.UpsertFCMToken(r.Context(), userID, req.FCMToken); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{})
}
