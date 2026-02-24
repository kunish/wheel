package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
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
	body, err := fetchModelsDevAPI()
	if err != nil {
		return nil, err
	}
	return parseMetadata(body)
}

// ──── Builtin Profiles ────

// profile name → metadata provider key
var builtinProfileProviders = map[string]string{
	"Anthropic": "anthropic",
	"OpenAI":    "openai",
	"Google":    "google",
}

// UpsertBuiltinProfiles ensures builtin workspace profiles exist.
func UpsertBuiltinProfiles(ctx context.Context, db *bun.DB) {
	metadata, err := FetchAndFlattenMetadata()
	if err != nil {
		log.Printf("[profiles] failed to fetch models.dev metadata: %v", err)
		for name, provider := range builtinProfileProviders {
			_ = dal.UpsertBuiltinProfile(ctx, db, name, provider, nil)
		}
		return
	}
	UpsertBuiltinProfilesFromMetadata(ctx, db, metadata)
}

// UpsertBuiltinProfilesFromMetadata ensures builtin profiles are up-to-date
// using already-fetched metadata.
func UpsertBuiltinProfilesFromMetadata(ctx context.Context, db *bun.DB, metadata map[string]ModelMeta) {
	modelsByProvider := make(map[string][]string, len(builtinProfileProviders))
	for modelID, meta := range metadata {
		if modelID == "" {
			continue
		}
		modelsByProvider[meta.Provider] = append(modelsByProvider[meta.Provider], modelID)
	}

	for provider, modelIDs := range modelsByProvider {
		sort.Strings(modelIDs)
		modelsByProvider[provider] = modelIDs
	}

	for name, provider := range builtinProfileProviders {
		models := modelsByProvider[provider]
		if models == nil {
			models = []string{}
		}
		if err := dal.UpsertBuiltinProfile(ctx, db, name, provider, models); err != nil {
			log.Printf("[profiles] failed to upsert builtin profile %s: %v", name, err)
		}
	}
}

// ──── Price Sync ────

var supportedProviders = []string{
	"openai", "anthropic", "google", "deepseek", "xai",
	"alibaba", "zhipuai", "minimax", "moonshotai",
}

// SyncResult holds the outcome of a full models.dev sync operation.
type SyncResult struct {
	PriceSynced  int `json:"priceSynced"`
	PriceUpdated int `json:"priceUpdated"`
}

// SyncAllFromModelsDev fetches models.dev once and syncs prices + builtin profiles.
func SyncAllFromModelsDev(ctx context.Context, db *bun.DB) (*SyncResult, error) {
	body, err := fetchModelsDevAPI()
	if err != nil {
		return nil, err
	}

	// Sync builtin profiles from metadata
	metadata, err := parseMetadata(body)
	if err == nil {
		UpsertBuiltinProfilesFromMetadata(ctx, db, metadata)
	}

	// Sync prices
	result, err := syncPricesFromRaw(ctx, db, body)
	if err != nil {
		return nil, err
	}

	dal.SetLastPriceSyncTime(ctx, db)
	return result, nil
}

func fetchModelsDevAPI() ([]byte, error) {
	resp, err := http.Get("https://models.dev/api.json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("models.dev returned %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func syncPricesFromRaw(ctx context.Context, db *bun.DB, body []byte) (*SyncResult, error) {
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

	result := &SyncResult{}
	for _, providerName := range supportedProviders {
		provider, ok := data[providerName]
		if !ok || provider.Models == nil {
			continue
		}
		for _, modelInfo := range provider.Models {
			if modelInfo.ID == "" {
				continue
			}
			inputPrice, outputPrice := 0.0, 0.0
			cacheReadPrice, cacheWritePrice := 0.0, 0.0
			if modelInfo.Cost != nil {
				inputPrice = modelInfo.Cost.Input
				outputPrice = modelInfo.Cost.Output
				cacheReadPrice = modelInfo.Cost.CacheRead
				cacheWritePrice = modelInfo.Cost.CacheWrite
			}
			if inputPrice == 0 && outputPrice == 0 {
				continue
			}
			upsertResult, err := dal.UpsertLLMPrice(ctx, db, modelInfo.ID, inputPrice, outputPrice, cacheReadPrice, cacheWritePrice, "sync")
			if err != nil {
				continue
			}
			if upsertResult == "created" {
				result.PriceSynced++
			} else {
				result.PriceUpdated++
			}
		}
	}
	return result, nil
}

func parseMetadata(body []byte) (map[string]ModelMeta, error) {
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
			if model.ID == "" || strings.Contains(model.ID, "@") {
				continue
			}
			if hasCanonicalPrefix(model.ID) && !isCanonicalProvider(model.ID, providerKey) {
				continue
			}
			entry := ModelMeta{
				Name:         model.Name,
				Provider:     providerKey,
				ProviderName: providerDisplayName,
				LogoURL:      logoURL,
			}
			if entry.Name == "" {
				entry.Name = model.ID
			}
			result[model.ID] = entry
		}
	}
	return result, nil
}
