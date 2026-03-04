package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// ──── Virtual Key Routes (Admin, JWT-protected) ────

// ListVirtualKeys godoc
// @Summary List all virtual keys
// @Tags Virtual Keys
// @Produce json
// @Success 200 {object} object "{success: true, data: {virtualKeys: []VirtualKey}}"
// @Security BearerAuth
// @Router /api/v1/virtual-key/list [get]
func (h *Handler) ListVirtualKeys(c *gin.Context) {
	keys, err := dal.ListVirtualKeys(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, gin.H{"virtualKeys": keys})
}

// CreateVirtualKey godoc
// @Summary Create a new virtual key
// @Tags Virtual Keys
// @Accept json
// @Produce json
// @Param body body types.VirtualKeyCreateRequest true "Virtual key config"
// @Success 200 {object} object "{success: true, data: VirtualKey}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/virtual-key/create [post]
func (h *Handler) CreateVirtualKey(c *gin.Context) {
	var body types.VirtualKeyCreateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	vk, err := dal.CreateVirtualKey(c.Request.Context(), h.DB, body)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	go dal.CreateAuditLog(c.Request.Context(), h.DB, "admin", "create", "virtual_key", "Created virtual key: "+body.Name)
	successJSON(c, vk)
}

// UpdateVirtualKey godoc
// @Summary Update a virtual key
// @Tags Virtual Keys
// @Accept json
// @Produce json
// @Param body body types.VirtualKeyUpdateRequest true "Partial virtual key fields to update (id required)"
// @Success 200 {object} object "{success: true}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/virtual-key/update [post]
func (h *Handler) UpdateVirtualKey(c *gin.Context) {
	var body types.VirtualKeyUpdateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	data := make(map[string]any)
	if body.Name != nil {
		data["name"] = *body.Name
	}
	if body.Description != nil {
		data["description"] = *body.Description
	}
	if body.Enabled != nil {
		data["enabled"] = *body.Enabled
	}
	if body.RateLimitRPM != nil {
		data["rate_limit_rpm"] = *body.RateLimitRPM
	}
	if body.RateLimitTPM != nil {
		data["rate_limit_tpm"] = *body.RateLimitTPM
	}
	if body.MaxBudget != nil {
		data["max_budget"] = *body.MaxBudget
	}
	if body.AllowedModels != nil {
		modelsJSON, err := json.Marshal(*body.AllowedModels)
		if err != nil {
			errorJSON(c, http.StatusBadRequest, "Invalid allowed models")
			return
		}
		data["allowed_models"] = string(modelsJSON)
	}

	if err := dal.UpdateVirtualKey(c.Request.Context(), h.DB, body.ID, data); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successNoData(c)
}

// DeleteVirtualKey godoc
// @Summary Delete a virtual key
// @Tags Virtual Keys
// @Produce json
// @Param id path int true "Virtual Key ID"
// @Success 200 {object} object "{success: true}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/virtual-key/delete/{id} [delete]
func (h *Handler) DeleteVirtualKey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid virtual key ID")
		return
	}

	if err := dal.DeleteVirtualKey(c.Request.Context(), h.DB, id); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	go dal.CreateAuditLog(c.Request.Context(), h.DB, "admin", "delete", "virtual_key", "Deleted virtual key ID "+strconv.Itoa(id))
	successNoData(c)
}
