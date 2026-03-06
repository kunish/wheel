package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	codexruntime "github.com/kunish/wheel/apps/worker/internal/codexruntime"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/types"
	codexauthsdk "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/uptrace/bun"
)

const codexQuotaEndpoint = "https://chatgpt.com/backend-api/wham/usage"

type codexAuthFile struct {
	Name      string         `json:"name"`
	Provider  string         `json:"provider"`
	Type      string         `json:"type"`
	Email     string         `json:"email,omitempty"`
	Disabled  bool           `json:"disabled"`
	AuthIndex string         `json:"authIndex,omitempty"`
	Path      string         `json:"-"`
	Raw       map[string]any `json:"-"`
}

type codexCapabilities struct {
	LocalEnabled      bool `json:"localEnabled"`
	ManagementEnabled bool `json:"managementEnabled"`
	OAuthEnabled      bool `json:"oauthEnabled"`
	ModelsEnabled     bool `json:"modelsEnabled"`
}

type codexQuotaWindow struct {
	UsedPercent        float64 `json:"usedPercent"`
	LimitWindowSeconds int64   `json:"limitWindowSeconds"`
	ResetAfterSeconds  int64   `json:"resetAfterSeconds"`
	ResetAt            string  `json:"resetAt"`
	Allowed            bool    `json:"allowed"`
	LimitReached       bool    `json:"limitReached"`
}

type codexQuotaItem struct {
	Name       string           `json:"name"`
	Email      string           `json:"email,omitempty"`
	AuthIndex  string           `json:"authIndex,omitempty"`
	PlanType   string           `json:"planType,omitempty"`
	Weekly     codexQuotaWindow `json:"weekly"`
	CodeReview codexQuotaWindow `json:"codeReview"`
	Error      string           `json:"error,omitempty"`
}

type codexOAuthSession struct {
	ChannelID int
	Existing  map[string]struct{}
}

type codexAuthUploadResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type codexAuthUploadResponse struct {
	Total        int                     `json:"total"`
	SuccessCount int                     `json:"successCount"`
	FailedCount  int                     `json:"failedCount"`
	Results      []codexAuthUploadResult `json:"results"`
}

var codexOAuthSessions sync.Map

func (h *Handler) codexCapabilities() codexCapabilities {
	managementEnabled := h != nil && h.Config != nil && strings.TrimSpace(h.Config.CodexRuntimeManagementKey) != ""
	localEnabled := h != nil && h.DB != nil
	return codexCapabilities{
		LocalEnabled:      localEnabled,
		ManagementEnabled: managementEnabled,
		OAuthEnabled:      managementEnabled,
		ModelsEnabled:     managementEnabled,
	}
}

func (h *Handler) resolveCodexLocalAuthDir() (string, error) {
	if h == nil || h.Config == nil {
		return "", fmt.Errorf("handler config not initialized")
	}
	return codexruntime.ManagedAuthDir(), nil
}

func (h *Handler) listLocalAuthFiles(authDir string) ([]codexAuthFile, error) {
	if strings.TrimSpace(authDir) == "" {
		return nil, fmt.Errorf("auth dir is required")
	}
	if _, err := os.Stat(authDir); err != nil {
		if os.IsNotExist(err) {
			return []codexAuthFile{}, nil
		}
		return nil, err
	}
	out := make([]codexAuthFile, 0)
	err := filepath.WalkDir(authDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".json") {
			return nil
		}
		rawBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		raw := map[string]any{}
		if err := json.Unmarshal(rawBytes, &raw); err != nil {
			return nil
		}
		rel, err := filepath.Rel(authDir, path)
		if err != nil {
			return err
		}
		entry := codexAuthFile{
			Name:      rel,
			Provider:  strings.ToLower(stringFromMap(raw, "provider", "type")),
			Type:      strings.ToLower(stringFromMap(raw, "type", "provider")),
			Email:     stringFromMap(raw, "email"),
			Disabled:  boolFromMap(raw, "disabled"),
			AuthIndex: localAuthIndex(rel),
			Path:      path,
			Raw:       raw,
		}
		if entry.Provider == "" {
			entry.Provider = entry.Type
		}
		out = append(out, entry)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func (h *Handler) uploadLocalAuthFile(authDir string, filename string, content []byte) error {
	if strings.TrimSpace(filename) == "" {
		return fmt.Errorf("filename is required")
	}
	name := filepath.Base(filename)
	if !strings.HasSuffix(strings.ToLower(name), ".json") {
		return fmt.Errorf("file must be .json")
	}
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		return fmt.Errorf("create auth dir: %w", err)
	}
	return os.WriteFile(filepath.Join(authDir, name), content, 0o600)
}

func (h *Handler) patchLocalAuthFileDisabled(authDir string, name string, disabled bool) error {
	path, err := localAuthPath(authDir, name)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	raw := map[string]any{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("invalid auth file json: %w", err)
	}
	raw["disabled"] = disabled
	encoded, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshal auth file: %w", err)
	}
	return os.WriteFile(path, encoded, 0o600)
}

func (h *Handler) deleteLocalAuthFile(authDir string, name string) error {
	path, err := localAuthPath(authDir, name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("auth file not found")
		}
		return err
	}
	return nil
}

func localAuthPath(authDir string, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("name is required")
	}
	cleanBase := filepath.Clean(authDir)
	cleanPath := filepath.Clean(filepath.Join(cleanBase, name))
	if cleanPath != cleanBase && !strings.HasPrefix(cleanPath, cleanBase+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid auth file name")
	}
	return cleanPath, nil
}

func localAuthIndex(name string) string {
	auth := &codexauthsdk.Auth{FileName: name}
	return auth.EnsureIndex()
}

func managedAuthRelativeName(channelID int, name string) string {
	return codexruntime.ManagedAuthFileName(channelID, name)
}

func managedAuthIndex(channelID int, name string) string {
	return localAuthIndex(managedAuthRelativeName(channelID, name))
}

func (h *Handler) listManagedCodexAuthFiles(ctx context.Context, channelID int) ([]codexAuthFile, error) {
	items, err := dal.ListCodexAuthFiles(ctx, h.DB, channelID)
	if err != nil {
		return nil, err
	}
	out := make([]codexAuthFile, 0, len(items))
	for _, item := range items {
		raw := map[string]any{}
		if err := json.Unmarshal([]byte(item.Content), &raw); err != nil {
			continue
		}
		provider := strings.ToLower(strings.TrimSpace(item.Provider))
		if provider == "" {
			provider = strings.ToLower(stringFromMap(raw, "type", "provider"))
		}
		if provider == "" {
			provider = "codex"
		}
		out = append(out, codexAuthFile{
			Name:      item.Name,
			Provider:  provider,
			Type:      provider,
			Email:     item.Email,
			Disabled:  item.Disabled,
			AuthIndex: managedAuthIndex(item.ChannelID, item.Name),
			Raw:       raw,
		})
	}
	return out, nil
}

func parseCodexAuthContent(content []byte) (provider string, email string, disabled bool, normalized string, raw map[string]any, err error) {
	raw = map[string]any{}
	if err = json.Unmarshal(content, &raw); err != nil {
		return "", "", false, "", nil, fmt.Errorf("invalid auth file json")
	}
	provider = strings.ToLower(strings.TrimSpace(stringFromMap(raw, "type", "provider")))
	if provider == "" {
		provider = "codex"
		raw["type"] = provider
	}
	email = strings.TrimSpace(stringFromMap(raw, "email"))
	disabled = boolFromMap(raw, "disabled")
	encoded, marshalErr := json.Marshal(raw)
	if marshalErr != nil {
		return "", "", false, "", nil, fmt.Errorf("failed to encode auth file json")
	}
	return provider, email, disabled, string(encoded), raw, nil
}

func uniqueCodexAuthFileName(ctx context.Context, db *bun.DB, channelID int, baseName string) (string, error) {
	name := filepath.Base(baseName)
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	if stem == "" {
		stem = "auth"
	}
	for i := 0; i < 1000; i++ {
		candidate := name
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d%s", stem, i, ext)
		}
		item, err := dal.GetCodexAuthFileByName(ctx, db, channelID, candidate)
		if err != nil {
			return "", err
		}
		if item == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("failed to allocate unique auth file name")
}

func (h *Handler) importOAuthAuthFilesToDB(ctx context.Context, channelID int, snapshot map[string]struct{}) error {
	authDir, err := h.resolveCodexLocalAuthDir()
	if err != nil {
		return err
	}
	files, err := h.listLocalAuthFiles(authDir)
	if err != nil {
		return err
	}
	for _, file := range files {
		if _, exists := snapshot[file.Name]; exists {
			continue
		}
		if strings.HasPrefix(file.Name, "channel-") {
			continue
		}
		content, err := os.ReadFile(file.Path)
		if err != nil {
			return err
		}
		provider, email, disabled, normalized, _, err := parseCodexAuthContent(content)
		if err != nil {
			return err
		}
		name, err := uniqueCodexAuthFileName(ctx, h.DB, channelID, filepath.Base(file.Name))
		if err != nil {
			return err
		}
		item := &types.CodexAuthFile{
			ChannelID: channelID,
			Name:      name,
			Provider:  provider,
			Email:     email,
			Disabled:  disabled,
			Content:   normalized,
		}
		if err := dal.CreateCodexAuthFile(ctx, h.DB, item); err != nil {
			return err
		}
		if err := codexruntime.MaterializeOneAuthFile(item); err != nil {
			return err
		}
		if err := os.Remove(file.Path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	h.bestEffortSyncCodexChannelModels(ctx, channelID)
	return nil
}

func (h *Handler) ListCodexAuthFiles(c *gin.Context) {
	channel, err := h.validateCodexChannel(c)
	if err != nil {
		return
	}
	_ = channel // channel validated but not used for filtering

	provider := strings.TrimSpace(c.Query("provider"))
	search := strings.TrimSpace(c.Query("search"))
	page := parsePositiveInt(c.Query("page"), 1)
	pageSize := parsePositiveInt(c.Query("pageSize"), 20)
	if pageSize > 200 {
		pageSize = 200
	}
	capabilities := h.codexCapabilities()

	var files []codexAuthFile
	if capabilities.LocalEnabled {
		files, err = h.listManagedCodexAuthFiles(c.Request.Context(), channel.ID)
		if err != nil {
			errorJSON(c, http.StatusBadGateway, err.Error())
			return
		}
	} else {
		if err := h.ensureCodexManagementConfigured(); err != nil {
			errorJSON(c, http.StatusBadRequest, err.Error())
			return
		}
		var resp struct {
			Files []map[string]any `json:"files"`
		}
		if err := h.codexManagementCall(c, http.MethodGet, "/auth-files", nil, nil, &resp); err != nil {
			errorJSON(c, http.StatusBadGateway, err.Error())
			return
		}
		files = parseAuthFiles(resp.Files)
	}

	items, total := filterAndPaginateAuthFiles(files, provider, search, page, pageSize)
	successJSON(c, gin.H{"files": items, "total": total, "page": page, "pageSize": pageSize, "capabilities": capabilities})
}

func (h *Handler) PatchCodexAuthFileStatus(c *gin.Context) {
	channel, err := h.validateCodexChannel(c)
	if err != nil {
		return
	}
	_ = channel

	var req struct {
		Name     string `json:"name"`
		Disabled bool   `json:"disabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		errorJSON(c, http.StatusBadRequest, "name is required")
		return
	}

	var out map[string]any
	if h.codexCapabilities().LocalEnabled {
		item, err := dal.GetCodexAuthFileByName(c.Request.Context(), h.DB, channel.ID, req.Name)
		if err != nil {
			errorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}
		if item == nil {
			errorJSON(c, http.StatusNotFound, "auth file not found")
			return
		}
		_, _, _, _, raw, err := parseCodexAuthContent([]byte(item.Content))
		if err != nil {
			errorJSON(c, http.StatusBadRequest, err.Error())
			return
		}
		raw["disabled"] = req.Disabled
		encoded, err := json.Marshal(raw)
		if err != nil {
			errorJSON(c, http.StatusInternalServerError, "failed to encode auth file")
			return
		}
		if err := dal.UpdateCodexAuthFile(c.Request.Context(), h.DB, item.ID, map[string]any{"disabled": req.Disabled, "content": string(encoded)}); err != nil {
			errorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}
		item.Disabled = req.Disabled
		item.Content = string(encoded)
		if err := codexruntime.MaterializeOneAuthFile(item); err != nil {
			errorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}
		h.bestEffortSyncCodexChannelModels(c.Request.Context(), channel.ID)
		out = map[string]any{"status": "ok", "disabled": req.Disabled}
	} else {
		if err := h.ensureCodexManagementConfigured(); err != nil {
			errorJSON(c, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.codexManagementCall(c, http.MethodPatch, "/auth-files/status", nil, req, &out); err != nil {
			errorJSON(c, http.StatusBadGateway, err.Error())
			return
		}
	}
	successJSON(c, out)
}

func (h *Handler) UploadCodexAuthFile(c *gin.Context) {
	channel, err := h.validateCodexChannel(c)
	if err != nil {
		return
	}

	fileHeaders, err := collectCodexUploadFiles(c)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "failed to parse multipart form")
		return
	}
	if len(fileHeaders) == 0 {
		errorJSON(c, http.StatusBadRequest, "file is required")
		return
	}

	response := codexAuthUploadResponse{
		Total:   len(fileHeaders),
		Results: make([]codexAuthUploadResult, 0, len(fileHeaders)),
	}
	if h.codexCapabilities().LocalEnabled {
		for _, fileHeader := range fileHeaders {
			result := h.uploadCodexLocalAuthFile(c.Request.Context(), channel.ID, fileHeader)
			response.Results = append(response.Results, result)
			if result.Status == "ok" {
				response.SuccessCount++
				continue
			}
			response.FailedCount++
		}
		if response.SuccessCount > 0 {
			h.bestEffortSyncCodexChannelModels(c.Request.Context(), channel.ID)
		}
	} else {
		if err := h.ensureCodexManagementConfigured(); err != nil {
			errorJSON(c, http.StatusBadRequest, err.Error())
			return
		}
		for _, fileHeader := range fileHeaders {
			result := h.uploadCodexManagedAuthFile(c.Request.Context(), fileHeader)
			response.Results = append(response.Results, result)
			if result.Status == "ok" {
				response.SuccessCount++
				continue
			}
			response.FailedCount++
		}
	}
	successJSON(c, response)
}

func collectCodexUploadFiles(c *gin.Context) ([]*multipart.FileHeader, error) {
	form, err := c.MultipartForm()
	if err != nil {
		return nil, err
	}
	if form == nil || form.File == nil {
		return nil, nil
	}
	files := make([]*multipart.FileHeader, 0, len(form.File["files"])+len(form.File["file"]))
	files = append(files, form.File["files"]...)
	files = append(files, form.File["file"]...)
	return files, nil
}

func readUploadedCodexAuthFile(fileHeader *multipart.FileHeader) ([]byte, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open uploaded file")
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read uploaded file")
	}
	return content, nil
}

func (h *Handler) uploadCodexLocalAuthFile(ctx context.Context, channelID int, fileHeader *multipart.FileHeader) codexAuthUploadResult {
	result := codexAuthUploadResult{Name: filepath.Base(fileHeader.Filename)}
	content, err := readUploadedCodexAuthFile(fileHeader)
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result
	}
	provider, email, disabled, normalized, _, err := parseCodexAuthContent(content)
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result
	}
	item := &types.CodexAuthFile{
		ChannelID: channelID,
		Name:      result.Name,
		Provider:  provider,
		Email:     email,
		Disabled:  disabled,
		Content:   normalized,
	}
	if err := dal.CreateCodexAuthFile(ctx, h.DB, item); err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result
	}
	if err := codexruntime.MaterializeOneAuthFile(item); err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result
	}
	result.Status = "ok"
	return result
}

func (h *Handler) uploadCodexManagedAuthFile(ctx context.Context, fileHeader *multipart.FileHeader) codexAuthUploadResult {
	result := codexAuthUploadResult{Name: filepath.Base(fileHeader.Filename)}
	content, err := readUploadedCodexAuthFile(fileHeader)
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result
	}
	var out map[string]any
	if err := h.codexManagementUploadFile(ctx, fileHeader.Filename, content, &out); err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result
	}
	result.Status = "ok"
	return result
}

func (h *Handler) DeleteCodexAuthFile(c *gin.Context) {
	channel, err := h.validateCodexChannel(c)
	if err != nil {
		return
	}
	_ = channel

	name := strings.TrimSpace(c.Query("name"))
	all := strings.TrimSpace(c.Query("all"))
	if name == "" && all == "" {
		errorJSON(c, http.StatusBadRequest, "name or all is required")
		return
	}

	query := url.Values{}
	if name != "" {
		query.Set("name", name)
	}
	if all != "" {
		query.Set("all", all)
	}

	var out map[string]any
	if h.codexCapabilities().LocalEnabled {
		if all == "true" || all == "1" || all == "*" {
			items, err := dal.ListCodexAuthFiles(c.Request.Context(), h.DB, channel.ID)
			if err != nil {
				errorJSON(c, http.StatusInternalServerError, err.Error())
				return
			}
			deleted := 0
			for i := range items {
				if err := dal.DeleteCodexAuthFile(c.Request.Context(), h.DB, items[i].ID); err == nil {
					_ = codexruntime.RemoveOneAuthFile(&items[i])
					deleted++
				}
			}
			h.bestEffortSyncCodexChannelModels(c.Request.Context(), channel.ID)
			out = map[string]any{"status": "ok", "deleted": deleted}
		} else {
			item, err := dal.GetCodexAuthFileByName(c.Request.Context(), h.DB, channel.ID, name)
			if err != nil {
				errorJSON(c, http.StatusInternalServerError, err.Error())
				return
			}
			if item == nil {
				errorJSON(c, http.StatusNotFound, "auth file not found")
				return
			}
			if err := dal.DeleteCodexAuthFile(c.Request.Context(), h.DB, item.ID); err != nil {
				errorJSON(c, http.StatusInternalServerError, err.Error())
				return
			}
			_ = codexruntime.RemoveOneAuthFile(item)
			h.bestEffortSyncCodexChannelModels(c.Request.Context(), channel.ID)
			out = map[string]any{"status": "ok", "deleted": 1}
		}
	} else {
		if err := h.ensureCodexManagementConfigured(); err != nil {
			errorJSON(c, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.codexManagementCall(c, http.MethodDelete, "/auth-files", query, nil, &out); err != nil {
			errorJSON(c, http.StatusBadGateway, err.Error())
			return
		}
	}
	successJSON(c, out)
}

func (h *Handler) GetCodexAuthFileModels(c *gin.Context) {
	channel, err := h.validateCodexChannel(c)
	if err != nil {
		return
	}
	_ = channel

	if err := h.ensureCodexManagementConfigured(); err != nil {
		errorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	name := strings.TrimSpace(c.Query("name"))
	if name == "" {
		errorJSON(c, http.StatusBadRequest, "name is required")
		return
	}

	queryName := name
	if h.codexCapabilities().LocalEnabled {
		queryName = managedAuthRelativeName(channel.ID, name)
	}
	query := url.Values{"name": []string{queryName}}
	var out map[string]any
	if err := h.codexManagementCall(c, http.MethodGet, "/auth-files/models", query, nil, &out); err != nil {
		errorJSON(c, http.StatusBadGateway, err.Error())
		return
	}
	successJSON(c, out)
}

func (h *Handler) ListCodexQuota(c *gin.Context) {
	channel, err := h.validateCodexChannel(c)
	if err != nil {
		return
	}
	_ = channel

	search := strings.TrimSpace(c.Query("search"))
	page := parsePositiveInt(c.Query("page"), 1)
	pageSize := parsePositiveInt(c.Query("pageSize"), 12)
	if pageSize > 50 {
		pageSize = 50
	}

	var files []codexAuthFile
	if h.codexCapabilities().LocalEnabled {
		files, err = h.listManagedCodexAuthFiles(c.Request.Context(), channel.ID)
		if err != nil {
			errorJSON(c, http.StatusBadGateway, err.Error())
			return
		}
	} else {
		if err := h.ensureCodexManagementConfigured(); err != nil {
			errorJSON(c, http.StatusBadRequest, err.Error())
			return
		}
		var resp struct {
			Files []map[string]any `json:"files"`
		}
		if err := h.codexManagementCall(c, http.MethodGet, "/auth-files", nil, nil, &resp); err != nil {
			errorJSON(c, http.StatusBadGateway, err.Error())
			return
		}
		files = parseAuthFiles(resp.Files)
	}

	paged, total := filterAndPaginateAuthFiles(files, "codex", search, page, pageSize)

	items := make([]codexQuotaItem, 0, len(paged))
	for _, file := range paged {
		item := codexQuotaItem{
			Name:      file.Name,
			Email:     file.Email,
			AuthIndex: file.AuthIndex,
		}
		if file.AuthIndex == "" {
			item.Error = "missing auth_index"
			items = append(items, item)
			continue
		}
		accountID := stringFromMap(file.Raw, "account_id", "accountId")
		if accountID == "" {
			accountID = extractCodexAccountID(file.Raw)
		}
		if accountID == "" {
			item.Error = "missing chatgpt account id"
			items = append(items, item)
			continue
		}

		var weekly codexQuotaWindow
		var codeReview codexQuotaWindow
		var planType string
		if h.codexCapabilities().LocalEnabled {
			weekly, codeReview, planType, err = h.fetchLocalCodexQuota(c.Request.Context(), file.Raw)
		} else {
			weekly, codeReview, planType, err = h.fetchCodexQuota(c, file.AuthIndex, accountID)
		}
		if err != nil {
			item.Error = err.Error()
			items = append(items, item)
			continue
		}
		item.PlanType = planType
		item.Weekly = weekly
		item.CodeReview = codeReview
		items = append(items, item)
	}

	successJSON(c, gin.H{"items": items, "total": total, "page": page, "pageSize": pageSize})
}

// validateCodexChannel verifies the channel exists and is type Codex (33).
// On failure it writes an error response and returns nil.
func (h *Handler) validateCodexChannel(c *gin.Context) (*types.Channel, error) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "invalid channel ID")
		return nil, err
	}
	channel, err := dal.GetChannel(c.Request.Context(), h.DB, id)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return nil, err
	}
	if channel == nil {
		errorJSON(c, http.StatusNotFound, "channel not found")
		return nil, fmt.Errorf("channel not found")
	}
	if channel.Type != types.OutboundCodex {
		errorJSON(c, http.StatusBadRequest, "channel is not a Codex channel")
		return nil, fmt.Errorf("not a codex channel")
	}
	return channel, nil
}

// StartCodexOAuth initiates a Codex OAuth flow via Codex management API.
// It proxies GET /v0/management/codex-auth-url?is_webui=true and returns {url, state}.
func (h *Handler) StartCodexOAuth(c *gin.Context) {
	channel, err := h.validateCodexChannel(c)
	if err != nil {
		return
	}
	_ = channel

	if err := h.ensureCodexManagementConfigured(); err != nil {
		errorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	query := url.Values{"is_webui": []string{"true"}}
	authDir, err := h.resolveCodexLocalAuthDir()
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	existingFiles, err := h.listLocalAuthFiles(authDir)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	snapshot := make(map[string]struct{}, len(existingFiles))
	for _, file := range existingFiles {
		snapshot[file.Name] = struct{}{}
	}
	var resp struct {
		URL   string `json:"url"`
		State string `json:"state"`
	}
	if err := h.codexManagementCall(c, http.MethodGet, "/codex-auth-url", query, nil, &resp); err != nil {
		errorJSON(c, http.StatusBadGateway, err.Error())
		return
	}
	codexOAuthSessions.Store(resp.State, codexOAuthSession{ChannelID: channel.ID, Existing: snapshot})
	successJSON(c, gin.H{"url": resp.URL, "state": resp.State})
}

// GetCodexOAuthStatus polls the OAuth flow status from Codex management API.
// It proxies GET /v0/management/get-auth-status?state=xxx and returns {status, error?}.
func (h *Handler) GetCodexOAuthStatus(c *gin.Context) {
	channel, err := h.validateCodexChannel(c)
	if err != nil {
		return
	}
	_ = channel

	if err := h.ensureCodexManagementConfigured(); err != nil {
		errorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	state := strings.TrimSpace(c.Query("state"))
	if state == "" {
		errorJSON(c, http.StatusBadRequest, "state parameter is required")
		return
	}

	query := url.Values{"state": []string{state}}
	var resp struct {
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}
	if err := h.codexManagementCall(c, http.MethodGet, "/get-auth-status", query, nil, &resp); err != nil {
		errorJSON(c, http.StatusBadGateway, err.Error())
		return
	}
	if resp.Status == "ok" {
		if v, ok := codexOAuthSessions.LoadAndDelete(state); ok {
			if session, ok := v.(codexOAuthSession); ok {
				if err := h.importOAuthAuthFilesToDB(c.Request.Context(), session.ChannelID, session.Existing); err != nil {
					errorJSON(c, http.StatusInternalServerError, err.Error())
					return
				}
			}
		}
	}
	if resp.Status == "error" {
		codexOAuthSessions.Delete(state)
	}
	result := gin.H{"status": resp.Status}
	if resp.Error != "" {
		result["error"] = resp.Error
	}
	successJSON(c, result)
}

// SyncCodexKeys fetches auth files from Codex auth management and syncs them as channel keys.
func (h *Handler) SyncCodexKeys(c *gin.Context) {
	channel, err := h.validateCodexChannel(c)
	if err != nil {
		return
	}
	var files []codexAuthFile
	if h.codexCapabilities().LocalEnabled {
		files, err = h.listManagedCodexAuthFiles(c.Request.Context(), channel.ID)
		if err != nil {
			errorJSON(c, http.StatusBadGateway, err.Error())
			return
		}
	} else {
		if err := h.ensureCodexManagementConfigured(); err != nil {
			errorJSON(c, http.StatusBadRequest, err.Error())
			return
		}
		var resp struct {
			Files []map[string]any `json:"files"`
		}
		if err := h.codexManagementCall(c, http.MethodGet, "/auth-files", nil, nil, &resp); err != nil {
			errorJSON(c, http.StatusBadGateway, err.Error())
			return
		}
		files = parseAuthFiles(resp.Files)
	}
	// Filter to codex provider only
	var codexFiles []codexAuthFile
	for _, f := range files {
		if strings.EqualFold(f.Provider, "codex") && !f.Disabled {
			codexFiles = append(codexFiles, f)
		}
	}

	// Convert auth files to channel keys
	keys := make([]types.ChannelKeyInput, 0, len(codexFiles))
	for _, f := range codexFiles {
		remark := f.Email
		if remark == "" {
			remark = f.Name
		}
		key := f.AuthIndex
		if key == "" {
			key = f.Name
		}
		keys = append(keys, types.ChannelKeyInput{
			ChannelKey: key,
			Remark:     remark,
		})
	}

	// Sync keys to channel
	if err := dal.SyncChannelKeys(c.Request.Context(), h.DB, channel.ID, keys); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.bestEffortSyncCodexChannelModels(c.Request.Context(), channel.ID)

	h.Cache.Delete("channels")
	successJSON(c, gin.H{"synced": len(keys), "authFiles": len(codexFiles)})
}

func (h *Handler) collectCodexChannelModels(ctx context.Context, channelID int, files []codexAuthFile) ([]string, error) {
	if err := h.ensureCodexManagementConfigured(); err != nil {
		return nil, err
	}
	models := make([]string, 0)
	seen := make(map[string]struct{})
	var firstErr error
	successCount := 0
	for _, file := range files {
		if file.Disabled || !strings.EqualFold(file.Provider, "codex") {
			continue
		}
		query := url.Values{"name": []string{managedAuthRelativeName(channelID, file.Name)}}
		var out struct {
			Models []struct {
				ID string `json:"id"`
			} `json:"models"`
		}
		if err := h.codexManagementCallContext(ctx, http.MethodGet, "/auth-files/models", query, nil, &out); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		successCount++
		for _, model := range out.Models {
			id := strings.TrimSpace(model.ID)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			models = append(models, id)
		}
	}
	if successCount == 0 && firstErr != nil {
		return nil, firstErr
	}
	return models, nil
}

func (h *Handler) persistCodexChannelModels(ctx context.Context, channelID int, models []string) error {
	encoded, err := json.Marshal(models)
	if err != nil {
		return fmt.Errorf("marshal codex channel models: %w", err)
	}
	return dal.UpdateChannel(ctx, h.DB, channelID, map[string]any{
		"model":         string(encoded),
		"fetched_model": string(encoded),
	})
}

func (h *Handler) syncCodexChannelModels(ctx context.Context, channelID int) error {
	if h == nil || h.DB == nil {
		return nil
	}
	files, err := h.listManagedCodexAuthFiles(ctx, channelID)
	if err != nil {
		return err
	}
	models, err := h.collectCodexChannelModels(ctx, channelID, files)
	if err != nil {
		return err
	}
	if err := h.persistCodexChannelModels(ctx, channelID, models); err != nil {
		return err
	}
	h.Cache.Delete("channels")
	return nil
}

func (h *Handler) bestEffortSyncCodexChannelModels(ctx context.Context, channelID int) {
	if err := h.syncCodexChannelModels(ctx, channelID); err != nil {
		log.Printf("[codex] sync channel %d models failed: %v", channelID, err)
	}
}

func (h *Handler) fetchCodexQuota(c *gin.Context, authIndex string, accountID string) (codexQuotaWindow, codexQuotaWindow, string, error) {
	req := map[string]any{
		"authIndex": authIndex,
		"method":    "GET",
		"url":       codexQuotaEndpoint,
		"header": map[string]string{
			"Authorization":      "Bearer $TOKEN$",
			"Chatgpt-Account-Id": accountID,
			"Content-Type":       "application/json",
			"User-Agent":         "wheel/codex-quota",
		},
	}

	var out struct {
		StatusCode int    `json:"status_code"`
		Body       string `json:"body"`
	}
	if err := h.codexManagementCall(c, http.MethodPost, "/api-call", nil, req, &out); err != nil {
		return codexQuotaWindow{}, codexQuotaWindow{}, "", err
	}
	if out.StatusCode < 200 || out.StatusCode >= 300 {
		return codexQuotaWindow{}, codexQuotaWindow{}, "", fmt.Errorf("quota request returned status %d", out.StatusCode)
	}
	return parseCodexQuotaBody(out.Body)
}

func (h *Handler) fetchLocalCodexQuota(ctx context.Context, raw map[string]any) (codexQuotaWindow, codexQuotaWindow, string, error) {
	accessToken := extractAccessToken(raw)
	if accessToken == "" {
		return codexQuotaWindow{}, codexQuotaWindow{}, "", fmt.Errorf("missing access token")
	}
	accountID := stringFromMap(raw, "account_id", "accountId")
	if accountID == "" {
		accountID = extractCodexAccountID(raw)
	}
	if accountID == "" {
		return codexQuotaWindow{}, codexQuotaWindow{}, "", fmt.Errorf("missing chatgpt account id")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, codexQuotaEndpoint, nil)
	if err != nil {
		return codexQuotaWindow{}, codexQuotaWindow{}, "", fmt.Errorf("build quota request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Chatgpt-Account-Id", accountID)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "wheel/codex-quota")

	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return codexQuotaWindow{}, codexQuotaWindow{}, "", fmt.Errorf("request codex quota: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return codexQuotaWindow{}, codexQuotaWindow{}, "", fmt.Errorf("read codex quota response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return codexQuotaWindow{}, codexQuotaWindow{}, "", fmt.Errorf("quota request returned status %d", resp.StatusCode)
	}
	return parseCodexQuotaBody(string(body))
}

func parseCodexQuotaBody(body string) (codexQuotaWindow, codexQuotaWindow, string, error) {
	bodyMap := map[string]any{}
	if err := json.Unmarshal([]byte(body), &bodyMap); err != nil {
		return codexQuotaWindow{}, codexQuotaWindow{}, "", fmt.Errorf("invalid quota response")
	}

	planType := stringFromMap(bodyMap, "plan_type", "planType")
	weeklyRate := mapFromAny(bodyMap["rate_limit"])
	if len(weeklyRate) == 0 {
		weeklyRate = mapFromAny(bodyMap["rateLimit"])
	}
	codeReviewRate := mapFromAny(bodyMap["code_review_rate_limit"])
	if len(codeReviewRate) == 0 {
		codeReviewRate = mapFromAny(bodyMap["codeReviewRateLimit"])
	}
	if len(codeReviewRate) == 0 {
		additional := sliceFromAny(bodyMap["additional_rate_limits"])
		if len(additional) == 0 {
			additional = sliceFromAny(bodyMap["additionalRateLimits"])
		}
		for _, item := range additional {
			rate := mapFromAny(valueByKeys(mapFromAny(item), "rate_limit", "rateLimit"))
			if len(rate) == 0 {
				continue
			}
			feature := strings.ToLower(stringFromMap(mapFromAny(item), "metered_feature", "meteredFeature", "limit_name", "limitName"))
			if strings.Contains(feature, "review") || strings.Contains(feature, "other") || len(codeReviewRate) == 0 {
				codeReviewRate = rate
				if strings.Contains(feature, "review") || strings.Contains(feature, "other") {
					break
				}
			}
		}
	}

	weekly := parseQuotaWindow(mapFromAny(valueByKeys(weeklyRate, "secondary_window", "secondaryWindow")))
	if weekly.LimitWindowSeconds == 0 {
		weekly = parseQuotaWindow(mapFromAny(valueByKeys(weeklyRate, "primary_window", "primaryWindow")))
	}
	weekly.Allowed = boolFromMap(weeklyRate, "allowed")
	weekly.LimitReached = boolFromMap(weeklyRate, "limit_reached", "limitReached")

	codeReview := parseQuotaWindow(mapFromAny(valueByKeys(codeReviewRate, "primary_window", "primaryWindow")))
	if codeReview.LimitWindowSeconds == 0 {
		codeReview = parseQuotaWindow(mapFromAny(valueByKeys(codeReviewRate, "secondary_window", "secondaryWindow")))
	}
	codeReview.Allowed = boolFromMap(codeReviewRate, "allowed")
	codeReview.LimitReached = boolFromMap(codeReviewRate, "limit_reached", "limitReached")

	return weekly, codeReview, planType, nil
}

func (h *Handler) ensureCodexManagementConfigured() error {
	if h == nil || h.Config == nil {
		return fmt.Errorf("handler config not initialized")
	}
	if strings.TrimSpace(h.Config.CodexRuntimeManagementURL) == "" {
		return fmt.Errorf("codex runtime management URL is not configured")
	}
	if strings.TrimSpace(h.Config.CodexRuntimeManagementKey) == "" {
		return fmt.Errorf("codex runtime management key is not configured")
	}
	return nil
}

func (h *Handler) codexManagementCall(c *gin.Context, method string, path string, query url.Values, reqBody any, out any) error {
	return h.codexManagementCallContext(c.Request.Context(), method, path, query, reqBody, out)
}

func (h *Handler) codexManagementCallContext(ctx context.Context, method string, path string, query url.Values, reqBody any, out any) error {
	base := strings.TrimRight(strings.TrimSpace(h.Config.CodexRuntimeManagementURL), "/")
	fullURL := base + "/v0/management" + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	var payload io.Reader
	if reqBody != nil {
		data, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		payload = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, payload)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Management-Key", strings.TrimSpace(h.Config.CodexRuntimeManagementKey))

	httpClient := &http.Client{Timeout: 60 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request codex management: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read codex management response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("codex management error: %s", msg)
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("decode codex management response: %w", err)
		}
	}

	return nil
}

func (h *Handler) codexManagementUploadFile(ctx context.Context, filename string, content []byte, out any) error {
	base := strings.TrimRight(strings.TrimSpace(h.Config.CodexRuntimeManagementURL), "/")
	fullURL := base + "/v0/management/auth-files"

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return fmt.Errorf("create multipart form file: %w", err)
	}
	if _, err := part.Write(content); err != nil {
		return fmt.Errorf("write multipart file content: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, &body)
	if err != nil {
		return fmt.Errorf("build upload request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Management-Key", strings.TrimSpace(h.Config.CodexRuntimeManagementKey))

	httpClient := &http.Client{Timeout: 60 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request codex management upload: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read codex management upload response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(respBody))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("codex management error: %s", msg)
	}

	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode codex management upload response: %w", err)
		}
	}

	return nil
}

func parseAuthFiles(files []map[string]any) []codexAuthFile {
	out := make([]codexAuthFile, 0, len(files))
	for _, raw := range files {
		if len(raw) == 0 {
			continue
		}
		entry := codexAuthFile{
			Name:      stringFromMap(raw, "name"),
			Provider:  strings.ToLower(stringFromMap(raw, "provider")),
			Type:      strings.ToLower(stringFromMap(raw, "type")),
			Email:     stringFromMap(raw, "email"),
			Disabled:  boolFromMap(raw, "disabled"),
			AuthIndex: stringFromMap(raw, "auth_index", "authIndex"),
			Raw:       raw,
		}
		if entry.Provider == "" {
			entry.Provider = entry.Type
		}
		if entry.Name == "" {
			entry.Name = stringFromMap(raw, "id")
		}
		if entry.Name == "" {
			continue
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func filterAndPaginateAuthFiles(files []codexAuthFile, provider string, search string, page int, pageSize int) ([]codexAuthFile, int) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	search = strings.ToLower(strings.TrimSpace(search))

	filtered := make([]codexAuthFile, 0, len(files))
	for _, file := range files {
		p := strings.ToLower(strings.TrimSpace(file.Provider))
		if p == "" {
			p = strings.ToLower(strings.TrimSpace(file.Type))
		}
		if provider != "" && p != provider {
			continue
		}
		if search != "" {
			hit := strings.Contains(strings.ToLower(file.Name), search) || strings.Contains(strings.ToLower(file.Email), search)
			if !hit {
				continue
			}
		}
		filtered = append(filtered, file)
	}

	total := len(filtered)
	if total == 0 {
		return []codexAuthFile{}, 0
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= total {
		return []codexAuthFile{}, total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return filtered[start:end], total
}

func extractCodexAccountID(auth map[string]any) string {
	idToken := mapFromAny(auth["id_token"])
	if len(idToken) == 0 {
		return ""
	}
	return stringFromMap(idToken, "chatgpt_account_id", "chatgptAccountId")
}

func extractAccessToken(auth map[string]any) string {
	if token := stringFromMap(auth, "access_token", "accessToken"); token != "" {
		return token
	}
	tokenMap := mapFromAny(auth["token"])
	return stringFromMap(tokenMap, "access_token", "accessToken")
}

func parseQuotaWindow(raw map[string]any) codexQuotaWindow {
	if len(raw) == 0 {
		return codexQuotaWindow{}
	}
	return codexQuotaWindow{
		UsedPercent:        floatFromMap(raw, "used_percent", "usedPercent"),
		LimitWindowSeconds: int64FromMap(raw, "limit_window_seconds", "limitWindowSeconds"),
		ResetAfterSeconds:  int64FromMap(raw, "reset_after_seconds", "resetAfterSeconds"),
		ResetAt:            stringFromMap(raw, "reset_at", "resetAt"),
	}
}

func parsePositiveInt(raw string, fallback int) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func valueByKeys(m map[string]any, keys ...string) any {
	if len(m) == 0 {
		return nil
	}
	for _, key := range keys {
		if value, ok := m[key]; ok {
			return value
		}
	}
	return nil
}

func mapFromAny(v any) map[string]any {
	m, ok := v.(map[string]any)
	if !ok || len(m) == 0 {
		return map[string]any{}
	}
	return m
}

func sliceFromAny(v any) []map[string]any {
	items, ok := v.([]any)
	if !ok || len(items) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m := mapFromAny(item); len(m) > 0 {
			out = append(out, m)
		}
	}
	return out
}

func stringFromMap(m map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := m[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if s := strings.TrimSpace(typed); s != "" {
				return s
			}
		}
	}
	return ""
}

func boolFromMap(m map[string]any, keys ...string) bool {
	for _, key := range keys {
		value, ok := m[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed
		case string:
			v := strings.TrimSpace(strings.ToLower(typed))
			if v == "true" || v == "1" || v == "yes" {
				return true
			}
		}
	}
	return false
}

func int64FromMap(m map[string]any, keys ...string) int64 {
	for _, key := range keys {
		value, ok := m[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return int64(typed)
		case float32:
			return int64(typed)
		case int:
			return int64(typed)
		case int32:
			return int64(typed)
		case int64:
			return typed
		case string:
			if v, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64); err == nil {
				return v
			}
		}
	}
	return 0
}

func floatFromMap(m map[string]any, keys ...string) float64 {
	for _, key := range keys {
		value, ok := m[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return typed
		case float32:
			return float64(typed)
		case int:
			return float64(typed)
		case int32:
			return float64(typed)
		case int64:
			return float64(typed)
		case string:
			if v, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
				return v
			}
		}
	}
	return 0
}
