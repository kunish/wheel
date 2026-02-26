package relay

import (
	"fmt"
	"regexp"
	"strings"
)

// ContentFilterConfig defines the configuration for content filtering.
type ContentFilterConfig struct {
	BlockedKeywords []string // case-insensitive keyword blocklist
	BlockedPatterns []string // regex patterns to block
	MaxInputLength  int      // max total input length in characters, 0 = unlimited
	Enabled         bool
}

// ContentFilterPlugin is a relay plugin that filters request content
// before it reaches the upstream provider.
type ContentFilterPlugin struct {
	config   ContentFilterConfig
	patterns []*regexp.Regexp
}

// NewContentFilterPlugin creates a content filter plugin with pre-compiled patterns.
func NewContentFilterPlugin(config ContentFilterConfig) *ContentFilterPlugin {
	var patterns []*regexp.Regexp
	for _, p := range config.BlockedPatterns {
		if re, err := regexp.Compile(p); err == nil {
			patterns = append(patterns, re)
		}
	}
	return &ContentFilterPlugin{
		config:   config,
		patterns: patterns,
	}
}

func (p *ContentFilterPlugin) Name() string { return "content_filter" }

func (p *ContentFilterPlugin) PreHook(ctx *RelayContext) *ShortCircuit {
	if !p.config.Enabled {
		return nil
	}

	content := extractUserContent(ctx.Body)
	if content == "" {
		return nil
	}

	// Check max input length
	if p.config.MaxInputLength > 0 && len(content) > p.config.MaxInputLength {
		return &ShortCircuit{
			StatusCode: 400,
			Body: map[string]any{
				"error": map[string]any{
					"message": fmt.Sprintf("Input too long: %d characters (max %d)", len(content), p.config.MaxInputLength),
					"type":    "invalid_request_error",
				},
			},
		}
	}

	// Check blocked keywords (case-insensitive)
	lower := strings.ToLower(content)
	for _, kw := range p.config.BlockedKeywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			return &ShortCircuit{
				StatusCode: 400,
				Body: map[string]any{
					"error": map[string]any{
						"message": "Request blocked by content filter",
						"type":    "content_filter_error",
					},
				},
			}
		}
	}

	// Check blocked patterns
	for _, re := range p.patterns {
		if re.MatchString(content) {
			return &ShortCircuit{
				StatusCode: 400,
				Body: map[string]any{
					"error": map[string]any{
						"message": "Request blocked by content filter",
						"type":    "content_filter_error",
					},
				},
			}
		}
	}

	return nil
}

func (p *ContentFilterPlugin) PostHook(ctx *RelayContext, resp *RelayPluginResponse) {
	// Content filtering is pre-request only
}

// extractUserContent extracts all user message content from the request body.
func extractUserContent(body map[string]any) string {
	messages, ok := body["messages"].([]any)
	if !ok {
		return ""
	}

	var parts []string
	for _, m := range messages {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		if role != "user" && role != "system" {
			continue
		}
		switch c := msg["content"].(type) {
		case string:
			parts = append(parts, c)
		case []any:
			for _, item := range c {
				if block, ok := item.(map[string]any); ok {
					if text, ok := block["text"].(string); ok {
						parts = append(parts, text)
					}
				}
			}
		}
	}
	return strings.Join(parts, "\n")
}
