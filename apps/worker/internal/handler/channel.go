package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/cache"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/types"
	"github.com/uptrace/bun"
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
		Name:         req.Name,
		Type:         types.OutboundType(req.Type),
		Enabled:      req.Enabled,
		BaseUrls:     types.BaseUrlList(req.BaseUrls),
		Model:        types.StringList(req.Model),
		CustomModel:  req.CustomModel,
		CustomHeader: types.CustomHeaderList(req.CustomHeader),
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
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	idFloat, ok := body["id"].(float64)
	if !ok {
		errorJSON(c, http.StatusBadRequest, "id is required")
		return
	}
	id := int(idFloat)

	// Build update map with DB column names
	data := make(map[string]interface{})
	if v, ok := body["name"]; ok {
		data["name"] = v
	}
	if v, ok := body["type"]; ok {
		data["type"] = v
	}
	if v, ok := body["enabled"]; ok {
		if b, ok := v.(bool); ok {
			data["enabled"] = b
		}
	}
	if v, ok := body["baseUrls"]; ok {
		jsonBytes, _ := json.Marshal(v)
		data["base_urls"] = string(jsonBytes)
	}
	if v, ok := body["model"]; ok {
		jsonBytes, _ := json.Marshal(v)
		data["model"] = string(jsonBytes)
	}
	if v, ok := body["customModel"]; ok {
		data["custom_model"] = v
	}
	if v, ok := body["proxy"]; ok {
		if b, ok := v.(bool); ok {
			data["proxy"] = b
		}
	}
	if v, ok := body["autoSync"]; ok {
		if b, ok := v.(bool); ok {
			data["auto_sync"] = b
		}
	}
	if v, ok := body["autoGroup"]; ok {
		data["auto_group"] = v
	}
	if v, ok := body["customHeader"]; ok {
		jsonBytes, _ := json.Marshal(v)
		data["custom_header"] = string(jsonBytes)
	}
	if v, ok := body["paramOverride"]; ok {
		data["param_override"] = v
	}
	if v, ok := body["channelProxy"]; ok {
		data["channel_proxy"] = v
	}
	if v, ok := body["fetchedModel"]; ok {
		jsonBytes, _ := json.Marshal(v)
		data["fetched_model"] = string(jsonBytes)
	}

	ctx := c.Request.Context()

	if err := dal.UpdateChannel(ctx, h.DB, id, data); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Sync keys if provided
	if keysRaw, ok := body["keys"]; ok {
		keysJSON, _ := json.Marshal(keysRaw)
		var keys []types.ChannelKeyInput
		json.Unmarshal(keysJSON, &keys)
		if err := dal.SyncChannelKeys(ctx, h.DB, id, keys); err != nil {
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

	models, _, err := fetchModelsFromChannel(channel, h.Cache)
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

	if req.BaseUrl == "" || req.Key == "" {
		errorJSON(c, http.StatusBadRequest, "baseUrl and key are required")
		return
	}

	channelType := req.Type
	if channelType == 0 {
		channelType = 1
	}

	pseudoChannel := &types.Channel{
		Type:     types.OutboundType(channelType),
		Enabled:  true,
		BaseUrls: types.BaseUrlList{{URL: req.BaseUrl, Delay: 0}},
		Keys: []types.ChannelKey{
			{Enabled: true, ChannelKey: req.Key},
		},
	}

	models, isFallback, err := fetchModelsFromChannel(pseudoChannel, h.Cache)
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
	result, err := syncAllModels(c.Request.Context(), h.DB, h.Cache)
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

const browserUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

func setBrowserHeaders(req *http.Request) {
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "application/json, text/plain, */*")
}



func fallbackModelsFromMetadata(kv *cache.MemoryKV, providerKey string) []string {
	cached, ok := cache.Get[map[string]modelMeta](kv, metadataKVKey)
	if !ok || cached == nil {
		// Try fetching fresh metadata
		metadata, err := fetchAndFlattenMetadata()
		if err != nil {
			return nil
		}
		kv.Put(metadataKVKey, metadata, metadataTTL)
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
func fetchModelsFromChannel(channel *types.Channel, kv *cache.MemoryKV) ([]string, bool, error) {
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
	if baseUrl == "" {
		return []string{}, false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch channel.Type {
	case types.OutboundAnthropic:
		models, err := fetchAnthropicModels(ctx, baseUrl, key.ChannelKey)
		if err != nil && kv != nil {
			if fb := fallbackModelsFromMetadata(kv, "anthropic"); len(fb) > 0 {
				return fb, true, nil
			}
		}
		return models, false, err
	case types.OutboundGemini:
		models, err := fetchGeminiModels(ctx, baseUrl, key.ChannelKey)
		return models, false, err
	default:
		models, err := fetchOpenAIModels(ctx, baseUrl, key.ChannelKey)
		return models, false, err
	}
}

func fetchOpenAIModels(ctx context.Context, baseUrl, apiKey string) ([]string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", baseUrl+"/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	setBrowserHeaders(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return nil, fmt.Errorf("OpenAI models API returned %d: %s", resp.StatusCode, snippet)
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	models := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, m.ID)
	}
	return models, nil
}

func fetchAnthropicModels(ctx context.Context, baseUrl, apiKey string) ([]string, error) {
	var allModels []string
	afterID := ""

	for {
		url := baseUrl + "/v1/models?limit=100"
		if afterID != "" {
			url += "&after_id=" + afterID
		}

		req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		setBrowserHeaders(req)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusOK {
			snippet := string(body)
			if len(snippet) > 200 {
				snippet = snippet[:200]
			}
			return nil, fmt.Errorf("Anthropic models API returned %d: %s", resp.StatusCode, snippet)
		}

		var result struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
			HasMore bool   `json:"has_more"`
			LastID  string `json:"last_id"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, err
		}

		for _, m := range result.Data {
			allModels = append(allModels, m.ID)
		}

		if !result.HasMore || result.LastID == "" {
			break
		}
		afterID = result.LastID
	}

	return allModels, nil
}

func fetchGeminiModels(ctx context.Context, baseUrl, apiKey string) ([]string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", baseUrl+"/v1/models?key="+apiKey, nil)
	setBrowserHeaders(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return nil, fmt.Errorf("Gemini models API returned %d: %s", resp.StatusCode, snippet)
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	models := make([]string, 0, len(result.Models))
	for _, m := range result.Models {
		models = append(models, strings.TrimPrefix(m.Name, "models/"))
	}
	return models, nil
}

// ──── Sync all models ────

func syncAllModels(ctx context.Context, db *bun.DB, kv *cache.MemoryKV) (*types.SyncResult, error) {
	result := &types.SyncResult{
		NewModels:     []string{},
		RemovedModels: []string{},
		Errors:        []string{},
	}

	channels, err := dal.ListChannels(ctx, db)
	if err != nil {
		return nil, err
	}

	for i := range channels {
		ch := &channels[i]
		if !ch.AutoSync {
			continue
		}

		upstreamModels, isFallback, err := fetchModelsFromChannel(ch, kv)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Channel %s: %s", ch.Name, err.Error()))
			continue
		}
		if len(upstreamModels) == 0 {
			continue
		}

		oldModels := []string(ch.Model)
		newSet := make(map[string]bool)
		for _, m := range upstreamModels {
			newSet[m] = true
		}
		oldSet := make(map[string]bool)
		for _, m := range oldModels {
			oldSet[m] = true
		}

		for _, m := range upstreamModels {
			if !oldSet[m] {
				result.NewModels = append(result.NewModels, m)
			}
		}
		for _, m := range oldModels {
			if !newSet[m] {
				result.RemovedModels = append(result.RemovedModels, m)
			}
		}

		// Update channel model list; only mark as fetched if from real API
		modelJSON, _ := json.Marshal(upstreamModels)
		updates := map[string]interface{}{
			"model": string(modelJSON),
		}
		if !isFallback {
			updates["fetched_model"] = string(modelJSON)
		}
		dal.UpdateChannel(ctx, db, ch.ID, updates)

		result.SyncedChannels++
	}

	// Save last sync time
	now := fmt.Sprintf("%d", time.Now().Unix())
	dal.UpdateSettings(ctx, db, map[string]string{"last_sync_time": now})

	kv.Delete("channels")
	kv.Delete("groups")

	return result, nil
}
