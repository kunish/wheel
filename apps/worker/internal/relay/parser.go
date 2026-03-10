package relay

import "strings"

// Request type constants for all supported API endpoints.
const (
	RequestTypeChat            = "openai-chat"
	RequestTypeCompletions     = "openai-completions"
	RequestTypeAnthropicMsg    = "anthropic-messages"
	RequestTypeEmbeddings      = "openai-embeddings"
	RequestTypeResponses       = "openai-responses"
	RequestTypeImageGeneration = "openai-images"
	RequestTypeAudioSpeech     = "openai-audio-speech"
	RequestTypeAudioTranscribe = "openai-audio-transcription"
	RequestTypeAudioTranslate  = "openai-audio-translation"
	RequestTypeModerations     = "openai-moderations"
)

// DetectRequestType determines the inbound request type from the URL path.
func DetectRequestType(path string) string {
	if strings.Contains(path, "/chat/completions") {
		return RequestTypeChat
	}
	if strings.Contains(path, "/completions") {
		return RequestTypeCompletions
	}
	if strings.Contains(path, "/v1/messages") {
		return RequestTypeAnthropicMsg
	}
	if strings.Contains(path, "/embeddings") {
		return RequestTypeEmbeddings
	}
	if strings.Contains(path, "/responses") {
		return RequestTypeResponses
	}
	if strings.Contains(path, "/images/generations") {
		return RequestTypeImageGeneration
	}
	if strings.Contains(path, "/audio/speech") {
		return RequestTypeAudioSpeech
	}
	if strings.Contains(path, "/audio/transcriptions") {
		return RequestTypeAudioTranscribe
	}
	if strings.Contains(path, "/audio/translations") {
		return RequestTypeAudioTranslate
	}
	if strings.Contains(path, "/moderations") {
		return RequestTypeModerations
	}
	return ""
}

// isMultimodalRequest returns true if the request type is a multimodal endpoint
// (images, audio) that may need special handling (binary responses, multipart bodies).
func isMultimodalRequest(requestType string) bool {
	switch requestType {
	case RequestTypeImageGeneration, RequestTypeAudioSpeech,
		RequestTypeAudioTranscribe, RequestTypeAudioTranslate:
		return true
	}
	return false
}

// RequiresMultipartForm returns true for endpoints that accept multipart/form-data.
func RequiresMultipartForm(requestType string) bool {
	switch requestType {
	case RequestTypeAudioTranscribe, RequestTypeAudioTranslate:
		return true
	}
	return false
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
