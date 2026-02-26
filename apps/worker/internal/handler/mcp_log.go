package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// ──── MCP Log Routes ────

// ListMCPLogs godoc
// @Summary List MCP tool call logs with pagination and filters
// @Tags MCP Logs
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size (max 200)" default(50)
// @Param clientId query int false "Filter by client ID"
// @Param toolName query string false "Filter by tool name"
// @Param status query string false "Filter by status"
// @Param startTime query int false "Filter by start timestamp"
// @Param endTime query int false "Filter by end timestamp"
// @Success 200 {object} object "{success: true, data: {logs, total, page, pageSize}}"
// @Security BearerAuth
// @Router /api/v1/mcp-log/list [get]
func (h *Handler) ListMCPLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	clientID, _ := strconv.Atoi(c.DefaultQuery("clientId", "0"))
	startTime, _ := strconv.ParseInt(c.DefaultQuery("startTime", "0"), 10, 64)
	endTime, _ := strconv.ParseInt(c.DefaultQuery("endTime", "0"), 10, 64)

	opts := types.MCPLogListOpts{
		Page:      page,
		PageSize:  pageSize,
		ClientID:  clientID,
		ToolName:  c.Query("toolName"),
		Status:    c.Query("status"),
		StartTime: startTime,
		EndTime:   endTime,
	}

	logs, total, err := dal.ListMCPLogs(c.Request.Context(), h.DB, opts)
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

// ClearMCPLogs godoc
// @Summary Clear all MCP logs
// @Tags MCP Logs
// @Produce json
// @Success 200 {object} object "{success: true}"
// @Security BearerAuth
// @Router /api/v1/mcp-log/clear [delete]
func (h *Handler) ClearMCPLogs(c *gin.Context) {
	if err := dal.ClearMCPLogs(c.Request.Context(), h.DB); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successNoData(c)
}
