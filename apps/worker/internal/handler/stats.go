package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
)

// ──── Stats Routes ────

// GetGlobalStats godoc
// @Summary Get global statistics overview
// @Tags Stats
// @Produce json
// @Success 200 {object} object "{success: true, data: GlobalStatsResponse}"
// @Security BearerAuth
// @Router /api/v1/stats/global [get]
func (h *Handler) GetGlobalStats(c *gin.Context) {
	stats, err := dal.GetGlobalStats(c.Request.Context(), h.LogDB, h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, stats)
}

// GetChannelStats godoc
// @Summary Get per-channel statistics
// @Tags Stats
// @Produce json
// @Success 200 {object} object "{success: true, data: {channels: []ChannelStatsItem}}"
// @Security BearerAuth
// @Router /api/v1/stats/channel [get]
func (h *Handler) GetChannelStats(c *gin.Context) {
	stats, err := dal.GetChannelStats(c.Request.Context(), h.LogDB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, stats)
}

// GetTotalStats godoc
// @Summary Get all-time total statistics
// @Tags Stats
// @Produce json
// @Success 200 {object} object "{success: true, data: DailyStatsItem}"
// @Security BearerAuth
// @Router /api/v1/stats/total [get]
func (h *Handler) GetTotalStats(c *gin.Context) {
	stats, err := dal.GetTotalStats(c.Request.Context(), h.LogDB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, stats)
}

// GetTodayStats godoc
// @Summary Get today's statistics
// @Tags Stats
// @Produce json
// @Param tz query string false "Timezone offset (e.g. +08:00)"
// @Success 200 {object} object "{success: true, data: DailyStatsItem}"
// @Security BearerAuth
// @Router /api/v1/stats/today [get]
func (h *Handler) GetTodayStats(c *gin.Context) {
	tz := c.DefaultQuery("tz", "")
	stats, err := dal.GetTodayStats(c.Request.Context(), h.LogDB, tz)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, stats)
}

// GetDailyStats godoc
// @Summary Get daily statistics
// @Tags Stats
// @Produce json
// @Param tz query string false "Timezone offset (e.g. +08:00)"
// @Success 200 {object} object "{success: true, data: []DailyStatsItem}"
// @Security BearerAuth
// @Router /api/v1/stats/daily [get]
func (h *Handler) GetDailyStats(c *gin.Context) {
	tz := c.DefaultQuery("tz", "")
	stats, err := dal.GetDailyStats(c.Request.Context(), h.LogDB, tz)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, stats)
}

// GetHourlyStats godoc
// @Summary Get hourly statistics for a date range
// @Tags Stats
// @Produce json
// @Param start query string false "Start date (YYYYMMDD)"
// @Param end query string false "End date (YYYYMMDD)"
// @Param tz query string false "Timezone offset (e.g. +08:00)"
// @Success 200 {object} object "{success: true, data: []HourlyStatsItem}"
// @Security BearerAuth
// @Router /api/v1/stats/hourly [get]
func (h *Handler) GetHourlyStats(c *gin.Context) {
	start := c.DefaultQuery("start", "")
	end := c.DefaultQuery("end", "")
	tz := c.DefaultQuery("tz", "")
	stats, err := dal.GetHourlyStats(c.Request.Context(), h.LogDB, start, end, tz)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, stats)
}

// GetModelStats godoc
// @Summary Get per-model statistics
// @Tags Stats
// @Produce json
// @Success 200 {object} object "{success: true, data: []ModelStatsItem}"
// @Security BearerAuth
// @Router /api/v1/stats/model [get]
func (h *Handler) GetModelStats(c *gin.Context) {
	stats, err := dal.GetModelStats(c.Request.Context(), h.LogDB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, stats)
}

// GetApiKeyStats godoc
// @Summary Get per-API-key statistics
// @Tags Stats
// @Produce json
// @Success 200 {object} object "{success: true, data: []object}"
// @Security BearerAuth
// @Router /api/v1/stats/apikey [get]
func (h *Handler) GetApiKeyStats(c *gin.Context) {
	stats, err := dal.GetApiKeyStats(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, stats)
}
