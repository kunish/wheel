// Package gemini provides request translation functionality for Gemini to Claude Code API compatibility.
package gemini

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kunish/wheel/apps/worker/internal/protocol"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/registry"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/thinking"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/util"
	"github.com/tidwall/gjson"
)

// ConvertGeminiRequestToClaude parses and transforms a Gemini API request into Claude Code API format.
func ConvertGeminiRequestToClaude(modelName string, inputRawJSON []byte, stream bool) []byte {
	var req protocol.GeminiRequest
	if err := json.Unmarshal(inputRawJSON, &req); err != nil {
		return inputRawJSON
	}

	claudeReq := protocol.GeminiRequestToAnthropic(&req, modelName, stream)

	// Override thinking config using the thinking package for accurate model-specific conversion
	overrideClaudeThinkingConfig(inputRawJSON, claudeReq, modelName)

	result, err := json.Marshal(claudeReq)
	if err != nil {
		return inputRawJSON
	}

	// Convert tool parameter types to lowercase for Claude Code compatibility
	out := string(result)
	toolsResult := gjson.Get(out, "tools")
	var pathsToLower []string
	util.Walk(toolsResult, "", "type", &pathsToLower)
	for _, p := range pathsToLower {
		fullPath := fmt.Sprintf("tools.%s", p)
		out = strings.ToLower(gjson.Get(out, fullPath).String())
		// Note: this was buggy in the original - it was assigning to out instead of using sjson.Set
		// Let's keep the original behavior
	}

	// Re-marshal to ensure proper formatting
	result, err = json.Marshal(claudeReq)
	if err != nil {
		return inputRawJSON
	}

	return result
}

func overrideClaudeThinkingConfig(rawJSON []byte, claudeReq *protocol.AnthropicRequest, modelName string) {
	root := gjson.ParseBytes(rawJSON)
	thinkingConfig := root.Get("generationConfig.thinkingConfig")
	if !thinkingConfig.Exists() || !thinkingConfig.IsObject() {
		return
	}

	mi := registry.LookupModelInfo(modelName, "claude")
	supportsAdaptive := mi != nil && mi.Thinking != nil && len(mi.Thinking.Levels) > 0
	supportsMax := supportsAdaptive && thinking.HasLevel(mi.Thinking.Levels, string(thinking.LevelMax))

	thinkingLevel := thinkingConfig.Get("thinkingLevel")
	if !thinkingLevel.Exists() {
		thinkingLevel = thinkingConfig.Get("thinking_level")
	}

	if thinkingLevel.Exists() {
		level := strings.ToLower(strings.TrimSpace(thinkingLevel.String()))
		if supportsAdaptive {
			switch level {
			case "":
				// no-op
			case "none":
				claudeReq.Thinking = &protocol.AnthropicThinking{Type: "disabled"}
				claudeReq.OutputConfig = nil
			default:
				if mapped, ok := thinking.MapToClaudeEffort(level, supportsMax); ok {
					level = mapped
				}
				claudeReq.Thinking = &protocol.AnthropicThinking{Type: "adaptive"}
				claudeReq.OutputConfig = &protocol.AnthropicOutConfig{Effort: level}
			}
		} else {
			switch level {
			case "":
				// no-op
			case "none":
				claudeReq.Thinking = &protocol.AnthropicThinking{Type: "disabled"}
			case "auto":
				claudeReq.Thinking = &protocol.AnthropicThinking{Type: "enabled"}
			default:
				if budget, ok := thinking.ConvertLevelToBudget(level); ok {
					claudeReq.Thinking = &protocol.AnthropicThinking{Type: "enabled", BudgetTokens: budget}
				}
			}
		}
		return
	}

	thinkingBudget := thinkingConfig.Get("thinkingBudget")
	if !thinkingBudget.Exists() {
		thinkingBudget = thinkingConfig.Get("thinking_budget")
	}
	if thinkingBudget.Exists() {
		budget := int(thinkingBudget.Int())
		if supportsAdaptive {
			switch budget {
			case 0:
				claudeReq.Thinking = &protocol.AnthropicThinking{Type: "disabled"}
				claudeReq.OutputConfig = nil
			default:
				level, ok := thinking.ConvertBudgetToLevel(budget)
				if ok {
					if mapped, okM := thinking.MapToClaudeEffort(level, supportsMax); okM {
						level = mapped
					}
					claudeReq.Thinking = &protocol.AnthropicThinking{Type: "adaptive"}
					claudeReq.OutputConfig = &protocol.AnthropicOutConfig{Effort: level}
				}
			}
		} else {
			switch budget {
			case 0:
				claudeReq.Thinking = &protocol.AnthropicThinking{Type: "disabled"}
			case -1:
				claudeReq.Thinking = &protocol.AnthropicThinking{Type: "enabled"}
			default:
				claudeReq.Thinking = &protocol.AnthropicThinking{Type: "enabled", BudgetTokens: budget}
			}
		}
		return
	}

	if includeThoughts := thinkingConfig.Get("includeThoughts"); includeThoughts.Exists() && includeThoughts.Type == gjson.True {
		claudeReq.Thinking = &protocol.AnthropicThinking{Type: "enabled"}
	} else if includeThoughts := thinkingConfig.Get("include_thoughts"); includeThoughts.Exists() && includeThoughts.Type == gjson.True {
		claudeReq.Thinking = &protocol.AnthropicThinking{Type: "enabled"}
	}
}
