package handler

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// reloadRoutingRules refreshes the in-memory routing engine from DB.
func (h *RelayHandler) reloadRoutingRules(c *gin.Context) {
	if h.RoutingEngine == nil {
		return
	}
	rules, err := dal.ListRoutingRules(c.Request.Context(), h.DB)
	if err != nil {
		log.Printf("[routing] failed to reload routing rules: %v", err)
		return
	}
	h.RoutingEngine.LoadFromModels(rules)
}

// ──── Routing Rule Routes ────

// ListRoutingRules godoc
// @Summary List all routing rules
// @Tags RoutingRules
// @Produce json
// @Success 200 {object} object "{success: true, data: {rules: []RoutingRuleModel}}"
// @Security BearerAuth
// @Router /api/v1/routing-rule/list [get]
func (h *RelayHandler) ListRoutingRules(c *gin.Context) {
	rules, err := dal.ListRoutingRules(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, gin.H{"rules": rules})
}

// CreateRoutingRule godoc
// @Summary Create a new routing rule
// @Tags RoutingRules
// @Accept json
// @Produce json
// @Param body body types.RoutingRuleCreateRequest true "Routing rule"
// @Success 200 {object} object "{success: true, data: RoutingRuleModel}"
// @Security BearerAuth
// @Router /api/v1/routing-rule/create [post]
func (h *RelayHandler) CreateRoutingRule(c *gin.Context) {
	var req types.RoutingRuleCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate action type
	switch req.Action.Type {
	case "reject", "route", "rewrite":
		// valid
	default:
		errorJSON(c, http.StatusBadRequest, "Invalid action type: must be reject, route, or rewrite")
		return
	}

	rule := &types.RoutingRuleModel{
		Name:       req.Name,
		Priority:   req.Priority,
		Enabled:    req.Enabled,
		Conditions: types.ConditionList(req.Conditions),
		Action:     types.ActionJSON(req.Action),
	}

	if err := dal.CreateRoutingRule(c.Request.Context(), h.DB, rule); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	h.reloadRoutingRules(c)
	successJSON(c, rule)
}

// UpdateRoutingRule godoc
// @Summary Update a routing rule
// @Tags RoutingRules
// @Accept json
// @Produce json
// @Param body body types.RoutingRuleUpdateRequest true "Partial routing rule fields"
// @Success 200 {object} object "{success: true}"
// @Security BearerAuth
// @Router /api/v1/routing-rule/update [post]
func (h *RelayHandler) UpdateRoutingRule(c *gin.Context) {
	var req types.RoutingRuleUpdateRequest
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
	if req.Priority != nil {
		data["priority"] = *req.Priority
	}
	if req.Enabled != nil {
		data["enabled"] = *req.Enabled
	}
	if req.Conditions != nil {
		data["conditions"] = types.ConditionList(req.Conditions)
	}
	if req.Action != nil {
		data["action"] = types.ActionJSON(*req.Action)
	}

	if err := dal.UpdateRoutingRule(c.Request.Context(), h.DB, req.ID, data); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	h.reloadRoutingRules(c)
	successNoData(c)
}

// DeleteRoutingRule godoc
// @Summary Delete a routing rule
// @Tags RoutingRules
// @Produce json
// @Param id path int true "Rule ID"
// @Success 200 {object} object "{success: true}"
// @Security BearerAuth
// @Router /api/v1/routing-rule/delete/{id} [delete]
func (h *RelayHandler) DeleteRoutingRule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid rule ID")
		return
	}

	if err := dal.DeleteRoutingRule(c.Request.Context(), h.DB, id); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	h.reloadRoutingRules(c)
	successNoData(c)
}

// ──── Channel Health ────

// GetChannelHealth godoc
// @Summary Get health status of all channels
// @Tags Channels
// @Produce json
// @Success 200 {object} object "{success: true, data: {health: map[string]int}}"
// @Security BearerAuth
// @Router /api/v1/channel/health [get]
func (h *RelayHandler) GetChannelHealth(c *gin.Context) {
	if h.HealthChecker == nil {
		successJSON(c, gin.H{"health": map[string]int{}})
		return
	}
	raw := h.HealthChecker.GetAllHealth()
	health := make(map[string]int, len(raw))
	for id, status := range raw {
		health[strconv.Itoa(id)] = int(status)
	}
	successJSON(c, gin.H{"health": health})
}
