package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	codexruntime "github.com/kunish/wheel/apps/worker/internal/codexruntime"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/runtimeauth"
	"github.com/kunish/wheel/apps/worker/internal/types"
	"github.com/uptrace/bun"
)

// ── Local auth file operations ──────────────────────────────────────

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
	return out, nil
}

func runtimeAuthFileProvider(storedProvider string, raw map[string]any) string {
	if len(raw) > 0 {
		if rawProvider := canonicalRuntimeProvider(stringFromMap(raw, "provider", "type")); rawProvider != "" {
			return rawProvider
		}
	}
	provider := canonicalRuntimeProvider(storedProvider)
	if provider != "" {
		return provider
	}
	return ""
}

func runtimePersistedAuthProvider(provider string) string {
	switch canonicalRuntimeProvider(provider) {
	case "copilot":
		return "github-copilot"
	case "codex-cli":
		return "openai-codex-cli"
	case "antigravity":
		return "antigravity"
	case "codex":
		return "codex"
	default:
		return ""
	}
}

func runtimeImportProviderMatchesScope(importScope string, provider string) bool {
	provider = canonicalRuntimeProvider(provider)
	importScope = canonicalRuntimeProvider(importScope)
	if provider == importScope {
		return true
	}
	return importScope == "codex-cli" && provider == "codex"
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

// ── Managed (DB-backed) auth file operations ─────────────────────

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
			raw = map[string]any{}
		}
		provider := runtimeAuthFileProvider(item.Provider, raw)
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
	provider = runtimeAuthFileProvider("", raw)
	if provider == "" {
		return "", "", false, "", nil, fmt.Errorf("missing provider metadata")
	}
	if !isSupportedRuntimeProvider(provider) {
		return "", "", false, "", nil, fmt.Errorf("unsupported provider metadata")
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

func (h *Handler) importOAuthAuthFilesToDB(ctx context.Context, channelID int, snapshot map[string]struct{}, importProvider string) error {
	authDir, err := h.resolveCodexLocalAuthDir()
	if err != nil {
		return err
	}
	importScope := canonicalRuntimeProvider(importProvider)
	files, err := h.listLocalAuthFiles(authDir)
	if err != nil {
		return err
	}
	sawWrongProvider := false
	sawAmbiguousProvider := false
	importedAny := false
	for _, file := range files {
		if _, exists := snapshot[file.Name]; exists {
			continue
		}
		if strings.HasPrefix(file.Name, "channel-") {
			continue
		}
		if canonicalRuntimeProvider(file.Provider) == "" {
			sawAmbiguousProvider = true
			continue
		}
		if importScope != "" && !runtimeImportProviderMatchesScope(importScope, file.Provider) {
			sawWrongProvider = true
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
		if importScope == "codex-cli" && provider == "codex" {
			var raw map[string]any
			if err := json.Unmarshal([]byte(normalized), &raw); err == nil {
				raw["type"] = runtimePersistedAuthProvider(importScope)
				if encoded, err := json.Marshal(raw); err == nil {
					normalized = string(encoded)
				}
			}
			provider = importScope
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
		importedAny = true
	}
	if !importedAny && (sawWrongProvider || sawAmbiguousProvider) {
		return fmt.Errorf("no importable auth file in import scope")
	}
	h.bestEffortSyncCodexChannelModels(ctx, channelID)
	h.bestEffortSyncCodexChannelKeys(ctx, channelID)
	return nil
}

// ── Per-item operation helpers for batch endpoints ───────────────

func (h *Handler) uploadCodexLocalAuthFile(ctx context.Context, channelID int, channelType types.OutboundType, file codexUploadFile) codexAuthUploadResult {
	result := codexAuthUploadResult{Name: file.Name}
	content := file.Content
	provider, email, disabled, normalized, _, err := parseCodexAuthContent(content)
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result
	}
	if !runtimeProviderMatches(channelType, provider) {
		result.Status = "error"
		result.Error = "provider does not belong to this runtime channel"
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

func (h *Handler) uploadCodexManagedAuthFile(ctx context.Context, channelType types.OutboundType, file codexUploadFile) codexAuthUploadResult {
	result := codexAuthUploadResult{Name: file.Name}
	content := file.Content
	provider, _, _, _, _, err := parseCodexAuthContent(content)
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result
	}
	if !runtimeProviderMatches(channelType, provider) {
		result.Status = "error"
		result.Error = "provider does not belong to this runtime channel"
		return result
	}
	var out map[string]any
	if err := h.codexManagementUploadFile(ctx, file.Name, content, &out); err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result
	}
	result.Status = "ok"
	return result
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
	if err := h.codexManagementCallContext(ctx, "GET", "/auth-files", nil, nil, &resp); err != nil {
		return nil, err
	}
	return parseAuthFiles(resp.Files), nil
}

func (h *Handler) patchCodexLocalAuthFileStatus(ctx context.Context, file codexAuthFile, disabled bool) codexAuthUploadResult {
	result := codexAuthUploadResult{Name: file.Name}
	_, _, _, _, raw, err := parseCodexAuthContent([]byte(file.RawContent))
	if err != nil {
		if strings.Contains(err.Error(), "missing provider metadata") && file.Provider != "" {
			raw = map[string]any{}
			if decodeErr := json.Unmarshal([]byte(file.RawContent), &raw); decodeErr != nil {
				result.Status = "error"
				result.Error = decodeErr.Error()
				return result
			}
			persisted := runtimePersistedAuthProvider(file.Provider)
			if persisted == "" {
				result.Status = "error"
				result.Error = err.Error()
				return result
			}
			raw["type"] = persisted
		} else {
			result.Status = "error"
			result.Error = err.Error()
			return result
		}
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
	if err := h.codexManagementCall(c, "PATCH", "/auth-files/status", nil, map[string]any{"name": name, "disabled": disabled}, &out); err != nil {
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
	if err := codexruntime.RemoveOneAuthFile(&types.CodexAuthFile{ID: file.ID, ChannelID: file.ChannelID, Name: file.Name, Provider: file.Provider, Email: file.Email, Disabled: file.Disabled, Content: file.RawContent}); err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result
	}
	result.Status = "ok"
	return result
}

func (h *Handler) deleteCodexManagedAuthFile(c *gin.Context, name string) codexAuthUploadResult {
	result := codexAuthUploadResult{Name: name}
	var out map[string]any
	query := url.Values{"name": []string{name}}
	if err := h.codexManagementCall(c, "DELETE", "/auth-files", query, nil, &out); err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result
	}
	result.Status = "ok"
	return result
}

// batchUploadCodexLocalAuthFiles parses all files, then batch-upserts
// valid items into the database in a single operation instead of
// issuing one INSERT per file.
func (h *Handler) batchUploadCodexLocalAuthFiles(ctx context.Context, channelID int, channelType types.OutboundType, files []codexUploadFile) codexAuthUploadResponse {
	response := codexAuthUploadResponse{
		Total:   len(files),
		Results: make([]codexAuthUploadResult, 0, len(files)),
	}

	// Phase 1: parse all files (CPU-only, fast)
	validItems := make([]types.CodexAuthFile, 0, len(files))
	validIndices := make([]int, 0, len(files))
	for _, file := range files {
		result := codexAuthUploadResult{Name: file.Name}
		provider, email, disabled, normalized, _, err := parseCodexAuthContent(file.Content)
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
			response.Results = append(response.Results, result)
			response.FailedCount++
			continue
		}
		if !runtimeProviderMatches(channelType, provider) {
			result.Status = "error"
			result.Error = "provider does not belong to this runtime channel"
			response.Results = append(response.Results, result)
			response.FailedCount++
			continue
		}
		validIndices = append(validIndices, len(response.Results))
		response.Results = append(response.Results, codexAuthUploadResult{Name: file.Name})
		validItems = append(validItems, types.CodexAuthFile{
			ChannelID: channelID,
			Name:      file.Name,
			Provider:  provider,
			Email:     email,
			Disabled:  disabled,
			Content:   normalized,
		})
	}

	// Phase 2: batch upsert (single SQL per 500-item chunk)
	if len(validItems) > 0 {
		if err := dal.UpsertCodexAuthFiles(ctx, h.DB, validItems); err != nil {
			// Batch failed — mark all valid items as error
			for _, idx := range validIndices {
				response.Results[idx].Status = "error"
				response.Results[idx].Error = err.Error()
			}
			response.FailedCount += len(validItems)
			return response
		}
		for _, idx := range validIndices {
			response.Results[idx].Status = "ok"
		}
		response.SuccessCount = len(validItems)
	}
	return response
}

// concurrentUploadCodexManagedAuthFiles uploads files to the codex
// runtime management API using a bounded worker pool for concurrency.
func (h *Handler) concurrentUploadCodexManagedAuthFiles(ctx context.Context, channelType types.OutboundType, files []codexUploadFile) codexAuthUploadResponse {
	const maxWorkers = 10
	response := codexAuthUploadResponse{
		Total:   len(files),
		Results: make([]codexAuthUploadResult, len(files)),
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxWorkers)
	for i, file := range files {
		wg.Add(1)
		sem <- struct{}{} // acquire
		go func(idx int, f codexUploadFile) {
			defer wg.Done()
			defer func() { <-sem }() // release
			response.Results[idx] = h.uploadCodexManagedAuthFile(ctx, channelType, f)
		}(i, file)
	}
	wg.Wait()

	for _, r := range response.Results {
		if r.Status == "ok" {
			response.SuccessCount++
		} else {
			response.FailedCount++
		}
	}
	return response
}
