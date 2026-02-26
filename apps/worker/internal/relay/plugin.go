package relay

import (
	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// ── Plugin Context ──────────────────────────────────────────────

// RelayContext carries request-scoped data through the plugin chain.
type RelayContext struct {
	GinCtx             *gin.Context
	RequestModel       string
	TargetModel        string
	Body               map[string]any
	BodyBytes          []byte
	ApiKeyID           int
	IsStream           bool
	IsAnthropicInbound bool
	RequestType        string
	Channel            *types.Channel
	SelectedKey        *types.ChannelKey
	Group              *types.Group

	// Values is a free-form store for inter-plugin communication.
	Values map[string]any
}

// Set stores a value in the context.
func (c *RelayContext) Set(key string, val any) {
	if c.Values == nil {
		c.Values = make(map[string]any)
	}
	c.Values[key] = val
}

// Get retrieves a value from the context.
func (c *RelayContext) Get(key string) (any, bool) {
	if c.Values == nil {
		return nil, false
	}
	v, ok := c.Values[key]
	return v, ok
}

// ── Plugin Response ─────────────────────────────────────────────

// RelayPluginResponse wraps the result of a relay call for PostHook.
type RelayPluginResponse struct {
	Success             bool
	StatusCode          int
	Body                map[string]any
	InputTokens         int
	OutputTokens        int
	CacheReadTokens     int
	CacheCreationTokens int
	Cost                float64
	Error               error
}

// ── Short-Circuit ───────────────────────────────────────────────

// ShortCircuit signals that a PreHook wants to skip the provider call
// and return a response directly.
type ShortCircuit struct {
	StatusCode int
	Body       map[string]any
}

// ── Plugin Interface ────────────────────────────────────────────

// RelayPlugin is the interface for relay middleware plugins.
// PreHook runs before the provider call (in registration order).
// PostHook runs after the provider call (in REVERSE order — onion model).
type RelayPlugin interface {
	// Name returns the plugin's identifier.
	Name() string

	// PreHook runs before the upstream call.
	// Return a *ShortCircuit to skip the provider and all subsequent PreHooks.
	// Return nil to continue the chain.
	PreHook(ctx *RelayContext) *ShortCircuit

	// PostHook runs after the upstream call (or after short-circuit).
	// It can modify the response or recover from errors.
	// Only PostHooks for PreHooks that already executed will run (symmetry).
	PostHook(ctx *RelayContext, resp *RelayPluginResponse)
}
