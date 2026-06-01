package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/internal/service"
)

// CreateGroupRequest は POST /groups のリクエストボディ
type CreateGroupRequest struct {
	RepoFullName string `json:"repo_full_name" example:"owner/repo"`
	Name         string `json:"name" example:"My Repo"`
}

// GroupJSON は GET /groups レスポンスのグループ型
type GroupJSON struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	RepoFullName string `json:"repo_full_name"`
	AvatarURL    string `json:"avatar_url"`
}

// GroupListResponse は GET /groups のレスポンス
type GroupListResponse struct {
	Groups []GroupJSON `json:"groups"`
}

// GroupDetailJSON は GET /groups/:id レスポンスの詳細型
type GroupDetailJSON struct {
	GroupJSON
	Members []GroupMemberJSON `json:"members"`
}

// GroupMemberJSON はグループメンバーの JSON 型
type GroupMemberJSON struct {
	UserID    int64  `json:"user_id"`
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
	Role      string `json:"role"`
}

// MemberListResponse は POST /groups/:id/sync-members のレスポンス
type MemberListResponse struct {
	Members []GroupMemberJSON `json:"members"`
}

// GroupHandler はグループ（リポジトリ）エンドポイントのハンドラ
type GroupHandler struct {
	groupService service.GroupService
}

// NewGroupHandler は GroupHandler を作成する
func NewGroupHandler(groupService service.GroupService) *GroupHandler {
	return &GroupHandler{groupService: groupService}
}

// List は参加グループ一覧を返す。
//
//	@Summary		参加グループ（リポジトリ）一覧
//	@Tags			groups
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	GroupListResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/groups [get]
func (h *GroupHandler) List(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	groups, err := h.groupService.ListGroups(c.Request.Context(), userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	result := make([]GroupJSON, 0, len(groups))
	for _, g := range groups {
		result = append(result, GroupJSON{
			ID:           g.ID,
			Name:         g.Name,
			RepoFullName: g.RepoFullName,
			AvatarURL:    g.AvatarURL,
		})
	}

	c.JSON(http.StatusOK, GroupListResponse{Groups: result})
}

// Create はグループ（リポジトリ登録 + Webhook 登録）を作成する。
//
//	@Summary		グループ作成
//	@Description	GitHub リポジトリを登録し Webhook を設定する
//	@Tags			groups
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		CreateGroupRequest	true	"作成するリポジトリ"
//	@Success		201		{object}	GroupJSON
//	@Failure		401		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse
//	@Failure		422		{object}	ErrorResponse
//	@Failure		502		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/groups [post]
func (h *GroupHandler) Create(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	accessToken := accessTokenFromContext(c)

	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.RepoFullName == "" {
		respondError(c, http.StatusUnprocessableEntity, "repo_full_name: required")
		return
	}
	if req.Name == "" {
		respondError(c, http.StatusUnprocessableEntity, "name: required")
		return
	}

	group, err := h.groupService.CreateGroup(c.Request.Context(), service.CreateGroupRequest{
		RepoFullName: req.RepoFullName,
		Name:         req.Name,
		AccessToken:  accessToken,
	}, userID)
	if err != nil {
		if errors.Is(err, service.ErrExternalAPI) {
			respondError(c, http.StatusBadGateway, "external api error")
			return
		}
		if errors.Is(err, service.ErrConflict) {
			respondError(c, http.StatusConflict, "conflict")
			return
		}
		respondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusCreated, GroupJSON{
		ID:           group.ID,
		Name:         group.Name,
		RepoFullName: group.RepoFullName,
		AvatarURL:    group.AvatarURL,
	})
}

// Get はグループ詳細とメンバー一覧を返す。
//
//	@Summary		グループ詳細 + メンバー
//	@Tags			groups
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"グループ ID"
//	@Success		200	{object}	GroupDetailJSON
//	@Failure		401	{object}	ErrorResponse
//	@Failure		403	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/groups/{id} [get]
func (h *GroupHandler) Get(c *gin.Context) {
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

	detail, err := h.groupService.GetGroup(c.Request.Context(), groupID, userID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			respondError(c, http.StatusNotFound, "not found")
			return
		}
		if errors.Is(err, service.ErrForbidden) {
			respondError(c, http.StatusForbidden, "forbidden")
			return
		}
		respondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	members := make([]GroupMemberJSON, 0, len(detail.Members))
	for _, m := range detail.Members {
		members = append(members, GroupMemberJSON{
			UserID:    m.UserID,
			Login:     m.Login,
			AvatarURL: m.AvatarURL,
			Role:      m.Role,
		})
	}

	c.JSON(http.StatusOK, GroupDetailJSON{
		GroupJSON: GroupJSON{
			ID:           detail.ID,
			Name:         detail.Name,
			RepoFullName: detail.RepoFullName,
			AvatarURL:    detail.AvatarURL,
		},
		Members: members,
	})
}

// SyncMembers は GitHub コラボレーターとグループメンバーを同期する。
//
//	@Summary		メンバー同期
//	@Description	GitHub コラボレーターを取得し、BeGit 登録済みユーザーをグループに追加（加算的）して最新のメンバー一覧を返す。
//	@Tags			groups
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"グループ ID"
//	@Success		200	{object}	MemberListResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		403	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		502	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/groups/{id}/sync-members [post]
func (h *GroupHandler) SyncMembers(c *gin.Context) {
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

	members, err := h.groupService.SyncMembers(c.Request.Context(), groupID, accessToken)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrExternalAPI), errors.Is(err, service.ErrUnauthorized):
			respondError(c, http.StatusBadGateway, "external api error")
		case errors.Is(err, service.ErrNotFound):
			respondError(c, http.StatusNotFound, "not found")
		default:
			respondError(c, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	result := make([]GroupMemberJSON, 0, len(members))
	for _, m := range members {
		result = append(result, GroupMemberJSON{
			UserID:    m.UserID,
			Login:     m.Login,
			AvatarURL: m.AvatarURL,
			Role:      m.Role,
		})
	}

	c.JSON(http.StatusOK, MemberListResponse{Members: result})
}
