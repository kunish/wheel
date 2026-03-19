package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"net/http"

	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

func (h *Handler) collectCodexChannelModels(ctx context.Context, channelID int, channelType types.OutboundType, files []codexAuthFile) ([]string, error) {
	if err := h.ensureCodexManagementConfigured(); err != nil {
		return nil, err
	}
	providerFilter := runtimeProviderFilter(channelType)
	localMode := h.codexCapabilities().LocalEnabled
	var retryUntil time.Time
	if localMode {
		retryUntil = time.Now().Add(codexModelSyncRetryWindow)
	}
	models := make([]string, 0)
	seen := make(map[string]struct{})
	var firstErr error
	successCount := 0
	for _, file := range files {
		if file.Disabled || canonicalRuntimeProvider(file.Provider) != providerFilter {
			continue
		}
		query := url.Values{"name": []string{managedAuthRelativeName(channelID, file.Name)}}
		out, err := h.listCodexAuthFileModelsWithRetry(ctx, query, retryUntil)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		successCount++
		for _, model := range out.Models {
			id := strings.TrimSpace(model.ID)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			models = append(models, id)
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return models, nil
}

func (h *Handler) collectCodexChannelModelsBestEffort(ctx context.Context, channelID int, channelType types.OutboundType, files []codexAuthFile) ([]string, error) {
	if err := h.ensureCodexManagementConfigured(); err != nil {
		return nil, err
	}
	providerFilter := runtimeProviderFilter(channelType)
	localMode := h.codexCapabilities().LocalEnabled
	var retryUntil time.Time
	if localMode {
		retryUntil = time.Now().Add(codexModelSyncRetryWindow)
	}
	models := make([]string, 0)
	seen := make(map[string]struct{})
	var firstErr error
	hadSuccess := false
	for _, file := range files {
		if file.Disabled || canonicalRuntimeProvider(file.Provider) != providerFilter {
			continue
		}
		query := url.Values{"name": []string{managedAuthRelativeName(channelID, file.Name)}}
		out, err := h.listCodexAuthFileModelsWithRetry(ctx, query, retryUntil)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		hadSuccess = true
		for _, model := range out.Models {
			id := strings.TrimSpace(model.ID)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			models = append(models, id)
		}
	}
	if !hadSuccess && firstErr != nil {
		return nil, firstErr
	}
	return models, nil
}

func (h *Handler) listCodexAuthFileModelsWithRetry(ctx context.Context, query url.Values, retryUntil time.Time) (struct {
	Models []struct {
		ID string `json:"id"`
	} `json:"models"`
}, error) {
	var out struct {
		Models []struct {
			ID string `json:"id"`
		} `json:"models"`
	}

	for {
		err := h.codexManagementCallContext(ctx, http.MethodGet, "/auth-files/models", query, nil, &out)
		shouldRetry := !retryUntil.IsZero() && time.Now().Before(retryUntil)
		if err == nil && (len(out.Models) > 0 || !shouldRetry) {
			return out, nil
		}
		if err != nil && !shouldRetry {
			return out, err
		}
		if !shouldRetry {
			return out, nil
		}

		timer := time.NewTimer(codexModelSyncRetryDelay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return out, ctx.Err()
		case <-timer.C:
		}
	}

	return out, nil
}

func (h *Handler) persistCodexChannelModels(ctx context.Context, channelID int, models []string) error {
	encoded, err := json.Marshal(models)
	if err != nil {
		return fmt.Errorf("marshal codex channel models: %w", err)
	}
	return dal.UpdateChannel(ctx, h.DB, channelID, map[string]any{
		"model":         string(encoded),
		"fetched_model": string(encoded),
	})
}

func (h *Handler) syncCodexChannelModels(ctx context.Context, channelID int) error {
	if h == nil || h.DB == nil {
		return nil
	}
	// Look up channel type for provider filtering
	channel, err := dal.GetChannel(ctx, h.DB, channelID)
	if err != nil {
		return err
	}
	if channel == nil {
		return fmt.Errorf("channel %d not found", channelID)
	}
	files, err := h.listManagedCodexAuthFiles(ctx, channelID)
	if err != nil {
		return err
	}
	hasOwnedAuth := false
	for _, file := range files {
		if file.Disabled {
			continue
		}
		if runtimeProviderMatches(channel.Type, file.Provider) {
			hasOwnedAuth = true
			break
		}
	}
	models, err := h.collectCodexChannelModels(ctx, channelID, channel.Type, files)
	if err != nil {
		// If model collection fails (e.g. 403 TOS violation), treat as empty
		// so the fallback logic below can provide default models.
		models = nil
	}
	// Do not overwrite existing models with an empty list; the upstream
	// model endpoint may be temporarily unavailable (e.g. TOS violation).
	// For Antigravity channels, use a built-in fallback list when the
	// API cannot return models.
	if len(models) == 0 {
		if !hasOwnedAuth {
			if err := h.persistCodexChannelModels(ctx, channelID, []string{}); err != nil {
				return err
			}
			h.Cache.Delete("channels")
			return nil
		}
		if channel.Type == types.OutboundAntigravity {
			models = defaultAntigravityModels()
		} else {
			return nil
		}
	}
	if err := h.persistCodexChannelModels(ctx, channelID, models); err != nil {
		return err
	}
	h.Cache.Delete("channels")
	return nil
}

// defaultAntigravityModels returns the built-in model list for Antigravity
// channels when the upstream model API is unavailable.
func defaultAntigravityModels() []string {
	return []string{
		"gemini-2.5-pro",
		"gemini-2.5-flash",
		"claude-sonnet-4-6-thinking",
		"claude-opus-4-6-thinking",
	}
}

func (h *Handler) bestEffortSyncCodexChannelModels(ctx context.Context, channelID int) {
	if err := h.syncCodexChannelModels(ctx, channelID); err != nil {
		log.Printf("[codex] sync channel %d models failed: %v", channelID, err)
	}
}
