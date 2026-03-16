package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RegisterNativeProviderRoutes registers provider-native routes that act as
// drop-in replacements for direct provider SDK usage.
// These routes accept provider-native request formats and proxy them through
// Wheel's channel selection and load balancing.
func (h *RelayHandler) RegisterNativeProviderRoutes(r *gin.Engine) {
	// Anthropic native: POST /anthropic/v1/messages
	anthropic := r.Group("/anthropic")
	anthropic.POST("/v1/messages", h.handleAnthropicNative)

	// Gemini native: POST /gemini/v1beta/models/:model:generateContent
	gemini := r.Group("/gemini")
	gemini.POST("/v1beta/models/:model", h.handleGeminiNative)

	// Generic passthrough for any provider: POST /:provider/*path
	// This allows SDKs to use Wheel as a base URL directly
}

// handleAnthropicNative handles Anthropic-format requests at /anthropic/v1/messages.
// It reads the Anthropic request body, extracts the model, and routes through
// Wheel's standard relay pipeline.
func (h *RelayHandler) handleAnthropicNative(c *gin.Context) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"type": "error", "error": gin.H{
			"type":    "invalid_request_error",
			"message": "Failed to read request body",
		}})
		return
	}
	defer c.Request.Body.Close()

	var body map[string]any
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"type": "error", "error": gin.H{
			"type":    "invalid_request_error",
			"message": "Invalid JSON body",
		}})
		return
	}

	model, _ := body["model"].(string)
	if model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"type": "error", "error": gin.H{
			"type":    "invalid_request_error",
			"message": "model is required",
		}})
		return
	}

	// Mark as Anthropic inbound so the relay knows to use passthrough
	c.Set("anthropic_native", true)
	c.Request.Header.Set("anthropic-version", "2023-06-01")

	// Rewrite to standard relay path and let existing handler process it
	c.Request.URL.Path = "/v1/messages"
	c.Request.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
	c.Request.ContentLength = int64(len(bodyBytes))

	h.handleRelay(c)
}

// handleGeminiNative handles Gemini-format requests at /gemini/v1beta/models/:model.
// It extracts the model from the path and routes through Wheel's relay.
func (h *RelayHandler) handleGeminiNative(c *gin.Context) {
	modelParam := c.Param("model")
	// Gemini paths look like: models/gemini-pro:generateContent
	model := strings.TrimSuffix(modelParam, ":generateContent")
	model = strings.TrimSuffix(model, ":streamGenerateContent")

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{
			"code":    400,
			"message": "Failed to read request body",
		}})
		return
	}
	defer c.Request.Body.Close()

	var body map[string]any
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{
			"code":    400,
			"message": "Invalid JSON body",
		}})
		return
	}

	// Convert to OpenAI-compatible format for the relay
	messages := convertGeminiToOpenAI(body)
	stream := strings.Contains(c.Param("model"), ":streamGenerateContent")

	openaiBody := map[string]any{
		"model":    model,
		"messages": messages,
		"stream":   stream,
	}

	newBodyBytes, _ := json.Marshal(openaiBody)
	c.Set("gemini_native", true)
	c.Request.URL.Path = "/v1/chat/completions"
	c.Request.Body = io.NopCloser(strings.NewReader(string(newBodyBytes)))
	c.Request.ContentLength = int64(len(newBodyBytes))

	h.handleRelay(c)
}

// convertGeminiToOpenAI converts Gemini request format to OpenAI messages format.
func convertGeminiToOpenAI(body map[string]any) []map[string]any {
	var messages []map[string]any

	if sysInst, ok := body["system_instruction"].(map[string]any); ok {
		if parts, ok := sysInst["parts"].([]any); ok {
			for _, p := range parts {
				if part, ok := p.(map[string]any); ok {
					if text, ok := part["text"].(string); ok {
						messages = append(messages, map[string]any{
							"role":    "system",
							"content": text,
						})
					}
				}
			}
		}
	}

	contents, ok := body["contents"].([]any)
	if !ok {
		return messages
	}

	for _, c := range contents {
		content, ok := c.(map[string]any)
		if !ok {
			continue
		}
		role, _ := content["role"].(string)
		openaiRole := "user"
		if role == "model" {
			openaiRole = "assistant"
		}

		parts, ok := content["parts"].([]any)
		if !ok {
			continue
		}

		var textParts []string
		for _, p := range parts {
			part, ok := p.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := part["text"].(string); ok {
				textParts = append(textParts, text)
			}
		}

		if len(textParts) > 0 {
			messages = append(messages, map[string]any{
				"role":    openaiRole,
				"content": fmt.Sprintf("%s", strings.Join(textParts, "\n")),
			})
		}
	}

	return messages
}
