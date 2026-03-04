package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
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

	job := h.AsyncStore.CreateJob(model, body)

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

	// Build a placeholder response based on the request
	response := map[string]any{
		"id":      jobID,
		"object":  "chat.completion",
		"model":   job.Model,
		"choices": []map[string]any{},
	}

	usage := map[string]any{
		"prompt_tokens":     0,
		"completion_tokens": 0,
		"total_tokens":      0,
	}

	h.AsyncStore.CompleteJob(jobID, response, usage)
	slog.Info("async job completed", "job_id", jobID, "model", job.Model)
}
