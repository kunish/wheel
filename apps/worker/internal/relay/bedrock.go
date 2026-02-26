package relay

import (
	"encoding/json"
	"fmt"
	"strings"
)

// buildBedrockRequest builds a request for AWS Bedrock.
// Bedrock uses Anthropic Messages API format for Claude models,
// and a different format for other models. For simplicity, we support
// the OpenAI-compatible proxy mode (via Bedrock Access Gateway or LiteLLM).
//
// If the base URL points to a Bedrock-compatible proxy (e.g. bedrock-access-gateway),
// we send OpenAI-format requests. For native Bedrock, we convert to Anthropic format.
func buildBedrockRequest(baseUrl, key string, body map[string]any, model string, channel ChannelConfig) UpstreamRequest {
	// Strip "bedrock/" prefix if present
	bedrockModel := strings.TrimPrefix(model, "bedrock/")

	// Determine if this is a Claude model (uses Anthropic Messages API)
	isClaude := strings.Contains(strings.ToLower(bedrockModel), "claude") ||
		strings.Contains(strings.ToLower(bedrockModel), "anthropic")

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// If key looks like "ACCESS_KEY:SECRET_KEY" or "ACCESS_KEY:SECRET_KEY:SESSION_TOKEN",
	// we pass it as Authorization header for proxy mode
	if key != "" {
		headers["Authorization"] = "Bearer " + key
	}

	for _, h := range channel.CustomHeader {
		headers[h.Key] = h.Value
	}

	if isClaude {
		return buildBedrockClaudeRequest(baseUrl, headers, body, bedrockModel, channel)
	}

	// For non-Claude models, use OpenAI-compatible format (proxy mode)
	return buildBedrockOpenAICompatRequest(baseUrl, headers, body, bedrockModel, channel)
}

// buildBedrockClaudeRequest converts OpenAI messages to Anthropic Messages API format for Bedrock.
func buildBedrockClaudeRequest(baseUrl string, headers map[string]string, body map[string]any, model string, channel ChannelConfig) UpstreamRequest {
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
		case "system", "developer":
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
	}

	bedrockBody := map[string]any{
		"anthropic_version": "bedrock-2023-05-31",
		"messages":          anthropicMessages,
		"max_tokens":        maxTokens,
	}

	if systemMsg != "" {
		bedrockBody["system"] = systemMsg
	}
	if s, ok := body["stream"].(bool); ok && s {
		bedrockBody["stream"] = true
	}
	if t, ok := body["temperature"]; ok {
		bedrockBody["temperature"] = t
	}
	if tp, ok := body["top_p"]; ok {
		bedrockBody["top_p"] = tp
	}
	if stop, ok := body["stop"]; ok {
		bedrockBody["stop_sequences"] = stop
	}
	if tools, ok := body["tools"].([]any); ok {
		bedrockBody["tools"] = convertOpenAITools(tools)
	}

	applyParamOverrides(bedrockBody, channel.ParamOverride)

	stream, _ := body["stream"].(bool)
	var endpoint string
	if stream {
		endpoint = fmt.Sprintf("/model/%s/invoke-with-response-stream", model)
	} else {
		endpoint = fmt.Sprintf("/model/%s/invoke", model)
	}

	bodyJSON, _ := json.Marshal(bedrockBody)
	return UpstreamRequest{
		URL:     baseUrl + endpoint,
		Headers: headers,
		Body:    string(bodyJSON),
	}
}

// buildBedrockOpenAICompatRequest sends OpenAI-format requests for Bedrock proxy mode.
func buildBedrockOpenAICompatRequest(baseUrl string, headers map[string]string, body map[string]any, model string, channel ChannelConfig) UpstreamRequest {
	outBody := copyBody(body)
	outBody["model"] = model
	applyParamOverrides(outBody, channel.ParamOverride)

	bodyJSON, _ := json.Marshal(outBody)
	return UpstreamRequest{
		URL:     baseUrl + "/v1/chat/completions",
		Headers: headers,
		Body:    string(bodyJSON),
	}
}
