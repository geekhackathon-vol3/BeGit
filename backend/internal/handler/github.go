package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/internal/service"
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
