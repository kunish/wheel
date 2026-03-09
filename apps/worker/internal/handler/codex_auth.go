package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	codexruntime "github.com/kunish/wheel/apps/worker/internal/codexruntime"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/runtimeauth"
	"github.com/kunish/wheel/apps/worker/internal/types"
	"github.com/uptrace/bun"
)

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
			result := h.patchCodexLocalAuthFileStatus(c.Request.Context(), file, req.Disabled)
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

func (h *Handler) patchCodexLocalAuthFileStatus(ctx context.Context, file codexAuthFile, disabled bool) codexAuthUploadResult {
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
	if err := dal.UpdateCodexAuthFile(ctx, h.DB, file.ID, map[string]any{"disabled": disabled, "content": string(encoded)}); err != nil {
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
