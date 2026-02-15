package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/service"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// Default settings values matching the TS implementation.
var defaultSettings = map[string]string{
	"log_retention_days":        "365",
	"circuit_breaker_threshold": "5",
	"circuit_breaker_cooldown":  "60",
	"circuit_breaker_max_cooldown": "600",
}

// ──── Setting Routes ────

// GetSettings godoc
// @Summary Get all settings
// @Tags Settings
// @Produce json
// @Success 200 {object} object "{success: true, data: {settings: map[string]string}}"
// @Security BearerAuth
// @Router /api/v1/setting/ [get]
func (h *Handler) GetSettings(c *gin.Context) {
	settings, err := dal.GetAllSettings(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	merged := make(map[string]string)
	for k, v := range defaultSettings {
		merged[k] = v
	}
	for k := range defaultSettings {
		if v, ok := settings[k]; ok {
			merged[k] = v
		}
	}

	successJSON(c, gin.H{"settings": merged})
}

// UpdateSettings godoc
// @Summary Update settings
// @Tags Settings
// @Accept json
// @Produce json
// @Param body body types.SettingsUpdateRequest true "Settings key-value pairs"
// @Success 200 {object} object "{success: true}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/setting/update [post]
func (h *Handler) UpdateSettings(c *gin.Context) {
	var req types.SettingsUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := dal.UpdateSettings(c.Request.Context(), h.DB, req.Settings); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	successNoData(c)
}

// ExportData godoc
// @Summary Export all data as JSON file
// @Tags Settings
// @Produce json
// @Param include_logs query string false "Include relay logs (true|false)"
// @Success 200 {file} file "JSON export file"
// @Security BearerAuth
// @Router /api/v1/setting/export [get]
func (h *Handler) ExportData(c *gin.Context) {
	includeLogs := c.Query("include_logs") == "true"
	ctx := c.Request.Context()

	channels, err := dal.ListChannels(ctx, h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	groups, err := dal.ListGroups(ctx, h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	apiKeys, err := dal.ListApiKeys(ctx, h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	settings, err := dal.GetAllSettings(ctx, h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	settingsList := make([]types.Setting, 0, len(settings))
	for k, v := range settings {
		settingsList = append(settingsList, types.Setting{Key: k, Value: v})
	}

	dump := types.DBDump{
		Version:    1,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Channels:   channels,
		Groups:     groups,
		APIKeys:    apiKeys,
		Settings:   settingsList,
	}

	if includeLogs {
		logs, _, err := dal.ListLogs(ctx, h.LogDB, dal.ListLogsOpts{Page: 1, PageSize: 999999})
		if err != nil {
			errorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}
		dump.RelayLogs = logs
	}

	data, err := json.MarshalIndent(dump, "", "  ")
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "Failed to marshal export data")
		return
	}

	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="wheel-export-%d.json"`, time.Now().UnixMilli()))
	c.Data(http.StatusOK, "application/json", data)
}

// ImportData godoc
// @Summary Import data from JSON file or body
// @Tags Settings
// @Accept json,mpfd
// @Produce json
// @Param file formData file false "JSON export file"
// @Success 200 {object} object "{success: true, data: ImportResult}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/setting/import [post]
func (h *Handler) ImportData(c *gin.Context) {
	var dump types.DBDump

	contentType := c.GetHeader("Content-Type")
	if strings.Contains(contentType, "multipart/form-data") {
		file, _, err := c.Request.FormFile("file")
		if err != nil {
			errorJSON(c, http.StatusBadRequest, "No file provided")
			return
		}
		defer file.Close()
		if err := json.NewDecoder(file).Decode(&dump); err != nil {
			errorJSON(c, http.StatusBadRequest, "Invalid JSON in file")
			return
		}
	} else {
		if err := c.ShouldBindJSON(&dump); err != nil {
			errorJSON(c, http.StatusBadRequest, "Invalid request body")
			return
		}
	}

	if dump.Version == 0 || dump.ExportedAt == "" {
		errorJSON(c, http.StatusBadRequest, "Invalid dump format")
		return
	}

	result := service.ImportData(c.Request.Context(), h.DB, &dump)

	// Invalidate caches
	h.Cache.Delete("channels")
	h.Cache.Delete("groups")
	h.Cache.Delete("apikeys")
	h.Cache.Delete("settings")

	successJSON(c, result)
}
