package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/service"
)

// stubReactionService は AddReaction だけを差し替えられる ReactionService
type stubReactionService struct {
	service.ReactionService
	addErr error
}

func (s *stubReactionService) AddReaction(ctx context.Context, groupID, postID, userID int64, reactionType string) ([]model.Reaction, error) {
	return nil, s.addErr
}

// 不正なリアクションの種類は入力の誤りなので 400（今までは 500 だった。#139）
func TestReactionHandler_Create_InvalidTypeIsBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewReactionHandler(&stubReactionService{addErr: service.ErrInvalidReactionType})
	r.POST("/groups/:id/posts/:postId/reactions", func(c *gin.Context) {
		c.Set(ctxUserID, int64(1))
		c.Next()
	}, h.Create)

	req := httptest.NewRequest(http.MethodPost, "/groups/5/posts/42/reactions", strings.NewReader(`{"reaction_type":"lgtm"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), `"invalid reaction type"`) {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
}
