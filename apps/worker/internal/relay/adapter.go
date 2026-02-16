package relay

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ConvertAnthropicResponse converts an Anthropic response to OpenAI format.
func ConvertAnthropicResponse(anthropicResp map[string]any) map[string]any {
	content, _ := anthropicResp["content"].([]any)

	var textParts []string
	var toolCalls []any
	tcIdx := 0
	for _, c := range content {
		block, ok := c.(map[string]any)
		if !ok {
			continue
		}
		blockType, _ := block["type"].(string)
		if blockType == "text" {
			if t, ok := block["text"].(string); ok {
				textParts = append(textParts, t)
			}
		} else if blockType == "tool_use" {
			id, _ := block["id"].(string)
			if id == "" {
				id = fmt.Sprintf("call_%d", tcIdx)
			}
			name, _ := block["name"].(string)
			inputJSON, _ := json.Marshal(block["input"])
			toolCalls = append(toolCalls, map[string]any{
				"index": tcIdx,
				"id":    id,
				"type":  "function",
				"function": map[string]any{
					"name":      name,
					"arguments": string(inputJSON),
				},
			})
			tcIdx++
		}
	}

	text := strings.Join(textParts, "")

	message := map[string]any{
		"role": "assistant",
	}
	if text != "" {
		message["content"] = text
	} else if len(toolCalls) > 0 {
		message["content"] = nil
	} else {
		message["content"] = ""
	}
	if len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
	}

	usage, _ := anthropicResp["usage"].(map[string]any)
	inputTokens := toInt(usage["input_tokens"])
	outputTokens := toInt(usage["output_tokens"])

	stopReason, _ := anthropicResp["stop_reason"].(string)

	id, _ := anthropicResp["id"].(string)
	if id == "" {
		id = "chatcmpl-unknown"
	}

	return map[string]any{
		"id":      id,
		"object":  "chat.completion",
		"created": float64(currentUnixSec()),
		"model":   anthropicResp["model"],
		"choices": []any{
			map[string]any{
				"index":         0,
				"message":       message,
				"finish_reason": mapAnthropicStopReason(stopReason),
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     inputTokens,
			"completion_tokens": outputTokens,
			"total_tokens":      inputTokens + outputTokens,
		},
	}
}

// ConvertToAnthropicResponse converts an OpenAI response to Anthropic format.
func ConvertToAnthropicResponse(openaiResp map[string]any) map[string]any {
	choices, _ := openaiResp["choices"].([]any)
	var content string
	var finishReason string
	var toolCalls []any
	if len(choices) > 0 {
		choice, _ := choices[0].(map[string]any)
		if msg, ok := choice["message"].(map[string]any); ok {
			content, _ = msg["content"].(string)
			toolCalls, _ = msg["tool_calls"].([]any)
		}
		finishReason, _ = choice["finish_reason"].(string)
	}

	usage, _ := openaiResp["usage"].(map[string]any)
	promptTokens := toInt(usage["prompt_tokens"])
	completionTokens := toInt(usage["completion_tokens"])

	id, _ := openaiResp["id"].(string)
	if id == "" {
		id = "msg_unknown"
	}

	// Build content blocks
	var contentBlocks []any
	if content != "" {
		contentBlocks = append(contentBlocks, map[string]any{"type": "text", "text": content})
	}

	// Convert tool_calls to tool_use blocks
	for i, tc := range toolCalls {
		tcMap, ok := tc.(map[string]any)
		if !ok {
			continue
		}
		fn, _ := tcMap["function"].(map[string]any)
		if fn == nil {
			continue
		}
		name, _ := fn["name"].(string)
		argsStr, _ := fn["arguments"].(string)
		var input any = map[string]any{}
		if argsStr != "" {
			var parsed any
			if err := json.Unmarshal([]byte(argsStr), &parsed); err == nil {
				input = parsed
			}
		}
		toolID, _ := tcMap["id"].(string)
		if toolID == "" {
			toolID = fmt.Sprintf("call_%d", i)
		}
		contentBlocks = append(contentBlocks, map[string]any{
			"type":  "tool_use",
			"id":    toolID,
			"name":  name,
			"input": input,
		})
	}

	// If no content blocks, add empty text
	if len(contentBlocks) == 0 {
		contentBlocks = append(contentBlocks, map[string]any{"type": "text", "text": ""})
	}

	return map[string]any{
		"id":            id,
		"type":          "message",
		"role":          "assistant",
		"model":         openaiResp["model"],
		"content":       contentBlocks,
		"stop_reason":   mapOpenAIFinishReason(finishReason),
		"stop_sequence": nil,
		"usage": map[string]any{
			"input_tokens":  promptTokens,
			"output_tokens": completionTokens,
		},
	}
}

func mapAnthropicStopReason(reason string) string {
	switch reason {
	case "end_turn", "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	default:
		return "stop"
	}
}

func mapOpenAIFinishReason(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	default:
		return "end_turn"
	}
}

func copyBody(body map[string]any) map[string]any {
	data, err := json.Marshal(body)
	if err != nil {
		// Fallback to shallow copy if marshal fails
		out := make(map[string]any, len(body))
		for k, v := range body {
			out[k] = v
		}
		return out
	}
	var out map[string]any
	json.Unmarshal(data, &out)
	return out
}

func toInt(v any) int {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}
