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
	"github.com/kunish/wheel/apps/worker/internal/runtimeauth"
	"github.com/kunish/wheel/apps/worker/internal/types"
	"github.com/uptrace/bun"
)

const (
	codexQuotaEndpoint         = "https://chatgpt.com/backend-api/wham/usage"
	codexQuotaFetchConcurrency = 4
	codexModelSyncRetryWindow  = time.Second
	codexModelSyncRetryDelay   = 100 * time.Millisecond
)

type codexAuthFile struct {
	ID         int            `json:"-"`
	ChannelID  int            `json:"-"`
	Name       string         `json:"name"`
	Provider   string         `json:"provider"`
	Type       string         `json:"type"`
	Email      string         `json:"email,omitempty"`
	Disabled   bool           `json:"disabled"`
	AuthIndex  string         `json:"authIndex,omitempty"`
	Path       string         `json:"-"`
	RawContent string         `json:"-"`
	Raw        map[string]any `json:"-"`
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
	Snapshots  []quotaSnapshot  `json:"snapshots,omitempty"`
	ResetAt    string           `json:"resetAt,omitempty"`
	Error      string           `json:"error,omitempty"`
}

type quotaSnapshot struct {
	ID               string  `json:"id"`
	Label            string  `json:"label"`
	PercentRemaining float64 `json:"percentRemaining"`
	Remaining        float64 `json:"remaining,omitempty"`
	Entitlement      float64 `json:"entitlement,omitempty"`
	Unlimited        bool    `json:"unlimited,omitempty"`
}

type codexOAuthSession struct {
	ChannelID int
	Existing  map[string]struct{}
}

type codexUploadFile struct {
	Name    string
	Content []byte
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

type codexAuthBatchScope struct {
	Names        []string `json:"names"`
	AllMatching  bool     `json:"allMatching"`
	Search       string   `json:"search"`
	Provider     string   `json:"provider"`
	ExcludeNames []string `json:"excludeNames"`
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
			Provider:  canonicalRuntimeProvider(stringFromMap(raw, "provider", "type")),
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
	return runtimeauth.EnsureAuthIndex(name, "", "")
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
		provider := canonicalRuntimeProvider(item.Provider)
		if provider == "" {
			provider = canonicalRuntimeProvider(stringFromMap(raw, "type", "provider"))
		}
		if provider == "" {
			provider = "codex"
		}
		out = append(out, codexAuthFile{
			ID:         item.ID,
			ChannelID:  item.ChannelID,
			Name:       item.Name,
			Provider:   provider,
			Type:       provider,
			Email:      item.Email,
			Disabled:   item.Disabled,
			AuthIndex:  managedAuthIndex(item.ChannelID, item.Name),
			RawContent: item.Content,
			Raw:        raw,
		})
	}
	return out, nil
}

func parseCodexAuthContent(content []byte) (provider string, email string, disabled bool, normalized string, raw map[string]any, err error) {
	raw = map[string]any{}
	if err = json.Unmarshal(content, &raw); err != nil {
		return "", "", false, "", nil, fmt.Errorf("invalid auth file json")
	}
	provider = canonicalRuntimeProvider(stringFromMap(raw, "type", "provider"))
	if provider == "" {
		provider = "codex" // default; overridden for copilot channels at upload time
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

func (h *Handler) PatchCodexAuthFileStatusBatch(c *gin.Context) {
	channel, err := h.validateCodexChannel(c)
	if err != nil {
		return
	}

	var req struct {
		codexAuthBatchScope
		Disabled bool `json:"disabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	files, err := h.listAllCodexAuthFiles(c.Request.Context(), channel.ID)
	if err != nil {
		errorJSON(c, http.StatusBadGateway, err.Error())
		return
	}
	selected, err := selectCodexAuthFilesForBatch(files, req.codexAuthBatchScope)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	response := codexAuthUploadResponse{Total: len(selected), Results: make([]codexAuthUploadResult, 0, len(selected))}
	if h.codexCapabilities().LocalEnabled {
		for _, file := range selected {
			result := h.patchCodexLocalAuthFileStatus(file, req.Disabled)
			response.Results = append(response.Results, result)
			if result.Status == "ok" {
				response.SuccessCount++
			} else {
				response.FailedCount++
			}
		}
		if response.SuccessCount > 0 {
			h.bestEffortSyncCodexChannelModels(c.Request.Context(), channel.ID)
		}
	} else {
		if err := h.ensureCodexManagementConfigured(); err != nil {
			errorJSON(c, http.StatusBadRequest, err.Error())
			return
		}
		for _, file := range selected {
			result := h.patchCodexManagedAuthFileStatus(c, file.Name, req.Disabled)
			response.Results = append(response.Results, result)
			if result.Status == "ok" {
				response.SuccessCount++
			} else {
				response.FailedCount++
			}
		}
	}
	successJSON(c, response)
}

func (h *Handler) UploadCodexAuthFile(c *gin.Context) {
	channel, err := h.validateCodexChannel(c)
	if err != nil {
		return
	}

	files, err := collectCodexUploadFiles(c)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "failed to parse multipart form")
		return
	}
	if len(files) == 0 {
		errorJSON(c, http.StatusBadRequest, "file is required")
		return
	}

	response := codexAuthUploadResponse{
		Total:   len(files),
		Results: make([]codexAuthUploadResult, 0, len(files)),
	}
	if h.codexCapabilities().LocalEnabled {
		for _, file := range files {
			result := h.uploadCodexLocalAuthFile(c.Request.Context(), channel.ID, file)
			response.Results = append(response.Results, result)
			if result.Status == "ok" {
				response.SuccessCount++
				continue
			}
			response.FailedCount++
		}
		if response.SuccessCount > 0 {
			if err := codexruntime.MaterializeChannelAuthFiles(c.Request.Context(), h.DB, channel.ID); err != nil {
				log.Printf("[codex] materialize channel %d auth files failed: %v", channel.ID, err)
			} else {
				h.bestEffortSyncCodexChannelModels(c.Request.Context(), channel.ID)
			}
		}
	} else {
		if err := h.ensureCodexManagementConfigured(); err != nil {
			errorJSON(c, http.StatusBadRequest, err.Error())
			return
		}
		for _, file := range files {
			result := h.uploadCodexManagedAuthFile(c.Request.Context(), file)
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

func collectCodexUploadFiles(c *gin.Context) ([]codexUploadFile, error) {
	reader, err := c.Request.MultipartReader()
	if err != nil {
		return nil, err
	}

	files := make([]codexUploadFile, 0, 1)
	for {
		part, err := reader.NextPart()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if part.FormName() != "files" && part.FormName() != "file" {
			_ = part.Close()
			continue
		}

		filename := filepath.Base(part.FileName())
		if filename == "" {
			_ = part.Close()
			continue
		}

		content, err := io.ReadAll(part)
		_ = part.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read uploaded file")
		}

		files = append(files, codexUploadFile{Name: filename, Content: content})
	}

	return files, nil
}

func (h *Handler) uploadCodexLocalAuthFile(ctx context.Context, channelID int, file codexUploadFile) codexAuthUploadResult {
	result := codexAuthUploadResult{Name: file.Name}
	content := file.Content
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
	result.Status = "ok"
	return result
}

func (h *Handler) uploadCodexManagedAuthFile(ctx context.Context, file codexUploadFile) codexAuthUploadResult {
	result := codexAuthUploadResult{Name: file.Name}
	content := file.Content
	var out map[string]any
	if err := h.codexManagementUploadFile(ctx, file.Name, content, &out); err != nil {
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

func (h *Handler) DeleteCodexAuthFileBatch(c *gin.Context) {
	channel, err := h.validateCodexChannel(c)
	if err != nil {
		return
	}

	var req codexAuthBatchScope
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	files, err := h.listAllCodexAuthFiles(c.Request.Context(), channel.ID)
	if err != nil {
		errorJSON(c, http.StatusBadGateway, err.Error())
		return
	}
	selected, err := selectCodexAuthFilesForBatch(files, req)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	response := codexAuthUploadResponse{Total: len(selected), Results: make([]codexAuthUploadResult, 0, len(selected))}
	if h.codexCapabilities().LocalEnabled {
		for _, file := range selected {
			result := h.deleteCodexLocalAuthFile(c.Request.Context(), channel.ID, file)
			response.Results = append(response.Results, result)
			if result.Status == "ok" {
				response.SuccessCount++
			} else {
				response.FailedCount++
			}
		}
		if response.SuccessCount > 0 {
			h.bestEffortSyncCodexChannelModels(c.Request.Context(), channel.ID)
		}
	} else {
		if err := h.ensureCodexManagementConfigured(); err != nil {
			errorJSON(c, http.StatusBadRequest, err.Error())
			return
		}
		for _, file := range selected {
			result := h.deleteCodexManagedAuthFile(c, file.Name)
			response.Results = append(response.Results, result)
			if result.Status == "ok" {
				response.SuccessCount++
			} else {
				response.FailedCount++
			}
		}
	}
	successJSON(c, response)
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

	search := strings.TrimSpace(c.Query("search"))
	page := parsePositiveInt(c.Query("page"), 1)
	pageSize := parsePositiveInt(c.Query("pageSize"), 12)
	if pageSize > 50 {
		pageSize = 50
	}

	providerFilter := runtimeProviderFilter(channel.Type)

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

	paged, total := filterAndPaginateAuthFiles(files, providerFilter, search, page, pageSize)

	if channel.Type == types.OutboundCopilot {
		items := h.collectCopilotQuotaItems(c.Request.Context(), paged, codexQuotaFetchConcurrency, func(ctx context.Context, file codexAuthFile) ([]quotaSnapshot, string, string, error) {
			if h.codexCapabilities().LocalEnabled {
				return h.fetchLocalCopilotQuota(ctx, file.Raw)
			}
			return h.fetchCopilotQuota(ctx, file.AuthIndex)
		})
		successJSON(c, gin.H{"items": items, "total": total, "page": page, "pageSize": pageSize})
		return
	}

	items := h.collectCodexQuotaItems(c.Request.Context(), paged, codexQuotaFetchConcurrency, func(ctx context.Context, file codexAuthFile) (codexQuotaWindow, codexQuotaWindow, string, error) {
		accountID := stringFromMap(file.Raw, "account_id", "accountId")
		if accountID == "" {
			accountID = extractCodexAccountID(file.Raw)
		}
		if h.codexCapabilities().LocalEnabled {
			return h.fetchLocalCodexQuota(ctx, file.Raw)
		}
		return h.fetchCodexQuota(ctx, file.AuthIndex, accountID)
	})

	successJSON(c, gin.H{"items": items, "total": total, "page": page, "pageSize": pageSize})
}

func (h *Handler) collectCodexQuotaItems(ctx context.Context, files []codexAuthFile, concurrency int, fetch func(context.Context, codexAuthFile) (codexQuotaWindow, codexQuotaWindow, string, error)) []codexQuotaItem {
	items := make([]codexQuotaItem, len(files))
	if concurrency <= 1 {
		for i, file := range files {
			items[i] = buildCodexQuotaItem(ctx, file, fetch)
		}
		return items
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, file := range files {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, file codexAuthFile) {
			defer wg.Done()
			defer func() { <-sem }()
			items[i] = buildCodexQuotaItem(ctx, file, fetch)
		}(i, file)
	}
	wg.Wait()
	return items
}

func (h *Handler) collectCopilotQuotaItems(ctx context.Context, files []codexAuthFile, concurrency int, fetch func(context.Context, codexAuthFile) ([]quotaSnapshot, string, string, error)) []codexQuotaItem {
	items := make([]codexQuotaItem, len(files))
	if concurrency <= 1 {
		for i, file := range files {
			items[i] = buildCopilotQuotaItem(ctx, file, fetch)
		}
		return items
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, file := range files {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, file codexAuthFile) {
			defer wg.Done()
			defer func() { <-sem }()
			items[i] = buildCopilotQuotaItem(ctx, file, fetch)
		}(i, file)
	}
	wg.Wait()
	return items
}

func buildCopilotQuotaItem(ctx context.Context, file codexAuthFile, fetch func(context.Context, codexAuthFile) ([]quotaSnapshot, string, string, error)) codexQuotaItem {
	item := codexQuotaItem{
		Name:      file.Name,
		Email:     file.Email,
		AuthIndex: file.AuthIndex,
	}
	if file.AuthIndex == "" {
		item.Error = "missing auth_index"
		return item
	}
	snapshots, planType, resetAt, err := fetch(ctx, file)
	if err != nil {
		item.Error = err.Error()
		return item
	}
	if len(snapshots) == 0 {
		item.Error = "copilot quota unavailable"
		return item
	}
	item.PlanType = planType
	item.ResetAt = resetAt
	item.Snapshots = snapshots
	return item
}

func buildCodexQuotaItem(ctx context.Context, file codexAuthFile, fetch func(context.Context, codexAuthFile) (codexQuotaWindow, codexQuotaWindow, string, error)) codexQuotaItem {
	item := codexQuotaItem{
		Name:      file.Name,
		Email:     file.Email,
		AuthIndex: file.AuthIndex,
	}
	if file.AuthIndex == "" {
		item.Error = "missing auth_index"
		return item
	}
	accountID := stringFromMap(file.Raw, "account_id", "accountId")
	if accountID == "" {
		accountID = extractCodexAccountID(file.Raw)
	}
	if accountID == "" {
		item.Error = "missing chatgpt account id"
		return item
	}
	weekly, codeReview, planType, err := fetch(ctx, file)
	if err != nil {
		item.Error = err.Error()
		return item
	}
	item.PlanType = planType
	item.Weekly = weekly
	item.CodeReview = codeReview
	return item
}

// isRuntimeChannel returns true if the channel type uses the embedded CLIProxyAPI runtime.
func isRuntimeChannel(t types.OutboundType) bool {
	return t == types.OutboundCodex || t == types.OutboundCopilot
}

// runtimeProviderFilter returns the auth file provider filter for the given runtime channel type.
func runtimeProviderFilter(t types.OutboundType) string {
	switch t {
	case types.OutboundCopilot:
		return "copilot"
	default:
		return "codex"
	}
}

func canonicalRuntimeProvider(provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	switch provider {
	case "github-copilot", "github", "copilot":
		return "copilot"
	default:
		return provider
	}
}

func runtimeProviderMatches(channelType types.OutboundType, provider string) bool {
	return canonicalRuntimeProvider(provider) == runtimeProviderFilter(channelType)
}

// validateCodexChannel verifies the channel exists and is a runtime-managed type (Codex or Copilot).
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
	if !isRuntimeChannel(channel.Type) {
		errorJSON(c, http.StatusBadRequest, "channel is not a Codex/Copilot channel")
		return nil, fmt.Errorf("not a runtime channel")
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
	// Filter to matching provider only
	var codexFiles []codexAuthFile
	for _, f := range files {
		if runtimeProviderMatches(channel.Type, f.Provider) && !f.Disabled {
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

func (h *Handler) collectCodexChannelModels(ctx context.Context, channelID int, channelType types.OutboundType, files []codexAuthFile) ([]string, error) {
	if err := h.ensureCodexManagementConfigured(); err != nil {
		return nil, err
	}
	providerFilter := runtimeProviderFilter(channelType)
	localMode := h.codexCapabilities().LocalEnabled
	var retryUntil time.Time
	if localMode {
		retryUntil = time.Now().Add(codexModelSyncRetryWindow)
	}
	models := make([]string, 0)
	seen := make(map[string]struct{})
	var firstErr error
	successCount := 0
	for _, file := range files {
		if file.Disabled || canonicalRuntimeProvider(file.Provider) != providerFilter {
			continue
		}
		query := url.Values{"name": []string{managedAuthRelativeName(channelID, file.Name)}}
		out, err := h.listCodexAuthFileModelsWithRetry(ctx, query, retryUntil)
		if err != nil {
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

func (h *Handler) listCodexAuthFileModelsWithRetry(ctx context.Context, query url.Values, retryUntil time.Time) (struct {
	Models []struct {
		ID string `json:"id"`
	} `json:"models"`
}, error) {
	var out struct {
		Models []struct {
			ID string `json:"id"`
		} `json:"models"`
	}

	for {
		err := h.codexManagementCallContext(ctx, http.MethodGet, "/auth-files/models", query, nil, &out)
		shouldRetry := !retryUntil.IsZero() && time.Now().Before(retryUntil)
		if err == nil && (len(out.Models) > 0 || !shouldRetry) {
			return out, nil
		}
		if err != nil && !shouldRetry {
			return out, err
		}
		if !shouldRetry {
			return out, nil
		}

		timer := time.NewTimer(codexModelSyncRetryDelay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return out, ctx.Err()
		case <-timer.C:
		}
	}

	return out, nil
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
	// Look up channel type for provider filtering
	channel, err := dal.GetChannel(ctx, h.DB, channelID)
	if err != nil {
		return err
	}
	if channel == nil {
		return fmt.Errorf("channel %d not found", channelID)
	}
	files, err := h.listManagedCodexAuthFiles(ctx, channelID)
	if err != nil {
		return err
	}
	models, err := h.collectCodexChannelModels(ctx, channelID, channel.Type, files)
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

func (h *Handler) fetchCodexQuota(ctx context.Context, authIndex string, accountID string) (codexQuotaWindow, codexQuotaWindow, string, error) {
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
	if err := h.codexManagementCallContext(ctx, http.MethodPost, "/api-call", nil, req, &out); err != nil {
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

	resp, err := h.doCodexQuotaRequest(req)
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

func (h *Handler) fetchCopilotQuota(ctx context.Context, authIndex string) ([]quotaSnapshot, string, string, error) {
	query := url.Values{}
	if strings.TrimSpace(authIndex) != "" {
		query.Set("auth_index", authIndex)
	}
	var out struct {
		AccessTypeSKU     string         `json:"access_type_sku"`
		CopilotPlan       string         `json:"copilot_plan"`
		QuotaResetDate    string         `json:"quota_reset_date"`
		QuotaSnapshots    map[string]any `json:"quota_snapshots"`
		MonthlyQuotas     map[string]any `json:"monthly_quotas"`
		LimitedUser       map[string]any `json:"limited_user_quotas"`
		LimitedReset      string         `json:"limited_user_reset_date"`
		QuotaResetDateUTC string         `json:"quota_reset_date_utc"`
	}
	if err := h.codexManagementCallContext(ctx, http.MethodGet, "/copilot-quota", query, nil, &out); err != nil {
		return nil, "", "", err
	}
	body, err := json.Marshal(out)
	if err != nil {
		return nil, "", "", fmt.Errorf("encode copilot quota response: %w", err)
	}
	return parseCopilotQuotaBody(string(body))
}

func (h *Handler) fetchLocalCopilotQuota(ctx context.Context, raw map[string]any) ([]quotaSnapshot, string, string, error) {
	accessToken := extractAccessToken(raw)
	if accessToken == "" {
		return nil, "", "", fmt.Errorf("missing access token")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/copilot_internal/user", nil)
	if err != nil {
		return nil, "", "", fmt.Errorf("build copilot quota request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", "wheel/copilot-quota")
	req.Header.Set("Accept", "application/json")

	resp, err := h.doCodexQuotaRequest(req)
	if err != nil {
		return nil, "", "", fmt.Errorf("request copilot quota: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", "", fmt.Errorf("read copilot quota response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", "", fmt.Errorf("quota request returned status %d", resp.StatusCode)
	}
	return parseCopilotQuotaBody(string(body))
}

func (h *Handler) doCodexQuotaRequest(req *http.Request) (*http.Response, error) {
	if h != nil && h.codexQuotaDo != nil {
		return h.codexQuotaDo(req)
	}
	return (&http.Client{Timeout: 60 * time.Second}).Do(req)
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

func parseCopilotQuotaBody(body string) ([]quotaSnapshot, string, string, error) {
	bodyMap := map[string]any{}
	if err := json.Unmarshal([]byte(body), &bodyMap); err != nil {
		return nil, "", "", fmt.Errorf("invalid copilot quota response")
	}

	planType := stringFromMap(bodyMap, "copilot_plan", "access_type_sku", "accessTypeSKU")
	resetAt := stringFromMap(bodyMap, "quota_reset_date", "quotaResetDate", "quota_reset_date_utc", "quotaResetDateUtc", "limited_user_reset_date", "limitedUserResetDate")

	snapshotsMap := mapFromAny(bodyMap["quota_snapshots"])
	if len(snapshotsMap) > 0 {
		snapshots := collectCopilotSnapshotsFromQuotaSnapshots(snapshotsMap)
		if len(snapshots) == 0 {
			return nil, planType, resetAt, fmt.Errorf("copilot quota unavailable")
		}
		return snapshots, planType, resetAt, nil
	}

	monthlyQuotas := mapFromAny(bodyMap["monthly_quotas"])
	if len(monthlyQuotas) == 0 {
		monthlyQuotas = mapFromAny(bodyMap["monthlyQuotas"])
	}
	limitedQuotas := mapFromAny(bodyMap["limited_user_quotas"])
	if len(limitedQuotas) == 0 {
		limitedQuotas = mapFromAny(bodyMap["limitedUserQuotas"])
	}
	snapshots := collectCopilotSnapshotsFromMonthlyQuota(monthlyQuotas, limitedQuotas)
	if len(snapshots) == 0 {
		return nil, planType, resetAt, fmt.Errorf("copilot quota unavailable")
	}
	return snapshots, planType, resetAt, nil
}

func collectCopilotSnapshotsFromQuotaSnapshots(raw map[string]any) []quotaSnapshot {
	keys := []struct {
		id    string
		label string
	}{
		{id: "chat", label: "Chat"},
		{id: "completions", label: "Completions"},
		{id: "premium_interactions", label: "Premium Interactions"},
	}
	out := make([]quotaSnapshot, 0, len(keys))
	for _, key := range keys {
		detail := mapFromAny(raw[key.id])
		if len(detail) == 0 {
			continue
		}
		out = appendCopilotSnapshot(out, key.id, key.label, detail)
	}
	return out
}

func collectCopilotSnapshotsFromMonthlyQuota(monthlyQuotas map[string]any, limitedQuotas map[string]any) []quotaSnapshot {
	keys := []struct {
		id    string
		label string
	}{
		{id: "chat", label: "Chat"},
		{id: "completions", label: "Completions"},
	}
	out := make([]quotaSnapshot, 0, len(keys))
	for _, key := range keys {
		entitlement := floatFromMap(monthlyQuotas, key.id)
		if entitlement <= 0 {
			continue
		}
		remaining := entitlement
		if raw := valueByKeys(limitedQuotas, key.id); raw != nil {
			remaining = floatFromMap(map[string]any{"value": raw}, "value")
		}
		percentRemaining := 0.0
		if entitlement > 0 {
			percentRemaining = (remaining / entitlement) * 100
		}
		out = append(out, quotaSnapshot{
			ID:               key.id,
			Label:            key.label,
			PercentRemaining: clampPercent(percentRemaining),
			Remaining:        remaining,
			Entitlement:      entitlement,
		})
	}
	return out
}

func appendCopilotSnapshot(out []quotaSnapshot, id string, label string, detail map[string]any) []quotaSnapshot {
	if len(detail) == 0 {
		return out
	}
	percentRemaining := floatFromMap(detail, "percent_remaining", "percentRemaining")
	remaining := floatFromMap(detail, "quota_remaining", "quotaRemaining", "remaining")
	entitlement := floatFromMap(detail, "entitlement")
	unlimited := boolFromMap(detail, "unlimited")
	if percentRemaining == 0 && remaining == 0 && entitlement == 0 && !unlimited {
		return out
	}
	return append(out, quotaSnapshot{
		ID:               id,
		Label:            label,
		PercentRemaining: clampPercent(percentRemaining),
		Remaining:        remaining,
		Entitlement:      entitlement,
		Unlimited:        unlimited,
	})
}

func clampPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
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
			Provider:  canonicalRuntimeProvider(stringFromMap(raw, "provider", "type")),
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
	filtered := filterCodexAuthFiles(files, provider, search)

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

func filterCodexAuthFiles(files []codexAuthFile, provider string, search string) []codexAuthFile {
	provider = canonicalRuntimeProvider(provider)
	search = strings.ToLower(strings.TrimSpace(search))

	filtered := make([]codexAuthFile, 0, len(files))
	for _, file := range files {
		p := canonicalRuntimeProvider(file.Provider)
		if p == "" {
			p = canonicalRuntimeProvider(file.Type)
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
	return filtered
}

func selectCodexAuthFilesForBatch(files []codexAuthFile, scope codexAuthBatchScope) ([]codexAuthFile, error) {
	if scope.AllMatching {
		filtered := filterCodexAuthFiles(files, scope.Provider, scope.Search)
		excluded := make(map[string]struct{}, len(scope.ExcludeNames))
		for _, name := range scope.ExcludeNames {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			excluded[name] = struct{}{}
		}
		selected := make([]codexAuthFile, 0, len(filtered))
		for _, file := range filtered {
			if _, ok := excluded[file.Name]; ok {
				continue
			}
			selected = append(selected, file)
		}
		if len(selected) == 0 {
			return nil, fmt.Errorf("no auth files matched selection")
		}
		return selected, nil
	}

	selectedNames := make(map[string]struct{}, len(scope.Names))
	for _, name := range scope.Names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		selectedNames[name] = struct{}{}
	}
	if len(selectedNames) == 0 {
		return nil, fmt.Errorf("names is required")
	}
	selected := make([]codexAuthFile, 0, len(selectedNames))
	for _, file := range files {
		if _, ok := selectedNames[file.Name]; ok {
			selected = append(selected, file)
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no auth files matched selection")
	}
	return selected, nil
}

func (h *Handler) listAllCodexAuthFiles(ctx context.Context, channelID int) ([]codexAuthFile, error) {
	if h.codexCapabilities().LocalEnabled {
		return h.listManagedCodexAuthFiles(ctx, channelID)
	}
	if err := h.ensureCodexManagementConfigured(); err != nil {
		return nil, err
	}
	var resp struct {
		Files []map[string]any `json:"files"`
	}
	if err := h.codexManagementCallContext(ctx, http.MethodGet, "/auth-files", nil, nil, &resp); err != nil {
		return nil, err
	}
	return parseAuthFiles(resp.Files), nil
}

func (h *Handler) patchCodexLocalAuthFileStatus(file codexAuthFile, disabled bool) codexAuthUploadResult {
	result := codexAuthUploadResult{Name: file.Name}
	_, _, _, _, raw, err := parseCodexAuthContent([]byte(file.RawContent))
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result
	}
	raw["disabled"] = disabled
	encoded, err := json.Marshal(raw)
	if err != nil {
		result.Status = "error"
		result.Error = "failed to encode auth file"
		return result
	}
	if err := dal.UpdateCodexAuthFile(context.Background(), h.DB, file.ID, map[string]any{"disabled": disabled, "content": string(encoded)}); err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result
	}
	item := &types.CodexAuthFile{ID: file.ID, ChannelID: file.ChannelID, Name: file.Name, Provider: file.Provider, Email: file.Email, Disabled: disabled, Content: string(encoded)}
	if err := codexruntime.MaterializeOneAuthFile(item); err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result
	}
	result.Status = "ok"
	return result
}

func (h *Handler) patchCodexManagedAuthFileStatus(c *gin.Context, name string, disabled bool) codexAuthUploadResult {
	result := codexAuthUploadResult{Name: name}
	var out map[string]any
	if err := h.codexManagementCall(c, http.MethodPatch, "/auth-files/status", nil, map[string]any{"name": name, "disabled": disabled}, &out); err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result
	}
	result.Status = "ok"
	return result
}

func (h *Handler) deleteCodexLocalAuthFile(ctx context.Context, channelID int, file codexAuthFile) codexAuthUploadResult {
	_ = channelID
	result := codexAuthUploadResult{Name: file.Name}
	if err := dal.DeleteCodexAuthFile(ctx, h.DB, file.ID); err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result
	}
	_ = codexruntime.RemoveOneAuthFile(&types.CodexAuthFile{ID: file.ID, ChannelID: file.ChannelID, Name: file.Name, Provider: file.Provider, Email: file.Email, Disabled: file.Disabled, Content: file.RawContent})
	result.Status = "ok"
	return result
}

func (h *Handler) deleteCodexManagedAuthFile(c *gin.Context, name string) codexAuthUploadResult {
	result := codexAuthUploadResult{Name: name}
	var out map[string]any
	query := url.Values{"name": []string{name}}
	if err := h.codexManagementCall(c, http.MethodDelete, "/auth-files", query, nil, &out); err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result
	}
	result.Status = "ok"
	return result
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
