package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/middleware"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// HandleCountTokens handles POST /v1/count-tokens requests.
func (h *RelayHandler) HandleCountTokens(c *gin.Context) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	var req relay.CountTokensRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if req.Model == "" || req.Input == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model and input are required"})
		return
	}

	supportedModels, _ := c.Get("supportedModels")
	sm, _ := supportedModels.(string)
	if !middleware.CheckModelAccess(sm, req.Model) {
		c.JSON(http.StatusForbidden, gin.H{"error": "model not allowed for this API key"})
		return
	}

	apiKeyIDRaw, _ := c.Get("apiKeyId")
	apiKeyID, _ := apiKeyIDRaw.(int)

	var result *relay.CountTokensResponse
	err = h.executeFeatureWithRetry(req.Model, apiKeyID, func(channel *types.Channel, selectedKey *types.ChannelKey, targetModel string) error {
		if channel.Type != types.OutboundAnthropic && channel.Type != types.OutboundBedrock &&
			channel.Type != types.OutboundGemini && channel.Type != types.OutboundVertex {
			result = &relay.CountTokensResponse{
				InputTokens: estimateInputTokens(req.Input),
				Model:       targetModel,
			}
			return nil
		}

		baseURL := relay.SelectBaseUrl([]types.BaseUrl(channel.BaseUrls), channel.Type)
		upstreamURL := baseURL + "/v1/count-tokens"
		headers := map[string]string{"Content-Type": "application/json"}
		upstreamBody := ""

		switch channel.Type {
		case types.OutboundAnthropic, types.OutboundBedrock:
			body, buildErr := buildAnthropicCountTokensBody(targetModel, req.Input)
			if buildErr != nil {
				return buildErr
			}
			upstreamBody = body
			upstreamURL = baseURL + "/v1/messages/count_tokens"
			headers["x-api-key"] = selectedKey.ChannelKey
			headers["anthropic-version"] = "2023-06-01"
		case types.OutboundGemini:
			body, buildErr := buildGeminiCountTokensBody(req.Input)
			if buildErr != nil {
				return buildErr
			}
			upstreamBody = body
			upstreamURL = fmt.Sprintf("%s/v1beta/models/%s:countTokens?key=%s", baseURL, targetModel, selectedKey.ChannelKey)
		case types.OutboundVertex:
			body, buildErr := buildGeminiCountTokensBody(req.Input)
			if buildErr != nil {
				return buildErr
			}
			upstreamBody = body
			project := ""
			location := "us-central1"
			for _, ch := range channel.CustomHeader {
				switch strings.ToLower(ch.Key) {
				case "x-vertex-project":
					project = ch.Value
				case "x-vertex-location":
					location = ch.Value
				}
			}
			if project != "" {
				upstreamURL = fmt.Sprintf("%s/v1/projects/%s/locations/%s/publishers/google/models/%s:countTokens", baseURL, project, location, targetModel)
			} else {
				upstreamURL = fmt.Sprintf("%s/v1beta/models/%s:countTokens", baseURL, targetModel)
			}
			headers["Authorization"] = "Bearer " + selectedKey.ChannelKey
		case types.OutboundAzureOpenAI:
			headers["api-key"] = selectedKey.ChannelKey
		default:
			headers["Authorization"] = "Bearer " + selectedKey.ChannelKey
		}

		for _, ch := range channel.CustomHeader {
			if channel.Type == types.OutboundVertex && strings.HasPrefix(strings.ToLower(ch.Key), "x-vertex-") {
				continue
			}
			headers[ch.Key] = ch.Value
		}

		resp, proxyErr := relay.ProxyCountTokens(h.HTTPClient, upstreamURL, headers, upstreamBody, channel.Type)
		if proxyErr != nil {
			return proxyErr
		}
		resp.Model = targetModel
		result = resp
		return nil
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func estimateInputTokens(input any) int {
	encoded, err := json.Marshal(input)
	if err != nil || len(encoded) == 0 {
		return 0
	}
	return (len(encoded) + 3) / 4
}

func buildAnthropicCountTokensBody(model string, input any) (string, error) {
	messages := make([]map[string]any, 0)

	switch v := input.(type) {
	case string:
		messages = append(messages, map[string]any{"role": "user", "content": v})
	case []any:
		for _, it := range v {
			msg, ok := it.(map[string]any)
			if !ok {
				messages = append(messages, map[string]any{"role": "user", "content": fmt.Sprint(it)})
				continue
			}
			role, _ := msg["role"].(string)
			if role == "" {
				role = "user"
			}
			content, ok := msg["content"]
			if !ok {
				content = ""
			}
			messages = append(messages, map[string]any{"role": role, "content": content})
		}
	default:
		messages = append(messages, map[string]any{"role": "user", "content": fmt.Sprint(v)})
	}

	body := map[string]any{"model": model, "messages": messages}
	encoded, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func buildGeminiCountTokensBody(input any) (string, error) {
	contents := make([]map[string]any, 0)

	appendContent := func(role string, text string) {
		if role == "assistant" {
			role = "model"
		} else if role != "model" {
			role = "user"
		}
		contents = append(contents, map[string]any{
			"role":  role,
			"parts": []map[string]any{{"text": text}},
		})
	}

	switch v := input.(type) {
	case string:
		appendContent("user", v)
	case []any:
		for _, it := range v {
			msg, ok := it.(map[string]any)
			if !ok {
				appendContent("user", fmt.Sprint(it))
				continue
			}
			role, _ := msg["role"].(string)
			text := extractInputText(msg["content"])
			appendContent(role, text)
		}
	default:
		appendContent("user", fmt.Sprint(v))
	}

	body := map[string]any{"contents": contents}
	encoded, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func extractInputText(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, p := range v {
			part, ok := p.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := part["text"].(string); ok {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "")
	default:
		return fmt.Sprint(v)
	}
}
