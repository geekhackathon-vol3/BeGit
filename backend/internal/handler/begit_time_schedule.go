package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
	"github.com/irj0927/begit/internal/service"
)

type BeGitTimeScheduleJSON struct {
	Enabled   bool   `json:"enabled"`
	LocalTime string `json:"local_time"`
	Timezone  string `json:"timezone"`
	Weekdays  []int  `json:"weekdays"`
	NextRunAt string `json:"next_run_at,omitempty"`
}

type UpdateBeGitTimeScheduleRequest struct {
	Enabled   bool   `json:"enabled"`
	LocalTime string `json:"local_time" binding:"required"`
	Timezone  string `json:"timezone" binding:"required"`
	Weekdays  []int  `json:"weekdays" binding:"required"`
}

type BeGitTimeScheduleHandler struct {
	service service.BeGitTimeScheduleService
}

func NewBeGitTimeScheduleHandler(service service.BeGitTimeScheduleService) *BeGitTimeScheduleHandler {
	return &BeGitTimeScheduleHandler{service: service}
}

func beGitTimeScheduleJSON(schedule *model.BeGitTimeSchedule) BeGitTimeScheduleJSON {
	result := BeGitTimeScheduleJSON{
		Enabled: schedule.Enabled, LocalTime: schedule.LocalTime,
		Timezone: schedule.Timezone, Weekdays: schedule.Weekdays,
	}
	if schedule.NextRunAt != nil {
		result.NextRunAt = schedule.NextRunAt.UTC().Format(time.RFC3339)
	}
	return result
}

func parseScheduleGroupID(c *gin.Context) (int64, bool) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || groupID <= 0 {
		respondError(c, http.StatusBadRequest, "invalid group id")
		return 0, false
	}
	return groupID, true
}

// Get returns the automatic BeGit Time schedule. Only the repository owner can view it.
//
//	@Summary		BeGit Time 自動開催設定を取得
//	@Tags			begit-time-schedule
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"グループ ID"
//	@Success		200	{object}	BeGitTimeScheduleJSON
//	@Failure		403	{object}	ErrorResponse
//	@Router			/groups/{id}/begit-time-schedule [get]
func (h *BeGitTimeScheduleHandler) Get(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	groupID, ok := parseScheduleGroupID(c)
	if !ok {
		return
	}
	schedule, err := h.service.Get(c.Request.Context(), groupID, userID)
	if err != nil {
		respondScheduleError(c, err)
		return
	}
	c.JSON(http.StatusOK, beGitTimeScheduleJSON(schedule))
}

// Update creates or replaces the automatic BeGit Time schedule.
//
//	@Summary		BeGit Time 自動開催設定を更新
//	@Tags			begit-time-schedule
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int							true	"グループ ID"
//	@Param			request	body		UpdateBeGitTimeScheduleRequest	true	"自動開催設定"
//	@Success		200		{object}	BeGitTimeScheduleJSON
//	@Failure		422		{object}	ErrorResponse
//	@Router			/groups/{id}/begit-time-schedule [put]
func (h *BeGitTimeScheduleHandler) Update(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	groupID, ok := parseScheduleGroupID(c)
	if !ok {
		return
	}
	var request UpdateBeGitTimeScheduleRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}
	schedule, err := h.service.Update(c.Request.Context(), groupID, userID, service.UpdateBeGitTimeScheduleInput{
		Enabled: request.Enabled, LocalTime: request.LocalTime,
		Timezone: request.Timezone, Weekdays: request.Weekdays,
	})
	if err != nil {
		respondScheduleError(c, err)
		return
	}
	c.JSON(http.StatusOK, beGitTimeScheduleJSON(schedule))
}

func respondScheduleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrForbidden):
		respondError(c, http.StatusForbidden, "only the repository owner can manage the schedule")
	case errors.Is(err, service.ErrNotFound), errors.Is(err, repository.ErrNotFound):
		respondError(c, http.StatusNotFound, "group not found")
	case errors.Is(err, service.ErrValidation):
		respondError(c, http.StatusUnprocessableEntity, err.Error())
	default:
		respondError(c, http.StatusInternalServerError, "internal server error")
	}
}
