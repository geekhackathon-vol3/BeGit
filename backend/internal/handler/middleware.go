package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/irj0927/begit/internal/repository"
	"github.com/irj0927/begit/pkg/crypto"
)

// contextKey はコンテキストキーの型
type contextKey string

// UserIDKey はコンテキストに userID を格納するキー
const UserIDKey contextKey = "userID"

// AccessTokenKey はコンテキストに accessToken を格納するキー
const AccessTokenKey contextKey = "accessToken"

// writeError は JSON エラーレスポンスを書き込む
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// BearerAuthMiddleware は Bearer トークンを検証し、userID をコンテキストに注入するミドルウェア
func BearerAuthMiddleware(
	userRepo repository.UserRepository,
	encryptor crypto.Encryptor,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == "" {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			// トークンを暗号化して DB 検索
			encryptedToken, err := encryptor.Encrypt(token)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			user, err := userRepo.GetByEncryptedToken(r.Context(), encryptedToken)
			if err != nil {
				if errors.Is(err, repository.ErrNotFound) {
					writeError(w, http.StatusUnauthorized, "unauthorized")
					return
				}
				writeError(w, http.StatusInternalServerError, "internal server error")
				return
			}

			// userID と accessToken をコンテキストに注入
			ctx := context.WithValue(r.Context(), UserIDKey, user.ID)
			ctx = context.WithValue(ctx, AccessTokenKey, token)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GroupMemberMiddleware は URL パラメータの groupID とコンテキストの userID から
// グループメンバーシップを確認するミドルウェア
func GroupMemberMiddleware(
	groupRepo repository.GroupRepository,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := r.Context().Value(UserIDKey).(int64)
			if !ok || userID == 0 {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			// URL パスから groupID を取得 (Go 1.22 ServeMux の {id} パターン)
			groupIDStr := r.PathValue("id")
			if groupIDStr == "" {
				writeError(w, http.StatusBadRequest, "missing group id")
				return
			}

			var groupID int64
			if _, err := parseID(groupIDStr, &groupID); err != nil {
				writeError(w, http.StatusBadRequest, "invalid group id")
				return
			}

			isMember, err := groupRepo.IsMember(r.Context(), groupID, userID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal server error")
				return
			}
			if !isMember {
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// parseID は文字列を int64 に変換する
func parseID(s string, id *int64) (int64, error) {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errors.New("invalid id")
		}
		n = n*10 + int64(c-'0')
	}
	*id = n
	return n, nil
}
