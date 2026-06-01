package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

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
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
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
		})
	}

	c.JSON(http.StatusOK, PostListResponse{Posts: result})
}
