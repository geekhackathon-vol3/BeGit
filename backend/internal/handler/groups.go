package handler

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/service"
)

// CreateGroupRequest は POST /groups のリクエストボディ
type CreateGroupRequest struct {
	RepoFullName   string `json:"repo_full_name" example:"owner/repo"`
	Name           string `json:"name" example:"My Repo"`
	InstallationID int64  `json:"installation_id,omitempty" example:"160548256"`
	ReadOnly       bool   `json:"read_only,omitempty" example:"true"`
}

// GroupJSON は GET /groups レスポンスのグループ型
type GroupJSON struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	RepoFullName string `json:"repo_full_name"`
	AvatarURL    string `json:"avatar_url"`
	ReadOnly     bool   `json:"read_only"`
	MemberCount  int    `json:"member_count,omitempty"`
}

// GroupListResponse は GET /groups のレスポンス
type GroupListResponse struct {
	Groups []GroupDetailJSON `json:"groups"`
}

// GroupDetailJSON は GET /groups/:id レスポンスの詳細型
type GroupDetailJSON struct {
	GroupJSON
	Members []GroupMemberJSON `json:"members"`
	// ActiveChallenge は進行中の BeGit Time チャレンジ。無ければ省略（GET /groups/:id のみ。一覧では返さない）
	ActiveChallenge *ActiveChallengeJSON `json:"active_challenge,omitempty"`
}

// ActiveChallengeJSON は進行中の BeGit Time チャレンジ（GET /groups/:id の active_challenge）
type ActiveChallengeJSON struct {
	NotificationID int64                     `json:"notification_id"`
	SprintID       int64                     `json:"sprint_id"`
	SentBy         ActiveChallengeIssuerJSON `json:"sent_by"`
	SentAt         string                    `json:"sent_at" example:"2026-09-18T01:25:46Z"`
	// EndsAt は締め切り（sent_at + 1h）。残り時間の表示に使う
	EndsAt string `json:"ends_at" example:"2026-09-18T02:25:46Z"`
	// CanEnd は要求ユーザーが発行者で、途中中断（POST /groups/:id/notifications/:nid/end）できるか
	CanEnd bool `json:"can_end"`
	// MyPost は要求ユーザー自身のこの通知への投稿。無ければ省略。is_draft=true なら投稿作成（カメラ）へ誘導できる
	MyPost *ActiveChallengePostJSON `json:"my_post,omitempty"`
}

// ActiveChallengeIssuerJSON は進行中チャレンジの発行者
type ActiveChallengeIssuerJSON struct {
	UserID int64 `json:"user_id"`
	// Login は発行者がグループを離脱済みの場合は空文字
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

// ActiveChallengePostJSON は進行中チャレンジに対する要求ユーザー自身の投稿
type ActiveChallengePostJSON struct {
	PostID  int64 `json:"post_id"`
	IsDraft bool  `json:"is_draft"`
	// Status は "on_time" | "late" | "missed"。未設定なら省略
	Status *string `json:"status,omitempty" example:"on_time"`
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
	// notificationService は GET /groups/:id の active_challenge 取得に使う（nil 可: その場合は省略）
	notificationService service.NotificationService
}

// NewGroupHandler は GroupHandler を作成する
func NewGroupHandler(groupService service.GroupService, notificationService service.NotificationService) *GroupHandler {
	return &GroupHandler{groupService: groupService, notificationService: notificationService}
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

	result := make([]GroupDetailJSON, 0, len(groups))
	for _, g := range groups {
		result = append(result, toGroupDetailJSON(g))
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
	if req.InstallationID < 0 {
		respondError(c, http.StatusUnprocessableEntity, "installation_id: must be positive")
		return
	}

	group, err := h.groupService.CreateGroup(c.Request.Context(), service.CreateGroupRequest{
		RepoFullName:   req.RepoFullName,
		Name:           req.Name,
		InstallationID: req.InstallationID,
		ReadOnly:       req.ReadOnly,
		AccessToken:    accessToken,
	}, userID)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorized) {
			respondError(c, http.StatusUnauthorized, "GitHub authentication required")
			return
		}
		if errors.Is(err, service.ErrForbidden) {
			respondError(c, http.StatusForbidden, "GitHub App installation has no access to this repository")
			return
		}
		if errors.Is(err, service.ErrExternalAPI) {
			respondError(c, http.StatusBadGateway, "external api error")
			return
		}
		if errors.Is(err, service.ErrValidation) {
			respondError(c, http.StatusUnprocessableEntity, "invalid group request")
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
		ReadOnly:     group.ReadOnly,
	})
}

// Delete はログイン中ユーザーのHOMEからグループを削除する。
//
//	@Summary		HOMEからリポジトリを削除
//	@Description	ログイン中ユーザーをグループから外す。GitHub上のリポジトリと他ユーザーの参加状態は変更しない。
//	@Tags			groups
//	@Security		BearerAuth
//	@Param			id	path	int	true	"グループ ID"
//	@Success		204
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/groups/{id} [delete]
func (h *GroupHandler) Delete(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || groupID <= 0 {
		respondError(c, http.StatusBadRequest, "invalid group id")
		return
	}

	if err := h.groupService.LeaveGroup(c.Request.Context(), groupID, userID); err != nil {
		respondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.Status(http.StatusNoContent)
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

	resp := toGroupDetailJSON(*detail)

	// 進行中の BeGit Time チャレンジ（ベストエフォート: 取得失敗はログのみで、グループ詳細は返す）
	if h.notificationService != nil {
		active, err := h.notificationService.GetActiveChallenge(c.Request.Context(), groupID, userID)
		if err != nil {
			log.Printf("groups: GetActiveChallenge failed for group %d: %v", groupID, err)
		} else if active != nil {
			resp.ActiveChallenge = toActiveChallengeJSON(active, detail.Members, userID)
		}
	}

	c.JSON(http.StatusOK, resp)
}

// toActiveChallengeJSON は進行中チャレンジをレスポンス型へ変換する。
// 発行者の login / avatar は既に取得済みのメンバー一覧から引く（離脱済みなら空文字）。
func toActiveChallengeJSON(active *service.ActiveChallenge, members []model.GroupMember, userID int64) *ActiveChallengeJSON {
	notif := active.Notification
	issuer := ActiveChallengeIssuerJSON{UserID: notif.SentBy}
	for _, m := range members {
		if m.UserID == notif.SentBy {
			issuer.Login = m.Login
			issuer.AvatarURL = m.AvatarURL
			break
		}
	}

	out := &ActiveChallengeJSON{
		NotificationID: notif.ID,
		SprintID:       notif.SprintID,
		SentBy:         issuer,
		SentAt:         notif.SentAt.UTC().Format(time.RFC3339),
		EndsAt:         active.EndsAt.UTC().Format(time.RFC3339),
		CanEnd:         notif.SentBy == userID,
	}
	if active.MyPost != nil {
		out.MyPost = &ActiveChallengePostJSON{
			PostID:  active.MyPost.ID,
			IsDraft: active.MyPost.IsDraft,
			Status:  active.MyPost.Status,
		}
	}
	return out
}

func toGroupDetailJSON(detail service.GroupDetail) GroupDetailJSON {
	members := make([]GroupMemberJSON, 0, len(detail.Members))
	for _, m := range detail.Members {
		members = append(members, GroupMemberJSON{
			UserID:    m.UserID,
			Login:     m.Login,
			AvatarURL: m.AvatarURL,
			Role:      m.Role,
		})
	}

	return GroupDetailJSON{
		GroupJSON: GroupJSON{
			ID:           detail.ID,
			Name:         detail.Name,
			RepoFullName: detail.RepoFullName,
			AvatarURL:    detail.AvatarURL,
			ReadOnly:     detail.ReadOnly,
			MemberCount:  len(members),
		},
		Members: members,
	}
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
