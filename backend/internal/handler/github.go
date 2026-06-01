package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/internal/service"
	githubpkg "github.com/irj0927/begit/pkg/github"
)

// RepoJSON は GitHub リポジトリレスポンス型
type RepoJSON struct {
	FullName   string `json:"full_name"`
	Name       string `json:"name"`
	Private    bool   `json:"private"`
	OwnerLogin string `json:"owner_login"`
	AvatarURL  string `json:"avatar_url"`
	CanPush    bool   `json:"can_push"`
	CanAdmin   bool   `json:"can_admin"`
}

// RepoListResponse は GET /github/repos のレスポンス
type RepoListResponse struct {
	Repos []RepoJSON `json:"repos"`
}

// GitHubHandler は GitHub プロキシエンドポイントのハンドラ
type GitHubHandler struct {
	githubService service.GitHubService
}

// NewGitHubHandler は GitHubHandler を作成する
func NewGitHubHandler(githubService service.GitHubService) *GitHubHandler {
	return &GitHubHandler{githubService: githubService}
}

// ListRepos は認証ユーザーの GitHub リポジトリ一覧を返す。
//
//	@Summary		GitHub リポジトリ一覧
//	@Description	認証ユーザーが push / admin 権限を持つリポジトリ一覧を返す。グループ作成時のリポジトリ選択に使う。
//	@Tags			github
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	RepoListResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		502	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/github/repos [get]
func (h *GitHubHandler) ListRepos(c *gin.Context) {
	if _, ok := userIDFromContext(c); !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	accessToken := accessTokenFromContext(c)

	repos, err := h.githubService.ListRepos(c.Request.Context(), accessToken)
	if err != nil {
		if errors.Is(err, service.ErrExternalAPI) {
			respondError(c, http.StatusBadGateway, "external api error")
			return
		}
		respondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	result := make([]RepoJSON, 0, len(repos))
	for _, r := range repos {
		result = append(result, RepoJSON{
			FullName:   r.FullName,
			Name:       r.Name,
			Private:    r.Private,
			OwnerLogin: r.OwnerLogin,
			AvatarURL:  r.AvatarURL,
			CanPush:    r.CanPush,
			CanAdmin:   r.CanAdmin,
		})
	}

	c.JSON(http.StatusOK, RepoListResponse{Repos: result})
}

// CommitJSON はコミットレスポンス型
type CommitJSON struct {
	SHA         string `json:"sha"`
	Message     string `json:"message"`
	AuthorName  string `json:"author_name"`
	AuthorLogin string `json:"author_login"`
	Date        string `json:"date"`
	Additions   int    `json:"additions"`
	Deletions   int    `json:"deletions"`
}

// CommitListResponse は GET /groups/:id/commits のレスポンス
type CommitListResponse struct {
	Commits []CommitJSON `json:"commits"`
}

// ListCommits はグループに紐づくリポジトリのコミット一覧を返す。
//
//	@Summary		コミット一覧
//	@Description	グループに紐づく GitHub リポジトリのコミット履歴を返す（GET /repos/{owner}/{repo}/commits のプロキシ）。
//	@Tags			github
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id			path		int		true	"グループ ID"
//	@Param			author		query		string	false	"author でフィルタ（login or email）"
//	@Param			since		query		string	false	"ISO8601 これ以降"
//	@Param			until		query		string	false	"ISO8601 これ以前"
//	@Param			per_page	query		int		false	"取得件数（1〜50、既定 20）"
//	@Success		200			{object}	CommitListResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		502			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Router			/groups/{id}/commits [get]
func (h *GitHubHandler) ListCommits(c *gin.Context) {
	if _, ok := userIDFromContext(c); !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid group id")
		return
	}

	accessToken := accessTokenFromContext(c)

	opts := githubpkg.CommitListOptions{
		Author: c.Query("author"),
		Since:  c.Query("since"),
		Until:  c.Query("until"),
	}
	if pp := c.Query("per_page"); pp != "" {
		if v, err := strconv.Atoi(pp); err == nil {
			opts.PerPage = v
		}
	}

	commits, err := h.githubService.ListGroupCommits(c.Request.Context(), groupID, accessToken, opts)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrExternalAPI):
			respondError(c, http.StatusBadGateway, "external api error")
		case errors.Is(err, service.ErrNotFound):
			respondError(c, http.StatusNotFound, "group not found")
		default:
			respondError(c, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	result := make([]CommitJSON, 0, len(commits))
	for _, cm := range commits {
		result = append(result, CommitJSON{
			SHA:         cm.SHA,
			Message:     cm.Message,
			AuthorName:  cm.AuthorName,
			AuthorLogin: cm.AuthorLogin,
			Date:        cm.Date,
			Additions:   cm.Additions,
			Deletions:   cm.Deletions,
		})
	}

	c.JSON(http.StatusOK, CommitListResponse{Commits: result})
}
