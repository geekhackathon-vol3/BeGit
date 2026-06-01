package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/service"
)

// CreateReactionRequest は POST /groups/:id/posts/:postId/reactions のリクエストボディ
type CreateReactionRequest struct {
	ReactionType string `json:"reaction_type"`
}

// ReactionJSON はリアクションレスポンス型
type ReactionJSON struct {
	ID           int64  `json:"id"`
	PostID       int64  `json:"post_id"`
	UserID       int64  `json:"user_id"`
	ReactionType string `json:"reaction_type"`
	Login        string `json:"login"`
	AvatarURL    string `json:"avatar_url"`
}

// ReactionListResponse はリアクション一覧レスポンス
type ReactionListResponse struct {
	Reactions []ReactionJSON `json:"reactions"`
}

// ReactionHandler はリアクションエンドポイントのハンドラ
type ReactionHandler struct {
	reactionService service.ReactionService
}

// NewReactionHandler は ReactionHandler を作成する
func NewReactionHandler(reactionService service.ReactionService) *ReactionHandler {
	return &ReactionHandler{reactionService: reactionService}
}

// toReactionJSON は model.Reaction のスライスをレスポンス型へ変換する
func toReactionJSON(reactions []model.Reaction) ReactionListResponse {
	result := make([]ReactionJSON, 0, len(reactions))
	for _, r := range reactions {
		result = append(result, ReactionJSON{
			ID:           r.ID,
			PostID:       r.PostID,
			UserID:       r.UserID,
			ReactionType: r.ReactionType,
			Login:        r.Login,
			AvatarURL:    r.AvatarURL,
		})
	}
	return ReactionListResponse{Reactions: result}
}

// parseGroupAndPostID は URL パラメータ id / postId をパースする
func parseGroupAndPostID(c *gin.Context) (groupID, postID int64, ok bool) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid group id")
		return 0, 0, false
	}
	postID, err = strconv.ParseInt(c.Param("postId"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid post id")
		return 0, 0, false
	}
	return groupID, postID, true
}

// Create はリアクションを追加する。
//
//	@Summary		リアクション追加
//	@Tags			reactions
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int						true	"グループ ID"
//	@Param			postId	path		int						true	"投稿 ID"
//	@Param			request	body		CreateReactionRequest	true	"リアクション種別"
//	@Success		201		{object}	ReactionListResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/groups/{id}/posts/{postId}/reactions [post]
func (h *ReactionHandler) Create(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	groupID, postID, ok := parseGroupAndPostID(c)
	if !ok {
		return
	}

	var req CreateReactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ReactionType == "" {
		respondError(c, http.StatusBadRequest, "reaction_type is required")
		return
	}

	reactions, err := h.reactionService.AddReaction(c.Request.Context(), groupID, postID, userID, req.ReactionType)
	if err != nil {
		respondReactionError(c, err)
		return
	}

	c.JSON(http.StatusCreated, toReactionJSON(reactions))
}

// Delete はリアクションを削除する（トグル用）。
//
//	@Summary		リアクション削除
//	@Tags			reactions
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id				path		int		true	"グループ ID"
//	@Param			postId			path		int		true	"投稿 ID"
//	@Param			reactionType	path		string	true	"リアクション種別"
//	@Success		200				{object}	ReactionListResponse
//	@Failure		400				{object}	ErrorResponse
//	@Failure		401				{object}	ErrorResponse
//	@Failure		403				{object}	ErrorResponse
//	@Failure		404				{object}	ErrorResponse
//	@Failure		500				{object}	ErrorResponse
//	@Router			/groups/{id}/posts/{postId}/reactions/{reactionType} [delete]
func (h *ReactionHandler) Delete(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	groupID, postID, ok := parseGroupAndPostID(c)
	if !ok {
		return
	}

	reactionType := c.Param("reactionType")
	if reactionType == "" {
		respondError(c, http.StatusBadRequest, "reaction_type is required")
		return
	}

	reactions, err := h.reactionService.RemoveReaction(c.Request.Context(), groupID, postID, userID, reactionType)
	if err != nil {
		respondReactionError(c, err)
		return
	}

	c.JSON(http.StatusOK, toReactionJSON(reactions))
}

// List は投稿のリアクション一覧を返す。
//
//	@Summary		リアクション一覧
//	@Tags			reactions
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int	true	"グループ ID"
//	@Param			postId	path		int	true	"投稿 ID"
//	@Success		200		{object}	ReactionListResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/groups/{id}/posts/{postId}/reactions [get]
func (h *ReactionHandler) List(c *gin.Context) {
	if _, ok := userIDFromContext(c); !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	groupID, postID, ok := parseGroupAndPostID(c)
	if !ok {
		return
	}

	reactions, err := h.reactionService.ListReactions(c.Request.Context(), groupID, postID)
	if err != nil {
		respondReactionError(c, err)
		return
	}

	c.JSON(http.StatusOK, toReactionJSON(reactions))
}

// respondReactionError はサービス層のエラーを HTTP ステータスにマップする
func respondReactionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		respondError(c, http.StatusNotFound, "post not found")
	default:
		respondError(c, http.StatusInternalServerError, "internal server error")
	}
}
