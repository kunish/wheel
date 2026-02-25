package handler

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
	"log_retention_days":           "365",
	"circuit_breaker_threshold":    "5",
	"circuit_breaker_cooldown":     "60",
	"circuit_breaker_max_cooldown": "600",
	"active_profile_id":            "0",
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
	groups, err := dal.ListGroups(ctx, h.DB, 0)
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

// ResetCircuitBreakers godoc
// @Summary Reset all circuit breakers
// @Tags Settings
// @Produce json
// @Success 200 {object} object "{success: true, data: {reset: int}}"
// @Security BearerAuth
// @Router /api/v1/setting/reset-circuit-breakers [post]
func (h *Handler) ResetCircuitBreakers(c *gin.Context) {
	if h.CircuitBreakers == nil {
		errorJSON(c, http.StatusInternalServerError, "Circuit breaker manager not available")
		return
	}
	count := h.CircuitBreakers.ResetAll()
	successJSON(c, gin.H{"reset": count})
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

	// 2. Find matching binary asset and checksums asset
	wantName := fmt.Sprintf("wheel-%s-%s", runtime.GOOS, runtime.GOARCH)
	var downloadURL, checksumsURL string
	for _, a := range release.Assets {
		switch a.Name {
		case wantName:
			downloadURL = a.BrowserDownloadURL
		case "checksums.txt":
			checksumsURL = a.BrowserDownloadURL
		}
	}
	if downloadURL == "" {
		errorJSON(c, http.StatusBadGateway, fmt.Sprintf("No asset found for %s", wantName))
		return
	}
	if checksumsURL == "" {
		errorJSON(c, http.StatusBadGateway, "Release has no checksums.txt — cannot verify integrity. Please update manually.")
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

	// 4. Write to temp file, computing SHA256 as we go
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

	hasher := sha256.New()
	if _, err := io.Copy(out, io.TeeReader(dlResp.Body, hasher)); err != nil {
		out.Close()
		os.Remove(tmpPath)
		errorJSON(c, http.StatusInternalServerError, "Failed to write binary")
		return
	}
	out.Close()
	actualHash := hex.EncodeToString(hasher.Sum(nil))

	// 5. Download and verify checksum
	expectedHash, err := fetchExpectedChecksum(checksumsURL, wantName)
	if err != nil {
		os.Remove(tmpPath)
		slog.Error("checksum fetch failed", "error", err)
		errorJSON(c, http.StatusBadGateway, "Failed to fetch checksums")
		return
	}
	if actualHash != expectedHash {
		os.Remove(tmpPath)
		slog.Error("checksum mismatch", "expected", expectedHash, "actual", actualHash)
		errorJSON(c, http.StatusBadGateway, "Checksum verification failed — downloaded binary may be corrupted")
		return
	}

	// 6. chmod +x and atomic rename
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

	// 7. Respond success, then schedule self-termination
	successNoData(c)

	go func() {
		time.Sleep(500 * time.Millisecond)
		_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
	}()
}

// fetchExpectedChecksum downloads checksums.txt and returns the SHA256 hash
// for the given filename. The file format is "hash  filename" per line.
func fetchExpectedChecksum(checksumsURL, filename string) (string, error) {
	resp, err := http.Get(checksumsURL)
	if err != nil {
		return "", fmt.Errorf("download checksums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksums returned %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		// Format: "<hash>  <filename>" (two spaces, matching shasum output)
		parts := strings.Fields(scanner.Text())
		if len(parts) == 2 && parts[1] == filename {
			return parts[0], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading checksums: %w", err)
	}
	return "", fmt.Errorf("no checksum entry for %s", filename)
}
