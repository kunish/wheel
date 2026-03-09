package handler

import (
	"context"
	"fmt"

	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

func (h *RelayHandler) executeBackgroundNonStream(
	requestPath string,
	requestBody map[string]any,
	requestModel string,
	apiKeyID int,
) (*relay.ProxyResult, error) {
	requestType := relay.DetectRequestType(requestPath)
	if relay.IsDeferredExecutionUnsupported(requestType) {
		return nil, fmt.Errorf("background execution does not support audio endpoint %s", requestPath)
	}

	allChannels := h.loadChannels(context.Background())
	allGroups := h.loadGroups(context.Background())

	group := relay.MatchGroup(requestModel, allGroups)
	if group == nil || len(group.Items) == 0 {
		return nil, fmt.Errorf("no group matches model %q", requestModel)
	}

	orderedItems := h.Balancer.SelectChannelOrder(group.Mode, group.Items, group.ID)
	channelMap := make(map[int]*types.Channel, len(allChannels))
	for i := range allChannels {
		channelMap[allChannels[i].ID] = &allChannels[i]
	}

	cbBaseSec, cbMaxSec := h.CircuitBreakers.GetCooldownConfig(context.Background(), h.DB)
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
				targetModel = requestModel
			}
			targetModel = normalizeRuntimeTargetModel(channel.Type, targetModel)

			tripped, _ := h.CircuitBreakers.IsTripped(channel.ID, selectedKey.ID, targetModel, cbBaseSec, cbMaxSec)
			if tripped {
				continue
			}

			channelConfig := relay.ChannelConfig{
				Type:          channel.Type,
				BaseUrls:      []types.BaseUrl(channel.BaseUrls),
				CustomHeader:  []types.CustomHeader(channel.CustomHeader),
				ParamOverride: channel.ParamOverride,
			}
			upstream := relay.BuildUpstreamRequest(
				channelConfig,
				selectedKey.ChannelKey,
				requestBody,
				requestPath,
				targetModel,
				false,
			)

			var result *relay.ProxyResult
			var err error
			if relay.ShouldUseMultimodalExecution(requestType, channel.Type) {
				upstream = relay.BuildMultimodalUpstreamRequest(
					channelConfig,
					selectedKey.ChannelKey,
					requestBody,
					targetModel,
					requestType,
				)
				var proxyErr *relay.ProxyError
				result, proxyErr = relay.ProxyMultimodal(
					h.HTTPClient,
					upstream.URL,
					upstream.Headers,
					upstream.Body,
					requestType,
				)
				if proxyErr != nil {
					err = proxyErr
				}
			} else {
				result, err = relay.ProxyNonStreaming(
					h.HTTPClient,
					upstream.URL,
					upstream.Headers,
					upstream.Body,
					channel.Type,
					false,
				)
			}
			if err != nil {
				lastErr = err
				h.CircuitBreakers.RecordFailure(channel.ID, selectedKey.ID, targetModel, context.Background(), h.DB)

				if pe, ok := err.(*relay.ProxyError); ok && pe.StatusCode == 429 {
					if h.DB != nil {
						_ = dal.UpdateChannelKeyStatus(context.Background(), h.DB, selectedKey.ID, 429)
					}
					if h.Cache != nil {
						h.Cache.Delete("channels")
					}
				}
				continue
			}

			h.CircuitBreakers.RecordSuccess(channel.ID, selectedKey.ID, targetModel)
			if group.SessionKeepTime > 0 && idx == 0 {
				h.Sessions.SetSticky(apiKeyID, requestModel, channel.ID, selectedKey.ID)
			}
			if selectedKey.StatusCode == 429 {
				if h.DB != nil {
					_ = dal.UpdateChannelKeyStatus(context.Background(), h.DB, selectedKey.ID, 0)
				}
				if h.Cache != nil {
					h.Cache.Delete("channels")
				}
			}
			return result, nil
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("all channels exhausted after %d rounds", maxRetryRounds)
}
