package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/kunish/wheel/apps/worker/internal/cache"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/types"
	"github.com/uptrace/bun"
)

// SyncResult tracks the outcome of a model sync operation.
type SyncResult = types.SyncResult

// FetchModelsFromChannel fetches model names from an upstream channel provider.
// When kv is non-nil and Anthropic API fails (e.g. Cloudflare), falls back to models.dev metadata.
func FetchModelsFromChannel(channel types.Channel, kv *cache.MemoryKV) ([]string, error) {
	// Find first enabled key
	var apiKey string
	for _, k := range channel.Keys {
		if k.Enabled {
			apiKey = k.ChannelKey
			break
		}
	}
	if apiKey == "" {
		return nil, nil
	}

	baseUrl := ""
	if len(channel.BaseUrls) > 0 {
		baseUrl = strings.TrimRight(channel.BaseUrls[0].URL, "/")
	}
	if baseUrl == "" {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch channel.Type {
	case types.OutboundAnthropic:
		models, err := fetchAnthropicModels(ctx, baseUrl, apiKey)
		if err != nil && kv != nil {
			if fb := fallbackModelsFromMetadata(kv, "anthropic"); len(fb) > 0 {
				return fb, nil
			}
		}
		return models, err
	case types.OutboundGemini:
		return fetchGeminiModels(ctx, baseUrl, apiKey)
	default:
		return fetchOpenAIModels(ctx, baseUrl, apiKey)
	}
}

const browserUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

func setBrowserHeaders(req *http.Request) {
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "application/json, text/plain, */*")
}

func fallbackModelsFromMetadata(kv *cache.MemoryKV, providerKey string) []string {
	cached, ok := cache.Get[map[string]string](kv, "model-metadata-providers")
	if !ok || cached == nil {
		metadata, err := fetchModelsDevProviders()
		if err != nil {
			return nil
		}
		kv.Put("model-metadata-providers", metadata, 86400)
		cached = &metadata
	}
	var models []string
	for modelID, provider := range *cached {
		if provider == providerKey {
			models = append(models, modelID)
		}
	}
	sort.Strings(models)
	return models
}

func fetchModelsDevProviders() (map[string]string, error) {
	resp, err := http.Get("https://models.dev/api.json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("models.dev returned %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	type modelEntry struct {
		ID string `json:"id"`
	}
	type providerEntry struct {
		Models map[string]modelEntry `json:"models"`
	}
	var data map[string]providerEntry
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for providerKey, provider := range data {
		for _, model := range provider.Models {
			if model.ID != "" {
				result[model.ID] = providerKey
			}
		}
	}
	return result, nil
}

func fetchOpenAIModels(ctx context.Context, baseUrl, apiKey string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", baseUrl+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	setBrowserHeaders(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
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

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		setBrowserHeaders(req)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != 200 {
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
	req, err := http.NewRequestWithContext(ctx, "GET", baseUrl+"/v1/models?key="+apiKey, nil)
	if err != nil {
		return nil, err
	}
	setBrowserHeaders(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
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

// SyncAllModels syncs models for all channels with autoSync enabled.
func SyncAllModels(ctx context.Context, db *bun.DB, kv *cache.MemoryKV) (*SyncResult, error) {
	result := &SyncResult{}

	allChannels, err := dal.ListChannels(ctx, db)
	if err != nil {
		return nil, err
	}

	for _, channel := range allChannels {
		if !channel.AutoSync {
			continue
		}

		upstreamModels, err := FetchModelsFromChannel(channel, kv)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Channel %s: %v", channel.Name, err))
			continue
		}
		if len(upstreamModels) == 0 {
			continue
		}

		oldModels := []string(channel.Model)

		newSet := make(map[string]bool)
		for _, m := range upstreamModels {
			newSet[m] = true
		}
		oldSet := make(map[string]bool)
		for _, m := range oldModels {
			oldSet[m] = true
		}

		var added, removed []string
		for _, m := range upstreamModels {
			if !oldSet[m] {
				added = append(added, m)
			}
		}
		for _, m := range oldModels {
			if !newSet[m] {
				removed = append(removed, m)
			}
		}

		// Update channel model field
		modelJSON, _ := json.Marshal(upstreamModels)
		if err := dal.UpdateChannel(ctx, db, channel.ID, map[string]any{"model": string(modelJSON)}); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Channel %s: failed to update models: %v", channel.Name, err))
			continue
		}

		// Remove group items for disappeared models
		for _, modelName := range removed {
			db.NewDelete().TableExpr("group_items").
				Where("channel_id = ?", channel.ID).
				Where("model_name = ?", modelName).
				Exec(ctx)
		}

		// Auto group if configured
		if channel.AutoGroup != types.AutoGroupNone {
			autoGroupChannel(ctx, db, channel, upstreamModels)
		}

		result.SyncedChannels++
		result.NewModels = append(result.NewModels, added...)
		result.RemovedModels = append(result.RemovedModels, removed...)
	}

	// Save last sync time
	now := fmt.Sprintf("%d", time.Now().Unix())
	dal.UpdateSettings(ctx, db, map[string]string{"last_sync_time": now})

	// Invalidate caches
	kv.Delete("channels")
	kv.Delete("groups")

	return result, nil
}

// autoGroupChannel creates groups and group items based on channel autoGroup setting.
func autoGroupChannel(ctx context.Context, db *bun.DB, channel types.Channel, models []string) {
	allGroups, err := dal.ListGroups(ctx, db)
	if err != nil {
		return
	}

	for _, modelName := range models {
		var targetGroupName string

		switch channel.AutoGroup {
		case types.AutoGroupExact:
			targetGroupName = modelName
		case types.AutoGroupFuzzy:
			targetGroupName = fuzzyMatchGroup(modelName, allGroups)
			if targetGroupName == "" {
				targetGroupName = modelName
			}
		default:
			continue
		}

		if targetGroupName == "" {
			continue
		}

		// Find or create the group
		var groupID int
		found := false
		for _, g := range allGroups {
			if g.Name == targetGroupName {
				groupID = g.ID
				found = true
				break
			}
		}

		if !found {
			created, err := dal.CreateGroup(ctx, db, types.Group{
				Name: targetGroupName,
				Mode: types.GroupModeRoundRobin,
			}, nil)
			if err != nil {
				continue
			}
			groupID = created.ID
			allGroups = append(allGroups, *created)
		}

		// Check if group item already exists
		count, _ := db.NewSelect().TableExpr("group_items").
			Where("group_id = ?", groupID).
			Where("channel_id = ?", channel.ID).
			Where("model_name = ?", modelName).
			Count(ctx)

		if count == 0 {
			item := &types.GroupItem{
				GroupID:   groupID,
				ChannelID: channel.ID,
				ModelName: modelName,
				Priority:  0,
				Weight:    1,
			}
			db.NewInsert().Model(item).Exec(ctx)
		}
	}
}

// fuzzyMatchGroup tries to fuzzy match a model name to an existing group.
func fuzzyMatchGroup(modelName string, groups []types.Group) string {
	normalized := strings.ToLower(modelName)

	for _, g := range groups {
		gName := strings.ToLower(g.Name)
		if normalized == gName {
			return g.Name
		}
		if strings.HasPrefix(normalized, gName) {
			return g.Name
		}
		if strings.HasPrefix(gName, normalized) {
			return g.Name
		}
	}

	return ""
}
