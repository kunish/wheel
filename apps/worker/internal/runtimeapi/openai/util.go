package openai

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/runtime/sdk/api/handlers"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func RequireOpenAIModel(c *gin.Context, rawJSON []byte) (string, bool) {
	modelName := gjson.GetBytes(rawJSON, "model").String()
	if modelName == "" {
		c.JSON(http.StatusBadRequest, handlers.ErrorResponse{
			Error: handlers.ErrorDetail{
				Message: "Model is required",
				Type:    "invalid_request_error",
			},
		})
		return "", false
	}
	return modelName, true
}

func ShouldTreatAsResponsesFormat(rawJSON []byte) bool {
	if gjson.GetBytes(rawJSON, "messages").Exists() {
		return false
	}
	if gjson.GetBytes(rawJSON, "input").Exists() {
		return true
	}
	if gjson.GetBytes(rawJSON, "instructions").Exists() {
		return true
	}
	return false
}

func WrapResponsesPayloadAsCompleted(payload []byte) []byte {
	if gjson.GetBytes(payload, "type").Exists() {
		return payload
	}
	if gjson.GetBytes(payload, "object").String() != "response" {
		return payload
	}
	wrapped := `{"type":"response.completed","response":{}}`
	wrapped, _ = sjson.SetRaw(wrapped, "response", string(payload))
	return []byte(wrapped)
}

func FormatStreamChunk(chunk []byte) string {
	trimmed := bytes.TrimSpace(chunk)
	if len(trimmed) == 0 {
		return "\n\n"
	}
	if bytes.HasPrefix(trimmed, []byte("data:")) || bytes.HasPrefix(trimmed, []byte("event:")) || bytes.HasPrefix(trimmed, []byte(":")) {
		return string(trimmed) + "\n\n"
	}
	return fmt.Sprintf("data: %s\n\n", trimmed)
}
