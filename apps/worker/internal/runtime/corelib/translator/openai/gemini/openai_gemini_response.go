// Package gemini provides response translation functionality for OpenAI to Gemini API.
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kunish/wheel/apps/worker/internal/protocol"
)

// ConvertOpenAIResponseToGemini converts OpenAI Chat Completions streaming response to Gemini API format.
func ConvertOpenAIResponseToGemini(_ context.Context, _ string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) []string {
	if *param == nil {
		accum := protocol.NewOpenAIToGeminiAccum()
		*param = accum
	}

	if strings.TrimSpace(string(rawJSON)) == "[DONE]" {
		return []string{}
	}

	if bytes.HasPrefix(rawJSON, []byte("data:")) {
		rawJSON = bytes.TrimSpace(rawJSON[5:])
	}

	var chunk protocol.OpenAIChatResponse
	if err := json.Unmarshal(rawJSON, &chunk); err != nil {
		return []string{}
	}

	accum := (*param).(*protocol.OpenAIToGeminiAccum)
	return protocol.ConvertOpenAIChunkToGemini(&chunk, accum)
}

// ConvertOpenAIResponseToGeminiNonStream converts a non-streaming OpenAI response to Gemini format.
func ConvertOpenAIResponseToGeminiNonStream(_ context.Context, _ string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, _ *any) string {
	var resp protocol.OpenAIChatResponse
	if err := json.Unmarshal(rawJSON, &resp); err != nil {
		return string(rawJSON)
	}

	geminiResp := protocol.OpenAIResponseToGemini(&resp)

	result, err := json.Marshal(geminiResp)
	if err != nil {
		return string(rawJSON)
	}
	return string(result)
}

func GeminiTokenCount(ctx context.Context, count int64) string {
	return fmt.Sprintf(`{"totalTokens":%d,"promptTokensDetails":[{"modality":"TEXT","tokenCount":%d}]}`, count, count)
}
