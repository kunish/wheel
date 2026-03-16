// Package gemini provides request translation functionality for Gemini to OpenAI API.
package gemini

import (
	"encoding/json"
	"strings"

	"github.com/kunish/wheel/apps/worker/internal/protocol"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/thinking"
	"github.com/tidwall/gjson"
)

// ConvertGeminiRequestToOpenAI parses and transforms a Gemini API request into OpenAI Chat Completions API format.
func ConvertGeminiRequestToOpenAI(modelName string, inputRawJSON []byte, stream bool) []byte {
	var req protocol.GeminiRequest
	if err := json.Unmarshal(inputRawJSON, &req); err != nil {
		return inputRawJSON
	}

	out := protocol.GeminiRequestToOpenAI(&req, modelName, stream)

	// Use the thinking package's ConvertBudgetToLevel for accurate conversion
	out.ReasoningEffort = convertGeminiThinkingWithPackage(inputRawJSON)

	result, err := json.Marshal(out)
	if err != nil {
		return inputRawJSON
	}

	return result
}

func convertGeminiThinkingWithPackage(rawJSON []byte) string {
	root := gjson.ParseBytes(rawJSON)

	// Support both camelCase and snake_case for thinking config
	thinkingConfig := root.Get("generationConfig.thinkingConfig")
	if !thinkingConfig.Exists() || !thinkingConfig.IsObject() {
		return ""
	}

	thinkingLevel := thinkingConfig.Get("thinkingLevel")
	if !thinkingLevel.Exists() {
		thinkingLevel = thinkingConfig.Get("thinking_level")
	}
	if thinkingLevel.Exists() {
		effort := strings.ToLower(strings.TrimSpace(thinkingLevel.String()))
		if effort != "" {
			return effort
		}
	}

	thinkingBudget := thinkingConfig.Get("thinkingBudget")
	if !thinkingBudget.Exists() {
		thinkingBudget = thinkingConfig.Get("thinking_budget")
	}
	if thinkingBudget.Exists() {
		if effort, ok := thinking.ConvertBudgetToLevel(int(thinkingBudget.Int())); ok {
			return effort
		}
	}

	return ""
}
