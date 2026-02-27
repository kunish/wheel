package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/cache"
	"github.com/kunish/wheel/apps/worker/internal/db"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	mcpgw "github.com/kunish/wheel/apps/worker/internal/mcp"
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
	OriginalModel      string // preserved for logs/metrics after routing rules modify Model
	Stream             bool
	ApiKeyID           int
}

// parseRelayRequest reads the request body, extracts model/stream, and checks access.
// Returns nil if an error response was already written to the client.
func (h *RelayHandler) parseRelayRequest(c *gin.Context) *relayRequest {
	path := c.Request.URL.Path

	requestType := relay.DetectRequestType(path)
	isAnthropicInbound := requestType == relay.RequestTypeAnthropicMsg
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

	// For multimodal endpoints, fall back to default model if not specified
	if model == "" && relay.IsMultimodalRequest(requestType) {
		model = relay.ExtractMultimodalModel(body, requestType)
	}

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

	// ── New: Plugin pipeline, routing engine, health checker ──
	Plugins       *relay.PluginPipeline
	RoutingEngine *relay.RoutingEngine
	HealthChecker *relay.HealthChecker

	// ── MCP Gateway ──
	MCPManager *mcpgw.Manager
	MCPServer  *mcpgw.Server
}

// RegisterRelayRoutes registers the /v1/* relay routes on a Gin engine.
func (h *RelayHandler) RegisterRelayRoutes(r *gin.Engine) {
	v1 := r.Group("/v1")
	v1.Use(middleware.ApiKeyAuth(h.DB))
	v1.GET("/models", h.handleModels)
	v1.POST("/*path", h.handleRelay)

	// MCP Server endpoint (API Key auth)
	if h.MCPServer != nil {
		mcpHandler := h.MCPServer.ServeHTTP()
		mcp := r.Group("/mcp")
		mcp.Use(middleware.ApiKeyAuth(h.DB))
		mcp.Any("/*path", gin.WrapH(mcpHandler))
	}
}

// RegisterRelayAdminRoutes registers admin-level routes that need RelayHandler
// (routing rules CRUD, channel health). Must be called after Handler.RegisterRoutes.
func (h *RelayHandler) RegisterRelayAdminRoutes(r *gin.Engine) {
	admin := r.Group("/api/v1")
	admin.Use(middleware.JWTAuth(h.Config.JWTSecret))

	// Routing rules
	admin.GET("/routing-rule/list", h.ListRoutingRules)
	admin.POST("/routing-rule/create", h.CreateRoutingRule)
	admin.POST("/routing-rule/update", h.UpdateRoutingRule)
	admin.DELETE("/routing-rule/delete/:id", h.DeleteRoutingRule)

	// Channel health
	admin.GET("/channel/health", h.GetChannelHealth)

	// MCP clients
	admin.GET("/mcp/client/list", h.ListMCPClients)
	admin.POST("/mcp/client/create", h.CreateMCPClient)
	admin.POST("/mcp/client/update", h.UpdateMCPClient)
	admin.DELETE("/mcp/client/delete/:id", h.DeleteMCPClient)
	admin.POST("/mcp/client/reconnect/:id", h.ReconnectMCPClient)
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
	// Dispatch MCP tool execute before relay processing
	if c.Param("path") == "/mcp/tool/execute" {
		h.ExecuteMCPTool(c)
		return
	}

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

	// ── Plugin PreHooks ─────────────────────────────────────
	var preResult relay.PreHookResult
	var pluginCtx *relay.RelayContext
	if h.Plugins != nil {
		pluginCtx = &relay.RelayContext{
			GinCtx:             c,
			RequestModel:       req.Model,
			Body:               req.Body,
			BodyBytes:          req.BodyBytes,
			ApiKeyID:           req.ApiKeyID,
			IsStream:           req.Stream,
			IsAnthropicInbound: req.IsAnthropicInbound,
			RequestType:        req.RequestType,
		}
		preResult = h.Plugins.RunPreHooks(pluginCtx)
		if preResult.ShortCircuit != nil {
			sc := preResult.ShortCircuit
			c.JSON(sc.StatusCode, sc.Body)
			// Run PostHooks for symmetry
			h.Plugins.RunPostHooks(pluginCtx, &relay.RelayPluginResponse{
				StatusCode: sc.StatusCode,
				Error:      fmt.Errorf("short-circuited by plugin"),
			}, preResult.ExecutedCount)
			return
		}
	}

	// ── Routing Rules ───────────────────────────────────────
	originalModel := req.Model // preserve for logs/metrics
	if h.RoutingEngine != nil {
		headers := make(map[string]string)
		for k := range c.Request.Header {
			headers[strings.ToLower(k)] = c.Request.Header.Get(k)
		}
		ruleResult := h.RoutingEngine.Evaluate(&relay.RuleEvalContext{
			Model:       originalModel,
			Headers:     headers,
			ApiKeyID:    req.ApiKeyID,
			ApiKeyName:  c.GetString("apiKeyName"),
			RequestType: req.RequestType,
			Body:        req.Body,
		})
		if ruleResult.Matched {
			switch ruleResult.Action.Type {
			case "reject":
				status := ruleResult.Action.StatusCode
				if status == 0 {
					status = 403
				}
				msg := ruleResult.Action.Message
				if msg == "" {
					msg = fmt.Sprintf("Blocked by routing rule: %s", ruleResult.RuleName)
				}
				apiError(c, status, "routing_rule_error", msg, req.IsAnthropicInbound)
				return
			case "route":
				// Route to a different group; req.Model becomes the group name for selectChannels.
				// The actual upstream model is determined by the group's channel ModelName.
				if ruleResult.Action.GroupName != "" {
					req.Model = ruleResult.Action.GroupName
				}
			case "rewrite":
				// Rewrite the model name sent to upstream provider
				if ruleResult.Action.ModelName != "" {
					req.Model = ruleResult.Action.ModelName
					// Sync body so upstream sees the rewritten model
					if req.Body != nil {
						req.Body["model"] = ruleResult.Action.ModelName
					}
				}
			}
		}
	}

	// Preserve original model for logs/metrics (req.Model may have been changed by routing rules)
	req.OriginalModel = originalModel

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

	// ── Plugin PostHooks ────────────────────────────────────
	if h.Plugins != nil && pluginCtx != nil {
		resp := &relay.RelayPluginResponse{Success: outcome.Success}
		if !outcome.Success {
			resp.Error = fmt.Errorf("%s", outcome.LastError)
		}
		h.Plugins.RunPostHooks(pluginCtx, resp, preResult.ExecutedCount)
	}

	if outcome.Success {
		return
	}

	// All retries exhausted — record error and respond
	h.handleExhaustion(c, req, outcome, startTime)
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

	// Read active_profile_id from settings to filter groups
	profileID := 0
	if val, _ := dal.GetSetting(context.Background(), h.DB, "active_profile_id"); val != nil {
		if id, err := strconv.Atoi(*val); err == nil {
			profileID = id
		}
	}

	g, err := dal.ListGroups(context.Background(), h.DB, profileID)
	if err != nil {
		return nil
	}
	h.Cache.Put("groups", g, 300)
	return g
}
