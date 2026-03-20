package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/cache"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/service"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// ──── Channel Routes ────

// ListChannels godoc
// @Summary List all channels
// @Tags Channels
// @Produce json
// @Success 200 {object} object "{success: true, data: {channels: []Channel}}"
// @Security BearerAuth
// @Router /api/v1/channel/list [get]
func (h *Handler) ListChannels(c *gin.Context) {
	channels, err := dal.ListChannels(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, gin.H{"channels": channels})
}

// CreateChannel godoc
// @Summary Create a new channel
// @Tags Channels
// @Accept json
// @Produce json
// @Param body body types.ChannelCreateRequest true "Channel configuration"
// @Success 200 {object} object "{success: true, data: Channel}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/channel/create [post]
func (h *Handler) CreateChannel(c *gin.Context) {
	var req types.ChannelCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	ch := types.Channel{
		Name:          req.Name,
		Type:          types.OutboundType(req.Type),
		Enabled:       req.Enabled,
		BaseUrls:      types.BaseUrlList(req.BaseUrls),
		Model:         types.StringList(req.Model),
		FetchedModel:  types.StringList(req.FetchedModel),
		CustomModel:   req.CustomModel,
		CustomHeader:  types.CustomHeaderList(req.CustomHeader),
		ParamOverride: req.ParamOverride,
	}
	if req.AutoSync != nil {
		ch.AutoSync = *req.AutoSync
	}
	if req.AutoGroup != nil {
		ch.AutoGroup = types.AutoGroupType(*req.AutoGroup)
	}

	created, err := dal.CreateChannel(c.Request.Context(), h.DB, ch, req.Keys)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	h.Cache.Delete("channels")
	successJSON(c, created)
}

// UpdateChannel godoc
// @Summary Update channel configuration
// @Tags Channels
// @Accept json
// @Produce json
// @Param body body object true "Partial channel fields to update (id required)"
// @Success 200 {object} object "{success: true}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/channel/update [post]
func (h *Handler) UpdateChannel(c *gin.Context) {
	var body types.ChannelUpdateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	if body.ID == 0 {
		errorJSON(c, http.StatusBadRequest, "id is required")
		return
	}

	// Build update map with DB column names from pointer fields
	data := make(map[string]any)
	if body.Name != nil {
		data["name"] = *body.Name
	}
	if body.Type != nil {
		data["type"] = *body.Type
	}
	if body.Enabled != nil {
		data["enabled"] = *body.Enabled
	}
	if body.BaseUrls != nil {
		jsonBytes, _ := json.Marshal(body.BaseUrls)
		data["base_urls"] = string(jsonBytes)
	}
	if body.Model != nil {
		jsonBytes, _ := json.Marshal(body.Model)
		data["model"] = string(jsonBytes)
	}
	if body.CustomModel != nil {
		data["custom_model"] = *body.CustomModel
	}
	if body.Proxy != nil {
		data["proxy"] = *body.Proxy
	}
	if body.AutoSync != nil {
		data["auto_sync"] = *body.AutoSync
	}
	if body.AutoGroup != nil {
		data["auto_group"] = *body.AutoGroup
	}
	if body.CustomHeader != nil {
		jsonBytes, _ := json.Marshal(body.CustomHeader)
		data["custom_header"] = string(jsonBytes)
	}
	if body.ParamOverride != nil {
		data["param_override"] = *body.ParamOverride
	}
	if body.ChannelProxy != nil {
		data["channel_proxy"] = *body.ChannelProxy
	}
	if body.FetchedModel != nil {
		jsonBytes, _ := json.Marshal(body.FetchedModel)
		data["fetched_model"] = string(jsonBytes)
	}

	ctx := c.Request.Context()

	if err := dal.UpdateChannel(ctx, h.DB, body.ID, data); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Sync keys if provided
	if body.Keys != nil {
		if err := dal.SyncChannelKeys(ctx, h.DB, body.ID, body.Keys); err != nil {
			errorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}
	}

	h.Cache.Delete("channels")
	successNoData(c)
}

// EnableChannel godoc
// @Summary Enable or disable a channel
// @Tags Channels
// @Accept json
// @Produce json
// @Param body body types.ChannelEnableRequest true "Channel ID and enabled state"
// @Success 200 {object} object "{success: true}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/channel/enable [post]
func (h *Handler) EnableChannel(c *gin.Context) {
	var req types.ChannelEnableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := dal.EnableChannel(c.Request.Context(), h.DB, req.ID, req.Enabled); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	h.Cache.Delete("channels")
	successNoData(c)
}

// DeleteChannel godoc
// @Summary Delete a channel
// @Tags Channels
// @Produce json
// @Param id path int true "Channel ID"
// @Success 200 {object} object "{success: true}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/channel/delete/{id} [delete]
func (h *Handler) DeleteChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid channel ID")
		return
	}

	if err := dal.DeleteChannel(c.Request.Context(), h.DB, id); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	h.Cache.Delete("channels")
	successNoData(c)
}

// ReorderChannels godoc
// @Summary Reorder channels
// @Tags Channels
// @Accept json
// @Produce json
// @Param body body object true "Ordered channel IDs"
// @Success 200 {object} object "{success: true}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/channel/reorder [post]
func (h *Handler) ReorderChannels(c *gin.Context) {
	var req struct {
		OrderedIds []int `json:"orderedIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := dal.ReorderChannels(c.Request.Context(), h.DB, req.OrderedIds); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	h.Cache.Delete("channels")
	successNoData(c)
}

// ──── Fetch Model / Sync ────

type fetchModelRequest struct {
	ID int `json:"id"`
}

// FetchModel godoc
// @Summary Fetch models from an existing channel
// @Tags Channels
// @Accept json
// @Produce json
// @Param body body fetchModelRequest true "Channel ID"
// @Success 200 {object} object "{success: true, data: {models: []string}}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/channel/fetch-model [post]
func (h *Handler) FetchModel(c *gin.Context) {
	var req fetchModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	channel, err := dal.GetChannel(c.Request.Context(), h.DB, req.ID)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	if channel == nil {
		errorJSON(c, http.StatusNotFound, "Channel not found")
		return
	}

	models, _, err := fetchModelsFromChannel(c.Request.Context(), channel, h.Cache)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	successJSON(c, gin.H{"models": models})
}

type fetchModelPreviewRequest struct {
	Type    int    `json:"type"`
	BaseUrl string `json:"baseUrl"`
	Key     string `json:"key"`
}

// FetchModelPreview godoc
// @Summary Preview models from channel credentials (without saving)
// @Tags Channels
// @Accept json
// @Produce json
// @Param body body fetchModelPreviewRequest true "Channel type, base URL, and API key"
// @Success 200 {object} object "{success: true, data: {models: []string}}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/channel/fetch-model-preview [post]
func (h *Handler) FetchModelPreview(c *gin.Context) {
	var req fetchModelPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Key == "" {
		errorJSON(c, http.StatusBadRequest, "key is required")
		return
	}

	channelType := req.Type
	if channelType == 0 {
		channelType = 1
	}
	if req.BaseUrl == "" && types.OutboundType(channelType) != types.OutboundCursor {
		errorJSON(c, http.StatusBadRequest, "baseUrl and key are required")
		return
	}

	pseudoChannel := &types.Channel{
		Type:     types.OutboundType(channelType),
		Enabled:  true,
		BaseUrls: types.BaseUrlList{{URL: req.BaseUrl, Delay: 0}},
		Keys: []types.ChannelKey{
			{Enabled: true, ChannelKey: req.Key},
		},
	}

	models, isFallback, err := fetchModelsFromChannel(c.Request.Context(), pseudoChannel, h.Cache)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	successJSON(c, gin.H{"models": models, "isFallback": isFallback})
}

// SyncAllModels godoc
// @Summary Sync models from all auto-sync channels
// @Tags Channels
// @Produce json
// @Success 200 {object} object "{success: true, data: SyncResult}"
// @Security BearerAuth
// @Router /api/v1/channel/sync [post]
func (h *Handler) SyncAllModels(c *gin.Context) {
	result, err := relay.SyncAllModels(c.Request.Context(), h.DB, h.Cache)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, result)
}

// LastSyncTime godoc
// @Summary Get last model sync timestamp
// @Tags Channels
// @Produce json
// @Success 200 {object} object "{success: true, data: {lastSyncTime: int}}"
// @Security BearerAuth
// @Router /api/v1/channel/last-sync-time [get]
func (h *Handler) LastSyncTime(c *gin.Context) {
	settings, err := dal.GetAllSettings(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	lastSyncTime := "0"
	if v, ok := settings["last_sync_time"]; ok {
		lastSyncTime = v
	}
	n, _ := strconv.ParseInt(lastSyncTime, 10, 64)
	successJSON(c, gin.H{"lastSyncTime": n})
}

// ──── Model fetch helpers ────

func fallbackModelsFromMetadata(kv *cache.MemoryKV, providerKey string) []string {
	cached, ok := cache.Get[map[string]service.ModelMeta](kv, service.MetadataKVKey)
	if !ok || cached == nil {
		// Try fetching fresh metadata
		metadata, err := service.FetchAndFlattenMetadata()
		if err != nil {
			return nil
		}
		kv.Put(service.MetadataKVKey, metadata, service.MetadataTTL)
		cached = &metadata
	}

	var models []string
	for modelID, meta := range *cached {
		if meta.Provider == providerKey {
			models = append(models, modelID)
		}
	}
	sort.Strings(models)
	return models
}

// fetchModelsFromChannel returns (models, isFallback, error).
// isFallback is true when the result came from models.dev metadata
// instead of the real upstream API (e.g. Anthropic API blocked by Cloudflare).
func fetchModelsFromChannel(ctx context.Context, channel *types.Channel, kv *cache.MemoryKV) ([]string, bool, error) {
	var key *types.ChannelKey
	for i := range channel.Keys {
		if channel.Keys[i].Enabled {
			key = &channel.Keys[i]
			break
		}
	}
	if key == nil {
		return []string{}, false, nil
	}

	baseUrl := ""
	if len(channel.BaseUrls) > 0 {
		baseUrl = strings.TrimRight(channel.BaseUrls[0].URL, "/")
	}
	// Cursor uses cursorChannelBaseURL() inside FetchUsableModels (default api2.cursor.sh);
	// do not bail out when base URL is empty.
	if baseUrl == "" && channel.Type != types.OutboundCursor {
		return []string{}, false, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	switch channel.Type {
	case types.OutboundCursor:
		models, err := NewCursorRelay().FetchUsableModels(ctx, channel, key.ChannelKey)
		return models, false, err
	case types.OutboundAnthropic:
		models, err := relay.FetchAnthropicModels(ctx, baseUrl, key.ChannelKey)
		if err != nil && kv != nil {
			if fb := fallbackModelsFromMetadata(kv, "anthropic"); len(fb) > 0 {
				return fb, true, nil
			}
		}
		return models, false, err
	case types.OutboundGemini:
		models, err := relay.FetchGeminiModels(ctx, baseUrl, key.ChannelKey)
		return models, false, err
	default:
		models, err := relay.FetchOpenAIModels(ctx, baseUrl, key.ChannelKey)
		return models, false, err
	}
}
