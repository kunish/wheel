package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
)

// ──── Log Routes ────

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

	logs, total, err := dal.ListLogs(c.Request.Context(), h.LogDB, opts)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	successJSON(c, gin.H{
		"logs":     logs,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (h *Handler) GetLog(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid log ID")
		return
	}

	log, err := dal.GetLog(c.Request.Context(), h.LogDB, id)
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

func (h *Handler) DeleteLog(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid log ID")
		return
	}

	if err := dal.DeleteLog(c.Request.Context(), h.LogDB, id); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successNoData(c)
}

func (h *Handler) ClearLogs(c *gin.Context) {
	if err := dal.ClearLogs(c.Request.Context(), h.LogDB); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successNoData(c)
}

// ReplayLog replays a logged request. Stubbed for now — full implementation
// requires the relay proxy engine (Phase 3).
func (h *Handler) ReplayLog(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid log ID")
		return
	}

	log, err := dal.GetLog(c.Request.Context(), h.LogDB, id)
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
