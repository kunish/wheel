package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// ──── Guardrail Rule Routes ────

// ListGuardrailRules godoc
// @Summary List all guardrail rules
// @Tags GuardrailRules
// @Produce json
// @Success 200 {object} object "{success: true, data: {rules: []GuardrailRule}}"
// @Security BearerAuth
// @Router /api/v1/guardrail/list [get]
func (h *Handler) ListGuardrailRules(c *gin.Context) {
	rules, err := dal.ListGuardrailRules(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, gin.H{"rules": rules})
}

// CreateGuardrailRule godoc
// @Summary Create a new guardrail rule
// @Tags GuardrailRules
// @Accept json
// @Produce json
// @Param body body types.GuardrailRuleCreateRequest true "Guardrail rule"
// @Success 200 {object} object "{success: true, data: GuardrailRule}"
// @Security BearerAuth
// @Router /api/v1/guardrail/create [post]
func (h *Handler) CreateGuardrailRule(c *gin.Context) {
	var req types.GuardrailRuleCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate type
	switch req.Type {
	case "keyword", "regex", "length", "pii":
		// valid
	default:
		errorJSON(c, http.StatusBadRequest, "Invalid type: must be keyword, regex, length, or pii")
		return
	}

	// Validate target
	switch req.Target {
	case "input", "output", "both":
		// valid
	default:
		errorJSON(c, http.StatusBadRequest, "Invalid target: must be input, output, or both")
		return
	}

	// Validate action
	switch req.Action {
	case "block", "warn", "redact":
		// valid
	default:
		errorJSON(c, http.StatusBadRequest, "Invalid action: must be block, warn, or redact")
		return
	}

	rule := &types.GuardrailRule{
		Name:      req.Name,
		Type:      req.Type,
		Target:    req.Target,
		Action:    req.Action,
		Pattern:   req.Pattern,
		MaxLength: req.MaxLength,
		Enabled:   req.Enabled,
	}

	if err := dal.CreateGuardrailRule(c.Request.Context(), h.DB, rule); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	successJSON(c, rule)
}

// UpdateGuardrailRule godoc
// @Summary Update a guardrail rule
// @Tags GuardrailRules
// @Accept json
// @Produce json
// @Param body body types.GuardrailRuleUpdateRequest true "Partial guardrail rule fields"
// @Success 200 {object} object "{success: true}"
// @Security BearerAuth
// @Router /api/v1/guardrail/update [post]
func (h *Handler) UpdateGuardrailRule(c *gin.Context) {
	var req types.GuardrailRuleUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.ID == 0 {
		errorJSON(c, http.StatusBadRequest, "id is required")
		return
	}

	data := make(map[string]any)
	if req.Name != nil {
		data["name"] = *req.Name
	}
	if req.Type != nil {
		data["type"] = *req.Type
	}
	if req.Target != nil {
		data["target"] = *req.Target
	}
	if req.Action != nil {
		data["action"] = *req.Action
	}
	if req.Pattern != nil {
		data["pattern"] = *req.Pattern
	}
	if req.MaxLength != nil {
		data["max_length"] = *req.MaxLength
	}
	if req.Enabled != nil {
		data["enabled"] = *req.Enabled
	}

	if err := dal.UpdateGuardrailRule(c.Request.Context(), h.DB, req.ID, data); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	successNoData(c)
}

// DeleteGuardrailRule godoc
// @Summary Delete a guardrail rule
// @Tags GuardrailRules
// @Produce json
// @Param id path int true "Rule ID"
// @Success 200 {object} object "{success: true}"
// @Security BearerAuth
// @Router /api/v1/guardrail/delete/{id} [delete]
func (h *Handler) DeleteGuardrailRule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid rule ID")
		return
	}

	if err := dal.DeleteGuardrailRule(c.Request.Context(), h.DB, id); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	successNoData(c)
}
