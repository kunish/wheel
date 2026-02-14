package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/uptrace/bun"
)

// ──── Model Metadata ────

const MetadataKVKey = "model-metadata"
const MetadataTTL = 86400 // 24h

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
	"qwen3-":    "alibaba",
	"glm-":      "zhipuai",
	"minimax-":  "minimax",
	"kimi-":     "moonshotai",
}

// ModelMeta holds metadata for a single model from models.dev.
type ModelMeta struct {
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

// FetchAndFlattenMetadata fetches model metadata from models.dev and returns
// a flat map of model ID → ModelMeta with canonical provider resolution.
func FetchAndFlattenMetadata() (map[string]ModelMeta, error) {
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

	result := make(map[string]ModelMeta)
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

			entry := ModelMeta{
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

// ──── Price Sync ────

var supportedProviders = []string{
	"openai", "anthropic", "google", "deepseek", "xai",
	"alibaba", "zhipuai", "minimax", "moonshotai",
}

// PriceSyncResult holds the outcome of a price sync operation.
type PriceSyncResult struct {
	Synced  int `json:"synced"`
	Updated int `json:"updated"`
}

// SyncPricesFromModelsDev fetches pricing from models.dev and upserts into the DB.
func SyncPricesFromModelsDev(ctx context.Context, db *bun.DB) (*PriceSyncResult, error) {
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

	result := &PriceSyncResult{}

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

			upsertResult, err := dal.UpsertLLMPrice(ctx, db, modelID, inputPrice, outputPrice, cacheReadPrice, cacheWritePrice, "sync")
			if err != nil {
				continue
			}

			if upsertResult == "created" {
				result.Synced++
			} else {
				result.Updated++
			}
		}
	}

	dal.SetLastPriceSyncTime(ctx, db)

	return result, nil
}
