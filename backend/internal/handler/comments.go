package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/service"
)

// CreateCommentRequest は POST /groups/:id/posts/:postId/comments のリクエストボディ
type CreateCommentRequest struct {
	Body string `json:"body"`
}

// CommentJSON はコメントレスポンス型
type CommentJSON struct {
	ID        int64  `json:"id"`
	PostID    int64  `json:"post_id"`
	UserID    int64  `json:"user_id"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

// CommentListResponse はコメント一覧レスポンス
type CommentListResponse struct {
	Comments []CommentJSON `json:"comments"`
}

// CommentHandler はコメントエンドポイントのハンドラ
type CommentHandler struct {
	commentService service.CommentService
}

// NewCommentHandler は CommentHandler を作成する
func NewCommentHandler(commentService service.CommentService) *CommentHandler {
	return &CommentHandler{commentService: commentService}
}

// toCommentJSON は model.Comment をレスポンス型へ変換する
func toCommentJSON(c model.Comment) CommentJSON {
	return CommentJSON{
		ID:        c.ID,
		PostID:    c.PostID,
		UserID:    c.UserID,
		Body:      c.Body,
		CreatedAt: c.CreatedAt.UTC().Format(time.RFC3339),
		Login:     c.Login,
		AvatarURL: c.AvatarURL,
	}
}

// Create はコメントを投稿する。
//
//	@Summary		コメント投稿
//	@Tags			comments
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int						true	"グループ ID"
//	@Param			postId	path		int						true	"投稿 ID"
//	@Param			request	body		CreateCommentRequest	true	"コメント本文"
//	@Success		201		{object}	CommentJSON
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/groups/{id}/posts/{postId}/comments [post]
func (h *CommentHandler) Create(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	groupID, postID, ok := parseGroupAndPostID(c)
	if !ok {
		return
	}

	var req CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Body == "" {
		respondError(c, http.StatusBadRequest, "body is required")
		return
	}

	comment, err := h.commentService.CreateComment(c.Request.Context(), groupID, postID, userID, req.Body)
	if err != nil {
		respondCommentError(c, err)
		return
	}

	c.JSON(http.StatusCreated, toCommentJSON(*comment))
}

// List は投稿のコメント一覧を返す。
//
//	@Summary		コメント一覧
//	@Tags			comments
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int	true	"グループ ID"
//	@Param			postId	path		int	true	"投稿 ID"
//	@Success		200		{object}	CommentListResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/groups/{id}/posts/{postId}/comments [get]
func (h *CommentHandler) List(c *gin.Context) {
	if _, ok := userIDFromContext(c); !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	groupID, postID, ok := parseGroupAndPostID(c)
	if !ok {
		return
	}

	comments, err := h.commentService.ListComments(c.Request.Context(), groupID, postID)
	if err != nil {
		respondCommentError(c, err)
		return
	}

	result := make([]CommentJSON, 0, len(comments))
	for _, cm := range comments {
		result = append(result, toCommentJSON(cm))
	}
	c.JSON(http.StatusOK, CommentListResponse{Comments: result})
}

// Delete はコメントを削除する（本人のみ）。
//
//	@Summary		コメント削除
//	@Tags			comments
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id			path	int	true	"グループ ID"
//	@Param			postId		path	int	true	"投稿 ID"
//	@Param			commentId	path	int	true	"コメント ID"
//	@Success		204
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		403	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/groups/{id}/posts/{postId}/comments/{commentId} [delete]
func (h *CommentHandler) Delete(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	groupID, postID, ok := parseGroupAndPostID(c)
	if !ok {
		return
	}

	commentID, err := strconv.ParseInt(c.Param("commentId"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid comment id")
		return
	}

	if err := h.commentService.DeleteComment(c.Request.Context(), groupID, postID, commentID, userID); err != nil {
		respondCommentError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// respondCommentError はサービス層のエラーを HTTP ステータスにマップする
func respondCommentError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrForbidden):
		respondError(c, http.StatusForbidden, "forbidden")
	case errors.Is(err, service.ErrNotFound):
		respondError(c, http.StatusNotFound, "not found")
	default:
		respondError(c, http.StatusInternalServerError, "internal server error")
	}
}
