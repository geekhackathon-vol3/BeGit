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

// CreatePostRequest は POST /groups/:id/posts のリクエストボディ
type CreatePostRequest struct {
	Body           *string `json:"body"`
	NotificationID *int64  `json:"notification_id"`
	GitHubLogin    string  `json:"github_login"`
	RepoFullName   string  `json:"repo_full_name"`
}

// PostJSON は投稿レスポンス型
type PostJSON struct {
	ID                  int64   `json:"id"`
	UserID              int64   `json:"user_id"`
	PostType            string  `json:"post_type"`
	Body                *string `json:"body"`
	RepoFullName        *string `json:"repo_full_name"`
	CommitCount         int     `json:"commit_count"`
	Additions           int     `json:"additions"`
	Deletions           int     `json:"deletions"`
	LatestCommitMessage *string `json:"latest_commit_message"`
	Status              *string `json:"status"`
	CreatedAt           string  `json:"created_at"`
}

// PostFeedJSON はフィードレスポンス型
type PostFeedJSON struct {
	PostJSON
	Login     string      `json:"login"`
	AvatarURL string      `json:"avatar_url"`
	Photos    []PhotoJSON `json:"photos"`
}

// PostListResponse は GET /groups/:id/posts のレスポンス
type PostListResponse struct {
	Posts []PostFeedJSON `json:"posts"`
}

// PostHandler は投稿エンドポイントのハンドラ
type PostHandler struct {
	postService service.PostService
}

// NewPostHandler は PostHandler を作成する
func NewPostHandler(postService service.PostService) *PostHandler {
	return &PostHandler{postService: postService}
}

// Create は投稿を作成する。
//
//	@Summary		投稿作成
//	@Tags			posts
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int					true	"グループ ID"
//	@Param			request	body		CreatePostRequest	true	"投稿内容"
//	@Success		201		{object}	PostJSON
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		502		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/groups/{id}/posts [post]
func (h *PostHandler) Create(c *gin.Context) {
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

	accessToken := accessTokenFromContext(c)

	var req CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	post, err := h.postService.CreatePost(c.Request.Context(), service.CreatePostRequest{
		Body:           req.Body,
		NotificationID: req.NotificationID,
		AccessToken:    accessToken,
		GitHubLogin:    req.GitHubLogin,
		RepoFullName:   req.RepoFullName,
	}, groupID, userID)
	if err != nil {
		if errors.Is(err, service.ErrExternalAPI) {
			respondError(c, http.StatusBadGateway, "external api error")
			return
		}
		respondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusCreated, PostJSON{
		ID:                  post.ID,
		UserID:              post.UserID,
		PostType:            post.PostType,
		Body:                post.Body,
		RepoFullName:        post.RepoFullName,
		CommitCount:         post.CommitCount,
		Additions:           post.Additions,
		Deletions:           post.Deletions,
		LatestCommitMessage: post.LatestCommitMessage,
		Status:              post.Status,
		CreatedAt:           post.CreatedAt.UTC().Format(time.RFC3339),
	})
}

// ConfirmPostRequest は POST /groups/:id/posts/:postId/confirm のリクエストボディ
type ConfirmPostRequest struct {
	Body *string `json:"body"`
}

// draftPostJSON は下書き取得/確定レスポンスを構築する
func draftPostJSON(p *model.Post) PostJSON {
	return PostJSON{
		ID:                  p.ID,
		UserID:              p.UserID,
		PostType:            p.PostType,
		Body:                p.Body,
		RepoFullName:        p.RepoFullName,
		CommitCount:         p.CommitCount,
		Additions:           p.Additions,
		Deletions:           p.Deletions,
		LatestCommitMessage: p.LatestCommitMessage,
		Status:              p.Status,
		CreatedAt:           p.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// GetDraft は下書き投稿を取得する（② Nice Work! プレフィル元）。
//
//	@Summary		下書き取得
//	@Description	② Nice Work! 通知の draft_post_id に対応する下書きを取得する
//	@Tags			posts
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int	true	"グループ ID"
//	@Param			postId	path		int	true	"投稿 ID"
//	@Success		200		{object}	PostJSON
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Router			/groups/{id}/posts/{postId}/draft [get]
func (h *PostHandler) GetDraft(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	groupID, postID, ok := parseGroupAndPostID(c)
	if !ok {
		return
	}

	post, err := h.postService.GetDraft(c.Request.Context(), groupID, postID, userID)
	if err != nil {
		respondPostDraftError(c, err)
		return
	}
	c.JSON(http.StatusOK, draftPostJSON(post))
}

// Confirm は下書きを確定（draft 解除）してフィードに表示可能にする。
//
//	@Summary		下書き確定
//	@Description	下書きを確定し is_draft を解除する。べき等。
//	@Tags			posts
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int					true	"グループ ID"
//	@Param			postId	path		int					true	"投稿 ID"
//	@Param			request	body		ConfirmPostRequest	false	"確定内容（本文の上書き等）"
//	@Success		200		{object}	PostJSON
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Router			/groups/{id}/posts/{postId}/confirm [post]
func (h *PostHandler) Confirm(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	groupID, postID, ok := parseGroupAndPostID(c)
	if !ok {
		return
	}

	var req ConfirmPostRequest
	// ボディは任意（空でも確定できる）。JSON パースエラーは 400 だが、空ボディは許可。
	if err := c.ShouldBindJSON(&req); err != nil {
		// 空リクエストボディの場合は err = EOF or io.EOF。それ以外は JSON パースエラー。
		// Gin の ShouldBindJSON は空ボディでも成功するため、実パースエラーのみ弾く。
		if c.Request.ContentLength > 0 {
			respondError(c, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	post, err := h.postService.ConfirmPost(c.Request.Context(), service.ConfirmPostRequest{Body: req.Body}, groupID, postID, userID)
	if err != nil {
		respondPostDraftError(c, err)
		return
	}
	c.JSON(http.StatusOK, draftPostJSON(post))
}

// respondPostDraftError は draft 系サービスエラーを HTTP ステータスにマップする
func respondPostDraftError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrForbidden):
		respondError(c, http.StatusForbidden, "forbidden")
	case errors.Is(err, service.ErrNotFound):
		respondError(c, http.StatusNotFound, "not found")
	default:
		respondError(c, http.StatusInternalServerError, "internal server error")
	}
}

// List はフィード（投稿一覧）を返す。
//
//	@Summary		フィード取得
//	@Tags			posts
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"グループ ID"
//	@Success		200	{object}	PostListResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/groups/{id}/posts [get]
func (h *PostHandler) List(c *gin.Context) {
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

	feeds, err := h.postService.ListPosts(c.Request.Context(), groupID, userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	result := make([]PostFeedJSON, 0, len(feeds))
	for _, feed := range feeds {
		photos := make([]PhotoJSON, 0, len(feed.Photos))
		for _, p := range feed.Photos {
			photos = append(photos, PhotoJSON{ID: p.ID, PhotoType: p.PhotoType, URL: p.URL})
		}
		result = append(result, PostFeedJSON{
			PostJSON: PostJSON{
				ID:                  feed.ID,
				UserID:              feed.UserID,
				PostType:            feed.PostType,
				Body:                feed.Body,
				RepoFullName:        feed.RepoFullName,
				CommitCount:         feed.CommitCount,
				Additions:           feed.Additions,
				Deletions:           feed.Deletions,
				LatestCommitMessage: feed.LatestCommitMessage,
				Status:              feed.Status,
				CreatedAt:           feed.CreatedAt.UTC().Format(time.RFC3339),
			},
			Login:     feed.Login,
			AvatarURL: feed.AvatarURL,
			Photos:    photos,
		})
	}

	c.JSON(http.StatusOK, PostListResponse{Posts: result})
}
