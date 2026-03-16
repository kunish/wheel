package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
)

// ──── Log Routes ────

// ListLogs godoc
// @Summary List relay logs with pagination and filters
// @Tags Logs
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size (max 200)" default(50)
// @Param channelId query int false "Filter by channel ID"
// @Param startTime query int false "Filter by start timestamp"
// @Param endTime query int false "Filter by end timestamp"
// @Param model query string false "Filter by model name"
// @Param keyword query string false "Search keyword"
// @Param status query string false "Filter by status (error|success)"
// @Success 200 {object} object "{success: true, data: {logs, total, page, pageSize}}"
// @Security BearerAuth
// @Router /api/v1/log/list [get]
func (h *Handler) ListLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	if pageSize > 200 {
		pageSize = 200
	}
	channelID, _ := strconv.Atoi(c.DefaultQuery("channelId", "0"))
	startTime, _ := strconv.ParseInt(c.DefaultQuery("startTime", "0"), 10, 64)
	endTime, _ := strconv.ParseInt(c.DefaultQuery("endTime", "0"), 10, 64)
	model := c.Query("model")
	keyword := c.Query("keyword")

	status := c.Query("status")
	var hasError *bool
	if status == "error" {
		t := true
		hasError = &t
	} else if status == "success" {
		f := false
		hasError = &f
	}

	opts := dal.ListLogsOpts{
		Page:      page,
		PageSize:  pageSize,
		Model:     model,
		ChannelID: channelID,
		HasError:  hasError,
		StartTime: startTime,
		EndTime:   endTime,
		Keyword:   keyword,
	}

	logs, total, stats, err := dal.ListLogs(c.Request.Context(), h.DB, opts)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	successJSON(c, gin.H{
		"logs":     logs,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
		"stats":    stats,
	})
}

// GetLog godoc
// @Summary Get a single relay log by ID
// @Tags Logs
// @Produce json
// @Param id path int true "Log ID"
// @Success 200 {object} object "{success: true, data: RelayLog}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Failure 404 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/log/{id} [get]
func (h *Handler) GetLog(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid log ID")
		return
	}

	log, err := dal.GetLog(c.Request.Context(), h.DB, id)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	if log == nil {
		errorJSON(c, http.StatusNotFound, "Log not found")
		return
	}

	successJSON(c, log)
}

// DeleteLog godoc
// @Summary Delete a relay log
// @Tags Logs
// @Produce json
// @Param id path int true "Log ID"
// @Success 200 {object} object "{success: true}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/log/delete/{id} [delete]
func (h *Handler) DeleteLog(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid log ID")
		return
	}

	if err := dal.DeleteLog(c.Request.Context(), h.DB, id); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successNoData(c)
}

// ClearLogs godoc
// @Summary Clear all relay logs
// @Tags Logs
// @Produce json
// @Success 200 {object} object "{success: true}"
// @Security BearerAuth
// @Router /api/v1/log/clear [delete]
func (h *Handler) ClearLogs(c *gin.Context) {
	if err := dal.ClearLogs(c.Request.Context(), h.DB); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successNoData(c)
}

// ReplayLog godoc
// @Summary Replay a logged request (not yet implemented)
// @Tags Logs
// @Produce json
// @Param id path int true "Log ID"
// @Success 200 {object} object "{success: true}"
// @Failure 501 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/log/replay/{id} [post]
func (h *Handler) ReplayLog(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid log ID")
		return
	}

	log, err := dal.GetLog(c.Request.Context(), h.DB, id)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	if log == nil {
		errorJSON(c, http.StatusNotFound, "Log not found")
		return
	}

	// Stub: replay requires relay engine from Phase 3
	errorJSON(c, http.StatusNotImplemented, "Replay not yet implemented — requires relay engine")
}
