package protocol

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"google.golang.org/genai"
)

// Finish reason mapping between providers.

func MapOpenAIFinishReasonToAnthropic(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	case "content_filter":
		return "end_turn"
	case "function_call":
		return "tool_use"
	default:
		return "end_turn"
	}
}

func MapAnthropicStopReasonToOpenAI(reason string) string {
	switch reason {
	case "end_turn", "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	default:
		return "stop"
	}
}

func MapOpenAIFinishReasonToGemini(reason string) genai.FinishReason {
	switch reason {
	case "stop":
		return genai.FinishReasonStop
	case "length":
		return genai.FinishReasonMaxTokens
	case "tool_calls":
		return genai.FinishReasonStop
	case "content_filter":
		return genai.FinishReasonSafety
	default:
		return genai.FinishReasonStop
	}
}

func MapOpenAIFinishReasonToGeminiString(reason string) string {
	switch reason {
	case "stop":
		return "STOP"
	case "length":
		return "MAX_TOKENS"
	case "tool_calls":
		return "STOP"
	case "content_filter":
		return "SAFETY"
	default:
		return "STOP"
	}
}

func MapGeminiFinishReasonToOpenAI(reason string) string {
	switch reason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY":
		return "content_filter"
	default:
		return "stop"
	}
}

func MapGeminiFinishReasonToAnthropic(reason string) string {
	switch reason {
	case "STOP":
		return "end_turn"
	case "MAX_TOKENS":
		return "max_tokens"
	case "SAFETY":
		return "end_turn"
	default:
		return "end_turn"
	}
}

func MapAnthropicStopReasonToGemini(reason string) string {
	switch reason {
	case "end_turn", "stop_sequence":
		return "STOP"
	case "max_tokens":
		return "MAX_TOKENS"
	case "tool_use":
		return "STOP"
	default:
		return "STOP"
	}
}

// ID generation.

func GenOpenAIToolCallID() string {
	return "call_" + randomAlphanumeric(24)
}

func GenAnthropicToolUseID() string {
	return "toolu_" + randomAlphanumeric(24)
}

func randomAlphanumeric(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var b strings.Builder
	for i := 0; i < n; i++ {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		b.WriteByte(letters[idx.Int64()])
	}
	return b.String()
}

// Ptr returns a pointer to the given value.
func Ptr[T any](v T) *T {
	return &v
}

// SafeJSONString marshals any value to a JSON string. Returns "{}" on error.
func SafeJSONString(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// ParseJSONArgs safely parses a JSON arguments string into a map.
func ParseJSONArgs(args string) map[string]any {
	args = strings.TrimSpace(args)
	if args == "" {
		return map[string]any{}
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(args), &result); err != nil {
		return map[string]any{}
	}
	return result
}

// ImageDataURL creates a data URL from media type and base64 data.
func ImageDataURL(mediaType, data string) string {
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	return fmt.Sprintf("data:%s;base64,%s", mediaType, data)
}

// ParseDataURL extracts media type and data from a data URL.
func ParseDataURL(dataURL string) (mediaType, data string) {
	after := strings.TrimPrefix(dataURL, "data:")
	parts := strings.SplitN(after, ",", 2)
	if len(parts) != 2 {
		return "application/octet-stream", ""
	}
	mediaType = strings.TrimSuffix(parts[0], ";base64")
	data = parts[1]
	return
}
