package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/irj0927/begit/internal/service"
)

// GroupJSON は GET /groups レスポンスのグループ型
type GroupJSON struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	RepoFullName string `json:"repo_full_name"`
	AvatarURL    string `json:"avatar_url"`
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

// groupHandler は GroupHandler の実装
type groupHandler struct {
	groupService service.GroupService
}

// NewGroupHandler は GroupHandler を作成する
func NewGroupHandler(groupService service.GroupService) http.Handler {
	return &groupHandler{groupService: groupService}
}

// ServeHTTP は /groups と /groups/:id を処理する
func (h *groupHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// URL に ID が含まれるかどうかで分岐
	id := r.PathValue("id")
	if id != "" {
		switch r.Method {
		case http.MethodGet:
			h.getGroup(w, r, id)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.listGroups(w, r)
	case http.MethodPost:
		h.createGroup(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// listGroups は GET /groups を処理する
func (h *groupHandler) listGroups(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(int64)
	if !ok || userID == 0 {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	groups, err := h.groupService.ListGroups(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
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

	json.NewEncoder(w).Encode(map[string]interface{}{"groups": result})
}

// createGroup は POST /groups を処理する
func (h *groupHandler) createGroup(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(int64)
	if !ok || userID == 0 {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	accessToken, _ := r.Context().Value(AccessTokenKey).(string)

	var req struct {
		RepoFullName string `json:"repo_full_name"`
		Name         string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.RepoFullName == "" {
		writeError(w, http.StatusUnprocessableEntity, "repo_full_name: required")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "name: required")
		return
	}

	group, err := h.groupService.CreateGroup(r.Context(), service.CreateGroupRequest{
		RepoFullName: req.RepoFullName,
		Name:         req.Name,
		AccessToken:  accessToken,
	}, userID)
	if err != nil {
		if errors.Is(err, service.ErrExternalAPI) {
			writeError(w, http.StatusBadGateway, "external api error")
			return
		}
		if errors.Is(err, service.ErrConflict) {
			writeError(w, http.StatusConflict, "conflict")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(GroupJSON{
		ID:           group.ID,
		Name:         group.Name,
		RepoFullName: group.RepoFullName,
		AvatarURL:    group.AvatarURL,
	})
}

// getGroup は GET /groups/:id を処理する
func (h *groupHandler) getGroup(w http.ResponseWriter, r *http.Request, idStr string) {
	userID, ok := r.Context().Value(UserIDKey).(int64)
	if !ok || userID == 0 {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	groupID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid group id")
		return
	}

	detail, err := h.groupService.GetGroup(r.Context(), groupID, userID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		if errors.Is(err, service.ErrForbidden) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
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

	json.NewEncoder(w).Encode(GroupDetailJSON{
		GroupJSON: GroupJSON{
			ID:           detail.ID,
			Name:         detail.Name,
			RepoFullName: detail.RepoFullName,
			AvatarURL:    detail.AvatarURL,
		},
		Members: members,
	})
}
