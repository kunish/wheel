package relay

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// processOpenAI handles SSE lines in OpenAI passthrough mode.
func processOpenAI(
	line string,
	state *streamingState,
	markFirstToken func(),
	w http.ResponseWriter,
	flusher http.Flusher,
) {
	if strings.HasPrefix(line, "data: ") && line != "data: [DONE]" {
		markFirstToken()

		var obj map[string]any
		if err := json.Unmarshal([]byte(line[6:]), &obj); err == nil {
			if usage, ok := obj["usage"].(map[string]any); ok {
				state.inputTokens = toIntOr(usage["prompt_tokens"], state.inputTokens)
				state.outputTokens = toIntOr(usage["completion_tokens"], state.outputTokens)
				if details, ok := usage["prompt_tokens_details"].(map[string]any); ok {
					if ct := toInt(details["cached_tokens"]); ct > 0 {
						state.cacheReadTokens = ct
					}
				}
			}
			// Accumulate text content
			if choices, ok := obj["choices"].([]any); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]any); ok {
					if delta, ok := choice["delta"].(map[string]any); ok {
						if content, ok := delta["content"].(string); ok {
							state.appendContent(content)
						}
						if reasoning, ok := delta["reasoning_content"].(string); ok {
							state.appendThinking(reasoning)
						}
					}
				}
			}
		}
	}

	fmt.Fprintf(w, "%s\n", line)
	flusher.Flush()
}

func toIntOr(v any, fallback int) int {
	n := toInt(v)
	if n > 0 {
		return n
	}
	return fallback
}

// ── OpenAI SSE → Anthropic SSE Converter ──────────────────────────

// CreateOpenAIToAnthropicSSEConverter creates a stateful converter
// from OpenAI SSE chunks to Anthropic SSE event lines.
// Exported for use by CopilotRelay and other direct-proxy paths.
func CreateOpenAIToAnthropicSSEConverter() func(string) []string {
	return createOpenAIToAnthropicSSEConverter()
}

// createOpenAIToAnthropicSSEConverter returns a stateful converter
// from OpenAI SSE chunks to Anthropic SSE event lines.
func createOpenAIToAnthropicSSEConverter() func(string) []string {
	started := false
	contentBlockOpen := false
	msgId := "msg_unknown"
	msgModel := ""

	return func(jsonStr string) []string {
		if jsonStr == "[DONE]" {
			var lines []string
			if contentBlockOpen {
				evt := map[string]any{
					"type":  "content_block_stop",
					"index": 0,
				}
				b, _ := json.Marshal(evt)
				lines = append(lines,
					"event: content_block_stop",
					"data: "+string(b),
					"",
				)
				contentBlockOpen = false
			}
			delta := map[string]any{
				"type":  "message_delta",
				"delta": map[string]any{"stop_reason": "end_turn", "stop_sequence": nil},
				"usage": map[string]any{"output_tokens": 0},
			}
			b, _ := json.Marshal(delta)
			lines = append(lines,
				"event: message_delta",
				"data: "+string(b),
				"",
			)
			stop := map[string]any{"type": "message_stop"}
			b, _ = json.Marshal(stop)
			lines = append(lines,
				"event: message_stop",
				"data: "+string(b),
				"",
			)
			return lines
		}

		var obj map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &obj); err != nil {
			return nil
		}

		if id, ok := obj["id"].(string); ok {
			msgId = id
		}
		if model, ok := obj["model"].(string); ok {
			msgModel = model
		}

		var lines []string

		if !started {
			started = true
			inputTokens := 0
			if usage, ok := obj["usage"].(map[string]any); ok {
				inputTokens = toInt(usage["prompt_tokens"])
			}
			lines = append(lines, openaiToAnthropicStart(msgId, msgModel, inputTokens)...)
		}

		choices, _ := obj["choices"].([]any)
		if len(choices) == 0 {
			if len(lines) > 0 {
				return lines
			}
			return nil
		}

		choice, _ := choices[0].(map[string]any)
		if choice == nil {
			return lines
		}

		delta, _ := choice["delta"].(map[string]any)
		finishReason, _ := choice["finish_reason"].(string)

		if delta != nil {
			content, _ := delta["content"].(string)

			if content != "" {
				if !contentBlockOpen {
					lines = append(lines,
						openaiToAnthropicBlockStart(0, "text")...)
					contentBlockOpen = true
				}
				evt := map[string]any{
					"type":  "content_block_delta",
					"index": 0,
					"delta": map[string]any{
						"type": "text_delta",
						"text": content,
					},
				}
				b, _ := json.Marshal(evt)
				lines = append(lines,
					"event: content_block_delta",
					"data: "+string(b),
					"",
				)
			}
		}

		if finishReason != "" {
			if contentBlockOpen {
				evt := map[string]any{
					"type":  "content_block_stop",
					"index": 0,
				}
				b, _ := json.Marshal(evt)
				lines = append(lines,
					"event: content_block_stop",
					"data: "+string(b),
					"",
				)
				contentBlockOpen = false
			}

			usage, _ := obj["usage"].(map[string]any)
			inTok := toInt(usage["prompt_tokens"])
			outTok := toInt(usage["completion_tokens"])

			stopReason := mapOpenAIFinishReason(finishReason)
			md := map[string]any{
				"type":  "message_delta",
				"delta": map[string]any{"stop_reason": stopReason, "stop_sequence": nil},
				"usage": map[string]any{
					"input_tokens":  inTok,
					"output_tokens": outTok,
				},
			}
			b, _ := json.Marshal(md)
			lines = append(lines,
				"event: message_delta",
				"data: "+string(b),
				"",
			)

			stop := map[string]any{"type": "message_stop"}
			b, _ = json.Marshal(stop)
			lines = append(lines,
				"event: message_stop",
				"data: "+string(b),
				"",
			)
		}

		if len(lines) > 0 {
			return lines
		}
		return nil
	}
}

func openaiToAnthropicStart(id, model string, inputTokens int) []string {
	msg := map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            id,
			"type":          "message",
			"role":          "assistant",
			"model":         model,
			"content":       []any{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage":         map[string]any{"input_tokens": inputTokens, "output_tokens": 0},
		},
	}
	b, _ := json.Marshal(msg)
	return []string{
		"event: message_start",
		"data: " + string(b),
		"",
	}
}

func openaiToAnthropicBlockStart(index int, blockType string) []string {
	evt := map[string]any{
		"type":  "content_block_start",
		"index": index,
		"content_block": map[string]any{
			"type": blockType,
			"text": "",
		},
	}
	b, _ := json.Marshal(evt)
	return []string{
		"event: content_block_start",
		"data: " + string(b),
		"",
	}
}

func processOpenAIToAnthropic(
	line string,
	convert func(string) []string,
	state *streamingState,
	markFirstToken func(),
	w http.ResponseWriter,
	flusher http.Flusher,
) {
	if !strings.HasPrefix(line, "data: ") {
		return
	}
	data := strings.TrimPrefix(line, "data: ")

	var obj map[string]any
	if data != "[DONE]" {
		if err := json.Unmarshal([]byte(data), &obj); err == nil {
			if usage, ok := obj["usage"].(map[string]any); ok {
				state.inputTokens = toIntOr(
					usage["prompt_tokens"], state.inputTokens)
				state.outputTokens = toIntOr(
					usage["completion_tokens"], state.outputTokens)
			}
			if choices, ok := obj["choices"].([]any); ok && len(choices) > 0 {
				if ch, ok := choices[0].(map[string]any); ok {
					if d, ok := ch["delta"].(map[string]any); ok {
						if c, ok := d["content"].(string); ok {
							state.appendContent(c)
						}
					}
				}
			}
		}
	}

	markFirstToken()

	converted := convert(data)
	if converted == nil {
		return
	}
	for _, l := range converted {
		fmt.Fprintf(w, "%s\n", l)
	}
	flusher.Flush()
}
