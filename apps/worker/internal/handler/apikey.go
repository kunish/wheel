package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
)

// ──── API Key Routes (Admin, JWT-protected) ────

// ListApiKeys godoc
// @Summary List all API keys
// @Tags API Keys
// @Produce json
// @Success 200 {object} object "{success: true, data: {apiKeys: []APIKey}}"
// @Security BearerAuth
// @Router /api/v1/apikey/list [get]
func (h *Handler) ListApiKeys(c *gin.Context) {
	keys, err := dal.ListApiKeys(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, gin.H{"apiKeys": keys})
}

// CreateApiKey godoc
// @Summary Create a new API key
// @Tags API Keys
// @Accept json
// @Produce json
// @Param body body object true "API key config: name, expireAt, maxCost, supportedModels"
// @Success 200 {object} object "{success: true, data: APIKey}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/apikey/create [post]
func (h *Handler) CreateApiKey(c *gin.Context) {
	var body struct {
		Name            string  `json:"name"`
		ExpireAt        int64   `json:"expireAt"`
		MaxCost         float64 `json:"maxCost"`
		SupportedModels string  `json:"supportedModels"`
		RPMLimit        int     `json:"rpmLimit"`
		TPMLimit        int     `json:"tpmLimit"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	key, err := dal.CreateApiKey(c.Request.Context(), h.DB, body.Name, body.ExpireAt, body.MaxCost, body.SupportedModels, body.RPMLimit, body.TPMLimit)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, key)
}

// UpdateApiKey godoc
// @Summary Update an API key
// @Tags API Keys
// @Accept json
// @Produce json
// @Param body body object true "Partial API key fields to update (id required)"
// @Success 200 {object} object "{success: true}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/apikey/update [post]
func (h *Handler) UpdateApiKey(c *gin.Context) {
	var body struct {
		ID              int      `json:"id"`
		Name            *string  `json:"name,omitempty"`
		Enabled         *bool    `json:"enabled,omitempty"`
		ExpireAt        *int64   `json:"expireAt,omitempty"`
		MaxCost         *float64 `json:"maxCost,omitempty"`
		SupportedModels *string  `json:"supportedModels,omitempty"`
		RPMLimit        *int     `json:"rpmLimit,omitempty"`
		TPMLimit        *int     `json:"tpmLimit,omitempty"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	data := make(map[string]any)
	if body.Name != nil {
		data["name"] = *body.Name
	}
	if body.Enabled != nil {
		data["enabled"] = *body.Enabled
	}
	if body.ExpireAt != nil {
		data["expire_at"] = *body.ExpireAt
	}
	if body.MaxCost != nil {
		data["max_cost"] = *body.MaxCost
	}
	if body.SupportedModels != nil {
		data["supported_models"] = *body.SupportedModels
	}
	if body.RPMLimit != nil {
		data["rpm_limit"] = *body.RPMLimit
	}
	if body.TPMLimit != nil {
		data["tpm_limit"] = *body.TPMLimit
	}

	if err := dal.UpdateApiKey(c.Request.Context(), h.DB, body.ID, data); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successNoData(c)
}

// DeleteApiKey godoc
// @Summary Delete an API key
// @Tags API Keys
// @Produce json
// @Param id path int true "API Key ID"
// @Success 200 {object} object "{success: true}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/apikey/delete/{id} [delete]
func (h *Handler) DeleteApiKey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid API key ID")
		return
	}

	if err := dal.DeleteApiKey(c.Request.Context(), h.DB, id); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successNoData(c)
}

// ──── API Key User Routes (API Key authenticated) ────

// ApiKeyLogin godoc
// @Summary Get API key info (end-user)
// @Tags API Keys
// @Produce json
// @Success 200 {object} object "{success: true, data: {id, name, enabled, expireAt, maxCost, totalCost, supportedModels}}"
// @Failure 404 {object} object "{success: false, error: string}"
// @Security ApiKeyAuth
// @Router /api/v1/user/apikey/login [get]
func (h *Handler) ApiKeyLogin(c *gin.Context) {
	apiKeyId, _ := c.Get("apiKeyId")

	keys, err := dal.ListApiKeys(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	for _, k := range keys {
		if k.ID == apiKeyId.(int) {
			successJSON(c, gin.H{
				"id":              k.ID,
				"name":            k.Name,
				"enabled":         k.Enabled,
				"expireAt":        k.ExpireAt,
				"maxCost":         k.MaxCost,
				"totalCost":       k.TotalCost,
				"supportedModels": k.SupportedModels,
			})
			return
		}
	}

	errorJSON(c, http.StatusNotFound, "API key not found")
}

// ApiKeyStats godoc
// @Summary Get API key usage stats (end-user)
// @Tags API Keys
// @Produce json
// @Success 200 {object} object "{success: true, data: {id, name, totalCost, maxCost, enabled, expireAt}}"
// @Failure 404 {object} object "{success: false, error: string}"
// @Security ApiKeyAuth
// @Router /api/v1/user/apikey/stats [get]
func (h *Handler) ApiKeyStats(c *gin.Context) {
	apiKeyId, _ := c.Get("apiKeyId")

	keys, err := dal.ListApiKeys(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	for _, k := range keys {
		if k.ID == apiKeyId.(int) {
			successJSON(c, gin.H{
				"id":        k.ID,
				"name":      k.Name,
				"totalCost": k.TotalCost,
				"maxCost":   k.MaxCost,
				"enabled":   k.Enabled,
				"expireAt":  k.ExpireAt,
			})
			return
		}
	}

	errorJSON(c, http.StatusNotFound, "API key not found")
}
