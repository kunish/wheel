package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/config"
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
		logs, _, err := dal.ListLogs(ctx, h.DB, dal.ListLogsOpts{Page: 1, PageSize: 999999})
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

// GetVersion godoc
// @Summary Get current server version
// @Tags Settings
// @Produce json
// @Success 200 {object} object "{success: true, data: {version: string}}"
// @Security BearerAuth
// @Router /api/v1/setting/version [get]
func (h *Handler) GetVersion(c *gin.Context) {
	successJSON(c, gin.H{"version": strings.TrimPrefix(config.Version, "v")})
}

// CheckUpdate godoc
// @Summary Check for new releases on GitHub
// @Tags Settings
// @Produce json
// @Success 200 {object} object "{success: true, data: {current, latest, hasUpdate, releaseUrl, releaseNotes}}"
// @Security BearerAuth
// @Router /api/v1/setting/check-update [get]
func (h *Handler) CheckUpdate(c *gin.Context) {
	current := strings.TrimPrefix(config.Version, "v")

	resp, err := http.Get("https://api.github.com/repos/kunish/wheel/releases/latest")
	if err != nil {
		errorJSON(c, http.StatusBadGateway, "Failed to reach GitHub API")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		errorJSON(c, http.StatusBadGateway, fmt.Sprintf("GitHub API returned %d: %s", resp.StatusCode, string(body)))
		return
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		errorJSON(c, http.StatusBadGateway, "Failed to parse GitHub response")
		return
	}

	latest := strings.TrimPrefix(release.TagName, "v")

	successJSON(c, gin.H{
		"current":      current,
		"latest":       latest,
		"hasUpdate":    latest != current,
		"releaseUrl":   release.HTMLURL,
		"releaseNotes": release.Body,
	})
}

// ApplyUpdate godoc
// @Summary Download latest release binary and restart
// @Tags Settings
// @Produce json
// @Success 200 {object} object "{success: true}"
// @Failure 502 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/setting/apply-update [post]
func (h *Handler) ApplyUpdate(c *gin.Context) {
	// 1. Fetch latest release info (with assets)
	resp, err := http.Get("https://api.github.com/repos/kunish/wheel/releases/latest")
	if err != nil {
		errorJSON(c, http.StatusBadGateway, "Failed to reach GitHub API")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorJSON(c, http.StatusBadGateway, fmt.Sprintf("GitHub API returned %d", resp.StatusCode))
		return
	}

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		errorJSON(c, http.StatusBadGateway, "Failed to parse GitHub response")
		return
	}

	// 2. Find matching asset
	wantName := fmt.Sprintf("wheel-%s-%s", runtime.GOOS, runtime.GOARCH)
	var downloadURL string
	for _, a := range release.Assets {
		if a.Name == wantName {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		errorJSON(c, http.StatusBadGateway, fmt.Sprintf("No asset found for %s", wantName))
		return
	}

	// 3. Download binary
	dlResp, err := http.Get(downloadURL)
	if err != nil {
		errorJSON(c, http.StatusBadGateway, "Failed to download binary")
		return
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		errorJSON(c, http.StatusBadGateway, fmt.Sprintf("Download returned %d", dlResp.StatusCode))
		return
	}

	// 4. Write to temp file next to current binary
	exe, err := os.Executable()
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "Cannot determine executable path")
		return
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "Cannot resolve executable path")
		return
	}

	tmpPath := exe + ".update"
	out, err := os.Create(tmpPath)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "Cannot create temp file")
		return
	}

	if _, err := io.Copy(out, dlResp.Body); err != nil {
		out.Close()
		os.Remove(tmpPath)
		errorJSON(c, http.StatusInternalServerError, "Failed to write binary")
		return
	}
	out.Close()

	// 5. chmod +x and atomic rename
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		os.Remove(tmpPath)
		errorJSON(c, http.StatusInternalServerError, "Failed to set permissions")
		return
	}
	if err := os.Rename(tmpPath, exe); err != nil {
		os.Remove(tmpPath)
		errorJSON(c, http.StatusInternalServerError, "Failed to replace binary")
		return
	}

	// 6. Respond success, then schedule self-termination
	successNoData(c)

	go func() {
		time.Sleep(500 * time.Millisecond)
		syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
	}()
}
