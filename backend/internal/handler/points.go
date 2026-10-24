package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/irj0927/begit/internal/service"
)

type PointsHandler struct{ points *service.PointsService }

func NewPointsHandler(points *service.PointsService) *PointsHandler {
	return &PointsHandler{points: points}
}

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
