package relay

import "strings"

// DetectRequestType determines the inbound request type from the URL path.
func DetectRequestType(path string) string {
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
func ExtractModel(body map[string]interface{}) (model string, stream bool) {
	if m, ok := body["model"].(string); ok {
		model = m
	}
	if s, ok := body["stream"].(bool); ok {
		stream = s
	}
	return
}
