package relay

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ── Anthropic SSE Converter ──────────────────────────────────────

type anthropicSSEResult struct {
	done                bool
	data                map[string]any
	inputTokens         int
	outputTokens        int
	cacheReadTokens     int
	cacheCreationTokens int
}

func mapStopReason(reason string) *string {
	var r string
	switch reason {
	case "end_turn", "stop_sequence":
		r = "stop"
	case "max_tokens":
		r = "length"
	default:
		return nil
	}
	return &r
}

// createAnthropicSSEConverter returns a stateful converter from Anthropic SSE to OpenAI SSE format.
func createAnthropicSSEConverter() func(string) *anthropicSSEResult {
	msgId := "chatcmpl-unknown"
	msgModel := ""
	toolCallIndex := 0
	blockMap := make(map[int]struct {
		blockType   string
		toolCallIdx int
	})

	makeChunk := func(choices []any, extra *anthropicSSEResult) *anthropicSSEResult {
		result := &anthropicSSEResult{
			data: map[string]any{
				"id":      msgId,
				"object":  "chat.completion.chunk",
				"created": float64(currentUnixSec()),
				"model":   msgModel,
				"choices": choices,
			},
		}
		if extra != nil {
			result.inputTokens = extra.inputTokens
			result.outputTokens = extra.outputTokens
			result.cacheReadTokens = extra.cacheReadTokens
			result.cacheCreationTokens = extra.cacheCreationTokens
		}
		return result
	}

	return func(jsonStr string) *anthropicSSEResult {
		var event map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &event); err != nil {
			return nil
		}

		evType, _ := event["type"].(string)

		switch evType {
		case "message_start":
			msg, _ := event["message"].(map[string]any)
			if msg != nil {
				if id, ok := msg["id"].(string); ok {
					msgId = id
				}
				if model, ok := msg["model"].(string); ok {
					msgModel = model
				}
			}
			var cr, cc, inTok int
			if msg != nil {
				if usage, ok := msg["usage"].(map[string]any); ok {
					cr = toInt(usage["cache_read_input_tokens"])
					cc = toInt(usage["cache_creation_input_tokens"])
					inTok = toInt(usage["input_tokens"])
				}
			}
			return makeChunk(
				[]any{map[string]any{
					"index":         0,
					"delta":         map[string]any{"role": "assistant", "content": ""},
					"finish_reason": nil,
				}},
				&anthropicSSEResult{cacheReadTokens: cr, cacheCreationTokens: cc, inputTokens: inTok},
			)

		case "content_block_start":
			idx := toInt(event["index"])
			block, _ := event["content_block"].(map[string]any)
			if block == nil {
				return nil
			}
			blockType, _ := block["type"].(string)

			if blockType == "tool_use" {
				tcIdx := toolCallIndex
				toolCallIndex++
				blockMap[idx] = struct {
					blockType   string
					toolCallIdx int
				}{"tool_use", tcIdx}

				return makeChunk([]any{map[string]any{
					"index": 0,
					"delta": map[string]any{
						"tool_calls": []any{map[string]any{
							"index":    tcIdx,
							"id":       block["id"],
							"type":     "function",
							"function": map[string]any{"name": block["name"], "arguments": ""},
						}},
					},
					"finish_reason": nil,
				}}, nil)
			}

			blockMap[idx] = struct {
				blockType   string
				toolCallIdx int
			}{blockType, 0}
			return nil

		case "content_block_delta":
			idx := toInt(event["index"])
			delta, _ := event["delta"].(map[string]any)
			if delta == nil {
				return nil
			}
			deltaType, _ := delta["type"].(string)

			if deltaType == "text_delta" {
				text, _ := delta["text"].(string)
				return makeChunk([]any{map[string]any{
					"index":         0,
					"delta":         map[string]any{"content": text},
					"finish_reason": nil,
				}}, nil)
			}

			if deltaType == "input_json_delta" {
				info, ok := blockMap[idx]
				if ok && info.blockType == "tool_use" {
					partialJSON, _ := delta["partial_json"].(string)
					return makeChunk([]any{map[string]any{
						"index": 0,
						"delta": map[string]any{
							"tool_calls": []any{map[string]any{
								"index":    info.toolCallIdx,
								"function": map[string]any{"arguments": partialJSON},
							}},
						},
						"finish_reason": nil,
					}}, nil)
				}
			}

			// thinking_delta — silently drop
			return nil

		case "message_delta":
			delta, _ := event["delta"].(map[string]any)
			usage, _ := event["usage"].(map[string]any)

			stopReason, _ := delta["stop_reason"].(string)
			var finishReason any
			if stopReason == "tool_use" {
				finishReason = "tool_calls"
			} else if r := mapStopReason(stopReason); r != nil {
				finishReason = *r
			}

			result := &anthropicSSEResult{
				data: map[string]any{
					"id":      msgId,
					"object":  "chat.completion.chunk",
					"created": float64(currentUnixSec()),
					"model":   msgModel,
					"choices": []any{map[string]any{
						"index":         0,
						"delta":         map[string]any{},
						"finish_reason": finishReason,
					}},
				},
			}

			if usage != nil {
				inTok := toInt(usage["input_tokens"])
				outTok := toInt(usage["output_tokens"])
				if inTok > 0 || outTok > 0 {
					result.inputTokens = inTok
					result.outputTokens = outTok
				}
			}
			return result

		case "message_stop":
			return &anthropicSSEResult{done: true}

		default:
			return nil
		}
	}
}

// extractThinking parses an Anthropic SSE JSON payload and accumulates thinking content.
func extractThinking(jsonStr string, state *streamingState) {
	var ev map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &ev); err != nil {
		return
	}
	evType, _ := ev["type"].(string)
	if evType != "content_block_delta" {
		return
	}
	delta, ok := ev["delta"].(map[string]any)
	if !ok {
		return
	}
	if delta["type"] == "thinking_delta" {
		if text, ok := delta["thinking"].(string); ok {
			state.appendThinking(text)
		}
	}
}

// processAnthropicPassthrough handles SSE lines in Anthropic passthrough mode.
func processAnthropicPassthrough(line string, state *streamingState, markFirstToken func()) {
	if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
		return
	}

	var ev map[string]any
	if err := json.Unmarshal([]byte(line[6:]), &ev); err != nil {
		return
	}

	evType, _ := ev["type"].(string)

	if !state.firstTokenReceived {
		if evType == "message_start" || evType == "content_block_start" || evType == "content_block_delta" {
			markFirstToken()
		}
	}

	if evType == "message_start" {
		if msg, ok := ev["message"].(map[string]any); ok {
			if usage, ok := msg["usage"].(map[string]any); ok {
				if inTok := toInt(usage["input_tokens"]); inTok > 0 {
					state.inputTokens = inTok
				}
				state.cacheReadTokens = toInt(usage["cache_read_input_tokens"])
				state.cacheCreationTokens = toInt(usage["cache_creation_input_tokens"])
			}
		}
	}

	if evType == "message_delta" {
		if usage, ok := ev["usage"].(map[string]any); ok {
			inTok := toInt(usage["input_tokens"])
			outTok := toInt(usage["output_tokens"])
			if inTok > 0 {
				state.inputTokens = inTok
			}
			if outTok > 0 {
				state.outputTokens = outTok
			}
		}
	}

	if evType == "content_block_delta" {
		if delta, ok := ev["delta"].(map[string]any); ok {
			if delta["type"] == "text_delta" {
				if text, ok := delta["text"].(string); ok {
					state.appendContent(text)
				}
			}
			if delta["type"] == "thinking_delta" {
				if text, ok := delta["thinking"].(string); ok {
					state.appendThinking(text)
				}
			}
		}
	}
}

// processAnthropicConverted handles SSE lines by converting Anthropic → OpenAI format.
func processAnthropicConverted(
	line string,
	convertChunk func(string) *anthropicSSEResult,
	state *streamingState,
	markFirstToken func(),
	w http.ResponseWriter,
	flusher http.Flusher,
) {
	if !strings.HasPrefix(line, "data: ") {
		return
	}

	payload := line[6:]

	// Extract thinking content before conversion (converter drops thinking_delta)
	extractThinking(payload, state)

	chunk := convertChunk(payload)
	if chunk == nil {
		return
	}

	markFirstToken()

	if chunk.cacheReadTokens > 0 {
		state.cacheReadTokens = chunk.cacheReadTokens
	}
	if chunk.cacheCreationTokens > 0 {
		state.cacheCreationTokens = chunk.cacheCreationTokens
	}

	if chunk.inputTokens > 0 {
		state.inputTokens = chunk.inputTokens
	}
	if chunk.outputTokens > 0 {
		state.outputTokens = chunk.outputTokens
	}

	if chunk.done {
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	} else if chunk.data != nil {
		// Accumulate text content
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
