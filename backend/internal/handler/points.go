package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/irj0927/begit/internal/repository"
	"github.com/irj0927/begit/internal/service"
)

type PointsHandler struct{ points *service.PointsService }

// ProjectFinalResultJSON is the immutable project completion snapshot.
type ProjectFinalResultJSON = repository.FinalResult

func NewPointsHandler(points *service.PointsService) *PointsHandler {
	return &PointsHandler{points: points}
}

// Complete finalizes a project and returns its immutable result.
//
//	@Summary		作品を完成する
//	@Description	確認済み管理者のみ実行可。進行中のBeGit Timeがある場合は完了できず、再実行しても同じ集計を返す
//	@Tags			points
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"グループ ID"
//	@Success		200	{object}	ProjectFinalResultJSON
//	@Failure		401	{object}	ErrorResponse
//	@Failure		403	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		409	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/groups/{id}/complete [post]
func (h *PointsHandler) Complete(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid group id")
		return
	}
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	result, err := h.points.Complete(c.Request.Context(), groupID, userID)
	if errors.Is(err, service.ErrForbidden) {
		respondError(c, http.StatusForbidden, "forbidden")
		return
	}
	if errors.Is(err, service.ErrConflict) {
		respondError(c, http.StatusConflict, "active BeGit Time must end before completion")
		return
	}
	if errors.Is(err, service.ErrNotFound) {
		respondError(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to complete work")
		return
	}
	c.JSON(http.StatusOK, result)
}

// Result returns the saved project result to any member.
//
//	@Summary		完成リザルトを取得する
//	@Tags			points
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"グループ ID"
//	@Success		200	{object}	ProjectFinalResultJSON
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/groups/{id}/final-result [get]
func (h *PointsHandler) Result(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid group id")
		return
	}
	result, err := h.points.Result(c.Request.Context(), groupID)
	if errors.Is(err, service.ErrNotFound) {
		respondError(c, http.StatusNotFound, "final result is not available")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to load final result")
		return
	}
	c.JSON(http.StatusOK, result)
}
