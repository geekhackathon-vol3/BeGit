package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
	"github.com/irj0927/begit/pkg/crypto"
)

// BearerAuth は Bearer トークンを検証し、userID と accessToken を
// gin.Context に注入するミドルウェア。
func BearerAuth(
	userRepo repository.UserRepository,
	encryptor crypto.Encryptor,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			respondError(c, http.StatusUnauthorized, "unauthorized")
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			respondError(c, http.StatusUnauthorized, "unauthorized")
			return
		}

		// トークンを暗号化して DB 検索
		encryptedToken, err := encryptor.Encrypt(token)
		if err != nil {
			respondError(c, http.StatusUnauthorized, "unauthorized")
			return
		}

		user, err := userRepo.GetByEncryptedToken(c.Request.Context(), encryptedToken)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				respondError(c, http.StatusUnauthorized, "unauthorized")
				return
			}
			respondError(c, http.StatusInternalServerError, "internal server error")
			return
		}

		// userID と accessToken をコンテキストに注入
		c.Set(ctxUserID, user.ID)
		c.Set(ctxAccessToken, token)
		c.Next()
	}
}

// GroupRole はグループ内roleを検証する。旧memberはposterとして扱う。
type groupMemberReader interface {
	GetMember(ctx context.Context, groupID, userID int64) (*model.GroupMember, error)
}

func GroupRole(groupRepo groupMemberReader, allowedRoles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedRoles))
	for _, role := range allowedRoles {
		allowed[model.NormalizeGroupRole(role)] = struct{}{}
	}
	return func(c *gin.Context) {
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
		member, err := groupRepo.GetMember(c.Request.Context(), groupID, userID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				respondError(c, http.StatusForbidden, "forbidden")
				return
			}
			respondError(c, http.StatusInternalServerError, "internal server error")
			return
		}
		if _, ok := allowed[model.NormalizeGroupRole(member.Role)]; !ok {
			respondError(c, http.StatusForbidden, "forbidden")
			return
		}
		c.Next()
	}
}

// GroupMember は URL パラメータの groupID とコンテキストの userID から
// グループメンバーシップを確認するミドルウェア。BearerAuth の後段で使う。
func GroupMember(groupRepo repository.GroupRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := userIDFromContext(c)
		if !ok {
			respondError(c, http.StatusUnauthorized, "unauthorized")
			return
		}

		groupIDStr := c.Param("id")
		if groupIDStr == "" {
			respondError(c, http.StatusBadRequest, "missing group id")
			return
		}

		groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, "invalid group id")
			return
		}

		isMember, err := groupRepo.IsMember(c.Request.Context(), groupID, userID)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "internal server error")
			return
		}
		if !isMember {
			respondError(c, http.StatusForbidden, "forbidden")
			return
		}

		c.Next()
	}
}
