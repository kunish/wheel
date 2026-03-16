// Package claude provides request translation functionality for Claude API.
package claude

import (
	"bytes"
	"encoding/json"
	"strings"

	"google.golang.org/genai"

	"github.com/kunish/wheel/apps/worker/internal/protocol"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/registry"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/gemini/common"
	"github.com/tidwall/gjson"
)

// ConvertClaudeRequestToGemini parses a Claude API request and returns a complete
// Gemini request body (as JSON bytes).
func ConvertClaudeRequestToGemini(modelName string, inputRawJSON []byte, _ bool) []byte {
	cleanJSON := bytes.Replace(inputRawJSON, []byte(`"url":{"type":"string","format":"uri",`), []byte(`"url":{"type":"string",`), -1)

	geminiReq, err := protocol.AnthropicRequestToGemini(cleanJSON, modelName)
	if err != nil {
		return inputRawJSON
	}

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
			budget := int32(b.Int())
			if geminiReq.GenerationConfig == nil {
				geminiReq.GenerationConfig = &genai.GenerateContentConfig{}
			}
			geminiReq.GenerationConfig.ThinkingConfig = &genai.ThinkingConfig{
				ThinkingBudget:  protocol.Ptr(budget),
				IncludeThoughts: true,
			}
		}
	case "adaptive", "auto":
		effort := ""
		if v := gjson.GetBytes(rawJSON, "output_config.effort"); v.Exists() && v.Type == gjson.String {
			effort = strings.ToLower(strings.TrimSpace(v.String()))
		}
		if geminiReq.GenerationConfig == nil {
			geminiReq.GenerationConfig = &genai.GenerateContentConfig{}
		}
		if effort != "" {
			geminiReq.GenerationConfig.ThinkingConfig = &genai.ThinkingConfig{
				ThinkingLevel:   genai.ThinkingLevel(effort),
				IncludeThoughts: true,
			}
		} else {
			maxBudget := 0
			if mi := registry.LookupModelInfo(modelName, "gemini"); mi != nil && mi.Thinking != nil {
				maxBudget = mi.Thinking.Max
			}
			if maxBudget > 0 {
				geminiReq.GenerationConfig.ThinkingConfig = &genai.ThinkingConfig{
					ThinkingBudget:  protocol.Ptr(int32(maxBudget)),
					IncludeThoughts: true,
				}
			} else {
				geminiReq.GenerationConfig.ThinkingConfig = &genai.ThinkingConfig{
					ThinkingLevel:   "high",
					IncludeThoughts: true,
				}
			}
		}
	}
}

