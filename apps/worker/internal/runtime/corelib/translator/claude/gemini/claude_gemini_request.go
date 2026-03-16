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
	"github.com/tidwall/sjson"
)

// ConvertGeminiRequestToClaude parses and transforms a Gemini API request into Claude Code API format.
func ConvertGeminiRequestToClaude(modelName string, inputRawJSON []byte, stream bool) []byte {
	var req protocol.GeminiRequest
	if err := json.Unmarshal(inputRawJSON, &req); err != nil {
		return inputRawJSON
	}

	result, err := protocol.GeminiRequestToAnthropic(&req, modelName, stream)
	if err != nil {
		return inputRawJSON
	}

	result = overrideClaudeThinkingConfig(inputRawJSON, result, modelName)

	// Convert tool parameter types to lowercase for Claude Code compatibility
	out := string(result)
	toolsResult := gjson.Get(out, "tools")
	var pathsToLower []string
	util.Walk(toolsResult, "", "type", &pathsToLower)
	for _, p := range pathsToLower {
		fullPath := fmt.Sprintf("tools.%s", p)
		_ = strings.ToLower(gjson.Get(out, fullPath).String())
	}

	return result
}

func overrideClaudeThinkingConfig(rawJSON []byte, result []byte, modelName string) []byte {
	root := gjson.ParseBytes(rawJSON)
	thinkingConfig := root.Get("generationConfig.thinkingConfig")
	if !thinkingConfig.Exists() || !thinkingConfig.IsObject() {
		return result
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
				result = setThinkingJSON(result, map[string]any{"type": "disabled"})
				result, _ = sjson.DeleteBytes(result, "output_config")
			default:
				if mapped, ok := thinking.MapToClaudeEffort(level, supportsMax); ok {
					level = mapped
				}
				result = setThinkingJSON(result, map[string]any{"type": "adaptive"})
				result = setOutputConfigJSON(result, map[string]any{"effort": level})
			}
		} else {
			switch level {
			case "":
				// no-op
			case "none":
				result = setThinkingJSON(result, map[string]any{"type": "disabled"})
			case "auto":
				result = setThinkingJSON(result, map[string]any{"type": "enabled"})
			default:
				if budget, ok := thinking.ConvertLevelToBudget(level); ok {
					result = setThinkingJSON(result, map[string]any{"type": "enabled", "budget_tokens": budget})
				}
			}
		}
		return result
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
				result = setThinkingJSON(result, map[string]any{"type": "disabled"})
				result, _ = sjson.DeleteBytes(result, "output_config")
			default:
				level, ok := thinking.ConvertBudgetToLevel(budget)
				if ok {
					if mapped, okM := thinking.MapToClaudeEffort(level, supportsMax); okM {
						level = mapped
					}
					result = setThinkingJSON(result, map[string]any{"type": "adaptive"})
					result = setOutputConfigJSON(result, map[string]any{"effort": level})
				}
			}
		} else {
			switch budget {
			case 0:
				result = setThinkingJSON(result, map[string]any{"type": "disabled"})
			case -1:
				result = setThinkingJSON(result, map[string]any{"type": "enabled"})
			default:
				result = setThinkingJSON(result, map[string]any{"type": "enabled", "budget_tokens": budget})
			}
		}
		return result
	}

	if includeThoughts := thinkingConfig.Get("includeThoughts"); includeThoughts.Exists() && includeThoughts.Type == gjson.True {
		result = setThinkingJSON(result, map[string]any{"type": "enabled"})
	} else if includeThoughts := thinkingConfig.Get("include_thoughts"); includeThoughts.Exists() && includeThoughts.Type == gjson.True {
		result = setThinkingJSON(result, map[string]any{"type": "enabled"})
	}

	return result
}

func setThinkingJSON(data []byte, thinking map[string]any) []byte {
	j, _ := json.Marshal(thinking)
	data, _ = sjson.SetRawBytes(data, "thinking", j)
	return data
}

func setOutputConfigJSON(data []byte, config map[string]any) []byte {
	j, _ := json.Marshal(config)
	data, _ = sjson.SetRawBytes(data, "output_config", j)
	return data
}
