package relay

import (
	"strings"
)

// LiteLLMCompatPlugin translates LiteLLM-style request conventions
// into Wheel's native format for drop-in compatibility with LiteLLM clients.
//
// LiteLLM conventions supported:
//   - model format: "provider/model" (e.g., "openai/gpt-4o", "anthropic/claude-3-5-sonnet")
//   - metadata.user_api_key: forwarded as x-api-key if present
//   - fallbacks: array of "provider/model" strings for failover
//   - num_retries: retry count hint
type LiteLLMCompatPlugin struct{}

func NewLiteLLMCompatPlugin() *LiteLLMCompatPlugin {
	return &LiteLLMCompatPlugin{}
}

func (p *LiteLLMCompatPlugin) Name() string { return "litellm_compat" }

func (p *LiteLLMCompatPlugin) PreHook(ctx *RelayContext) *ShortCircuit {
	if ctx.Body == nil {
		return nil
	}

	// Handle "provider/model" format → strip provider prefix, keep model
	if model, ok := ctx.Body["model"].(string); ok {
		if parts := strings.SplitN(model, "/", 2); len(parts) == 2 {
			ctx.Set("litellm_provider", parts[0])
			stripped := parts[1]
			ctx.Body["model"] = stripped
			ctx.RequestModel = stripped
		}
	}

	// Extract LiteLLM metadata
	if metadata, ok := ctx.Body["metadata"].(map[string]any); ok {
		if traceID, ok := metadata["trace_id"].(string); ok {
			ctx.Set("litellm_trace_id", traceID)
		}
		if tags, ok := metadata["tags"].([]any); ok {
			ctx.Set("litellm_tags", tags)
		}
		delete(ctx.Body, "metadata")
	}

	// Extract fallbacks and store for the relay handler
	if fallbacks, ok := ctx.Body["fallbacks"].([]any); ok && len(fallbacks) > 0 {
		var models []string
		for _, fb := range fallbacks {
			if s, ok := fb.(string); ok {
				if parts := strings.SplitN(s, "/", 2); len(parts) == 2 {
					models = append(models, parts[1])
				} else {
					models = append(models, s)
				}
			}
		}
		if len(models) > 0 {
			ctx.Set("litellm_fallbacks", models)
		}
		delete(ctx.Body, "fallbacks")
	}

	// Extract num_retries hint
	if retries, ok := ctx.Body["num_retries"].(float64); ok {
		ctx.Set("litellm_num_retries", int(retries))
		delete(ctx.Body, "num_retries")
	}

	// Remove LiteLLM-specific fields that upstream providers don't understand
	delete(ctx.Body, "api_base")
	delete(ctx.Body, "api_key")
	delete(ctx.Body, "api_version")

	return nil
}

func (p *LiteLLMCompatPlugin) PostHook(ctx *RelayContext, resp *RelayPluginResponse) {
	// No post-processing needed
}
