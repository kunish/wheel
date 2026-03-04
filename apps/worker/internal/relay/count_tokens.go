package relay

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

// CountTokensRequest represents a token counting request.
type CountTokensRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"` // Can be string or messages array
}

// CountTokensResponse represents the token counting response.
type CountTokensResponse struct {
	InputTokens int    `json:"input_tokens"`
	Model       string `json:"model"`
}

// ProxyCountTokens forwards a count tokens request to the upstream provider.
// Different providers have different endpoints:
// - OpenAI: Not directly supported, we estimate
// - Anthropic: POST /v1/messages/count_tokens
// - Gemini: POST /v1beta/models/{model}:countTokens
func ProxyCountTokens(
	client *http.Client,
	upstreamUrl string,
	upstreamHeaders map[string]string,
	body string,
	channelType types.OutboundType,
) (*CountTokensResponse, error) {
	req, err := http.NewRequest("POST", upstreamUrl, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create count tokens request: %w", err)
	}
	for k, v := range upstreamHeaders {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream count tokens request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("failed to read count tokens response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &ProxyError{
			Message:    fmt.Sprintf("Upstream count tokens error %d: %s", resp.StatusCode, string(bodyBytes)),
			StatusCode: resp.StatusCode,
		}
	}

	var data map[string]any
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return nil, fmt.Errorf("failed to parse count tokens response: %w", err)
	}

	// Normalize response from different providers
	result := &CountTokensResponse{}
	switch channelType {
	case types.OutboundAnthropic:
		result.InputTokens = toInt(data["input_tokens"])
	case types.OutboundGemini, types.OutboundVertex:
		result.InputTokens = toInt(data["totalTokens"])
	default:
		// OpenAI-compatible: try various field names
		if usage, ok := data["usage"].(map[string]any); ok {
			result.InputTokens = toInt(usage["prompt_tokens"])
		} else {
			result.InputTokens = toInt(data["total_tokens"])
			if result.InputTokens == 0 {
				result.InputTokens = toInt(data["input_tokens"])
			}
		}
	}

	return result, nil
}
