package handler

import (
	"context"
	"fmt"

	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

func (h *RelayHandler) executeFeatureWithRetry(
	ctx context.Context,
	model string,
	apiKeyID int,
	exec func(channel *types.Channel, selectedKey *types.ChannelKey, targetModel string) error,
) error {
	allChannels := h.loadChannels(ctx)
	allGroups := h.loadGroups(ctx)

	group := relay.MatchGroup(model, allGroups)
	if group == nil || len(group.Items) == 0 {
		return fmt.Errorf("no group matches model %q", model)
	}

	orderedItems := h.Balancer.SelectChannelOrder(group.Mode, group.Items, group.ID)
	channelMap := make(map[int]*types.Channel, len(allChannels))
	for i := range allChannels {
		channelMap[allChannels[i].ID] = &allChannels[i]
	}

	cbBaseSec, cbMaxSec := h.CircuitBreakers.GetCooldownConfig(ctx, h.DB)
	var lastErr error

	for round := 1; round <= maxRetryRounds; round++ {
		_ = round
		for idx, item := range orderedItems {
			channel := channelMap[item.ChannelID]
			if channel == nil || !channel.Enabled {
				continue
			}
			if h.HealthChecker != nil && !h.HealthChecker.IsHealthy(channel.ID) {
				continue
			}

			selectedKey := relay.SelectKey(channel.Keys)
			if selectedKey == nil {
				continue
			}

			targetModel := item.ModelName
			if targetModel == "" {
				targetModel = model
			}

			tripped, _ := h.CircuitBreakers.IsTripped(channel.ID, selectedKey.ID, targetModel, cbBaseSec, cbMaxSec)
			if tripped {
				continue
			}

			if err := exec(channel, selectedKey, targetModel); err != nil {
				pe, ok := err.(*relay.ProxyError)
				if !ok {
					return err
				}
				if !relay.IsRetryableStatusCode(pe.StatusCode) {
					return err
				}

				lastErr = err
				h.CircuitBreakers.RecordFailure(channel.ID, selectedKey.ID, targetModel, ctx, h.DB)

				if pe.StatusCode == 429 {
					if h.DB != nil {
						_ = dal.UpdateChannelKeyStatus(ctx, h.DB, selectedKey.ID, 429)
					}
					if h.Cache != nil {
						h.Cache.Delete("channels")
					}
				}
				continue
			}

			h.CircuitBreakers.RecordSuccess(channel.ID, selectedKey.ID, targetModel)
			if selectedKey.StatusCode == 429 {
				if h.DB != nil {
					_ = dal.UpdateChannelKeyStatus(ctx, h.DB, selectedKey.ID, 0)
				}
				if h.Cache != nil {
					h.Cache.Delete("channels")
				}
			}
			if group.SessionKeepTime > 0 && idx == 0 {
				h.Sessions.SetSticky(apiKeyID, model, channel.ID, selectedKey.ID)
			}
			return nil
		}
	}

	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("all channels exhausted for model %q", model)
}
