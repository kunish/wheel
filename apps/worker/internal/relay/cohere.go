package relay

import (
	"encoding/json"
)

// buildCohereRequest converts OpenAI chat completion format to Cohere Chat API format.
// Cohere uses a different API structure: /v2/chat with its own message format.
func buildCohereRequest(baseUrl, key string, body map[string]any, model string, channel ChannelConfig) UpstreamRequest {
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + key,
	}
	for _, h := range channel.CustomHeader {
		headers[h.Key] = h.Value
	}

	messages, _ := body["messages"].([]any)

	// Cohere v2 Chat API accepts a messages array similar to OpenAI,
	// but with some differences in role naming and structure.
	var cohereMessages []any
	for _, m := range messages {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		switch role {
		case "system", "developer":
			cohereMessages = append(cohereMessages, map[string]any{
				"role":    "system",
				"content": msg["content"],
			})
		case "assistant":
			cm := map[string]any{
				"role":    "assistant",
				"content": msg["content"],
			}
			if tc, ok := msg["tool_calls"]; ok {
				cm["tool_calls"] = tc
			}
			cohereMessages = append(cohereMessages, cm)
		case "tool":
			cohereMessages = append(cohereMessages, map[string]any{
				"role":         "tool",
				"tool_call_id": msg["tool_call_id"],
				"content":      msg["content"],
			})
		default:
			cohereMessages = append(cohereMessages, map[string]any{
				"role":    "user",
				"content": msg["content"],
			})
		}
	}

	cohereBody := map[string]any{
		"model":    model,
		"messages": cohereMessages,
	}

	if s, ok := body["stream"].(bool); ok && s {
		cohereBody["stream"] = true
	}
	if t, ok := body["temperature"]; ok {
		cohereBody["temperature"] = t
	}
	if tp, ok := body["top_p"]; ok {
		cohereBody["p"] = tp
	}
	if mt, ok := body["max_tokens"]; ok {
		cohereBody["max_tokens"] = mt
	}
	if stop, ok := body["stop"]; ok {
		cohereBody["stop_sequences"] = stop
	}

	// Convert OpenAI tools to Cohere tools format
	if tools, ok := body["tools"].([]any); ok {
		cohereBody["tools"] = convertOpenAIToolsToCohere(tools)
	}

	applyParamOverrides(cohereBody, channel.ParamOverride)

	bodyJSON, _ := json.Marshal(cohereBody)
	return UpstreamRequest{
		URL:     baseUrl + "/v2/chat",
		Headers: headers,
		Body:    string(bodyJSON),
	}
}

// convertOpenAIToolsToCohere converts OpenAI function tools to Cohere tool format.
func convertOpenAIToolsToCohere(tools []any) []any {
	var result []any
	for _, t := range tools {
		tool, ok := t.(map[string]any)
		if !ok || tool["type"] != "function" {
			continue
		}
		fn, ok := tool["function"].(map[string]any)
		if !ok {
			continue
		}
		result = append(result, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        fn["name"],
				"description": fn["description"],
				"parameters":  fn["parameters"],
			},
		})
	}
	return result
}

// convertCohereResponse converts a Cohere v2 Chat response to OpenAI format.
func convertCohereResponse(cohereResp map[string]any) map[string]any {
	text, _ := cohereResp["text"].(string)
	if text == "" {
		if msg, ok := cohereResp["message"].(map[string]any); ok {
			if content, ok := msg["content"].([]any); ok && len(content) > 0 {
				if block, ok := content[0].(map[string]any); ok {
					text, _ = block["text"].(string)
				}
			}
		}
	}

	message := map[string]any{
		"role":    "assistant",
		"content": text,
	}

	// Handle tool calls
	if toolCalls, ok := cohereResp["tool_calls"].([]any); ok && len(toolCalls) > 0 {
		var openaiToolCalls []any
		for i, tc := range toolCalls {
			tcMap, ok := tc.(map[string]any)
			if !ok {
				continue
			}
			name, _ := tcMap["name"].(string)
			params, _ := json.Marshal(tcMap["parameters"])
			id, _ := tcMap["id"].(string)
			if id == "" {
				id = name
			}
			openaiToolCalls = append(openaiToolCalls, map[string]any{
				"index": i,
				"id":    id,
				"type":  "function",
				"function": map[string]any{
					"name":      name,
					"arguments": string(params),
				},
			})
		}
		if len(openaiToolCalls) > 0 {
			message["tool_calls"] = openaiToolCalls
			if text == "" {
				message["content"] = nil
			}
		}
	}

	usage, _ := cohereResp["usage"].(map[string]any)
	tokens, _ := usage["tokens"].(map[string]any)
	inputTokens := toInt(tokens["input_tokens"])
	outputTokens := toInt(tokens["output_tokens"])

	finishReason := "stop"
	if fr, ok := cohereResp["finish_reason"].(string); ok {
		switch fr {
		case "COMPLETE", "STOP":
			finishReason = "stop"
		case "MAX_TOKENS":
			finishReason = "length"
		case "TOOL_CALL":
			finishReason = "tool_calls"
		}
	}

	id, _ := cohereResp["id"].(string)
	if id == "" {
		id = "chatcmpl-cohere"
	}

	return map[string]any{
		"id":      id,
		"object":  "chat.completion",
		"created": float64(currentUnixSec()),
		"model":   cohereResp["model"],
		"choices": []any{
			map[string]any{
				"index":         0,
				"message":       message,
				"finish_reason": finishReason,
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     inputTokens,
			"completion_tokens": outputTokens,
			"total_tokens":      inputTokens + outputTokens,
		},
	}
}

// createCohereSSEConverter returns a stateful converter from Cohere SSE to OpenAI SSE format.
// Cohere v2 streaming sends events like: text-generation, tool-call-start, tool-call-delta, etc.
func createCohereSSEConverter() func(string) *anthropicSSEResult {
	started := false
	msgModel := ""

	return func(jsonStr string) *anthropicSSEResult {
		var event map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &event); err != nil {
			return nil
		}

		evType, _ := event["type"].(string)
		if model, ok := event["model"].(string); ok && model != "" {
			msgModel = model
		}

		result := &anthropicSSEResult{}

		switch evType {
		case "message-start":
			started = true
			delta := map[string]any{"role": "assistant", "content": ""}
			result.data = map[string]any{
				"id": "chatcmpl-cohere", "object": "chat.completion.chunk",
				"created": float64(currentUnixSec()), "model": msgModel,
				"choices": []any{map[string]any{
					"index": 0, "delta": delta, "finish_reason": nil,
				}},
			}
			return result

		case "content-delta", "text-generation":
			if !started {
				started = true
			}
			text, _ := event["text"].(string)
			if text == "" {
				if delta, ok := event["delta"].(map[string]any); ok {
					if msg, ok := delta["message"].(map[string]any); ok {
						if content, ok := msg["content"].(map[string]any); ok {
							text, _ = content["text"].(string)
						}
					}
				}
			}
			if text == "" {
				return nil
			}
			result.data = map[string]any{
				"id": "chatcmpl-cohere", "object": "chat.completion.chunk",
				"created": float64(currentUnixSec()), "model": msgModel,
				"choices": []any{map[string]any{
					"index": 0, "delta": map[string]any{"content": text}, "finish_reason": nil,
				}},
			}
			return result

		case "message-end":
			var fr any = "stop"
			if delta, ok := event["delta"].(map[string]any); ok {
				if reason, ok := delta["finish_reason"].(string); ok {
					switch reason {
					case "COMPLETE", "STOP":
						fr = "stop"
					case "MAX_TOKENS":
						fr = "length"
					case "TOOL_CALL":
						fr = "tool_calls"
					}
				}
				if usage, ok := delta["usage"].(map[string]any); ok {
					if tokens, ok := usage["tokens"].(map[string]any); ok {
						result.inputTokens = toInt(tokens["input_tokens"])
						result.outputTokens = toInt(tokens["output_tokens"])
					}
				}
			}
			result.data = map[string]any{
				"id": "chatcmpl-cohere", "object": "chat.completion.chunk",
				"created": float64(currentUnixSec()), "model": msgModel,
				"choices": []any{map[string]any{
					"index": 0, "delta": map[string]any{}, "finish_reason": fr,
				}},
			}
			return result

		default:
			return nil
		}
	}
}
