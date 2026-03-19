package handler

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	codexruntime "github.com/kunish/wheel/apps/worker/internal/codexruntime"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

func (h *Handler) ListCodexAuthFiles(c *gin.Context) {
	channel, err := h.validateCodexChannel(c)
	if err != nil {
		return
	}

	provider := strings.TrimSpace(c.Query("provider"))
	search := strings.TrimSpace(c.Query("search"))
	disabled := strings.TrimSpace(c.Query("disabled"))
	status := strings.TrimSpace(c.Query("status")) // "error" or "exhausted"
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
	files = filterRuntimeOwnedCodexAuthFiles(files, channel.Type)

	// When status filter is active, use cached quota data for instant filtering.
	// No scanning of uncached files — cache is populated during normal page browsing.
	if status == "error" || status == "exhausted" {
		filtered := filterCodexAuthFiles(files, provider, search, disabled)

		var matchingFiles []codexAuthFile
		var matchingQuota []codexQuotaItem
		cachedCount := 0

		for _, file := range filtered {
			item, ok := h.loadQuotaCache(channel.ID, file.Name)
			if !ok {
				continue
			}
			cachedCount++
			match := false
			switch status {
			case "error":
				match = item.Error != ""
			case "exhausted":
				if item.Weekly.LimitReached || item.CodeReview.LimitReached {
					match = true
				}
				for _, s := range item.Snapshots {
					if !s.Unlimited && s.PercentRemaining <= 0 {
						match = true
						break
					}
				}
			}
			if match {
				matchingFiles = append(matchingFiles, file)
				matchingQuota = append(matchingQuota, item)
			}
		}

		total := len(matchingFiles)
		start := (page - 1) * pageSize
		if start >= total {
			successJSON(c, gin.H{"files": []codexAuthFile{}, "total": total, "page": page, "pageSize": pageSize, "capabilities": capabilities, "quotaItems": []codexQuotaItem{}, "cachedCount": cachedCount, "totalUnfiltered": len(filtered)})
			return
		}
		end := start + pageSize
		if end > total {
			end = total
		}
		successJSON(c, gin.H{"files": matchingFiles[start:end], "total": total, "page": page, "pageSize": pageSize, "capabilities": capabilities, "quotaItems": matchingQuota[start:end], "cachedCount": cachedCount, "totalUnfiltered": len(filtered)})
		return
	}

	items, total := filterAndPaginateAuthFiles(files, provider, search, disabled, page, pageSize)
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
		files, err := h.listManagedCodexAuthFiles(c.Request.Context(), channel.ID)
		if err != nil {
			errorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}
		owned := filterRuntimeOwnedCodexAuthFiles(files, channel.Type)
		var target *codexAuthFile
		for i := range owned {
			if owned[i].Name == req.Name {
				target = &owned[i]
				break
			}
		}
		if target == nil {
			errorJSON(c, http.StatusNotFound, "auth file not found")
			return
		}
		result := h.patchCodexLocalAuthFileStatus(c.Request.Context(), *target, req.Disabled)
		if result.Status != "ok" {
			errorJSON(c, http.StatusInternalServerError, result.Error)
			return
		}
		h.bestEffortSyncCodexChannelModels(c.Request.Context(), channel.ID)
		out = map[string]any{"status": "ok", "disabled": req.Disabled}
	} else {
		if err := h.ensureCodexManagementConfigured(); err != nil {
			errorJSON(c, http.StatusBadRequest, err.Error())
			return
		}
		files, err := h.listAllCodexAuthFiles(c.Request.Context(), channel.ID)
		if err != nil {
			errorJSON(c, http.StatusBadGateway, err.Error())
			return
		}
		owned := filterRuntimeOwnedCodexAuthFiles(files, channel.Type)
		found := false
		for i := range owned {
			if owned[i].Name == req.Name {
				found = true
				break
			}
		}
		if !found {
			errorJSON(c, http.StatusNotFound, "auth file not found")
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
	selected, err := selectCodexAuthFilesForBatch(filterRuntimeOwnedCodexAuthFiles(files, channel.Type), req.codexAuthBatchScope)
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
		response = h.batchUploadCodexLocalAuthFiles(c.Request.Context(), channel.ID, channel.Type, files)
		if response.SuccessCount > 0 {
			if err := codexruntime.MaterializeChannelAuthFiles(c.Request.Context(), h.DB, channel.ID); err != nil {
				log.Printf("[codex] materialize channel %d auth files failed: %v", channel.ID, err)
			} else {
				// Run model sync in a background goroutine to avoid blocking the response.
				go h.bestEffortSyncCodexChannelModels(context.Background(), channel.ID)
			}
		}
	} else {
		if err := h.ensureCodexManagementConfigured(); err != nil {
			errorJSON(c, http.StatusBadRequest, err.Error())
			return
		}
		response = h.concurrentUploadCodexManagedAuthFiles(c.Request.Context(), channel.Type, files)
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
			items, err := h.listManagedCodexAuthFiles(c.Request.Context(), channel.ID)
			if err != nil {
				errorJSON(c, http.StatusInternalServerError, err.Error())
				return
			}
			items = filterRuntimeOwnedCodexAuthFiles(items, channel.Type)
			deleted := 0
			failed := 0
			for i := range items {
				if result := h.deleteCodexLocalAuthFile(c.Request.Context(), channel.ID, items[i]); result.Status == "ok" {
					deleted++
				} else {
					failed++
				}
			}
			h.bestEffortSyncCodexChannelModels(c.Request.Context(), channel.ID)
			out = map[string]any{"status": "ok", "deleted": deleted, "failed": failed}
		} else {
			items, err := h.listManagedCodexAuthFiles(c.Request.Context(), channel.ID)
			if err != nil {
				errorJSON(c, http.StatusInternalServerError, err.Error())
				return
			}
			owned := filterRuntimeOwnedCodexAuthFiles(items, channel.Type)
			var item *codexAuthFile
			for i := range owned {
				if owned[i].Name == name {
					item = &owned[i]
					break
				}
			}
			if item == nil {
				errorJSON(c, http.StatusNotFound, "auth file not found")
				return
			}
			result := h.deleteCodexLocalAuthFile(c.Request.Context(), channel.ID, *item)
			if result.Status != "ok" {
				errorJSON(c, http.StatusInternalServerError, result.Error)
				return
			}
			h.bestEffortSyncCodexChannelModels(c.Request.Context(), channel.ID)
			out = map[string]any{"status": "ok", "deleted": 1}
		}
	} else {
		if err := h.ensureCodexManagementConfigured(); err != nil {
			errorJSON(c, http.StatusBadRequest, err.Error())
			return
		}
		if all == "true" || all == "1" || all == "*" {
			files, err := h.listAllCodexAuthFiles(c.Request.Context(), channel.ID)
			if err != nil {
				errorJSON(c, http.StatusBadGateway, err.Error())
				return
			}
			owned := filterRuntimeOwnedCodexAuthFiles(files, channel.Type)
			deleted := 0
			for _, file := range owned {
				result := h.deleteCodexManagedAuthFile(c, file.Name)
				if result.Status == "ok" {
					deleted++
				}
			}
			out = map[string]any{"status": "ok", "deleted": deleted}
			successJSON(c, out)
			return
		}
		if name != "" {
			files, err := h.listAllCodexAuthFiles(c.Request.Context(), channel.ID)
			if err != nil {
				errorJSON(c, http.StatusBadGateway, err.Error())
				return
			}
			owned := filterRuntimeOwnedCodexAuthFiles(files, channel.Type)
			found := false
			for i := range owned {
				if owned[i].Name == name {
					found = true
					break
				}
			}
			if !found {
				errorJSON(c, http.StatusNotFound, "auth file not found")
				return
			}
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
	selected, err := selectCodexAuthFilesForBatch(filterRuntimeOwnedCodexAuthFiles(files, channel.Type), req)
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

	if err := h.ensureCodexManagementConfigured(); err != nil {
		errorJSON(c, http.StatusBadRequest, err.Error())
		return
	}
	query := url.Values{}
	if name := c.Query("name"); name != "" {
		files, err := h.listAllCodexAuthFiles(c.Request.Context(), channel.ID)
		if err != nil {
			errorJSON(c, http.StatusBadGateway, err.Error())
			return
		}
		owned := filterRuntimeOwnedCodexAuthFiles(files, channel.Type)
		found := false
		for i := range owned {
			if owned[i].Name == name {
				found = true
				break
			}
		}
		if !found {
			successJSON(c, gin.H{"models": []any{}})
			return
		}
		query.Set("name", managedAuthRelativeName(channel.ID, name))
	} else {
		files, err := h.listAllCodexAuthFiles(c.Request.Context(), channel.ID)
		if err != nil {
			errorJSON(c, http.StatusBadGateway, err.Error())
			return
		}
		owned := filterRuntimeOwnedCodexAuthFiles(files, channel.Type)
		models, err := h.collectCodexChannelModelsBestEffort(c.Request.Context(), channel.ID, channel.Type, owned)
		if err != nil {
			errorJSON(c, http.StatusBadGateway, err.Error())
			return
		}
		if len(models) == 0 && channel.Type == types.OutboundAntigravity {
			hasOwnedAuth := false
			for _, file := range owned {
				if !file.Disabled && runtimeProviderMatches(channel.Type, file.Provider) {
					hasOwnedAuth = true
					break
				}
			}
			if hasOwnedAuth {
				models = defaultAntigravityModels()
			}
		}
		out := make([]map[string]any, 0, len(models))
		for _, model := range models {
			out = append(out, map[string]any{"id": model})
		}
		successJSON(c, gin.H{"models": out})
		return
	}
	var resp struct {
		Models []map[string]any `json:"models"`
	}
	if err := h.codexManagementCall(c, http.MethodGet, "/auth-files/models", query, nil, &resp); err != nil {
		errorJSON(c, http.StatusBadGateway, err.Error())
		return
	}
	successJSON(c, gin.H{"models": resp.Models})
}
