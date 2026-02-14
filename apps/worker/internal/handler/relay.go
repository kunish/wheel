package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/cache"
	"github.com/kunish/wheel/apps/worker/internal/db"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/middleware"
	"github.com/kunish/wheel/apps/worker/internal/observe"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/types"
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

// channelSelection holds the result of channel/group matching and ordering.
type channelSelection struct {
	OrderedItems      []types.GroupItem
	ChannelMap        map[int]*types.Channel
	FirstTokenTimeout int
	SessionKeepTime   int
}

// selectChannels loads channels/groups, matches the model, applies balancing and session stickiness.
// Returns nil if an error response was already written to the client.
func (h *RelayHandler) selectChannels(c *gin.Context, model string, apiKeyId int, isAnthropicInbound bool) *channelSelection {
	allChannels := h.loadChannels()
	allGroups := h.loadGroups()

	group := relay.MatchGroup(model, allGroups)
	if group == nil || len(group.Items) == 0 {
		apiError(c, 404, "invalid_request_error",
			fmt.Sprintf("No group matches model '%s'", model),
			isAnthropicInbound,
		)
		return nil
	}

	orderedItems := h.Balancer.SelectChannelOrder(group.Mode, group.Items, group.ID)

	channelMap := make(map[int]*types.Channel, len(allChannels))
	for i := range allChannels {
		channelMap[allChannels[i].ID] = &allChannels[i]
	}

	sessionKeepTime := group.SessionKeepTime
	if sessionKeepTime > 0 {
		sticky := h.Sessions.GetSticky(apiKeyId, model, sessionKeepTime)
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

	return &channelSelection{
		OrderedItems:      orderedItems,
		ChannelMap:        channelMap,
		FirstTokenTimeout: group.FirstTokenTimeOut,
		SessionKeepTime:   sessionKeepTime,
	}
}

// relayRequest holds the parsed relay request data.
type relayRequest struct {
	RequestType        string
	IsAnthropicInbound bool
	Body               map[string]any
	BodyBytes          []byte
	Model              string
	Stream             bool
	ApiKeyID           int
}

// ── Relay Strategy ──────────────────────────────────────────────

// relayAttemptParams holds per-attempt context shared by both strategies.
type relayAttemptParams struct {
	C                      *gin.Context
	Upstream               relay.UpstreamRequest
	Channel                *types.Channel
	SelectedKey            *types.ChannelKey
	TargetModel            string
	RequestModel           string
	Body                   map[string]any
	UpstreamBodyForLog     *string
	IsAnthropicPassthrough bool
	IsAnthropicInbound     bool
	FirstTokenTimeout      int
	ApiKeyID               int
	SessionKeepTime        int
	Attempts               []attemptRecord
	StartTime              time.Time
}

// relayResult is the unified result from either streaming or non-streaming proxy.
type relayResult struct {
	InputTokens         int
	OutputTokens        int
	CacheReadTokens     int
	CacheCreationTokens int
	FirstTokenTime      int
	ResponseContent     string
	ThinkingContent     string
	Response            map[string]any // non-streaming JSON response
	StreamID            string         // streaming: the stream ID
}

// RelayStrategy abstracts the streaming/non-streaming proxy execution.
type RelayStrategy interface {
	// Execute performs the proxy call. Returns a unified result on success.
	Execute(h *RelayHandler, p *relayAttemptParams) (*relayResult, error)
	// HandleSuccess writes the response and triggers async logging.
	HandleSuccess(h *RelayHandler, p *relayAttemptParams, result *relayResult)
	// CleanupOnFailure performs strategy-specific cleanup after a failed attempt.
	CleanupOnFailure(h *RelayHandler, p *relayAttemptParams, streamID string)
}

// ── Stream Strategy ─────────────────────────────────────────────

type streamStrategy struct{}

func (s *streamStrategy) Execute(h *RelayHandler, p *relayAttemptParams) (*relayResult, error) {
	streamId := fmt.Sprintf("%d-%d-%d", time.Now().UnixNano(), p.Channel.ID, p.ApiKeyID)

	h.Observer.StreamStarted(p.C.Request.Context())

	// Estimate input tokens from request body size
	bodyJSON, _ := json.Marshal(p.Body)
	estimatedInputTokens := len(bodyJSON) / 3

	// Lookup model pricing for real-time cost estimation
	var inputPrice, outputPrice float64
	if mp := relay.LookupModelPrice(p.TargetModel, context.Background(), h.DB); mp != nil {
		inputPrice = mp.InputPrice
		outputPrice = mp.OutputPrice
	}

	streamStartPayload := map[string]any{
		"streamId":             streamId,
		"requestModelName":     p.RequestModel,
		"actualModelName":      p.TargetModel,
		"channelId":            p.Channel.ID,
		"channelName":          p.Channel.Name,
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
		p.C.Writer,
		p.C.Request.Context(),
		h.StreamClient,
		p.Upstream.URL,
		p.Upstream.Headers,
		p.Upstream.Body,
		p.Channel.Type,
		p.FirstTokenTimeout,
		p.IsAnthropicPassthrough,
		p.IsAnthropicInbound,
		onContent,
	)

	if proxyErr != nil {
		// Attach streamId for cleanup
		return &relayResult{StreamID: streamId}, proxyErr
	}

	return &relayResult{
		InputTokens:         streamInfo.InputTokens,
		OutputTokens:        streamInfo.OutputTokens,
		CacheReadTokens:     streamInfo.CacheReadTokens,
		CacheCreationTokens: streamInfo.CacheCreationTokens,
		FirstTokenTime:      streamInfo.FirstTokenTime,
		ResponseContent:     streamInfo.ResponseContent,
		ThinkingContent:     streamInfo.ThinkingContent,
		StreamID:            streamId,
	}, nil
}

func (s *streamStrategy) HandleSuccess(h *RelayHandler, p *relayAttemptParams, result *relayResult) {
	go h.asyncRecordLog(
		p.RequestModel, p.TargetModel, p.Channel, p.SelectedKey, p.ApiKeyID,
		p.Body, p.UpstreamBodyForLog, result, p.Attempts, p.StartTime,
	)
	h.Observer.StreamEnded(p.C.Request.Context())
}

func (s *streamStrategy) CleanupOnFailure(h *RelayHandler, p *relayAttemptParams, streamID string) {
	if h.Broadcast != nil {
		h.Broadcast("log-stream-end", map[string]any{"streamId": streamID})
	}
	if h.StreamTracker != nil {
		h.StreamTracker.UntrackStream(streamID)
	}
	h.Observer.StreamEnded(p.C.Request.Context())
}

// ── Non-Stream Strategy ─────────────────────────────────────────

type nonStreamStrategy struct{}

func (s *nonStreamStrategy) Execute(h *RelayHandler, p *relayAttemptParams) (*relayResult, error) {
	result, proxyErr := relay.ProxyNonStreaming(
		h.HTTPClient,
		p.Upstream.URL,
		p.Upstream.Headers,
		p.Upstream.Body,
		p.Channel.Type,
		p.IsAnthropicPassthrough,
	)
	if proxyErr != nil {
		return nil, proxyErr
	}

	respJSON, _ := json.Marshal(result.Response)
	return &relayResult{
		InputTokens:         result.InputTokens,
		OutputTokens:        result.OutputTokens,
		CacheReadTokens:     result.CacheReadTokens,
		CacheCreationTokens: result.CacheCreationTokens,
		Response:            result.Response,
		ResponseContent:     string(respJSON),
	}, nil
}

func (s *nonStreamStrategy) HandleSuccess(h *RelayHandler, p *relayAttemptParams, result *relayResult) {
	go h.asyncRecordLog(
		p.RequestModel, p.TargetModel, p.Channel, p.SelectedKey, p.ApiKeyID,
		p.Body, p.UpstreamBodyForLog, result, p.Attempts, p.StartTime,
	)

	// Write response
	if p.IsAnthropicPassthrough {
		p.C.JSON(200, result.Response)
		return
	}
	if p.IsAnthropicInbound {
		p.C.JSON(200, relay.ConvertToAnthropicResponse(result.Response))
		return
	}
	p.C.JSON(200, result.Response)
}

func (s *nonStreamStrategy) CleanupOnFailure(h *RelayHandler, p *relayAttemptParams, streamID string) {
	// No cleanup needed for non-streaming
}

// parseRelayRequest reads the request body, extracts model/stream, and checks access.
// Returns nil if an error response was already written to the client.
func (h *RelayHandler) parseRelayRequest(c *gin.Context) *relayRequest {
	path := c.Request.URL.Path

	requestType := relay.DetectRequestType(path)
	isAnthropicInbound := requestType == "anthropic-messages"
	if requestType == "" {
		c.JSON(400, gin.H{"error": gin.H{"message": "Unsupported endpoint", "type": "invalid_request_error"}})
		return nil
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(c.Request.Body, 10*1024*1024))
	if err != nil {
		apiError(c, 400, "invalid_request_error", "Failed to read request body", isAnthropicInbound)
		return nil
	}

	var body map[string]any
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		apiError(c, 400, "invalid_request_error", "Invalid JSON body", isAnthropicInbound)
		return nil
	}

	model, stream := relay.ExtractModel(body)
	if model == "" {
		apiError(c, 400, "invalid_request_error", "Model is required", isAnthropicInbound)
		return nil
	}

	supportedModels, _ := c.Get("supportedModels")
	sm, _ := supportedModels.(string)
	if !middleware.CheckModelAccess(sm, model) {
		apiError(c, 403, "invalid_request_error",
			fmt.Sprintf("Model '%s' not allowed for this API key", model),
			isAnthropicInbound,
		)
		return nil
	}

	apiKeyIdRaw, _ := c.Get("apiKeyId")
	apiKeyId, _ := apiKeyIdRaw.(int)

	return &relayRequest{
		RequestType:        requestType,
		IsAnthropicInbound: isAnthropicInbound,
		Body:               body,
		BodyBytes:          bodyBytes,
		Model:              model,
		Stream:             stream,
		ApiKeyID:           apiKeyId,
	}
}

// RelayHandler holds dependencies for the relay routes.
type RelayHandler struct {
	Handler
	Broadcast       types.BroadcastFunc
	StreamTracker   types.StreamTracker
	LogWriter       *db.LogWriter
	Observer        *observe.Observer
	CircuitBreakers *relay.CircuitBreakerManager
	Sessions        *relay.SessionManager
	Balancer        *relay.BalancerState
	HTTPClient      *http.Client // non-streaming (120s timeout)
	StreamClient    *http.Client // streaming (30s connect timeout, no overall timeout)
}

// RegisterRelayRoutes registers the /v1/* relay routes on a Gin engine.
func (h *RelayHandler) RegisterRelayRoutes(r *gin.Engine) {
	v1 := r.Group("/v1")
	v1.Use(middleware.ApiKeyAuth(h.DB))
	v1.GET("/models", h.handleModels)
	v1.POST("/*path", h.handleRelay)
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

// retryOutcome holds the result of executeWithRetry for exhaustion handling.
type retryOutcome struct {
	Success          bool
	RateLimited      bool
	LastError        string
	LastRetryAfterMs int64
	LastStreamID     string
	Attempts         []attemptRecord
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
	model := req.Model
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

				go h.CircuitBreakers.RecordFailure(channel.ID, selectedKey.ID, targetModel, context.Background(), h.DB)

				if pe, ok := proxyErr.(*relay.ProxyError); ok && pe.StatusCode == 429 {
					lastRetryAfterMs = int64(math.Max(float64(lastRetryAfterMs), float64(pe.RetryAfterMs)))
					if lastRetryAfterMs == 0 {
						lastRetryAfterMs = 1000
					}
					rateLimited = true
					go dal.UpdateChannelKeyStatus(context.Background(), h.DB, selectedKey.ID, 429)
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
				go dal.UpdateChannelKeyStatus(context.Background(), h.DB, selectedKey.ID, 0)
			}

			// Update attempts in params for logging
			params.Attempts = attempts
			strategy.HandleSuccess(h, params, result)

			return &retryOutcome{Success: true, Attempts: attempts, LastStreamID: result.StreamID}
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

func (h *RelayHandler) handleRelay(c *gin.Context) {
	startTime := time.Now()

	// Start relay span (no-op if tracing disabled)
	ctx, relaySpan := h.Observer.StartRelaySpan(c.Request.Context(), "", "", 0)
	defer relaySpan.End()
	c.Request = c.Request.WithContext(ctx)

	// Parse and validate request
	req := h.parseRelayRequest(c)
	if req == nil {
		return
	}

	// Select channels and groups
	sel := h.selectChannels(c, req.Model, req.ApiKeyID, req.IsAnthropicInbound)
	if sel == nil {
		return
	}

	// Choose strategy
	var strategy RelayStrategy
	if req.Stream {
		strategy = &streamStrategy{}
	} else {
		strategy = &nonStreamStrategy{}
	}

	// Execute with retry
	outcome := h.executeWithRetry(c, req, sel, strategy, startTime)
	if outcome.Success {
		return
	}

	// All retries exhausted — record error and respond
	h.handleExhaustion(c, req, outcome, startTime)
}

// handleExhaustion handles the case where all retry rounds are exhausted.
func (h *RelayHandler) handleExhaustion(c *gin.Context, req *relayRequest, outcome *retryOutcome, startTime time.Time) {
	model := req.Model
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

// ── Async Logging ───────────────────────────────────────────────

func (h *RelayHandler) asyncRecordLog(
	model, targetModel string,
	channel *types.Channel,
	key *types.ChannelKey,
	apiKeyId int,
	body map[string]any,
	upstreamBodyForLog *string,
	result *relayResult,
	attempts []attemptRecord,
	startTime time.Time,
) {
	cost := relay.CalculateCost(targetModel, result.InputTokens, result.OutputTokens, context.Background(), h.DB,
		&relay.CacheTokens{
			CacheReadTokens:     result.CacheReadTokens,
			CacheCreationTokens: result.CacheCreationTokens,
		})

	logBody := serializeForLog(body)

	respContent := result.ResponseContent
	if result.ThinkingContent != "" {
		respContent = "<|thinking|>" + result.ThinkingContent + "<|/thinking|>" + respContent
	}
	if respContent == "" && result.StreamID != "" {
		respContent = "[streaming]"
	}

	attemptsJSON, _ := json.Marshal(attempts)
	var attemptsVal types.AttemptList
	json.Unmarshal(attemptsJSON, &attemptsVal)

	var costInfo *db.CostInfo
	if cost > 0 {
		costInfo = &db.CostInfo{
			ApiKeyID:     apiKeyId,
			ChannelKeyID: key.ID,
			Cost:         cost,
		}
	}

	h.LogWriter.Submit(types.RelayLog{
		Time:             time.Now().Unix(),
		RequestModelName: model,
		ChannelID:        channel.ID,
		ChannelName:      channel.Name,
		ActualModelName:  targetModel,
		InputTokens:      result.InputTokens,
		OutputTokens:     result.OutputTokens,
		FTUT:             result.FirstTokenTime,
		UseTime:          int(time.Since(startTime).Milliseconds()),
		Cost:             cost,
		RequestContent:   logBody,
		UpstreamContent:  upstreamBodyForLog,
		ResponseContent:  respContent,
		Error:            "",
		Attempts:         attemptsVal,
		TotalAttempts:    len(attempts),
	}, costInfo, result.StreamID)

	// Record observability metrics
	ctx := context.Background()
	h.Observer.RecordRequest(ctx, channel.Name, targetModel, "", 200)
	h.Observer.RecordDuration(ctx, channel.Name, targetModel, time.Since(startTime))
	h.Observer.RecordTokens(ctx, channel.Name, targetModel, "input", result.InputTokens)
	h.Observer.RecordTokens(ctx, channel.Name, targetModel, "output", result.OutputTokens)
	if cost > 0 {
		h.Observer.RecordCost(ctx, channel.Name, targetModel, cost)
	}
	if result.FirstTokenTime > 0 {
		h.Observer.RecordTTFB(ctx, channel.Name, targetModel, result.FirstTokenTime)
	}
}

func (h *RelayHandler) asyncErrorLog(
	model string,
	channelID int,
	channelName string,
	body map[string]any,
	lastError string,
	attempts []attemptRecord,
	startTime time.Time,
	streamID string,
) {
	logBody := serializeForLog(body)

	attemptsJSON, _ := json.Marshal(attempts)
	var attemptsVal types.AttemptList
	json.Unmarshal(attemptsJSON, &attemptsVal)

	h.LogWriter.Submit(types.RelayLog{
		Time:             time.Now().Unix(),
		RequestModelName: model,
		ChannelID:        channelID,
		ChannelName:      channelName,
		ActualModelName:  model,
		UseTime:          int(time.Since(startTime).Milliseconds()),
		RequestContent:   logBody,
		ResponseContent:  lastError,
		Error:            lastError,
		Attempts:         attemptsVal,
		TotalAttempts:    len(attempts),
	}, nil, streamID)
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
