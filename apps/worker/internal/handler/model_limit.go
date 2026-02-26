package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// ──── Model Limit Routes ────

// ListModelLimits godoc
// @Summary List all model limits
// @Tags Model Limits
// @Produce json
// @Success 200 {object} object "{success: true, data: {limits: []ModelLimit}}"
// @Security BearerAuth
// @Router /api/v1/model-limit/list [get]
func (h *Handler) ListModelLimits(c *gin.Context) {
	limits, err := dal.ListModelLimits(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, gin.H{"limits": limits})
}

// CreateModelLimit godoc
// @Summary Create a new model limit
// @Tags Model Limits
// @Accept json
// @Produce json
// @Param body body types.ModelLimitCreateRequest true "Model limit config"
// @Success 200 {object} object "{success: true, data: ModelLimit}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/model-limit/create [post]
func (h *Handler) CreateModelLimit(c *gin.Context) {
	var body types.ModelLimitCreateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	limit := &types.ModelLimit{
		Model:         body.Model,
		RPM:           body.RPM,
		TPM:           body.TPM,
		DailyRequests: body.DailyRequests,
		DailyTokens:   body.DailyTokens,
		Enabled:       body.Enabled,
	}

	if err := dal.CreateModelLimit(c.Request.Context(), h.DB, limit); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Record audit log
	dal.CreateAuditLog(c.Request.Context(), h.DB, "admin", "create", "model_limit", "Created model limit for "+body.Model)

	successJSON(c, limit)
}

// UpdateModelLimit godoc
// @Summary Update a model limit
// @Tags Model Limits
// @Accept json
// @Produce json
// @Param body body types.ModelLimitUpdateRequest true "Partial model limit fields to update (id required)"
// @Success 200 {object} object "{success: true}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/model-limit/update [post]
func (h *Handler) UpdateModelLimit(c *gin.Context) {
	var body types.ModelLimitUpdateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	if body.ID == 0 {
		errorJSON(c, http.StatusBadRequest, "id is required")
		return
	}

	data := make(map[string]any)
	if body.Model != nil {
		data["model"] = *body.Model
	}
	if body.RPM != nil {
		data["rpm"] = *body.RPM
	}
	if body.TPM != nil {
		data["tpm"] = *body.TPM
	}
	if body.DailyRequests != nil {
		data["daily_requests"] = *body.DailyRequests
	}
	if body.DailyTokens != nil {
		data["daily_tokens"] = *body.DailyTokens
	}
	if body.Enabled != nil {
		data["enabled"] = *body.Enabled
	}

	if err := dal.UpdateModelLimit(c.Request.Context(), h.DB, body.ID, data); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Record audit log
	dal.CreateAuditLog(c.Request.Context(), h.DB, "admin", "update", "model_limit", "Updated model limit ID "+strconv.Itoa(body.ID))

	successNoData(c)
}

// DeleteModelLimit godoc
// @Summary Delete a model limit
// @Tags Model Limits
// @Produce json
// @Param id path int true "Model Limit ID"
// @Success 200 {object} object "{success: true}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/model-limit/delete/{id} [delete]
func (h *Handler) DeleteModelLimit(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid model limit ID")
		return
	}

	if err := dal.DeleteModelLimit(c.Request.Context(), h.DB, id); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Record audit log
	dal.CreateAuditLog(c.Request.Context(), h.DB, "admin", "delete", "model_limit", "Deleted model limit ID "+strconv.Itoa(id))

	successNoData(c)
}
