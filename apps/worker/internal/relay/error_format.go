package relay

// OpenAIErrorBody returns a stable OpenAI-compatible error envelope.
func OpenAIErrorBody(errType, message string) map[string]any {
	return map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    errType,
			"param":   nil,
			"code":    nil,
		},
	}
}
