package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	codexruntime "github.com/kunish/wheel/apps/worker/internal/codexruntime"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
)

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
	var resp struct {
		Models []map[string]any `json:"models"`
	}
	if err := h.codexManagementCall(c, http.MethodGet, "/models", nil, nil, &resp); err != nil {
		errorJSON(c, http.StatusBadGateway, err.Error())
		return
	}
	successJSON(c, gin.H{"models": resp.Models})
}
