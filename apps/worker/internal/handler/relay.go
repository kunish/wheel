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
	maxRetryRounds    = 3
	maxMessageContent = 500
	maxLogJSON        = 10000
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

// RelayHandler holds dependencies for the relay routes.
type RelayHandler struct {
	DB        *bun.DB
	Cache     *cache.MemoryKV
	Broadcast BroadcastFunc
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
	if requestType == "" {
		c.JSON(400, gin.H{"error": gin.H{"message": "Unsupported endpoint", "type": "invalid_request_error"}})
		return
	}

	// Read and parse body
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{"message": "Failed to read request body", "type": "invalid_request_error"}})
		return
	}

	var body map[string]any
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		c.JSON(400, gin.H{"error": gin.H{"message": "Invalid JSON body", "type": "invalid_request_error"}})
		return
	}

	model, stream := relay.ExtractModel(body)
	if model == "" {
		c.JSON(400, gin.H{"error": gin.H{"message": "Model is required", "type": "invalid_request_error"}})
		return
	}

	// Check model access
	supportedModels, _ := c.Get("supportedModels")
	sm, _ := supportedModels.(string)
	if !checkModelAccess(sm, model) {
		c.JSON(403, gin.H{"error": gin.H{
			"message": fmt.Sprintf("Model '%s' not allowed for this API key", model),
			"type":    "invalid_request_error",
		}})
		return
	}

	isAnthropicInbound := requestType == "anthropic-messages"

	// Load channels and groups
	allChannels := h.loadChannels()
	allGroups := h.loadGroups()

	// Match group
	group := relay.MatchGroup(model, allGroups)
	if group == nil || len(group.Items) == 0 {
		c.JSON(404, gin.H{"error": gin.H{
			"message": fmt.Sprintf("No group matches model '%s'", model),
			"type":    "invalid_request_error",
		}})
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
				if len(s) > maxLogJSON {
					s = s[:maxLogJSON]
				}
				upstreamBodyForLog = &s
			}

			if stream {
				// ── Streaming path ──
				// We need to directly write to the response writer for SSE
				streamInfo, proxyErr := relay.ProxyStreaming(
					c.Writer,
					upstream.URL,
					upstream.Headers,
					upstream.Body,
					channel.Type,
					firstTokenTimeout,
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

	c.JSON(exhaustedStatus, gin.H{
		"error": gin.H{
			"message": fmt.Sprintf("All channels exhausted after %d rounds. Last error: %s", maxRetryRounds, lastError),
			"type":    errType,
		},
	})
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
) {
	if streamInfo == nil {
		return
	}

	cost := relay.CalculateCost(targetModel, streamInfo.InputTokens, streamInfo.OutputTokens, context.Background(), h.DB,
		&relay.CacheTokens{
			CacheReadTokens:     streamInfo.CacheReadTokens,
			CacheCreationTokens: streamInfo.CacheCreationTokens,
		})

	logBody := truncateForLog(body)
	respContent := streamInfo.ResponseContent
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

	logRow, err := dal.CreateLog(context.Background(), h.DB, types.RelayLog{
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
		h.Broadcast("log-created", logSummary(logRow))
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

	logBody := truncateForLog(body)
	respJSON, _ := json.Marshal(result.Response)
	respContent := string(respJSON)
	if len(respContent) > maxLogJSON {
		respContent = respContent[:maxLogJSON]
	}

	attemptsJSON, _ := json.Marshal(attempts)
	var attemptsVal types.AttemptList
	json.Unmarshal(attemptsJSON, &attemptsVal)

	logRow, err := dal.CreateLog(context.Background(), h.DB, types.RelayLog{
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
	logBody := truncateForLog(body)

	attemptsJSON, _ := json.Marshal(attempts)
	var attemptsVal types.AttemptList
	json.Unmarshal(attemptsJSON, &attemptsVal)

	logRow, err := dal.CreateLog(context.Background(), h.DB, types.RelayLog{
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
	retentionDays := 30
	if days != nil {
		if n, err := strconv.Atoi(*days); err == nil && n > 0 {
			retentionDays = n
		}
	}
	dal.CleanupOldLogs(context.Background(), h.DB, retentionDays)
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

// ── Log Truncation ──────────────────────────────────────────────

func truncateMessage(msg map[string]any) map[string]any {
	m := make(map[string]any, len(msg))
	for k, v := range msg {
		m[k] = v
	}

	if content, ok := m["content"].(string); ok && len(content) > maxMessageContent {
		m["content"] = fmt.Sprintf("%s... [truncated, %d chars total]", content[:maxMessageContent], len(content))
	} else if contentArr, ok := m["content"].([]any); ok {
		truncated := make([]any, 0, len(contentArr))
		for _, part := range contentArr {
			p, ok := part.(map[string]any)
			if !ok {
				truncated = append(truncated, part)
				continue
			}
			cp := make(map[string]any, len(p))
			for k, v := range p {
				cp[k] = v
			}
			pType, _ := cp["type"].(string)
			if pType == "image_url" || pType == "image" {
				cp = map[string]any{"type": pType, "_omitted": "[image data omitted]"}
			} else if text, ok := cp["text"].(string); ok && len(text) > maxMessageContent {
				cp["text"] = text[:maxMessageContent] + "... [truncated]"
			}
			truncated = append(truncated, cp)
		}
		m["content"] = truncated
	}
	return m
}

func truncateForLog(body map[string]any) string {
	clone := make(map[string]any, len(body))
	for k, v := range body {
		clone[k] = v
	}

	if messages, ok := clone["messages"].([]any); ok {
		truncatedMsgs := make([]any, 0, len(messages))
		for _, m := range messages {
			if msg, ok := m.(map[string]any); ok {
				truncatedMsgs = append(truncatedMsgs, truncateMessage(msg))
			} else {
				truncatedMsgs = append(truncatedMsgs, m)
			}
		}
		clone["messages"] = truncatedMsgs

		j, _ := json.Marshal(clone)
		if len(j) > maxLogJSON && len(messages) > 2 {
			keep := len(truncatedMsgs)
			if keep > 4 {
				keep = 4
			}
			if keep < 2 {
				keep = 2
			}
			dropped := len(truncatedMsgs) - keep
			newMsgs := []any{truncatedMsgs[0]}
			newMsgs = append(newMsgs, map[string]any{
				"role":    "system",
				"content": fmt.Sprintf("[%d messages omitted for storage]", dropped),
			})
			newMsgs = append(newMsgs, truncatedMsgs[len(truncatedMsgs)-keep+1:]...)
			clone["messages"] = newMsgs
		}

		result, _ := json.Marshal(clone)
		return string(result)
	}

	result, _ := json.Marshal(clone)
	s := string(result)
	if len(s) > maxLogJSON {
		s = s[:maxLogJSON]
	}
	return s
}
