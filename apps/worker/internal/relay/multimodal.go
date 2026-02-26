package relay

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

// multimodalPathMap maps request types to upstream API paths.
var multimodalPathMap = map[string]string{
	RequestTypeImageGeneration: "/v1/images/generations",
	RequestTypeAudioSpeech:     "/v1/audio/speech",
	RequestTypeAudioTranscribe: "/v1/audio/transcriptions",
	RequestTypeAudioTranslate:  "/v1/audio/translations",
	RequestTypeModerations:     "/v1/moderations",
}

// BuildMultimodalUpstreamRequest builds the upstream request for multimodal endpoints.
// These endpoints are OpenAI-compatible and use Bearer token auth.
func BuildMultimodalUpstreamRequest(
	channel ChannelConfig,
	key string,
	body map[string]any,
	model string,
	requestType string,
) UpstreamRequest {
	baseUrl := SelectBaseUrl(channel.BaseUrls, channel.Type)

	path, ok := multimodalPathMap[requestType]
	if !ok {
		path = "/v1/chat/completions"
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

// ProxyMultimodal proxies a multimodal request and returns the result.
// For JSON responses (images, transcriptions, moderations), it returns a ProxyResult.
// For binary responses (audio/speech), use ProxyBinaryResponse instead.
func ProxyMultimodal(
	client *http.Client,
	url string,
	headers map[string]string,
	body string,
	requestType string,
) (*ProxyResult, *ProxyError) {
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return nil, &ProxyError{Message: fmt.Sprintf("Failed to create request: %v", err), StatusCode: 500}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, &ProxyError{Message: fmt.Sprintf("Upstream request failed: %v", err), StatusCode: 502}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
	if err != nil {
		return nil, &ProxyError{Message: fmt.Sprintf("Failed to read response: %v", err), StatusCode: 502}
	}

	if resp.StatusCode >= 400 {
		msg := string(respBody)
		if len(msg) > 500 {
			msg = msg[:500]
		}
		return nil, &ProxyError{
			Message:    fmt.Sprintf("Upstream error %d: %s", resp.StatusCode, msg),
			StatusCode: resp.StatusCode,
		}
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, &ProxyError{Message: "Failed to parse upstream JSON response", StatusCode: 502}
	}

	// Extract token usage if present
	pr := &ProxyResult{
		Response:   result,
		StatusCode: resp.StatusCode,
	}
	if usage, ok := result["usage"].(map[string]any); ok {
		pr.InputTokens = toInt(usage["prompt_tokens"])
		pr.OutputTokens = toInt(usage["completion_tokens"])
	}

	return pr, nil
}

// ProxyBinaryResponse proxies a request that returns binary data (e.g., audio/speech).
// It streams the response body directly to the HTTP writer.
func ProxyBinaryResponse(
	w http.ResponseWriter,
	client *http.Client,
	url string,
	headers map[string]string,
	body string,
) *ProxyError {
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return &ProxyError{Message: fmt.Sprintf("Failed to create request: %v", err), StatusCode: 500}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return &ProxyError{Message: fmt.Sprintf("Upstream request failed: %v", err), StatusCode: 502}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return &ProxyError{
			Message:    fmt.Sprintf("Upstream error %d: %s", resp.StatusCode, string(respBody)),
			StatusCode: resp.StatusCode,
		}
	}

	// Copy response headers
	for _, h := range []string{"Content-Type", "Content-Length", "Content-Disposition"} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	io.Copy(w, resp.Body)
	return nil
}

// IsAudioBinaryResponse returns true if the request type produces a binary audio response.
func IsAudioBinaryResponse(requestType string) bool {
	return requestType == RequestTypeAudioSpeech
}

// ExtractMultimodalModel extracts the model from multimodal request bodies.
// Some endpoints like audio/transcriptions use "model" in form data,
// but we normalize everything to JSON in the gateway.
func ExtractMultimodalModel(body map[string]any, requestType string) string {
	if m, ok := body["model"].(string); ok && m != "" {
		return m
	}
	// Default models per endpoint type
	switch requestType {
	case RequestTypeImageGeneration:
		return "dall-e-3"
	case RequestTypeAudioSpeech:
		return "tts-1"
	case RequestTypeAudioTranscribe, RequestTypeAudioTranslate:
		return "whisper-1"
	case RequestTypeModerations:
		return "omni-moderation-latest"
	}
	return ""
}

// multimodalChannelTypes lists channel types that support multimodal endpoints.
func multimodalChannelTypes() []types.OutboundType {
	return []types.OutboundType{
		types.OutboundOpenAI,
		types.OutboundAzureOpenAI,
		types.OutboundOpenRouter,
		types.OutboundTogether,
	}
}
