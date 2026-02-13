package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/middleware"
)

// RegisterRoutes sets up all API routes on the given Gin engine.
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	// CORS
	r.Use(middleware.CORSMiddleware())

	// Health check
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"name": "wheel", "version": "0.1.0"})
	})

	// ──── API Docs ────
	r.GET("/docs", h.ServeDocs)
	r.GET("/docs/openapi.json", h.ServeOpenAPISpec)

	// ──── Public: User login ────
	userGroup := r.Group("/api/v1/user")
	userGroup.POST("/login", h.Login)

	// ──── API Key authenticated (end-user access) ────
	apikeyUser := r.Group("/api/v1/user/apikey")
	apikeyUser.Use(middleware.ApiKeyAuth(h.DB))
	apikeyUser.GET("/login", h.ApiKeyLogin)
	apikeyUser.GET("/stats", h.ApiKeyStats)

	// ──── JWT-protected user routes ────
	userProtected := r.Group("/api/v1/user")
	userProtected.Use(middleware.JWTAuth(h.Config.JWTSecret))
	userProtected.POST("/change-password", h.ChangePassword)
	userProtected.POST("/change-username", h.ChangeUsername)
	userProtected.GET("/status", h.UserStatus)

	// ──── Admin API: JWT protected ────
	admin := r.Group("/api/v1")
	admin.Use(middleware.JWTAuth(h.Config.JWTSecret))

	// Channel routes
	admin.GET("/channel/list", h.ListChannels)
	admin.POST("/channel/create", h.CreateChannel)
	admin.POST("/channel/update", h.UpdateChannel)
	admin.POST("/channel/enable", h.EnableChannel)
	admin.DELETE("/channel/delete/:id", h.DeleteChannel)
	admin.POST("/channel/fetch-model", h.FetchModel)
	admin.POST("/channel/fetch-model-preview", h.FetchModelPreview)
	admin.POST("/channel/sync", h.SyncAllModels)
	admin.GET("/channel/last-sync-time", h.LastSyncTime)
	admin.POST("/channel/reorder", h.ReorderChannels)

	// Group routes
	admin.GET("/group/list", h.ListGroups)
	admin.POST("/group/create", h.CreateGroup)
	admin.POST("/group/update", h.UpdateGroup)
	admin.DELETE("/group/delete/:id", h.DeleteGroup)
	admin.POST("/group/reorder", h.ReorderGroups)
	admin.GET("/group/model-list", h.GroupModelList)

	// API Key routes
	admin.GET("/apikey/list", h.ListApiKeys)
	admin.POST("/apikey/create", h.CreateApiKey)
	admin.POST("/apikey/update", h.UpdateApiKey)
	admin.DELETE("/apikey/delete/:id", h.DeleteApiKey)

	// Log routes
	admin.GET("/log/list", h.ListLogs)
	admin.GET("/log/:id", h.GetLog)
	admin.DELETE("/log/delete/:id", h.DeleteLog)
	admin.DELETE("/log/clear", h.ClearLogs)
	admin.POST("/log/replay/:id", h.ReplayLog)

	// Stats routes
	admin.GET("/stats/global", h.GetGlobalStats)
	admin.GET("/stats/channel", h.GetChannelStats)
	admin.GET("/stats/total", h.GetTotalStats)
	admin.GET("/stats/today", h.GetTodayStats)
	admin.GET("/stats/daily", h.GetDailyStats)
	admin.GET("/stats/hourly", h.GetHourlyStats)
	admin.GET("/stats/model", h.GetModelStats)
	admin.GET("/stats/apikey", h.GetApiKeyStats)

	// Setting routes
	admin.GET("/setting/", h.GetSettings)
	admin.POST("/setting/update", h.UpdateSettings)
	admin.GET("/setting/export", h.ExportData)
	admin.POST("/setting/import", h.ImportData)

	// Model routes
	admin.GET("/model/list", h.ListModels)
	admin.GET("/model/channel", h.ListModelsByChannel)
	admin.POST("/model/create", h.CreateModel)
	admin.POST("/model/update", h.UpdateModel)
	admin.POST("/model/delete", h.DeleteModel)
	admin.GET("/model/metadata", h.GetModelMetadata)
	admin.POST("/model/metadata/refresh", h.RefreshModelMetadata)
	admin.POST("/model/update-price", h.UpdatePrice)
	admin.GET("/model/last-update-time", h.GetLastUpdateTime)
}
