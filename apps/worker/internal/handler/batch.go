package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/middleware"
	"github.com/kunish/wheel/apps/worker/internal/relay"
)

// batchCreateRequest is the request body for POST /v1/batch.
type batchCreateRequest struct {
	Requests []relay.BatchRequest `json:"requests"`
	Model    string               `json:"model"`
}

// HandleCreateBatch handles POST /v1/batch — creates a batch job and starts processing in background.
func (h *RelayHandler) HandleCreateBatch(c *gin.Context) {
	if h.BatchStore == nil {
		apiError(c, http.StatusServiceUnavailable, "service_error", "Batch API not available", false)
		return
	}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		apiError(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body", false)
		return
	}

	var req batchCreateRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		apiError(c, http.StatusBadRequest, "invalid_request_error", "Invalid JSON body", false)
		return
	}

	if len(req.Requests) == 0 {
		apiError(c, http.StatusBadRequest, "invalid_request_error", "At least one request is required", false)
		return
	}

	supportedModels, _ := c.Get("supportedModels")
	sm, _ := supportedModels.(string)

	for i := range req.Requests {
		if req.Requests[i].Body == nil {
			apiError(c, http.StatusBadRequest, "invalid_request_error", "Request body is required", false)
			return
		}
		if req.Requests[i].Method != "" && strings.ToUpper(req.Requests[i].Method) != http.MethodPost {
			apiError(c, http.StatusBadRequest, "invalid_request_error", "Only POST requests are supported in batch", false)
			return
		}

		if req.Requests[i].URL == "" {
			req.Requests[i].URL = "/v1/chat/completions"
		}
		if !strings.HasPrefix(req.Requests[i].URL, "/") {
			req.Requests[i].URL = "/" + req.Requests[i].URL
		}
		requestType := relay.DetectRequestType(req.Requests[i].URL)
		if requestType == "" {
			apiError(c, http.StatusBadRequest, "invalid_request_error", fmt.Sprintf("Unsupported batch endpoint %s", req.Requests[i].URL), false)
			return
		}
		if relay.IsDeferredExecutionUnsupported(requestType) {
			apiError(c, http.StatusBadRequest, "invalid_request_error",
				"Batch API does not support audio endpoint "+req.Requests[i].URL,
				false,
			)
			return
		}

		model, _ := req.Requests[i].Body["model"].(string)
		if model == "" {
			model = req.Model
		}
		if model == "" {
			apiError(c, http.StatusBadRequest, "invalid_request_error", "Model is required for each batch request", false)
			return
		}
		if !middleware.CheckModelAccess(sm, model) {
			apiError(c, http.StatusForbidden, "invalid_request_error", "Model not allowed for this API key", false)
			return
		}
		req.Requests[i].Body["model"] = model
	}

	apiKeyIDRaw, _ := c.Get("apiKeyId")
	apiKeyID, _ := apiKeyIDRaw.(int)

	job := h.BatchStore.CreateJob(req.Requests, req.Model, apiKeyID)

	// Process batch in background
	go h.processBatchJob(job.ID)

	c.JSON(http.StatusOK, job)
}

// HandleGetBatch handles GET /v1/batch/:id — returns batch job status.
func (h *RelayHandler) HandleGetBatch(c *gin.Context) {
	if h.BatchStore == nil {
		apiError(c, http.StatusServiceUnavailable, "service_error", "Batch API not available", false)
		return
	}

	id := c.Param("id")
	job := h.BatchStore.GetJob(id)
	if job == nil {
		apiError(c, http.StatusNotFound, "not_found_error", "Batch job not found", false)
		return
	}

	c.JSON(http.StatusOK, job)
}

// HandleListBatches handles GET /v1/batch — lists all batch jobs.
func (h *RelayHandler) HandleListBatches(c *gin.Context) {
	if h.BatchStore == nil {
		apiError(c, http.StatusServiceUnavailable, "service_error", "Batch API not available", false)
		return
	}

	jobs := h.BatchStore.ListJobs()
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": jobs})
}

// HandleCancelBatch handles POST /v1/batch/:id/cancel — cancels a batch job.
func (h *RelayHandler) HandleCancelBatch(c *gin.Context) {
	if h.BatchStore == nil {
		apiError(c, http.StatusServiceUnavailable, "service_error", "Batch API not available", false)
		return
	}

	id := c.Param("id")
	if ok := h.BatchStore.CancelJob(id); !ok {
		apiError(c, http.StatusBadRequest, "invalid_request_error", "Cannot cancel batch job (not found or not in cancellable state)", false)
		return
	}

	job := h.BatchStore.GetJob(id)
	c.JSON(http.StatusOK, job)
}

// processBatchJob processes all requests in a batch job sequentially.
func (h *RelayHandler) processBatchJob(jobID string) {
	job := h.BatchStore.GetJob(jobID)
	if job == nil {
		return
	}

	h.BatchStore.UpdateJob(jobID, relay.BatchStatusProcessing, nil)

	responses := make([]relay.BatchResponse, 0, len(job.Requests))

	for _, req := range job.Requests {
		// Check if job has been cancelled
		currentJob := h.BatchStore.GetJob(jobID)
		if currentJob == nil || currentJob.Status == relay.BatchStatusCancelled {
			return
		}

		resp := relay.BatchResponse{
			CustomID: req.CustomID,
		}

		// Validate the request
		if req.Body == nil {
			resp.Error = &relay.BatchError{
				Code:    "invalid_request",
				Message: "Request body is required",
			}
			responses = append(responses, resp)
			h.BatchStore.UpdateJob(jobID, relay.BatchStatusProcessing, responses)
			continue
		}

		model, _ := req.Body["model"].(string)
		if model == "" {
			if job.Model != "" {
				model = job.Model
				req.Body["model"] = model
			} else {
				resp.Error = &relay.BatchError{Code: "invalid_request", Message: "Model is required"}
				responses = append(responses, resp)
				h.BatchStore.UpdateJob(jobID, relay.BatchStatusProcessing, responses)
				continue
			}
		}

		path := req.URL
		if path == "" {
			path = "/v1/chat/completions"
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}

		result, err := h.executeBackgroundNonStream(path, req.Body, model, job.ApiKeyID)
		if err != nil {
			resp.Error = &relay.BatchError{Code: "upstream_error", Message: err.Error()}
		} else {
			resp.Response = &relay.BatchResult{StatusCode: result.StatusCode, Body: result.Response}
		}

		responses = append(responses, resp)
		h.BatchStore.UpdateJob(jobID, relay.BatchStatusProcessing, responses)
	}

	// Mark job as completed
	finalStatus := relay.BatchStatusCompleted
	for _, r := range responses {
		if r.Error != nil {
			finalStatus = relay.BatchStatusCompleted // still completed, with partial failures
			break
		}
	}

	h.BatchStore.UpdateJob(jobID, finalStatus, responses)
	slog.Info("batch job completed", "job_id", jobID, "total", len(responses))
}
