package handler

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// retryOutcome holds the result of executeWithRetry for exhaustion handling.
type retryOutcome struct {
	Success          bool
	RateLimited      bool
	LastError        string
	LastRetryAfterMs int64
	LastStreamID     string
	Attempts         []attemptRecord
	Result           *relayResult // populated on success, carries token counts and content
}

// executeWithRetry runs the retry loop across ordered channels using the given strategy.
func (h *RelayHandler) executeWithRetry(
	c *gin.Context,
	req *relayRequest,
	sel *channelSelection,
	strategy RelayStrategy,
	startTime time.Time,
) *retryOutcome {
	body := req.Body
	model := req.OriginalModel // use original model for logs/metrics
	if model == "" {
		model = req.Model
	}
	isAnthropicInbound := req.IsAnthropicInbound
	apiKeyId := req.ApiKeyID

	orderedItems := sel.OrderedItems
	channelMap := sel.ChannelMap
	firstTokenTimeout := sel.FirstTokenTimeout
	sessionKeepTime := sel.SessionKeepTime

	var attempts []attemptRecord
	attemptCount := 0
	var lastError string
	var lastRetryAfterMs int64
	rateLimited := false

	cbBaseSec, cbMaxSec := h.CircuitBreakers.GetCooldownConfig(c.Request.Context(), h.DB)

	var lastStreamId string

	for round := 1; round <= maxRetryRounds; round++ {
		for idx, item := range orderedItems {
			channel := channelMap[item.ChannelID]
			if channel == nil || !channel.Enabled {
				attemptCount++
				msg := "channel not found"
				if channel != nil {
					msg = "channel disabled"
				}
				chName := "unknown"
				if channel != nil {
					chName = channel.Name
				}
				attempts = append(attempts, attemptRecord{
					ChannelID:   item.ChannelID,
					ChannelName: chName,
					ModelName:   item.ModelName,
					AttemptNum:  attemptCount,
					Status:      "skipped",
					Msg:         msg,
				})
				continue
			}

			// Health check: skip channels marked DOWN
			if h.HealthChecker != nil && !h.HealthChecker.IsHealthy(channel.ID) {
				attemptCount++
				attempts = append(attempts, attemptRecord{
					ChannelID:   channel.ID,
					ChannelName: channel.Name,
					ModelName:   item.ModelName,
					AttemptNum:  attemptCount,
					Status:      "skipped",
					Msg:         "channel unhealthy",
				})
				continue
			}

			selectedKey := relay.SelectKey(channel.Keys)
			if selectedKey == nil {
				attemptCount++
				attempts = append(attempts, attemptRecord{
					ChannelID:   channel.ID,
					ChannelName: channel.Name,
					ModelName:   item.ModelName,
					AttemptNum:  attemptCount,
					Status:      "skipped",
					Msg:         "no available key",
				})
				continue
			}

			targetModel := item.ModelName
			if targetModel == "" {
				targetModel = model
			}
			isSticky := sessionKeepTime > 0 && idx == 0 && h.Sessions.GetSticky(apiKeyId, model, sessionKeepTime) != nil
			stickyPtr := &isSticky

			// Check circuit breaker
			tripped, remainingMs := h.CircuitBreakers.IsTripped(channel.ID, selectedKey.ID, targetModel, cbBaseSec, cbMaxSec)
			if tripped {
				attemptCount++
				msg := "circuit breaker tripped"
				if remainingMs > 0 {
					msg = fmt.Sprintf("circuit breaker tripped, remaining cooldown: %ds", int(math.Ceil(float64(remainingMs)/1000)))
				}
				keyId := selectedKey.ID
				attempts = append(attempts, attemptRecord{
					ChannelID:    channel.ID,
					ChannelKeyID: &keyId,
					ChannelName:  channel.Name,
					ModelName:    targetModel,
					AttemptNum:   attemptCount,
					Status:       "circuit_break",
					Sticky:       stickyPtr,
					Msg:          msg,
				})
				lastError = msg
				h.Observer.AddCircuitBreakerEvent(c.Request.Context(), channel.Name, channel.ID)
				continue
			}

			attemptStart := time.Now()
			attemptCount++
			currentAttemptNum := attemptCount

			_, attemptSpan := h.Observer.StartAttemptSpan(c.Request.Context(), currentAttemptNum, channel.Name, channel.ID)

			// Build upstream request
			isAnthropicPassthrough := isAnthropicInbound && channel.Type == types.OutboundAnthropic
			upstream := relay.BuildUpstreamRequest(
				relay.ChannelConfig{
					Type:          channel.Type,
					BaseUrls:      []types.BaseUrl(channel.BaseUrls),
					CustomHeader:  []types.CustomHeader(channel.CustomHeader),
					ParamOverride: channel.ParamOverride,
				},
				selectedKey.ChannelKey,
				body,
				c.Request.URL.Path,
				targetModel,
				isAnthropicPassthrough,
			)

			originalBody, _ := json.Marshal(body)
			var upstreamBodyForLog *string
			if upstream.Body != string(originalBody) {
				s := upstream.Body
				upstreamBodyForLog = &s
			}

			params := &relayAttemptParams{
				C:                      c,
				Upstream:               upstream,
				Channel:                channel,
				SelectedKey:            selectedKey,
				TargetModel:            targetModel,
				RequestModel:           model,
				Body:                   body,
				UpstreamBodyForLog:     upstreamBodyForLog,
				IsAnthropicPassthrough: isAnthropicPassthrough,
				IsAnthropicInbound:     isAnthropicInbound,
				FirstTokenTimeout:      firstTokenTimeout,
				ApiKeyID:               apiKeyId,
				SessionKeepTime:        sessionKeepTime,
				Attempts:               attempts,
				StartTime:              startTime,
			}

			result, proxyErr := strategy.Execute(h, params)

			if proxyErr != nil {
				keyId := selectedKey.ID
				errMsg := proxyErr.Error()
				attemptDuration := int(time.Since(attemptStart).Milliseconds())
				attempts = append(attempts, attemptRecord{
					ChannelID:    channel.ID,
					ChannelKeyID: &keyId,
					ChannelName:  channel.Name,
					ModelName:    targetModel,
					AttemptNum:   currentAttemptNum,
					Status:       "failed",
					Duration:     attemptDuration,
					Sticky:       stickyPtr,
					Msg:          errMsg,
				})
				lastError = errMsg
				h.Observer.EndAttemptSpan(attemptSpan, 0, attemptDuration, proxyErr)
				h.Observer.RecordRetry(c.Request.Context(), channel.Name, targetModel)

				h.CircuitBreakers.RecordFailure(channel.ID, selectedKey.ID, targetModel, c.Request.Context(), h.DB)

				if pe, ok := proxyErr.(*relay.ProxyError); ok && pe.StatusCode == 429 {
					lastRetryAfterMs = int64(math.Max(float64(lastRetryAfterMs), float64(pe.RetryAfterMs)))
					if lastRetryAfterMs == 0 {
						lastRetryAfterMs = 1000
					}
					rateLimited = true
					_ = dal.UpdateChannelKeyStatus(c.Request.Context(), h.DB, selectedKey.ID, 429)
					h.Cache.Delete("channels")
				}

				streamID := ""
				if result != nil {
					streamID = result.StreamID
					lastStreamId = streamID
				}
				strategy.CleanupOnFailure(h, params, streamID)
				continue
			}

			// Success
			keyId := selectedKey.ID
			attemptDuration := int(time.Since(attemptStart).Milliseconds())
			attempts = append(attempts, attemptRecord{
				ChannelID:    channel.ID,
				ChannelKeyID: &keyId,
				ChannelName:  channel.Name,
				ModelName:    targetModel,
				AttemptNum:   currentAttemptNum,
				Status:       "success",
				Duration:     attemptDuration,
				Sticky:       stickyPtr,
			})
			h.Observer.EndAttemptSpan(attemptSpan, 200, attemptDuration, nil)

			h.CircuitBreakers.RecordSuccess(channel.ID, selectedKey.ID, targetModel)
			if sessionKeepTime > 0 {
				h.Sessions.SetSticky(apiKeyId, model, channel.ID, selectedKey.ID)
			}

			if selectedKey.StatusCode == 429 {
				_ = dal.UpdateChannelKeyStatus(c.Request.Context(), h.DB, selectedKey.ID, 0)
				h.Cache.Delete("channels")
			}

			// Update attempts in params for logging
			params.Attempts = attempts
			strategy.HandleSuccess(h, params, result)

			return &retryOutcome{Success: true, Attempts: attempts, LastStreamID: result.StreamID, Result: result}
		}
	}

	return &retryOutcome{
		Success:          false,
		RateLimited:      rateLimited,
		LastError:        lastError,
		LastRetryAfterMs: lastRetryAfterMs,
		LastStreamID:     lastStreamId,
		Attempts:         attempts,
	}
}

// handleExhaustion handles the case where all retry rounds are exhausted.
func (h *RelayHandler) handleExhaustion(c *gin.Context, req *relayRequest, outcome *retryOutcome, startTime time.Time) {
	model := req.OriginalModel // use original model for logs/metrics
	if model == "" {
		model = req.Model
	}
	isAnthropicInbound := req.IsAnthropicInbound

	exhaustedStatus := 502
	if outcome.RateLimited {
		exhaustedStatus = 429
	}
	retryAfterSecs := 0
	if outcome.RateLimited {
		retryAfterSecs = int(math.Ceil(float64(outcome.LastRetryAfterMs) / 1000))
		if retryAfterSecs == 0 {
			retryAfterSecs = 1
		}
	}

	// Find last failed attempt for logging
	var lastAttemptChannelID int
	var lastAttemptChannelName string
	for i := len(outcome.Attempts) - 1; i >= 0; i-- {
		if outcome.Attempts[i].Status == "failed" {
			lastAttemptChannelID = outcome.Attempts[i].ChannelID
			lastAttemptChannelName = outcome.Attempts[i].ChannelName
			break
		}
	}

	if req.Stream && outcome.LastStreamID != "" {
		if h.Broadcast != nil {
			h.Broadcast("log-stream-end", map[string]any{"streamId": outcome.LastStreamID})
		}
		if h.StreamTracker != nil {
			h.StreamTracker.UntrackStream(outcome.LastStreamID)
		}
	}

	// Record exhaustion metrics
	obsErrType := "exhausted"
	if outcome.RateLimited {
		obsErrType = "rate_limited"
	}
	h.Observer.RecordRequest(c.Request.Context(), lastAttemptChannelName, model, "", exhaustedStatus)
	h.Observer.RecordError(c.Request.Context(), lastAttemptChannelName, model, obsErrType)
	h.Observer.RecordDuration(c.Request.Context(), lastAttemptChannelName, model, time.Since(startTime))

	// Async error log
	go h.asyncErrorLog(
		model, lastAttemptChannelID, lastAttemptChannelName,
		req.Body, outcome.LastError, outcome.Attempts, startTime, outcome.LastStreamID,
	)

	if retryAfterSecs > 0 {
		c.Header("Retry-After", strconv.Itoa(retryAfterSecs))
	}

	errType := "server_error"
	if outcome.RateLimited {
		errType = "rate_limit_error"
	}

	apiError(c, exhaustedStatus, errType,
		fmt.Sprintf("All channels exhausted after %d rounds. Last error: %s", maxRetryRounds, outcome.LastError),
		isAnthropicInbound,
	)
}
