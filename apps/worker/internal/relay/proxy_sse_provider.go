package relay

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ── Gemini SSE Converter ──────────────────────────────────────

func createGeminiSSEConverter() func(string) *anthropicSSEResult {
	started := false

	return func(jsonStr string) *anthropicSSEResult {
		var resp map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
			return nil
		}

		candidates, _ := resp["candidates"].([]any)
		if len(candidates) == 0 {
			return nil
		}

		cand, _ := candidates[0].(map[string]any)
		finishReason, _ := cand["finishReason"].(string)

		var text string
		var toolCalls []any
		if content, ok := cand["content"].(map[string]any); ok {
			parts, _ := content["parts"].([]any)
			for _, p := range parts {
				part, ok := p.(map[string]any)
				if !ok {
					continue
				}
				if t, ok := part["text"].(string); ok {
					text += t
				}
				if fc, ok := part["functionCall"].(map[string]any); ok {
					name, _ := fc["name"].(string)
					argsJSON, _ := json.Marshal(fc["args"])
					toolCalls = append(toolCalls, map[string]any{
						"index": 0, "id": fmt.Sprintf("call_%s", name),
						"type":     "function",
						"function": map[string]any{"name": name, "arguments": string(argsJSON)},
					})
				}
			}
		}

		result := &anthropicSSEResult{}
		if usage, ok := resp["usageMetadata"].(map[string]any); ok {
			result.inputTokens = toInt(usage["promptTokenCount"])
			result.outputTokens = toInt(usage["candidatesTokenCount"])
			result.cacheReadTokens = toInt(usage["cachedContentTokenCount"])
		}

		delta := map[string]any{}
		if !started {
			started = true
			delta["role"] = "assistant"
		}
		if text != "" {
			delta["content"] = text
		}
		if len(toolCalls) > 0 {
			delta["tool_calls"] = toolCalls
		}

		var fr any
		if finishReason != "" {
			fr = mapGeminiFinishReason(finishReason)
		}

		result.data = map[string]any{
			"id": "chatcmpl-gemini", "object": "chat.completion.chunk",
			"created": float64(currentUnixSec()), "model": resp["modelVersion"],
			"choices": []any{map[string]any{
				"index": 0, "delta": delta, "finish_reason": fr,
			}},
		}
		return result
	}
}

// processConvertedSSE handles SSE lines by converting provider-specific format → OpenAI format.
// This is a unified implementation that replaces the previously duplicated
// processGeminiConverted and processCohereConverted functions.
func processConvertedSSE(
	line string,
	convert func(string) *anthropicSSEResult,
	state *streamingState,
	markFirstToken func(),
	w http.ResponseWriter,
	flusher http.Flusher,
) {
	if !strings.HasPrefix(line, "data: ") {
		return
	}

	chunk := convert(line[6:])
	if chunk == nil {
		return
	}

	markFirstToken()

	if chunk.inputTokens > 0 {
		state.inputTokens = chunk.inputTokens
	}
	if chunk.outputTokens > 0 {
		state.outputTokens = chunk.outputTokens
	}
	if chunk.cacheReadTokens > 0 {
		state.cacheReadTokens = chunk.cacheReadTokens
	}

	if chunk.data != nil {
		if choices, ok := chunk.data["choices"].([]any); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]any); ok {
				if delta, ok := choice["delta"].(map[string]any); ok {
					if content, ok := delta["content"].(string); ok {
						state.appendContent(content)
					}
				}
			}
		}
		dataJSON, _ := json.Marshal(chunk.data)
		fmt.Fprintf(w, "data: %s\n\n", dataJSON)
		flusher.Flush()
	}
}
