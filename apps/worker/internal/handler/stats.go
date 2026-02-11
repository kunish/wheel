package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
)

// ──── Stats Routes ────

func (h *Handler) GetGlobalStats(c *gin.Context) {
	stats, err := dal.GetGlobalStats(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, stats)
}

func (h *Handler) GetChannelStats(c *gin.Context) {
	stats, err := dal.GetChannelStats(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, stats)
}

func (h *Handler) GetTotalStats(c *gin.Context) {
	stats, err := dal.GetTotalStats(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, stats)
}

func (h *Handler) GetTodayStats(c *gin.Context) {
	tz := c.DefaultQuery("tz", "")
	stats, err := dal.GetTodayStats(c.Request.Context(), h.DB, tz)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, stats)
}

func (h *Handler) GetDailyStats(c *gin.Context) {
	tz := c.DefaultQuery("tz", "")
	stats, err := dal.GetDailyStats(c.Request.Context(), h.DB, tz)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, stats)
}

func (h *Handler) GetHourlyStats(c *gin.Context) {
	start := c.DefaultQuery("start", "")
	end := c.DefaultQuery("end", "")
	tz := c.DefaultQuery("tz", "")
	stats, err := dal.GetHourlyStats(c.Request.Context(), h.DB, start, end, tz)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, stats)
}

func (h *Handler) GetModelStats(c *gin.Context) {
	stats, err := dal.GetModelStats(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, stats)
}

func (h *Handler) GetApiKeyStats(c *gin.Context) {
	stats, err := dal.GetApiKeyStats(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, stats)
}
