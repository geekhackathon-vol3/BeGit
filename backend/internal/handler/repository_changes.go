package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/irj0927/begit/internal/repository"
)

type RepositoryChangeJSON struct {
	Sequence   int64  `json:"sequence"`
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`
	PostID     *int64 `json:"post_id,omitempty"`
	Operation  string `json:"operation"`
}

type RepositoryChangesResponse struct {
	Cursor        int64                  `json:"cursor"`
	Changes       []RepositoryChangeJSON `json:"changes"`
	ResetRequired bool                   `json:"reset_required"`
	HasMore       bool                   `json:"has_more"`
}

type RepositoryChangeHandler struct {
	changes repository.RepositoryChangeRepository
}

func NewRepositoryChangeHandler(changes repository.RepositoryChangeRepository) *RepositoryChangeHandler {
	return &RepositoryChangeHandler{changes: changes}
}

// List はRepository内でカーソル以降に変更された行だけを返す。
//
//	@Summary		Repository差分取得
//	@Tags			repository-changes
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id path int true "グループ ID"
//	@Param			after query int false "前回カーソル" default(0)
//	@Param			limit query int false "取得件数（最大200）" default(100)
//	@Success		200 {object} RepositoryChangesResponse
//	@Router			/groups/{id}/changes [get]
func (h *RepositoryChangeHandler) List(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid group id")
		return
	}
	after, err := strconv.ParseInt(c.DefaultQuery("after", "0"), 10, 64)
	if err != nil || after < 0 {
		respondError(c, http.StatusBadRequest, "invalid cursor")
		return
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if err != nil || limit < 1 {
		respondError(c, http.StatusBadRequest, "invalid limit")
		return
	}
	if limit > 200 {
		limit = 200
	}
	changes, cursor, reset, err := h.changes.ListAfter(c.Request.Context(), groupID, after, limit+1)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "internal server error")
		return
	}
	hasMore := len(changes) > limit
	if hasMore {
		changes = changes[:limit]
		cursor = changes[len(changes)-1].Sequence
	}
	result := make([]RepositoryChangeJSON, 0, len(changes))
	for _, change := range changes {
		result = append(result, RepositoryChangeJSON{
			Sequence: change.Sequence, EntityType: change.EntityType, EntityID: change.EntityID,
			PostID: change.PostID, Operation: change.Operation,
		})
	}
	c.JSON(http.StatusOK, RepositoryChangesResponse{Cursor: cursor, Changes: result, ResetRequired: reset, HasMore: hasMore})
}
