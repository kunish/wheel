package relay

import (
	"encoding/json"
	"fmt"
	"strings"
)

func buildGeminiRequest(baseUrl, key string, body map[string]any, model string, channel ChannelConfig) UpstreamRequest {
	outBody := copyBody(body)
	delete(outBody, "model")

	// Convert messages to Gemini contents + systemInstruction
	messages, _ := outBody["messages"].([]any)
	delete(outBody, "messages")

	var systemParts []any
	var contents []any

	for _, m := range messages {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		switch role {
		case "system":
			systemParts = append(systemParts, contentToGeminiParts(msg["content"]))
		case "assistant":
			parts := assistantToGeminiParts(msg)
			contents = append(contents, map[string]any{"role": "model", "parts": parts})
		case "tool":
			contents = append(contents, toolResultToGeminiContent(msg))
		default:
			contents = append(contents, map[string]any{
				"role":  "user",
				"parts": contentToGeminiParts(msg["content"]),
			})
		}
	}

	geminiBody := map[string]any{"contents": contents}

	if len(systemParts) > 0 {
		// Flatten all system parts into one systemInstruction
		var allParts []any
		for _, sp := range systemParts {
			if parts, ok := sp.([]any); ok {
				allParts = append(allParts, parts...)
			}
		}
		geminiBody["systemInstruction"] = map[string]any{"parts": allParts}
	}

	// generationConfig
	genConfig := map[string]any{}
	if t, ok := outBody["temperature"]; ok {
		genConfig["temperature"] = t
	}
	if tp, ok := outBody["top_p"]; ok {
		genConfig["topP"] = tp
	}
	if mt, ok := outBody["max_tokens"].(float64); ok && mt > 0 {
		genConfig["maxOutputTokens"] = int(mt)
	}
	if stop, ok := outBody["stop"].([]any); ok {
		var seqs []string
		for _, s := range stop {
			if str, ok := s.(string); ok {
				seqs = append(seqs, str)
			}
		}
		if len(seqs) > 0 {
			genConfig["stopSequences"] = seqs
		}
	}
	if len(genConfig) > 0 {
		geminiBody["generationConfig"] = genConfig
	}

	// Convert tools
	if tools, ok := outBody["tools"].([]any); ok {
		geminiBody["tools"] = convertOpenAIToolsToGemini(tools)
	}

	applyParamOverrides(geminiBody, channel.ParamOverride)

	// Determine endpoint based on stream flag
	stream, _ := body["stream"].(bool)
	var endpoint string
	if stream {
		endpoint = fmt.Sprintf("/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s", model, key)
	} else {
		endpoint = fmt.Sprintf("/v1beta/models/%s:generateContent?key=%s", model, key)
	}

	headers := map[string]string{"Content-Type": "application/json"}
	for _, h := range channel.CustomHeader {
		headers[h.Key] = h.Value
	}

	bodyJSON, _ := json.Marshal(geminiBody)
	return UpstreamRequest{
		URL:     baseUrl + endpoint,
		Headers: headers,
		Body:    string(bodyJSON),
	}
}

// contentToGeminiParts converts OpenAI content (string or array) to Gemini parts.
func contentToGeminiParts(content any) []any {
	if s, ok := content.(string); ok {
		return []any{map[string]any{"text": s}}
	}
	parts, ok := content.([]any)
	if !ok {
		return []any{map[string]any{"text": fmt.Sprint(content)}}
	}
	var result []any
	for _, p := range parts {
		part, ok := p.(map[string]any)
		if !ok {
			continue
		}
		partType, _ := part["type"].(string)
		switch partType {
		case "text":
			text, _ := part["text"].(string)
			result = append(result, map[string]any{"text": text})
		case "image_url":
			if imgURL, ok := part["image_url"].(map[string]any); ok {
				url, _ := imgURL["url"].(string)
				if strings.HasPrefix(url, "data:") {
					mediaType, data := parseDataURL(url)
					result = append(result, map[string]any{
						"inlineData": map[string]any{
							"mimeType": mediaType,
							"data":     data,
						},
					})
				}
			}
		}
	}
	if len(result) == 0 {
		result = append(result, map[string]any{"text": ""})
	}
	return result
}

func assistantToGeminiParts(msg map[string]any) []any {
	toolCalls, ok := msg["tool_calls"].([]any)
	if ok && len(toolCalls) > 0 {
		var parts []any
		if content, ok := msg["content"].(string); ok && content != "" {
			parts = append(parts, map[string]any{"text": content})
		}
		for _, tc := range toolCalls {
			tcMap, ok := tc.(map[string]any)
			if !ok {
				continue
			}
			fn, _ := tcMap["function"].(map[string]any)
			if fn == nil {
				continue
			}
			name, _ := fn["name"].(string)
			var args any = map[string]any{}
			if argsStr, ok := fn["arguments"].(string); ok {
				var parsed any
				if err := json.Unmarshal([]byte(argsStr), &parsed); err == nil {
					args = parsed
				}
			}
			parts = append(parts, map[string]any{
				"functionCall": map[string]any{"name": name, "args": args},
			})
		}
		return parts
	}
	return contentToGeminiParts(msg["content"])
}

func toolResultToGeminiContent(msg map[string]any) map[string]any {
	name, _ := msg["name"].(string)
	content := msg["content"]
	var response any
	if s, ok := content.(string); ok {
		var parsed any
		if err := json.Unmarshal([]byte(s), &parsed); err == nil {
			response = parsed
		} else {
			response = map[string]any{"result": s}
		}
	} else {
		response = content
	}
	return map[string]any{
		"role": "user",
		"parts": []any{
			map[string]any{
				"functionResponse": map[string]any{
					"name":     name,
					"response": response,
				},
			},
		},
	}
}

func convertOpenAIToolsToGemini(tools []any) []any {
	var decls []any
	for _, t := range tools {
		tool, ok := t.(map[string]any)
		if !ok || tool["type"] != "function" {
			continue
		}
		fn, ok := tool["function"].(map[string]any)
		if !ok {
			continue
		}
		decl := map[string]any{
			"name": fn["name"],
		}
		if desc, ok := fn["description"].(string); ok && desc != "" {
			decl["description"] = desc
		}
		if params := fn["parameters"]; params != nil {
			decl["parameters"] = params
		}
		decls = append(decls, decl)
	}
	if len(decls) == 0 {
		return nil
	}
	return []any{map[string]any{"functionDeclarations": decls}}
}

// convertGeminiResponse converts a Gemini response to OpenAI format.
func convertGeminiResponse(geminiResp map[string]any) map[string]any {
	candidates, _ := geminiResp["candidates"].([]any)

	var text string
	var toolCalls []any
	var finishReason string
	tcIdx := 0

	if len(candidates) > 0 {
		cand, _ := candidates[0].(map[string]any)
		finishReason, _ = cand["finishReason"].(string)
		if content, ok := cand["content"].(map[string]any); ok {
			parts, _ := content["parts"].([]any)
			for _, p := range parts {
				part, ok := p.(map[string]any)
				if !ok {
					continue
				}
				if t, ok := part["text"].(string); ok {
					text += t
				}
				if fc, ok := part["functionCall"].(map[string]any); ok {
					name, _ := fc["name"].(string)
					argsJSON, _ := json.Marshal(fc["args"])
					toolCalls = append(toolCalls, map[string]any{
						"index": tcIdx,
						"id":    fmt.Sprintf("call_%d", tcIdx),
						"type":  "function",
						"function": map[string]any{
							"name":      name,
							"arguments": string(argsJSON),
						},
					})
					tcIdx++
				}
			}
		}
	}

	message := map[string]any{"role": "assistant"}
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

	usage, _ := geminiResp["usageMetadata"].(map[string]any)
	promptTokens := toInt(usage["promptTokenCount"])
	completionTokens := toInt(usage["candidatesTokenCount"])
	cachedTokens := toInt(usage["cachedContentTokenCount"])

	usageMap := map[string]any{
		"prompt_tokens":     promptTokens,
		"completion_tokens": completionTokens,
		"total_tokens":      promptTokens + completionTokens,
	}
	if cachedTokens > 0 {
		usageMap["prompt_tokens_details"] = map[string]any{"cached_tokens": cachedTokens}
	}

	return map[string]any{
		"id":      "chatcmpl-gemini",
		"object":  "chat.completion",
		"created": float64(currentUnixSec()),
		"model":   geminiResp["modelVersion"],
		"choices": []any{
			map[string]any{
				"index":         0,
				"message":       message,
				"finish_reason": mapGeminiFinishReason(finishReason),
			},
		},
		"usage": usageMap,
	}
}

func mapGeminiFinishReason(reason string) string {
	switch reason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY", "RECITATION", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII":
		return "content_filter"
	default:
		return "stop"
	}
}
