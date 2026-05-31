package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/irj0927/begit/internal/service"
)

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

// postHandler は PostHandler の実装
type postHandler struct {
	postService service.PostService
}

// NewPostHandler は PostHandler を作成する
func NewPostHandler(postService service.PostService) http.Handler {
	return &postHandler{postService: postService}
}

// ServeHTTP は /groups/:id/posts を処理する
func (h *postHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID, ok := r.Context().Value(UserIDKey).(int64)
	if !ok || userID == 0 {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	groupIDStr := r.PathValue("id")
	groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid group id")
		return
	}

	switch r.Method {
	case http.MethodPost:
		h.createPost(w, r, groupID, userID)
	case http.MethodGet:
		h.listPosts(w, r, groupID, userID)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// createPost は POST /groups/:id/posts を処理する
func (h *postHandler) createPost(w http.ResponseWriter, r *http.Request, groupID, userID int64) {
	accessToken, _ := r.Context().Value(AccessTokenKey).(string)

	var req struct {
		Body           *string `json:"body"`
		NotificationID *int64  `json:"notification_id"`
		GitHubLogin    string  `json:"github_login"`
		RepoFullName   string  `json:"repo_full_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	post, err := h.postService.CreatePost(r.Context(), service.CreatePostRequest{
		Body:           req.Body,
		NotificationID: req.NotificationID,
		AccessToken:    accessToken,
		GitHubLogin:    req.GitHubLogin,
		RepoFullName:   req.RepoFullName,
	}, groupID, userID)
	if err != nil {
		if errors.Is(err, service.ErrExternalAPI) {
			writeError(w, http.StatusBadGateway, "external api error")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(PostJSON{
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
		CreatedAt:           post.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// listPosts は GET /groups/:id/posts を処理する
func (h *postHandler) listPosts(w http.ResponseWriter, r *http.Request, groupID, userID int64) {
	feeds, err := h.postService.ListPosts(r.Context(), groupID, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
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
				CreatedAt:           feed.CreatedAt.Format("2006-01-02T15:04:05Z"),
			},
			Login:     feed.Login,
			AvatarURL: feed.AvatarURL,
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"posts": result})
}
