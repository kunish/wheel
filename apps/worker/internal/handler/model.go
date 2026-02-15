package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/cache"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/service"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// ──── Model Routes ────

// ListModels godoc
// @Summary List all model prices
// @Tags Models
// @Produce json
// @Success 200 {object} object "{success: true, data: {models: []LLMPrice}}"
// @Security BearerAuth
// @Router /api/v1/model/list [get]
func (h *Handler) ListModels(c *gin.Context) {
	models, err := dal.ListLLMPrices(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, gin.H{"models": models})
}

// ListModelsByChannel godoc
// @Summary List models grouped by channel
// @Tags Models
// @Produce json
// @Success 200 {object} object "{success: true, data: []object}"
// @Security BearerAuth
// @Router /api/v1/model/channel [get]
func (h *Handler) ListModelsByChannel(c *gin.Context) {
	channels, err := dal.ListChannels(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	type channelModels struct {
		ChannelID   int      `json:"channelId"`
		ChannelName string   `json:"channelName"`
		Type        int      `json:"type"`
		Enabled     bool     `json:"enabled"`
		Models      []string `json:"models"`
	}

	result := make([]channelModels, 0, len(channels))
	for _, ch := range channels {
		models := make([]string, 0)
		models = append(models, []string(ch.Model)...)
		if ch.CustomModel != "" {
			for _, m := range strings.Split(ch.CustomModel, ",") {
				m = strings.TrimSpace(m)
				if m != "" {
					models = append(models, m)
				}
			}
		}
		result = append(result, channelModels{
			ChannelID:   ch.ID,
			ChannelName: ch.Name,
			Type:        int(ch.Type),
			Enabled:     ch.Enabled,
			Models:      models,
		})
	}

	successJSON(c, result)
}

// CreateModel godoc
// @Summary Create a model price entry
// @Tags Models
// @Accept json
// @Produce json
// @Param body body object true "Model name and prices"
// @Success 200 {object} object "{success: true, data: LLMPrice}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Failure 409 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/model/create [post]
func (h *Handler) CreateModel(c *gin.Context) {
	var body types.LLMCreateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	if body.Name == "" {
		errorJSON(c, http.StatusBadRequest, "name is required")
		return
	}

	model, err := dal.CreateLLMPrice(c.Request.Context(), h.DB, body.Name, body.InputPrice, body.OutputPrice, "manual")
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			errorJSON(c, http.StatusConflict, fmt.Sprintf("Model '%s' already exists", body.Name))
			return
		}
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	successJSON(c, model)
}

// UpdateModel godoc
// @Summary Update a model price entry
// @Tags Models
// @Accept json
// @Produce json
// @Param body body object true "Model ID and optional fields to update"
// @Success 200 {object} object "{success: true}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/model/update [post]
func (h *Handler) UpdateModel(c *gin.Context) {
	var body types.LLMUpdateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	if body.ID == 0 {
		errorJSON(c, http.StatusBadRequest, "id is required")
		return
	}

	data := make(map[string]any)
	if body.Name != nil {
		data["name"] = *body.Name
	}
	if body.InputPrice != nil {
		data["input_price"] = *body.InputPrice
	}
	if body.OutputPrice != nil {
		data["output_price"] = *body.OutputPrice
	}

	if err := dal.UpdateLLMPrice(c.Request.Context(), h.DB, body.ID, data); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	successNoData(c)
}

// DeleteModel godoc
// @Summary Delete a model price entry
// @Tags Models
// @Accept json
// @Produce json
// @Param body body object true "Model ID"
// @Success 200 {object} object "{success: true}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/model/delete [post]
func (h *Handler) DeleteModel(c *gin.Context) {
	var body types.LLMDeleteRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	if body.ID == 0 {
		errorJSON(c, http.StatusBadRequest, "id is required")
		return
	}

	if err := dal.DeleteLLMPrice(c.Request.Context(), h.DB, body.ID); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	successNoData(c)
}

// ──── Model Metadata ────

// GetModelMetadata godoc
// @Summary Get model metadata (provider, logo, etc.)
// @Tags Models
// @Produce json
// @Success 200 {object} object "{success: true, data: map[string]ModelMeta}"
// @Security BearerAuth
// @Router /api/v1/model/metadata [get]
func (h *Handler) GetModelMetadata(c *gin.Context) {
	cached, ok := cache.Get[map[string]service.ModelMeta](h.Cache, service.MetadataKVKey)
	if ok && cached != nil {
		successJSON(c, cached)
		return
	}

	metadata, err := service.FetchAndFlattenMetadata()
	if err != nil {
		successJSON(c, map[string]any{})
		return
	}

	h.Cache.Put(service.MetadataKVKey, metadata, service.MetadataTTL)
	successJSON(c, metadata)
}

// RefreshModelMetadata godoc
// @Summary Refresh model metadata from models.dev
// @Tags Models
// @Produce json
// @Success 200 {object} object "{success: true, data: {count: int}}"
// @Security BearerAuth
// @Router /api/v1/model/metadata/refresh [post]
func (h *Handler) RefreshModelMetadata(c *gin.Context) {
	h.Cache.Delete(service.MetadataKVKey)

	metadata, err := service.FetchAndFlattenMetadata()
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	h.Cache.Put(service.MetadataKVKey, metadata, service.MetadataTTL)
	successJSON(c, gin.H{"count": len(metadata)})
}

// ──── Price Sync ────

// UpdatePrice godoc
// @Summary Sync model prices from models.dev
// @Tags Models
// @Produce json
// @Success 200 {object} object "{success: true, data: {synced: int, updated: int}}"
// @Security BearerAuth
// @Router /api/v1/model/update-price [post]
func (h *Handler) UpdatePrice(c *gin.Context) {
	result, err := service.SyncPricesFromModelsDev(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, result)
}

// GetLastUpdateTime godoc
// @Summary Get last price sync timestamp
// @Tags Models
// @Produce json
// @Success 200 {object} object "{success: true, data: {lastUpdateTime: string}}"
// @Security BearerAuth
// @Router /api/v1/model/last-update-time [get]
func (h *Handler) GetLastUpdateTime(c *gin.Context) {
	lastUpdate, err := dal.GetLastPriceSyncTime(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	var val any
	if lastUpdate != nil {
		val = *lastUpdate
	}
	successJSON(c, gin.H{"lastUpdateTime": val})
}
