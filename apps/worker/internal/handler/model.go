package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/cache"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/uptrace/bun"
)

// ──── Model Routes ────

func (h *Handler) ListModels(c *gin.Context) {
	models, err := dal.ListLLMPrices(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, gin.H{"models": models})
}

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

func (h *Handler) CreateModel(c *gin.Context) {
	var body struct {
		Name        string  `json:"name"`
		InputPrice  float64 `json:"inputPrice"`
		OutputPrice float64 `json:"outputPrice"`
	}
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

func (h *Handler) UpdateModel(c *gin.Context) {
	var body struct {
		ID          int      `json:"id"`
		Name        *string  `json:"name,omitempty"`
		InputPrice  *float64 `json:"inputPrice,omitempty"`
		OutputPrice *float64 `json:"outputPrice,omitempty"`
	}
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

func (h *Handler) DeleteModel(c *gin.Context) {
	var body struct {
		ID int `json:"id"`
	}
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

const metadataKVKey = "model-metadata"
const metadataTTL = 86400 // 24h

// Canonical provider prefix mapping
var canonicalProviders = map[string]string{
	"gpt-":      "openai",
	"chatgpt-":  "openai",
	"o1-":       "openai",
	"o3-":       "openai",
	"o4-":       "openai",
	"claude-":   "anthropic",
	"gemini-":   "google",
	"deepseek-": "deepseek",
	"grok-":     "xai",
	"qwen-":     "alibaba",
	"glm-":      "zhipuai",
}

type modelMeta struct {
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	ProviderName string `json:"providerName"`
	LogoURL      string `json:"logoUrl"`
}

func isCanonicalProvider(modelID, providerKey string) bool {
	for prefix, canonical := range canonicalProviders {
		if strings.HasPrefix(modelID, prefix) {
			return providerKey == canonical
		}
	}
	return false
}

func hasCanonicalPrefix(modelID string) bool {
	for prefix := range canonicalProviders {
		if strings.HasPrefix(modelID, prefix) {
			return true
		}
	}
	return false
}

func fetchAndFlattenMetadata() (map[string]modelMeta, error) {
	resp, err := http.Get("https://models.dev/api.json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("models.dev returned %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)

	type modelEntry struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	type providerEntry struct {
		Name   string                `json:"name"`
		Models map[string]modelEntry `json:"models"`
	}

	var data map[string]providerEntry
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	// Pre-collect canonical provider display info
	type canonicalInfo struct {
		providerName string
		logoURL      string
	}
	canonicalInfoMap := make(map[string]canonicalInfo)
	for prefix, canonicalKey := range canonicalProviders {
		if provider, ok := data[canonicalKey]; ok {
			name := provider.Name
			if name == "" {
				name = canonicalKey
			}
			canonicalInfoMap[prefix] = canonicalInfo{
				providerName: name,
				logoURL:      "https://models.dev/logos/" + canonicalKey + ".svg",
			}
		}
	}

	result := make(map[string]modelMeta)
	for providerKey, provider := range data {
		if provider.Models == nil {
			continue
		}
		providerDisplayName := provider.Name
		if providerDisplayName == "" {
			providerDisplayName = providerKey
		}
		logoURL := "https://models.dev/logos/" + providerKey + ".svg"

		for _, model := range provider.Models {
			modelID := model.ID
			if modelID == "" {
				continue
			}

			entryProvider := providerKey
			entryProviderName := providerDisplayName
			entryLogoURL := logoURL

			if hasCanonicalPrefix(modelID) && !isCanonicalProvider(modelID, providerKey) {
				for prefix, info := range canonicalInfoMap {
					if strings.HasPrefix(modelID, prefix) {
						entryProvider = canonicalProviders[prefix]
						entryProviderName = info.providerName
						entryLogoURL = info.logoURL
						break
					}
				}
			}

			entry := modelMeta{
				Name:         model.Name,
				Provider:     entryProvider,
				ProviderName: entryProviderName,
				LogoURL:      entryLogoURL,
			}
			if entry.Name == "" {
				entry.Name = modelID
			}

			if existing, ok := result[modelID]; ok {
				if isCanonicalProvider(modelID, providerKey) && !isCanonicalProvider(modelID, existing.Provider) {
					result[modelID] = entry
				}
			} else {
				result[modelID] = entry
			}
		}
	}

	return result, nil
}

func (h *Handler) GetModelMetadata(c *gin.Context) {
	cached, ok := cache.Get[map[string]modelMeta](h.Cache, metadataKVKey)
	if ok && cached != nil {
		successJSON(c, cached)
		return
	}

	metadata, err := fetchAndFlattenMetadata()
	if err != nil {
		successJSON(c, map[string]any{})
		return
	}

	h.Cache.Put(metadataKVKey, metadata, metadataTTL)
	successJSON(c, metadata)
}

func (h *Handler) RefreshModelMetadata(c *gin.Context) {
	h.Cache.Delete(metadataKVKey)

	metadata, err := fetchAndFlattenMetadata()
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	h.Cache.Put(metadataKVKey, metadata, metadataTTL)
	successJSON(c, gin.H{"count": len(metadata)})
}

// ──── Price Sync ────

var supportedProviders = []string{
	"openai", "anthropic", "google", "deepseek", "xai",
	"alibaba", "zhipuai", "minimax", "moonshotai",
}

func (h *Handler) UpdatePrice(c *gin.Context) {
	result, err := SyncPricesFromModelsDev(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, result)
}

// SyncPricesFromModelsDev fetches pricing from models.dev and upserts into the DB.
func SyncPricesFromModelsDev(ctx context.Context, db *bun.DB) (gin.H, error) {
	resp, err := http.Get("https://models.dev/api.json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch models.dev: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)

	type costEntry struct {
		Input      float64 `json:"input"`
		Output     float64 `json:"output"`
		CacheRead  float64 `json:"cache_read"`
		CacheWrite float64 `json:"cache_write"`
	}
	type modelEntry struct {
		ID   string     `json:"id"`
		Cost *costEntry `json:"cost,omitempty"`
	}
	type providerEntry struct {
		Models map[string]modelEntry `json:"models"`
	}

	var data map[string]providerEntry
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	synced := 0
	updated := 0

	for _, providerName := range supportedProviders {
		provider, ok := data[providerName]
		if !ok || provider.Models == nil {
			continue
		}

		for _, modelInfo := range provider.Models {
			modelID := modelInfo.ID
			if modelID == "" {
				continue
			}

			inputPrice := 0.0
			outputPrice := 0.0
			cacheReadPrice := 0.0
			cacheWritePrice := 0.0

			if modelInfo.Cost != nil {
				inputPrice = modelInfo.Cost.Input
				outputPrice = modelInfo.Cost.Output
				cacheReadPrice = modelInfo.Cost.CacheRead
				cacheWritePrice = modelInfo.Cost.CacheWrite
			}

			if inputPrice == 0 && outputPrice == 0 {
				continue
			}

			result, err := dal.UpsertLLMPrice(ctx, db, modelID, inputPrice, outputPrice, cacheReadPrice, cacheWritePrice, "sync")
			if err != nil {
				continue
			}

			if result == "created" {
				synced++
			} else {
				updated++
			}
		}
	}

	dal.SetLastPriceSyncTime(ctx, db)

	return gin.H{"synced": synced, "updated": updated}, nil
}

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
