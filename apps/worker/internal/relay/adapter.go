package relay

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

// UpstreamRequest holds the prepared request to send to an upstream provider.
type UpstreamRequest struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

const defaultThinkingBudget = 10000

// SelectBaseUrl picks the base URL with the lowest delay.
// Falls back to the provider's default base URL if none configured.
func SelectBaseUrl(baseUrls []types.BaseUrl, channelType ...types.OutboundType) string {
	if len(baseUrls) == 0 {
		if len(channelType) > 0 {
			if def := types.DefaultBaseURL(channelType[0]); def != "" {
				return def
			}
		}
		return "https://api.openai.com"
	}
	best := baseUrls[0]
	for i := 1; i < len(baseUrls); i++ {
		if baseUrls[i].Delay < best.Delay {
			best = baseUrls[i]
		}
	}
	return strings.TrimRight(best.URL, "/")
}

func buildAnthropicHeaders(key string, customHeaders []types.CustomHeader) map[string]string {
	headers := map[string]string{
		"Content-Type":      "application/json",
		"x-api-key":         key,
		"anthropic-version": "2023-06-01",
	}
	for _, h := range customHeaders {
		headers[h.Key] = h.Value
	}
	return headers
}

func applyParamOverrides(body map[string]any, paramOverride *string) {
	if paramOverride == nil {
		return
	}
	var overrides map[string]any
	if err := json.Unmarshal([]byte(*paramOverride), &overrides); err != nil {
		return
	}
	for k, v := range overrides {
		body[k] = v
	}
}

func ensureThinkingParams(body map[string]any, model string) {
	if !strings.Contains(model, "thinking") {
		return
	}
	if _, ok := body["thinking"]; ok {
		return
	}

	body["thinking"] = map[string]any{
		"type":          "enabled",
		"budget_tokens": defaultThinkingBudget,
	}

	maxTokens := 4096
	if mt, ok := body["max_tokens"].(float64); ok {
		maxTokens = int(mt)
	}
	if maxTokens <= defaultThinkingBudget {
		body["max_tokens"] = float64(defaultThinkingBudget + maxTokens)
	}
}

func ensureMaxTokens(body map[string]any) {
	mt, ok := body["max_tokens"]
	if !ok {
		body["max_tokens"] = float64(8192)
		return
	}
	if mtf, ok := mt.(float64); ok && mtf < 1 {
		body["max_tokens"] = float64(8192)
	}
}

// ChannelConfig holds the channel fields needed for building upstream requests.
type ChannelConfig struct {
	Type          types.OutboundType
	BaseUrls      []types.BaseUrl
	CustomHeader  []types.CustomHeader
	ParamOverride *string
}

// BuildUpstreamRequest builds the upstream request based on channel type.
func BuildUpstreamRequest(
	channel ChannelConfig,
	key string,
	inboundBody map[string]any,
	inboundPath string,
	model string,
	anthropicPassthrough bool,
) UpstreamRequest {
	baseUrl := SelectBaseUrl(channel.BaseUrls, channel.Type)

	switch channel.Type {
	case types.OutboundAnthropic:
		if anthropicPassthrough {
			return buildAnthropicPassthroughRequest(baseUrl, key, inboundBody, model, channel)
		}
		return buildAnthropicRequest(baseUrl, key, inboundBody, model, channel)
	case types.OutboundGemini:
		return buildGeminiRequest(baseUrl, key, inboundBody, model, channel)
	case types.OutboundAzureOpenAI:
		return buildAzureOpenAIRequest(baseUrl, key, inboundBody, inboundPath, model, channel)
	case types.OutboundBedrock:
		return buildBedrockRequest(baseUrl, key, inboundBody, model, channel)
	case types.OutboundVertex:
		return buildVertexRequest(baseUrl, key, inboundBody, model, channel)
	case types.OutboundCohere:
		return buildCohereRequest(baseUrl, key, inboundBody, model, channel)
	default:
		// All OpenAI-compatible providers (Groq, Mistral, DeepSeek, xAI, etc.)
		return buildOpenAIRequest(baseUrl, key, inboundBody, inboundPath, model, channel)
	}
}

func buildOpenAIRequest(baseUrl, key string, body map[string]any, inboundPath, model string, channel ChannelConfig) UpstreamRequest {
	path := "/v1/chat/completions"
	if strings.Contains(inboundPath, "/embeddings") {
		path = "/v1/embeddings"
	} else if strings.Contains(inboundPath, "/responses") {
		path = "/v1/responses"
	} else if strings.Contains(inboundPath, "/images/generations") {
		path = "/v1/images/generations"
	} else if strings.Contains(inboundPath, "/audio/speech") {
		path = "/v1/audio/speech"
	} else if strings.Contains(inboundPath, "/audio/transcriptions") {
		path = "/v1/audio/transcriptions"
	} else if strings.Contains(inboundPath, "/audio/translations") {
		path = "/v1/audio/translations"
	} else if strings.Contains(inboundPath, "/moderations") {
		path = "/v1/moderations"
	}

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + key,
	}
	for _, h := range channel.CustomHeader {
		headers[h.Key] = h.Value
	}

	outBody := copyBody(body)
	outBody["model"] = model
	applyParamOverrides(outBody, channel.ParamOverride)

	bodyJSON, _ := json.Marshal(outBody)
	return UpstreamRequest{
		URL:     baseUrl + path,
		Headers: headers,
		Body:    string(bodyJSON),
	}
}

func buildAnthropicPassthroughRequest(baseUrl, key string, body map[string]any, model string, channel ChannelConfig) UpstreamRequest {
	headers := buildAnthropicHeaders(key, channel.CustomHeader)

	outBody := copyBody(body)
	outBody["model"] = model
	ensureThinkingParams(outBody, model)
	applyParamOverrides(outBody, channel.ParamOverride)
	ensureMaxTokens(outBody)

	bodyJSON, _ := json.Marshal(outBody)
	return UpstreamRequest{
		URL:     baseUrl + "/v1/messages",
		Headers: headers,
		Body:    string(bodyJSON),
	}
}

func buildAnthropicRequest(baseUrl, key string, body map[string]any, model string, channel ChannelConfig) UpstreamRequest {
	headers := buildAnthropicHeaders(key, channel.CustomHeader)

	messages, _ := body["messages"].([]any)
	var systemMsg string
	var anthropicMessages []any

	for _, m := range messages {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		switch role {
		case "system":
			if s, ok := msg["content"].(string); ok {
				systemMsg = s
			} else {
				b, _ := json.Marshal(msg["content"])
				systemMsg = string(b)
			}
		case "assistant":
			anthropicMessages = append(anthropicMessages, convertAssistantMessage(msg))
		case "tool":
			anthropicMessages = append(anthropicMessages, convertToolResultMessage(msg))
		default:
			anthropicMessages = append(anthropicMessages, map[string]any{
				"role":    "user",
				"content": convertOpenAIContentToAnthropic(msg["content"]),
			})
		}
	}

	maxTokens := 4096.0
	if mt, ok := body["max_tokens"].(float64); ok && mt > 0 {
		maxTokens = mt
	} else if mt, ok := body["maxTokens"].(float64); ok && mt > 0 {
		maxTokens = mt
	}

	anthropicBody := map[string]any{
		"model":      model,
		"messages":   anthropicMessages,
		"max_tokens": maxTokens,
	}

	if systemMsg != "" {
		anthropicBody["system"] = systemMsg
	}
	if s, ok := body["stream"].(bool); ok && s {
		anthropicBody["stream"] = true
	}
	if t, ok := body["temperature"]; ok {
		anthropicBody["temperature"] = t
	}
	if tp, ok := body["top_p"]; ok {
		anthropicBody["top_p"] = tp
	}
	if stop, ok := body["stop"]; ok {
		anthropicBody["stop_sequences"] = stop
	}

	ensureThinkingParams(anthropicBody, model)

	// Convert OpenAI tools to Anthropic tools format
	if tools, ok := body["tools"].([]any); ok {
		anthropicBody["tools"] = convertOpenAITools(tools)
	}

	applyParamOverrides(anthropicBody, channel.ParamOverride)
	ensureMaxTokens(anthropicBody)

	bodyJSON, _ := json.Marshal(anthropicBody)
	return UpstreamRequest{
		URL:     baseUrl + "/v1/messages",
		Headers: headers,
		Body:    string(bodyJSON),
	}
}

// convertAssistantMessage converts an OpenAI assistant message (possibly with tool_calls) to Anthropic format.
func convertAssistantMessage(msg map[string]any) map[string]any {
	toolCalls, ok := msg["tool_calls"].([]any)
	if ok && len(toolCalls) > 0 {
		var contentBlocks []any
		if content, ok := msg["content"].(string); ok && content != "" {
			contentBlocks = append(contentBlocks, map[string]any{
				"type": "text",
				"text": content,
			})
		}
		for i, tc := range toolCalls {
			tcMap, ok := tc.(map[string]any)
			if !ok {
				continue
			}
			fn, _ := tcMap["function"].(map[string]any)
			if fn == nil {
				continue
			}
			var input any = map[string]any{}
			if args, ok := fn["arguments"].(string); ok {
				var parsed any
				if err := json.Unmarshal([]byte(args), &parsed); err == nil {
					input = parsed
				}
			}
			id, _ := tcMap["id"].(string)
			if id == "" {
				id = fmt.Sprintf("call_%d", i)
			}
			contentBlocks = append(contentBlocks, map[string]any{
				"type":  "tool_use",
				"id":    id,
				"name":  fn["name"],
				"input": input,
			})
		}
		return map[string]any{"role": "assistant", "content": contentBlocks}
	}

	return map[string]any{"role": "assistant", "content": msg["content"]}
}

// convertToolResultMessage converts an OpenAI tool result message to Anthropic tool_result format.
func convertToolResultMessage(msg map[string]any) map[string]any {
	content := msg["content"]
	var contentStr string
	if s, ok := content.(string); ok {
		contentStr = s
	} else {
		b, _ := json.Marshal(content)
		contentStr = string(b)
	}

	return map[string]any{
		"role": "user",
		"content": []any{
			map[string]any{
				"type":        "tool_result",
				"tool_use_id": msg["tool_call_id"],
				"content":     contentStr,
			},
		},
	}
}

// convertOpenAITools converts OpenAI tools array to Anthropic tools format.
func convertOpenAITools(tools []any) []any {
	var result []any
	for _, t := range tools {
		tool, ok := t.(map[string]any)
		if !ok {
			continue
		}
		if tool["type"] != "function" {
			continue
		}
		fn, ok := tool["function"].(map[string]any)
		if !ok {
			continue
		}
		name, _ := fn["name"].(string)
		desc, _ := fn["description"].(string)
		params := fn["parameters"]
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		result = append(result, map[string]any{
			"name":         name,
			"description":  desc,
			"input_schema": params,
		})
	}
	return result
}

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
	cacheRead := toInt(usage["cache_read_input_tokens"])

	stopReason, _ := anthropicResp["stop_reason"].(string)

	id, _ := anthropicResp["id"].(string)
	if id == "" {
		id = "chatcmpl-unknown"
	}

	usageMap := map[string]any{
		"prompt_tokens":     inputTokens,
		"completion_tokens": outputTokens,
		"total_tokens":      inputTokens + outputTokens,
	}
	if cacheRead > 0 {
		usageMap["prompt_tokens_details"] = map[string]any{
			"cached_tokens": cacheRead,
		}
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
		"usage": usageMap,
	}
}

// ConvertToAnthropicResponse converts an OpenAI response to Anthropic format.
func ConvertToAnthropicResponse(openaiResp map[string]any) map[string]any {
	choices, _ := openaiResp["choices"].([]any)
	var content string
	var finishReason string
	if len(choices) > 0 {
		choice, _ := choices[0].(map[string]any)
		if msg, ok := choice["message"].(map[string]any); ok {
			content, _ = msg["content"].(string)
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

	return map[string]any{
		"id":    id,
		"type":  "message",
		"role":  "assistant",
		"model": openaiResp["model"],
		"content": []any{
			map[string]any{"type": "text", "text": content},
		},
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
	_ = json.Unmarshal(data, &out)
	return out
}

// convertOpenAIContentToAnthropic converts OpenAI content (string or array) to Anthropic format.
func convertOpenAIContentToAnthropic(content any) any {
	if _, ok := content.(string); ok {
		return content
	}
	parts, ok := content.([]any)
	if !ok {
		return content
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
			block := map[string]any{"type": "text", "text": part["text"]}
			if cc, ok := part["cache_control"]; ok {
				block["cache_control"] = cc
			}
			result = append(result, block)
		case "image_url":
			if imgURL, ok := part["image_url"].(map[string]any); ok {
				url, _ := imgURL["url"].(string)
				if strings.HasPrefix(url, "data:") {
					mediaType, data := parseDataURL(url)
					result = append(result, map[string]any{
						"type": "image",
						"source": map[string]any{
							"type":       "base64",
							"media_type": mediaType,
							"data":       data,
						},
					})
				} else {
					result = append(result, map[string]any{
						"type": "image",
						"source": map[string]any{
							"type": "url",
							"url":  url,
						},
					})
				}
			}
		default:
			result = append(result, part)
		}
	}
	return result
}

func parseDataURL(dataURL string) (mediaType, data string) {
	after := strings.TrimPrefix(dataURL, "data:")
	parts := strings.SplitN(after, ",", 2)
	if len(parts) != 2 {
		return "application/octet-stream", ""
	}
	mediaType = strings.TrimSuffix(parts[0], ";base64")
	data = parts[1]
	return
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
