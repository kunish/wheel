package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/middleware"
)

// asyncCreateRequest is the request body for POST /v1/async/chat/completions.
type asyncCreateRequest struct {
	Model    string         `json:"model"`
	Messages []any          `json:"messages"`
	Extra    map[string]any `json:"-"`
}

// HandleCreateAsyncInference handles POST /v1/async/chat/completions — creates an async job.
func (h *RelayHandler) HandleCreateAsyncInference(c *gin.Context) {
	if h.AsyncStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"message": "Async API not available", "type": "service_error"}})
		return
	}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Failed to read request body", "type": "invalid_request_error"}})
		return
	}

	var body map[string]any
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Invalid JSON body", "type": "invalid_request_error"}})
		return
	}

	model, _ := body["model"].(string)
	if model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Model is required", "type": "invalid_request_error"}})
		return
	}

	supportedModels, _ := c.Get("supportedModels")
	sm, _ := supportedModels.(string)
	if !middleware.CheckModelAccess(sm, model) {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"message": "Model not allowed for this API key", "type": "invalid_request_error"}})
		return
	}

	apiKeyIDRaw, _ := c.Get("apiKeyId")
	apiKeyID, _ := apiKeyIDRaw.(int)

	job := h.AsyncStore.CreateJob(model, apiKeyID, body)

	// Process async job in background
	go h.processAsyncJob(job.ID)

	c.JSON(http.StatusAccepted, job)
}

// HandleGetAsyncJob handles GET /v1/async/:id — returns async job status/result.
func (h *RelayHandler) HandleGetAsyncJob(c *gin.Context) {
	if h.AsyncStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"message": "Async API not available", "type": "service_error"}})
		return
	}

	id := c.Param("id")
	job := h.AsyncStore.GetJob(id)
	if job == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": "Async job not found", "type": "not_found_error"}})
		return
	}

	c.JSON(http.StatusOK, job)
}

// HandleListAsyncJobs handles GET /v1/async — lists async jobs with pagination.
func (h *RelayHandler) HandleListAsyncJobs(c *gin.Context) {
	if h.AsyncStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"message": "Async API not available", "type": "service_error"}})
		return
	}

	limit := 20
	offset := 0
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	jobs := h.AsyncStore.ListJobs(limit, offset)
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": jobs})
}

// processAsyncJob processes an async inference job in the background.
func (h *RelayHandler) processAsyncJob(jobID string) {
	job := h.AsyncStore.GetJob(jobID)
	if job == nil {
		return
	}

	// Mark as processing
	h.AsyncStore.MarkProcessing(jobID)

	body := job.Request
	if body == nil {
		h.AsyncStore.FailJob(jobID, "invalid async request body")
		return
	}
	if stream, ok := body["stream"].(bool); ok && stream {
		delete(body, "stream")
	}

	result, err := h.executeBackgroundNonStream("/v1/chat/completions", body, job.Model, job.ApiKeyID)
	if err != nil {
		h.AsyncStore.FailJob(jobID, err.Error())
		slog.Error("async job failed", "job_id", jobID, "model", job.Model, "error", err)
		return
	}

	usage := map[string]any{
		"prompt_tokens":     result.InputTokens,
		"completion_tokens": result.OutputTokens,
		"total_tokens":      result.InputTokens + result.OutputTokens,
	}

	h.AsyncStore.CompleteJob(jobID, result.Response, usage)
	slog.Info("async job completed", "job_id", jobID, "model", job.Model)
}
