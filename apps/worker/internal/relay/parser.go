package relay

import (
	"net/url"
	"regexp"
	"strings"
)

var geminiModelPathPattern = regexp.MustCompile(`/v1beta/models/([^:]+):(generateContent|streamGenerateContent)$`)

// DetectRequestType determines the inbound request type from the URL path.
func DetectRequestType(path string) string {
	if strings.Contains(path, ":streamGenerateContent") {
		return "gemini-stream-generate-content"
	}
	if strings.Contains(path, ":generateContent") {
		return "gemini-generate-content"
	}
	if strings.Contains(path, "/chat/completions") {
		return "openai-chat"
	}
	if strings.Contains(path, "/v1/messages") {
		return "anthropic-messages"
	}
	if strings.Contains(path, "/embeddings") {
		return "openai-embeddings"
	}
	if strings.Contains(path, "/responses") {
		return "openai-responses"
	}
	return ""
}

// ExtractModel extracts the model name and stream flag from the parsed request body.
func ExtractModel(body map[string]any) (model string, stream bool) {
	if m, ok := body["model"].(string); ok {
		model = m
	}
	if s, ok := body["stream"].(bool); ok {
		stream = s
	}
	return
}

func ExtractModelForRequest(path, requestType string, body map[string]any) (model string, stream bool) {
	switch requestType {
	case "gemini-generate-content", "gemini-stream-generate-content":
		match := geminiModelPathPattern.FindStringSubmatch(path)
		if len(match) == 3 {
			if decoded, err := url.PathUnescape(match[1]); err == nil {
				model = decoded
			} else {
				model = match[1]
			}
		}
		stream = requestType == "gemini-stream-generate-content"
		return
	default:
		return ExtractModel(body)
	}
}
