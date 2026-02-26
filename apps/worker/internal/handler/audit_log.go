package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// ──── Audit Log Routes ────

// ListAuditLogs godoc
// @Summary List audit logs with pagination and filters
// @Tags Audit Logs
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size (max 200)" default(50)
// @Param user query string false "Filter by user"
// @Param action query string false "Filter by action"
// @Param startTime query int false "Filter by start timestamp"
// @Param endTime query int false "Filter by end timestamp"
// @Success 200 {object} object "{success: true, data: {logs, total, page, pageSize}}"
// @Security BearerAuth
// @Router /api/v1/audit-log/list [get]
func (h *Handler) ListAuditLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	startTime, _ := strconv.ParseInt(c.DefaultQuery("startTime", "0"), 10, 64)
	endTime, _ := strconv.ParseInt(c.DefaultQuery("endTime", "0"), 10, 64)

	opts := types.AuditLogListOpts{
		Page:      page,
		PageSize:  pageSize,
		User:      c.Query("user"),
		Action:    c.Query("action"),
		StartTime: startTime,
		EndTime:   endTime,
	}

	logs, total, err := dal.ListAuditLogs(c.Request.Context(), h.DB, opts)
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

// ClearAuditLogs godoc
// @Summary Clear all audit logs
// @Tags Audit Logs
// @Produce json
// @Success 200 {object} object "{success: true}"
// @Security BearerAuth
// @Router /api/v1/audit-log/clear [delete]
func (h *Handler) ClearAuditLogs(c *gin.Context) {
	if err := dal.ClearAuditLogs(c.Request.Context(), h.DB); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successNoData(c)
}
