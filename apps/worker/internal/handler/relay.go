package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/cache"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/types"
	"github.com/uptrace/bun"
)

const (
	maxRetryRounds = 3
)

// attemptRecord tracks a single relay attempt for logging.
type attemptRecord struct {
	ChannelID    int    `json:"channelId"`
	ChannelKeyID *int   `json:"channelKeyId,omitempty"`
	ChannelName  string `json:"channelName"`
	ModelName    string `json:"modelName"`
	AttemptNum   int    `json:"attemptNum"`
	Status       string `json:"status"`
	Duration     int    `json:"duration"`
	Sticky       *bool  `json:"sticky,omitempty"`
	Msg          string `json:"msg,omitempty"`
}

// BroadcastFunc is the signature for WebSocket broadcast.
type BroadcastFunc func(event string, data ...any)

// StreamTracker tracks active streams so new WS clients get a snapshot.
type StreamTracker interface {
	TrackStream(streamId string, data map[string]any)
	UntrackStream(streamId string)
}

// RelayHandler holds dependencies for the relay routes.
type RelayHandler struct {
	DB            *bun.DB
	LogDB         *bun.DB
	Cache         *cache.MemoryKV
	Broadcast     BroadcastFunc
	StreamTracker StreamTracker
}

// RegisterRelayRoutes registers the /v1/* relay routes on a Gin engine.
func (h *RelayHandler) RegisterRelayRoutes(r *gin.Engine) {
	v1 := r.Group("/v1")
	v1.Use(h.apiKeyAuthMiddleware())
	v1.GET("/models", h.handleModels)
	v1.POST("/*path", h.handleRelay)
}

// ── API Key Auth Middleware ──────────────────────────────────────

func (h *RelayHandler) apiKeyAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract key from multiple header formats
		key := c.GetHeader("x-api-key")
		if key == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer sk-wheel-") {
				key = authHeader[7:]
			}
		}

		if key == "" || !strings.HasPrefix(key, "sk-wheel-") {
			c.JSON(401, gin.H{"success": false, "error": "Unauthorized: invalid API key"})
			c.Abort()
			return
		}

		apiKey, err := dal.GetApiKeyByKey(c.Request.Context(), h.DB, key)
		if err != nil || apiKey == nil {
			c.JSON(401, gin.H{"success": false, "error": "Unauthorized: API key not found"})
			c.Abort()
			return
		}

		if !apiKey.Enabled {
			c.JSON(401, gin.H{"success": false, "error": "Unauthorized: API key disabled"})
			c.Abort()
			return
		}

		// Check expiry
		if apiKey.ExpireAt > 0 && apiKey.ExpireAt < time.Now().Unix() {
			c.JSON(403, gin.H{"success": false, "error": "Forbidden: API key expired"})
			c.Abort()
			return
		}

		// Check cost limit
		if apiKey.MaxCost > 0 && apiKey.TotalCost >= apiKey.MaxCost {
			c.JSON(403, gin.H{"success": false, "error": "Forbidden: cost limit exceeded"})
			c.Abort()
			return
		}

		c.Set("apiKeyId", apiKey.ID)
		c.Set("supportedModels", apiKey.SupportedModels)
		c.Next()
	}
}

func checkModelAccess(supportedModels, model string) bool {
	if supportedModels == "" {
		return true
	}
	for _, m := range strings.Split(supportedModels, ",") {
		if strings.TrimSpace(m) == model {
			return true
		}
	}
	return false
}

// ── GET /v1/models ──────────────────────────────────────────────

func (h *RelayHandler) handleModels(c *gin.Context) {
	allGroups := h.loadGroups()

	// Collect unique model names
	modelSet := make(map[string]bool)
	for _, g := range allGroups {
		modelSet[g.Name] = true
	}

	// Filter by API Key whitelist
	supportedModels, _ := c.Get("supportedModels")
	sm, _ := supportedModels.(string)

	var models []string
	for m := range modelSet {
		models = append(models, m)
	}

	if sm != "" {
		allowed := make(map[string]bool)
		for _, a := range strings.Split(sm, ",") {
			allowed[strings.TrimSpace(a)] = true
		}
		var filtered []string
		for _, m := range models {
			if allowed[m] {
				filtered = append(filtered, m)
			}
		}
		models = filtered
	}

	// Detect format: Anthropic if anthropic-version header or x-api-key without Authorization
	isAnthropic := c.GetHeader("anthropic-version") != "" ||
		(c.GetHeader("x-api-key") != "" && c.GetHeader("Authorization") == "")

	if isAnthropic {
		data := make([]map[string]any, 0, len(models))
		for _, id := range models {
			data = append(data, map[string]any{
				"id":           id,
				"created_at":   time.Now().UTC().Format(time.RFC3339),
				"display_name": id,
				"type":         "model",
			})
		}
		c.JSON(200, gin.H{"data": data, "has_more": false})
		return
	}

	now := time.Now().Unix()
	data := make([]map[string]any, 0, len(models))
	for _, id := range models {
		data = append(data, map[string]any{
			"id":       id,
			"object":   "model",
			"created":  now,
			"owned_by": "wheel",
		})
	}
	c.JSON(200, gin.H{"object": "list", "data": data})
}

// ── POST /v1/* Relay Handler ────────────────────────────────────

func (h *RelayHandler) handleRelay(c *gin.Context) {
	startTime := time.Now()
	path := c.Request.URL.Path

	// Parse request type
	requestType := relay.DetectRequestType(path)
	isAnthropicInbound := requestType == "anthropic-messages"
	if requestType == "" {
		c.JSON(400, gin.H{"error": gin.H{"message": "Unsupported endpoint", "type": "invalid_request_error"}})
		return
	}

	// Read and parse body
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		apiError(c, 400, "invalid_request_error", "Failed to read request body", isAnthropicInbound)
		return
	}

	var body map[string]any
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		apiError(c, 400, "invalid_request_error", "Invalid JSON body", isAnthropicInbound)
		return
	}

	model, stream := relay.ExtractModel(body)
	if model == "" {
		apiError(c, 400, "invalid_request_error", "Model is required", isAnthropicInbound)
		return
	}

	// Check model access
	supportedModels, _ := c.Get("supportedModels")
	sm, _ := supportedModels.(string)
	if !checkModelAccess(sm, model) {
		apiError(c, 403, "invalid_request_error",
			fmt.Sprintf("Model '%s' not allowed for this API key", model),
			isAnthropicInbound,
		)
		return
	}

	// Load channels and groups
	allChannels := h.loadChannels()
	allGroups := h.loadGroups()

	// Match group
	group := relay.MatchGroup(model, allGroups)
	if group == nil || len(group.Items) == 0 {
		apiError(c, 404, "invalid_request_error",
			fmt.Sprintf("No group matches model '%s'", model),
			isAnthropicInbound,
		)
		return
	}

	// Select channel order
	orderedItems := relay.SelectChannelOrder(group.Mode, group.Items, group.ID)

	// Build channel lookup map
	channelMap := make(map[int]*types.Channel, len(allChannels))
	for i := range allChannels {
		channelMap[allChannels[i].ID] = &allChannels[i]
	}

	// Attempt tracking
	var attempts []attemptRecord
	attemptCount := 0

	var lastError string
	var lastRetryAfterMs int64
	rateLimited := false

	firstTokenTimeout := group.FirstTokenTimeOut
	sessionKeepTime := group.SessionKeepTime
	apiKeyIdRaw, _ := c.Get("apiKeyId")
	apiKeyId, _ := apiKeyIdRaw.(int)

	// Session stickiness reordering
	if sessionKeepTime > 0 {
		sticky := relay.GetSticky(apiKeyId, model, sessionKeepTime)
		if sticky != nil {
			for i, it := range orderedItems {
				if it.ChannelID == sticky.ChannelID && i > 0 {
					stickyItem := orderedItems[i]
					orderedItems = append(orderedItems[:i], orderedItems[i+1:]...)
					orderedItems = append([]types.GroupItem{stickyItem}, orderedItems...)
					break
				}
			}
		}
	}

	// Circuit breaker config
	cbBaseSec, cbMaxSec := relay.GetCooldownConfig(c.Request.Context(), h.DB)

	// Track the last streamId for cleanup on exhaustion
	var lastStreamId string

	// Retry loop
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

			// Select key
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
			isSticky := sessionKeepTime > 0 && idx == 0 && relay.GetSticky(apiKeyId, model, sessionKeepTime) != nil
			stickyPtr := &isSticky

			// Check circuit breaker
			tripped, remainingMs := relay.IsTripped(channel.ID, selectedKey.ID, targetModel, cbBaseSec, cbMaxSec)
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
				continue
			}

			attemptStart := time.Now()
			attemptCount++
			currentAttemptNum := attemptCount

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
				path,
				targetModel,
				isAnthropicPassthrough,
			)

			// Capture upstream body for logging
			originalBody, _ := json.Marshal(body)
			var upstreamBodyForLog *string
			if upstream.Body != string(originalBody) {
				s := upstream.Body
				upstreamBodyForLog = &s
			}

			if stream {
				// ── Streaming path ──
				// We need to directly write to the response writer for SSE
				streamId := fmt.Sprintf("%d-%d-%d", time.Now().UnixNano(), channel.ID, apiKeyId)
				lastStreamId = streamId

				// Estimate input tokens from request body size
				bodyJSON, _ := json.Marshal(body)
				estimatedInputTokens := len(bodyJSON) / 3

				// Lookup model pricing for real-time cost estimation
				var inputPrice, outputPrice float64
				if mp := relay.LookupModelPrice(targetModel, context.Background(), h.DB); mp != nil {
					inputPrice = mp.InputPrice
					outputPrice = mp.OutputPrice
				}

				streamStartPayload := map[string]any{
					"streamId":             streamId,
					"requestModelName":     model,
					"actualModelName":      targetModel,
					"channelId":            channel.ID,
					"channelName":          channel.Name,
					"time":                 time.Now().Unix(),
					"estimatedInputTokens": estimatedInputTokens,
					"inputPrice":           inputPrice,
					"outputPrice":          outputPrice,
				}
				if h.Broadcast != nil {
					h.Broadcast("log-stream-start", streamStartPayload)
				}
				if h.StreamTracker != nil {
					h.StreamTracker.TrackStream(streamId, streamStartPayload)
				}

				var onContent relay.StreamContentCallback
				if h.Broadcast != nil {
					onContent = func(thinking, response string) {
						h.Broadcast("log-streaming", map[string]any{
							"streamId":        streamId,
							"thinkingContent": thinking,
							"responseContent": response,
							"thinkingLength":  len(thinking),
							"responseLength":  len(response),
						})
					}
				}
				streamInfo, proxyErr := relay.ProxyStreaming(
					c.Writer,
					c.Request.Context(),
					upstream.URL,
					upstream.Headers,
					upstream.Body,
					channel.Type,
					firstTokenTimeout,
					isAnthropicPassthrough,
					isAnthropicInbound,
					onContent,
				)

				if proxyErr != nil {
					keyId := selectedKey.ID
					errMsg := proxyErr.Error()
					attempts = append(attempts, attemptRecord{
						ChannelID:    channel.ID,
						ChannelKeyID: &keyId,
						ChannelName:  channel.Name,
						ModelName:    targetModel,
						AttemptNum:   currentAttemptNum,
						Status:       "failed",
						Duration:     int(time.Since(attemptStart).Milliseconds()),
						Sticky:       stickyPtr,
						Msg:          errMsg,
					})
					lastError = errMsg

					// Record circuit breaker failure async
					go relay.RecordFailure(channel.ID, selectedKey.ID, targetModel, context.Background(), h.DB)

					if pe, ok := proxyErr.(*relay.ProxyError); ok && pe.StatusCode == 429 {
						lastRetryAfterMs = int64(math.Max(float64(lastRetryAfterMs), float64(pe.RetryAfterMs)))
						if lastRetryAfterMs == 0 {
							lastRetryAfterMs = 1000
						}
						rateLimited = true
						go dal.UpdateChannelKeyStatus(context.Background(), h.DB, selectedKey.ID, 429)
					}
					if h.Broadcast != nil {
						h.Broadcast("log-stream-end", map[string]any{"streamId": streamId})
					}
					if h.StreamTracker != nil {
						h.StreamTracker.UntrackStream(streamId)
					}
					continue
				}

				// Stream succeeded — record success
				keyId := selectedKey.ID
				attempts = append(attempts, attemptRecord{
					ChannelID:    channel.ID,
					ChannelKeyID: &keyId,
					ChannelName:  channel.Name,
					ModelName:    targetModel,
					AttemptNum:   currentAttemptNum,
					Status:       "success",
					Duration:     int(time.Since(attemptStart).Milliseconds()),
					Sticky:       stickyPtr,
				})

				relay.RecordSuccess(channel.ID, selectedKey.ID, targetModel)
				if sessionKeepTime > 0 {
					relay.SetSticky(apiKeyId, model, channel.ID, selectedKey.ID)
				}

				if selectedKey.StatusCode == 429 {
					go dal.UpdateChannelKeyStatus(context.Background(), h.DB, selectedKey.ID, 0)
				}

				// Async logging
				go h.asyncStreamLog(
					model, targetModel, channel, selectedKey, apiKeyId,
					body, upstreamBodyForLog, streamInfo, attempts, startTime,
					streamId,
				)
				return // Response already written by ProxyStreaming

			} else {
				// ── Non-streaming path ──
				result, proxyErr := relay.ProxyNonStreaming(
					upstream.URL,
					upstream.Headers,
					upstream.Body,
					channel.Type,
					isAnthropicPassthrough,
				)

				if proxyErr != nil {
					keyId := selectedKey.ID
					errMsg := proxyErr.Error()
					attempts = append(attempts, attemptRecord{
						ChannelID:    channel.ID,
						ChannelKeyID: &keyId,
						ChannelName:  channel.Name,
						ModelName:    targetModel,
						AttemptNum:   currentAttemptNum,
						Status:       "failed",
						Duration:     int(time.Since(attemptStart).Milliseconds()),
						Sticky:       stickyPtr,
						Msg:          errMsg,
					})
					lastError = errMsg

					go relay.RecordFailure(channel.ID, selectedKey.ID, targetModel, context.Background(), h.DB)

					if pe, ok := proxyErr.(*relay.ProxyError); ok && pe.StatusCode == 429 {
						lastRetryAfterMs = int64(math.Max(float64(lastRetryAfterMs), float64(pe.RetryAfterMs)))
						if lastRetryAfterMs == 0 {
							lastRetryAfterMs = 1000
						}
						rateLimited = true
						go dal.UpdateChannelKeyStatus(context.Background(), h.DB, selectedKey.ID, 429)
					}
					continue
				}

				// Non-stream succeeded
				keyId := selectedKey.ID
				attempts = append(attempts, attemptRecord{
					ChannelID:    channel.ID,
					ChannelKeyID: &keyId,
					ChannelName:  channel.Name,
					ModelName:    targetModel,
					AttemptNum:   currentAttemptNum,
					Status:       "success",
					Duration:     int(time.Since(attemptStart).Milliseconds()),
					Sticky:       stickyPtr,
				})

				relay.RecordSuccess(channel.ID, selectedKey.ID, targetModel)
				if sessionKeepTime > 0 {
					relay.SetSticky(apiKeyId, model, channel.ID, selectedKey.ID)
				}

				if selectedKey.StatusCode == 429 {
					go dal.UpdateChannelKeyStatus(context.Background(), h.DB, selectedKey.ID, 0)
				}

				// Async logging
				go h.asyncNonStreamLog(
					model, targetModel, channel, selectedKey, apiKeyId,
					body, upstreamBodyForLog, result, attempts, startTime,
				)

				// Return response
				if isAnthropicPassthrough {
					c.JSON(200, result.Response)
					return
				}
				if isAnthropicInbound {
					c.JSON(200, relay.ConvertToAnthropicResponse(result.Response))
					return
				}
				c.JSON(200, result.Response)
				return
			}
		}
	}

	// All retries exhausted
	exhaustedStatus := 502
	if rateLimited {
		exhaustedStatus = 429
	}
	retryAfterSecs := 0
	if rateLimited {
		retryAfterSecs = int(math.Ceil(float64(lastRetryAfterMs) / 1000))
		if retryAfterSecs == 0 {
			retryAfterSecs = 1
		}
	}

	// Find last failed attempt for logging
	var lastAttemptChannelID int
	var lastAttemptChannelName string
	for i := len(attempts) - 1; i >= 0; i-- {
		if attempts[i].Status == "failed" {
			lastAttemptChannelID = attempts[i].ChannelID
			lastAttemptChannelName = attempts[i].ChannelName
			break
		}
	}

	if stream && lastStreamId != "" {
		if h.Broadcast != nil {
			h.Broadcast("log-stream-end", map[string]any{"streamId": lastStreamId})
		}
		if h.StreamTracker != nil {
			h.StreamTracker.UntrackStream(lastStreamId)
		}
	}

	// Async error log
	go h.asyncErrorLog(
		model, lastAttemptChannelID, lastAttemptChannelName,
		body, lastError, attempts, startTime,
	)

	if retryAfterSecs > 0 {
		c.Header("Retry-After", strconv.Itoa(retryAfterSecs))
	}

	errType := "server_error"
	if rateLimited {
		errType = "rate_limit_error"
	}

	apiError(c, exhaustedStatus, errType,
		fmt.Sprintf("All channels exhausted after %d rounds. Last error: %s", maxRetryRounds, lastError),
		isAnthropicInbound,
	)
}

// ── Async Logging ───────────────────────────────────────────────

func (h *RelayHandler) asyncStreamLog(
	model, targetModel string,
	channel *types.Channel,
	key *types.ChannelKey,
	apiKeyId int,
	body map[string]any,
	upstreamBodyForLog *string,
	streamInfo *relay.StreamCompleteInfo,
	attempts []attemptRecord,
	startTime time.Time,
	streamId string,
) {
	if streamInfo == nil {
		return
	}

	cost := relay.CalculateCost(targetModel, streamInfo.InputTokens, streamInfo.OutputTokens, context.Background(), h.DB,
		&relay.CacheTokens{
			CacheReadTokens:     streamInfo.CacheReadTokens,
			CacheCreationTokens: streamInfo.CacheCreationTokens,
		})

	logBody := serializeForLog(body)
	respContent := streamInfo.ResponseContent
	if streamInfo.ThinkingContent != "" {
		respContent = "<|thinking|>" + streamInfo.ThinkingContent + "<|/thinking|>" + respContent
	}
	if respContent == "" {
		respContent = "[streaming]"
	}

	attemptsJSON, _ := json.Marshal(attempts)
	var attemptsVal types.AttemptList
	json.Unmarshal(attemptsJSON, &attemptsVal)

	var upstreamContent *string
	if upstreamBodyForLog != nil {
		upstreamContent = upstreamBodyForLog
	}

	logRow, err := dal.CreateLog(context.Background(), h.LogDB, types.RelayLog{
		Time:             time.Now().Unix(),
		RequestModelName: model,
		ChannelID:        channel.ID,
		ChannelName:      channel.Name,
		ActualModelName:  targetModel,
		InputTokens:      streamInfo.InputTokens,
		OutputTokens:     streamInfo.OutputTokens,
		FTUT:             streamInfo.FirstTokenTime,
		UseTime:          int(time.Since(startTime).Milliseconds()),
		Cost:             cost,
		RequestContent:   logBody,
		UpstreamContent:  upstreamContent,
		ResponseContent:  respContent,
		Error:            "",
		Attempts:         attemptsVal,
		TotalAttempts:    len(attempts),
	})
	if err != nil {
		return
	}

	if cost > 0 {
		dal.IncrementApiKeyCost(context.Background(), h.DB, apiKeyId, cost)
		dal.IncrementChannelKeyCost(context.Background(), h.DB, key.ID, cost)
	}

	if h.Broadcast != nil {
		h.Broadcast("stats-updated")
		summary := logSummary(logRow)
		summary["streamId"] = streamId
		h.Broadcast("log-created", summary)
	}
	if h.StreamTracker != nil {
		h.StreamTracker.UntrackStream(streamId)
	}

	h.maybeCleanupLogs()
}

func (h *RelayHandler) asyncNonStreamLog(
	model, targetModel string,
	channel *types.Channel,
	key *types.ChannelKey,
	apiKeyId int,
	body map[string]any,
	upstreamBodyForLog *string,
	result *relay.ProxyResult,
	attempts []attemptRecord,
	startTime time.Time,
) {
	cost := relay.CalculateCost(targetModel, result.InputTokens, result.OutputTokens, context.Background(), h.DB,
		&relay.CacheTokens{
			CacheReadTokens:     result.CacheReadTokens,
			CacheCreationTokens: result.CacheCreationTokens,
		})

	logBody := serializeForLog(body)
	respJSON, _ := json.Marshal(result.Response)
	respContent := string(respJSON)

	attemptsJSON, _ := json.Marshal(attempts)
	var attemptsVal types.AttemptList
	json.Unmarshal(attemptsJSON, &attemptsVal)

	logRow, err := dal.CreateLog(context.Background(), h.LogDB, types.RelayLog{
		Time:             time.Now().Unix(),
		RequestModelName: model,
		ChannelID:        channel.ID,
		ChannelName:      channel.Name,
		ActualModelName:  targetModel,
		InputTokens:      result.InputTokens,
		OutputTokens:     result.OutputTokens,
		FTUT:             0,
		UseTime:          int(time.Since(startTime).Milliseconds()),
		Cost:             cost,
		RequestContent:   logBody,
		UpstreamContent:  upstreamBodyForLog,
		ResponseContent:  respContent,
		Error:            "",
		Attempts:         attemptsVal,
		TotalAttempts:    len(attempts),
	})
	if err != nil {
		return
	}

	if cost > 0 {
		dal.IncrementApiKeyCost(context.Background(), h.DB, apiKeyId, cost)
		dal.IncrementChannelKeyCost(context.Background(), h.DB, key.ID, cost)
	}

	if h.Broadcast != nil {
		h.Broadcast("stats-updated")
		h.Broadcast("log-created", logSummary(logRow))
	}

	h.maybeCleanupLogs()
}

func (h *RelayHandler) asyncErrorLog(
	model string,
	channelID int,
	channelName string,
	body map[string]any,
	lastError string,
	attempts []attemptRecord,
	startTime time.Time,
) {
	logBody := serializeForLog(body)

	attemptsJSON, _ := json.Marshal(attempts)
	var attemptsVal types.AttemptList
	json.Unmarshal(attemptsJSON, &attemptsVal)

	logRow, err := dal.CreateLog(context.Background(), h.LogDB, types.RelayLog{
		Time:             time.Now().Unix(),
		RequestModelName: model,
		ChannelID:        channelID,
		ChannelName:      channelName,
		ActualModelName:  model,
		UseTime:          int(time.Since(startTime).Milliseconds()),
		RequestContent:   logBody,
		Error:            lastError,
		Attempts:         attemptsVal,
		TotalAttempts:    len(attempts),
	})
	if err != nil {
		return
	}

	if h.Broadcast != nil {
		h.Broadcast("stats-updated")
		h.Broadcast("log-created", logSummary(logRow))
	}

	h.maybeCleanupLogs()
}

func logSummary(log *types.RelayLog) map[string]any {
	return map[string]any{
		"log": map[string]any{
			"id":               log.ID,
			"time":             log.Time,
			"requestModelName": log.RequestModelName,
			"actualModelName":  log.ActualModelName,
			"channelId":        log.ChannelID,
			"channelName":      log.ChannelName,
			"inputTokens":      log.InputTokens,
			"outputTokens":     log.OutputTokens,
			"ftut":             log.FTUT,
			"useTime":          log.UseTime,
			"error":            log.Error,
			"cost":             log.Cost,
			"totalAttempts":    log.TotalAttempts,
		},
	}
}

// maybeCleanupLogs probabilistically cleans up old logs (1% chance per request).
func (h *RelayHandler) maybeCleanupLogs() {
	if rand.Float64() > 0.01 {
		return
	}
	days, err := dal.GetSetting(context.Background(), h.DB, "log_retention_days")
	if err != nil {
		return
	}
	retentionDays := 365
	if days != nil {
		if n, err := strconv.Atoi(*days); err == nil && n > 0 {
			retentionDays = n
		}
	}
	dal.CleanupOldLogs(context.Background(), h.LogDB, retentionDays)
}

// ── Cache Helpers ───────────────────────────────────────────────

func (h *RelayHandler) loadChannels() []types.Channel {
	channels, ok := cache.Get[[]types.Channel](h.Cache, "channels")
	if ok && channels != nil {
		return *channels
	}

	ch, err := dal.ListChannels(context.Background(), h.DB)
	if err != nil {
		return nil
	}
	h.Cache.Put("channels", ch, 300)
	return ch
}

func (h *RelayHandler) loadGroups() []types.Group {
	groups, ok := cache.Get[[]types.Group](h.Cache, "groups")
	if ok && groups != nil {
		return *groups
	}

	g, err := dal.ListGroups(context.Background(), h.DB)
	if err != nil {
		return nil
	}
	h.Cache.Put("groups", g, 300)
	return g
}

// ── Log Serialization ──────────────────────────────────────────────

func serializeForLog(body map[string]any) string {
	result, _ := json.Marshal(body)
	return string(result)
}

// apiError returns an error response in the correct format based on the request type.
// Anthropic format: {"type":"error","error":{"type":"...","message":"..."}}
// OpenAI format:    {"error":{"message":"...","type":"..."}}
func apiError(c *gin.Context, status int, errType, message string, isAnthropicInbound bool) {
	if isAnthropicInbound {
		c.JSON(status, gin.H{
			"type": "error",
			"error": gin.H{
				"type":    errType,
				"message": message,
			},
		})
	} else {
		c.JSON(status, gin.H{
			"error": gin.H{
				"message": message,
				"type":    errType,
			},
		})
	}
}
