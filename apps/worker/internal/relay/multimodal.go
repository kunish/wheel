package relay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"sort"
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
	if channel.Type == types.OutboundAzureOpenAI {
		return buildAzureOpenAIRequest(baseUrl, key, body, path, model, channel)
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	headers["Authorization"] = "Bearer " + key
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

// BuildMultipartUpstreamBody rebuilds multipart/form-data requests after model
// rewriting and param overrides have been applied while preserving file parts.
func BuildMultipartUpstreamBody(
	contentType string,
	bodyBytes []byte,
	body map[string]any,
	model string,
	paramOverride *string,
) ([]byte, string, error) {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.EqualFold(mediaType, "multipart/form-data") {
		return nil, "", fmt.Errorf("unsupported content type")
	}
	boundary := params["boundary"]
	if boundary == "" {
		return nil, "", fmt.Errorf("invalid multipart form data")
	}

	outBody := copyBody(body)
	outBody["model"] = model
	applyParamOverrides(outBody, paramOverride)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	keys := make([]string, 0, len(outBody))
	for key := range outBody {
		if key == "file" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := writeMultipartField(writer, key, outBody[key]); err != nil {
			return nil, "", err
		}
	}

	reader := multipart.NewReader(bytes.NewReader(bodyBytes), boundary)
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", err
		}
		if part.FileName() == "" {
			continue
		}

		hdr := make(map[string][]string, len(part.Header))
		for key, values := range part.Header {
			hdr[key] = append([]string(nil), values...)
		}
		dst, err := writer.CreatePart(hdr)
		if err != nil {
			return nil, "", err
		}
		if _, err := io.Copy(dst, part); err != nil {
			return nil, "", err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, "", err
	}

	return buf.Bytes(), writer.FormDataContentType(), nil
}

func writeMultipartField(writer *multipart.Writer, key string, value any) error {
	switch values := value.(type) {
	case []any:
		for _, item := range values {
			if err := writer.WriteField(key, stringifyMultipartFieldValue(item)); err != nil {
				return err
			}
		}
		return nil
	case []string:
		for _, item := range values {
			if err := writer.WriteField(key, item); err != nil {
				return err
			}
		}
		return nil
	default:
		return writer.WriteField(key, stringifyMultipartFieldValue(value))
	}
}

func stringifyMultipartFieldValue(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	encoded, err := json.Marshal(value)
	if err == nil {
		return string(encoded)
	}
	return fmt.Sprint(value)
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
		Response:        result,
		StatusCode:      resp.StatusCode,
		UpstreamHeaders: resp.Header.Clone(),
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

// RequiresEndpointAwareExecution returns true for request types that need
// dedicated multimodal execution instead of the generic JSON proxy flow.
func RequiresEndpointAwareExecution(requestType string) bool {
	return IsMultimodalRequest(requestType) || requestType == RequestTypeModerations
}

// ShouldUseMultimodalExecution returns true when the request should use the
// OpenAI-compatible multimodal execution path for the selected channel.
func ShouldUseMultimodalExecution(requestType string, channelType types.OutboundType) bool {
	if !RequiresEndpointAwareExecution(requestType) {
		return false
	}
	for _, supportedType := range multimodalChannelTypes() {
		if channelType == supportedType {
			return true
		}
	}
	return false
}

// IsDeferredExecutionUnsupported returns true for multimedia endpoints that are
// not yet supported by async/batch background execution.
func IsDeferredExecutionUnsupported(requestType string) bool {
	switch requestType {
	case RequestTypeAudioSpeech, RequestTypeAudioTranscribe, RequestTypeAudioTranslate:
		return true
	}
	return false
}

// ExtractMultimodalModel extracts the model from multimodal request bodies.
// Some endpoints like audio/transcriptions use "model" in form data.
// The handler decides when endpoint defaults are allowed.
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

// AllowsDefaultMultimodalModel returns true when the handler may infer a model.
func AllowsDefaultMultimodalModel(requestType string) bool {
	switch requestType {
	case RequestTypeImageGeneration, RequestTypeAudioSpeech, RequestTypeModerations:
		return true
	}
	return false
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
