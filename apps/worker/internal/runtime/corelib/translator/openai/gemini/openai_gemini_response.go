// Package gemini provides response translation functionality for OpenAI to Gemini API.
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openai/openai-go"

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

	accum := (*param).(*protocol.OpenAIToGeminiAccum)
	return protocol.ConvertOpenAIChunkToGemini(rawJSON, accum)
}

// ConvertOpenAIResponseToGeminiNonStream converts a non-streaming OpenAI response to Gemini format.
func ConvertOpenAIResponseToGeminiNonStream(_ context.Context, _ string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, _ *any) string {
	var resp openai.ChatCompletion
	if err := json.Unmarshal(rawJSON, &resp); err != nil {
		return string(rawJSON)
	}

	geminiResp := openaiCompletionToGeminiMap(&resp)
	result, err := json.Marshal(geminiResp)
	if err != nil {
		return string(rawJSON)
	}
	return string(result)
}

func openaiCompletionToGeminiMap(resp *openai.ChatCompletion) map[string]any {
	result := map[string]any{
		"model": resp.Model,
	}

	var candidates []any
	for _, choice := range resp.Choices {
		var parts []any
		if choice.Message.Content != "" {
			parts = append(parts, map[string]any{"text": choice.Message.Content})
		}
		for _, tc := range choice.Message.ToolCalls {
			args := protocol.ParseJSONArgs(tc.Function.Arguments)
			parts = append(parts, map[string]any{
				"functionCall": map[string]any{
					"name": tc.Function.Name,
					"args": args,
				},
			})
		}

		candidate := map[string]any{
			"content":      map[string]any{"parts": parts, "role": "model"},
			"index":        int(choice.Index),
			"finishReason": protocol.MapOpenAIFinishReasonToGeminiString(string(choice.FinishReason)),
		}
		candidates = append(candidates, candidate)
	}
	result["candidates"] = candidates

	result["usageMetadata"] = map[string]any{
		"promptTokenCount":     resp.Usage.PromptTokens,
		"candidatesTokenCount": resp.Usage.CompletionTokens,
		"totalTokenCount":      resp.Usage.TotalTokens,
	}

	return result
}

func GeminiTokenCount(ctx context.Context, count int64) string {
	return fmt.Sprintf(`{"totalTokens":%d,"promptTokensDetails":[{"modality":"TEXT","tokenCount":%d}]}`, count, count)
}
