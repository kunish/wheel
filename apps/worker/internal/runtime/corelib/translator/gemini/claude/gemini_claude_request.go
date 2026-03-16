// Package claude provides request translation functionality for Claude API.
package claude

import (
	"encoding/json"

	"github.com/kunish/wheel/apps/worker/internal/protocol"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/registry"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/thinking"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/gemini/common"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"bytes"
	"strings"
)

const geminiClaudeThoughtSignature = "skip_thought_signature_validator"

// ConvertClaudeRequestToGemini parses a Claude API request and returns a complete
// Gemini request body (as JSON bytes).
func ConvertClaudeRequestToGemini(modelName string, inputRawJSON []byte, _ bool) []byte {
	var req protocol.AnthropicRequest
	cleanJSON := bytes.Replace(inputRawJSON, []byte(`"url":{"type":"string","format":"uri",`), []byte(`"url":{"type":"string",`), -1)
	if err := json.Unmarshal(cleanJSON, &req); err != nil {
		return inputRawJSON
	}

	geminiReq := protocol.AnthropicRequestToGemini(&req, modelName)

	// Override thinking config using the thinking package for accurate model-specific conversion
	overrideThinkingConfig(inputRawJSON, geminiReq, modelName)

	result, err := json.Marshal(geminiReq)
	if err != nil {
		return inputRawJSON
	}

	result = common.AttachDefaultSafetySettings(result, "safetySettings")
	return result
}

func overrideThinkingConfig(rawJSON []byte, geminiReq *protocol.GeminiRequest, modelName string) {
	t := gjson.GetBytes(rawJSON, "thinking")
	if !t.Exists() || !t.IsObject() {
		return
	}

	switch t.Get("type").String() {
	case "enabled":
		if b := t.Get("budget_tokens"); b.Exists() && b.Type == gjson.Number {
			budget := int(b.Int())
			if geminiReq.GenerationConfig == nil {
				geminiReq.GenerationConfig = &protocol.GeminiGenConfig{}
			}
			geminiReq.GenerationConfig.ThinkingConfig = &protocol.GeminiThinkConfig{
				ThinkingBudget:  protocol.Ptr(budget),
				IncludeThoughts: protocol.Ptr(true),
			}
		}
	case "adaptive", "auto":
		effort := ""
		if v := gjson.GetBytes(rawJSON, "output_config.effort"); v.Exists() && v.Type == gjson.String {
			effort = strings.ToLower(strings.TrimSpace(v.String()))
		}
		if geminiReq.GenerationConfig == nil {
			geminiReq.GenerationConfig = &protocol.GeminiGenConfig{}
		}
		if effort != "" {
			geminiReq.GenerationConfig.ThinkingConfig = &protocol.GeminiThinkConfig{
				ThinkingLevel:   effort,
				IncludeThoughts: protocol.Ptr(true),
			}
		} else {
			maxBudget := 0
			if mi := registry.LookupModelInfo(modelName, "gemini"); mi != nil && mi.Thinking != nil {
				maxBudget = mi.Thinking.Max
			}
			if maxBudget > 0 {
				geminiReq.GenerationConfig.ThinkingConfig = &protocol.GeminiThinkConfig{
					ThinkingBudget:  protocol.Ptr(maxBudget),
					IncludeThoughts: protocol.Ptr(true),
				}
			} else {
				geminiReq.GenerationConfig.ThinkingConfig = &protocol.GeminiThinkConfig{
					ThinkingLevel:   "high",
					IncludeThoughts: protocol.Ptr(true),
				}
			}
		}
	}

	_ = thinking.ConvertBudgetToLevel // reference for IDE
	_ = sjson.Set                     // keep import for common package
}

func toolNameFromClaudeToolUseID(toolUseID string) string {
	parts := strings.Split(toolUseID, "-")
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts[0:len(parts)-1], "-")
}
