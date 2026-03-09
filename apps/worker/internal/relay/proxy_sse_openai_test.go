package relay

import (
	"encoding/json"
	"testing"
)

// helper: build an OpenAI SSE chunk as JSON.
func makeOpenAIChunk(id, model string, delta map[string]any, finishReason *string, usage map[string]any) string {
	choice := map[string]any{"index": 0, "delta": delta}
	if finishReason != nil {
		choice["finish_reason"] = *finishReason
	}
	obj := map[string]any{
		"id":      id,
		"object":  "chat.completion.chunk",
		"model":   model,
		"choices": []any{choice},
	}
	if usage != nil {
		obj["usage"] = usage
	}
	b, _ := json.Marshal(obj)
	return string(b)
}

func strPtr(s string) *string { return &s }

// findEvent scans converted lines for a line starting with "data: " that
// unmarshals to an object whose "type" equals the given eventType.
func findEvent(t *testing.T, lines []string, eventType string) map[string]any {
	t.Helper()
	for _, l := range lines {
		if len(l) > 6 && l[:6] == "data: " {
			var obj map[string]any
			if err := json.Unmarshal([]byte(l[6:]), &obj); err == nil {
				if obj["type"] == eventType {
					return obj
				}
			}
		}
	}
	return nil
}

// findAllEvents returns all data objects matching the given eventType.
func findAllEvents(t *testing.T, lines []string, eventType string) []map[string]any {
	t.Helper()
	var results []map[string]any
	for _, l := range lines {
		if len(l) > 6 && l[:6] == "data: " {
			var obj map[string]any
			if err := json.Unmarshal([]byte(l[6:]), &obj); err == nil {
				if obj["type"] == eventType {
					results = append(results, obj)
				}
			}
		}
	}
	return results
}

func TestCreateOpenAIToAnthropicSSEConverter_TextOnly(t *testing.T) {
	convert := createOpenAIToAnthropicSSEConverter()

	// First chunk: should emit message_start + content_block_start + content_block_delta.
	chunk1 := makeOpenAIChunk("chatcmpl-1", "gpt-4", map[string]any{"content": "Hello"}, nil, nil)
	lines1 := convert(chunk1)

	msgStart := findEvent(t, lines1, "message_start")
	if msgStart == nil {
		t.Fatal("expected message_start event")
	}
	msg, _ := msgStart["message"].(map[string]any)
	if msg["id"] != "chatcmpl-1" {
		t.Errorf("message id = %v, want chatcmpl-1", msg["id"])
	}

	blockStart := findEvent(t, lines1, "content_block_start")
	if blockStart == nil {
		t.Fatal("expected content_block_start event")
	}
	cb, _ := blockStart["content_block"].(map[string]any)
	if cb["type"] != "text" {
		t.Errorf("content_block type = %v, want text", cb["type"])
	}

	blockDelta := findEvent(t, lines1, "content_block_delta")
	if blockDelta == nil {
		t.Fatal("expected content_block_delta event")
	}
	d, _ := blockDelta["delta"].(map[string]any)
	if d["type"] != "text_delta" {
		t.Errorf("delta type = %v, want text_delta", d["type"])
	}
	if d["text"] != "Hello" {
		t.Errorf("delta text = %v, want Hello", d["text"])
	}

	// Second chunk: more text, no message_start.
	chunk2 := makeOpenAIChunk("chatcmpl-1", "gpt-4", map[string]any{"content": " world"}, nil, nil)
	lines2 := convert(chunk2)
	if findEvent(t, lines2, "message_start") != nil {
		t.Error("should not emit message_start twice")
	}
	bd2 := findEvent(t, lines2, "content_block_delta")
	if bd2 == nil {
		t.Fatal("expected second content_block_delta")
	}

	// Finish chunk: should emit content_block_stop + message_delta + message_stop.
	chunk3 := makeOpenAIChunk("chatcmpl-1", "gpt-4", map[string]any{}, strPtr("stop"),
		map[string]any{"prompt_tokens": 10, "completion_tokens": 5})
	lines3 := convert(chunk3)

	blockStop := findEvent(t, lines3, "content_block_stop")
	if blockStop == nil {
		t.Fatal("expected content_block_stop event")
	}

	msgDelta := findEvent(t, lines3, "message_delta")
	if msgDelta == nil {
		t.Fatal("expected message_delta event")
	}
	dd, _ := msgDelta["delta"].(map[string]any)
	if dd["stop_reason"] != "end_turn" {
		t.Errorf("stop_reason = %v, want end_turn", dd["stop_reason"])
	}

	msgStop := findEvent(t, lines3, "message_stop")
	if msgStop == nil {
		t.Fatal("expected message_stop event")
	}

	// [DONE] should not produce duplicate events.
	lines4 := convert("[DONE]")
	if findEvent(t, lines4, "message_stop") != nil {
		t.Error("[DONE] should not produce duplicate message_stop")
	}
}

func TestCreateOpenAIToAnthropicSSEConverter_ToolCalls(t *testing.T) {
	convert := createOpenAIToAnthropicSSEConverter()

	// Chunk 1: role:assistant, no content (start of tool call response).
	chunk1 := makeOpenAIChunk("chatcmpl-2", "gpt-4", map[string]any{"role": "assistant"}, nil, nil)
	lines1 := convert(chunk1)
	if findEvent(t, lines1, "message_start") == nil {
		t.Fatal("expected message_start")
	}

	// Chunk 2: first tool call start.
	chunk2 := makeOpenAIChunk("chatcmpl-2", "gpt-4", map[string]any{
		"tool_calls": []any{
			map[string]any{
				"index": 0,
				"id":    "call_abc123",
				"type":  "function",
				"function": map[string]any{
					"name":      "get_weather",
					"arguments": "",
				},
			},
		},
	}, nil, nil)
	lines2 := convert(chunk2)

	toolBlockStart := findEvent(t, lines2, "content_block_start")
	if toolBlockStart == nil {
		t.Fatal("expected content_block_start for tool_use")
	}
	tcb, _ := toolBlockStart["content_block"].(map[string]any)
	if tcb["type"] != "tool_use" {
		t.Errorf("content_block type = %v, want tool_use", tcb["type"])
	}
	if tcb["id"] != "call_abc123" {
		t.Errorf("tool call id = %v, want call_abc123", tcb["id"])
	}
	if tcb["name"] != "get_weather" {
		t.Errorf("tool call name = %v, want get_weather", tcb["name"])
	}

	// Chunk 3: argument fragment.
	chunk3 := makeOpenAIChunk("chatcmpl-2", "gpt-4", map[string]any{
		"tool_calls": []any{
			map[string]any{
				"index":    0,
				"function": map[string]any{"arguments": `{"city"`},
			},
		},
	}, nil, nil)
	lines3 := convert(chunk3)

	argDelta := findEvent(t, lines3, "content_block_delta")
	if argDelta == nil {
		t.Fatal("expected content_block_delta for argument chunk")
	}
	ad, _ := argDelta["delta"].(map[string]any)
	if ad["type"] != "input_json_delta" {
		t.Errorf("delta type = %v, want input_json_delta", ad["type"])
	}
	if ad["partial_json"] != `{"city"` {
		t.Errorf("partial_json = %v, want {\"city\"", ad["partial_json"])
	}

	// Chunk 4: more arguments.
	chunk4 := makeOpenAIChunk("chatcmpl-2", "gpt-4", map[string]any{
		"tool_calls": []any{
			map[string]any{
				"index":    0,
				"function": map[string]any{"arguments": `: "Paris"}`},
			},
		},
	}, nil, nil)
	lines4 := convert(chunk4)
	if findEvent(t, lines4, "content_block_delta") == nil {
		t.Fatal("expected content_block_delta for second argument chunk")
	}

	// Chunk 5: finish with tool_calls reason.
	chunk5 := makeOpenAIChunk("chatcmpl-2", "gpt-4", map[string]any{}, strPtr("tool_calls"),
		map[string]any{"prompt_tokens": 20, "completion_tokens": 15})
	lines5 := convert(chunk5)

	// Should have content_block_stop for the tool block.
	toolBlockStop := findEvent(t, lines5, "content_block_stop")
	if toolBlockStop == nil {
		t.Fatal("expected content_block_stop for tool block")
	}

	// Should have message_delta with stop_reason: tool_use.
	msgDelta := findEvent(t, lines5, "message_delta")
	if msgDelta == nil {
		t.Fatal("expected message_delta")
	}
	dd, _ := msgDelta["delta"].(map[string]any)
	if dd["stop_reason"] != "tool_use" {
		t.Errorf("stop_reason = %v, want tool_use", dd["stop_reason"])
	}

	// message_stop should be present.
	if findEvent(t, lines5, "message_stop") == nil {
		t.Fatal("expected message_stop")
	}

	// [DONE] should not duplicate.
	lines6 := convert("[DONE]")
	if findEvent(t, lines6, "message_stop") != nil {
		t.Error("[DONE] should not produce duplicate message_stop")
	}
}

func TestCreateOpenAIToAnthropicSSEConverter_MultipleToolCalls(t *testing.T) {
	convert := createOpenAIToAnthropicSSEConverter()

	// Start message.
	chunk1 := makeOpenAIChunk("chatcmpl-3", "gpt-4", map[string]any{"role": "assistant"}, nil, nil)
	convert(chunk1)

	// First tool call.
	chunk2 := makeOpenAIChunk("chatcmpl-3", "gpt-4", map[string]any{
		"tool_calls": []any{
			map[string]any{
				"index": 0,
				"id":    "call_1",
				"type":  "function",
				"function": map[string]any{
					"name":      "tool_a",
					"arguments": "",
				},
			},
		},
	}, nil, nil)
	lines2 := convert(chunk2)
	bs1 := findEvent(t, lines2, "content_block_start")
	if bs1 == nil {
		t.Fatal("expected content_block_start for first tool")
	}
	// Block index for first tool should be 1 (0 reserved for text).
	if int(bs1["index"].(float64)) != 1 {
		t.Errorf("first tool block index = %v, want 1", bs1["index"])
	}

	// Second tool call in same chunk.
	chunk3 := makeOpenAIChunk("chatcmpl-3", "gpt-4", map[string]any{
		"tool_calls": []any{
			map[string]any{
				"index": 1,
				"id":    "call_2",
				"type":  "function",
				"function": map[string]any{
					"name":      "tool_b",
					"arguments": `{"x":1}`,
				},
			},
		},
	}, nil, nil)
	lines3 := convert(chunk3)
	bs2 := findEvent(t, lines3, "content_block_start")
	if bs2 == nil {
		t.Fatal("expected content_block_start for second tool")
	}
	if int(bs2["index"].(float64)) != 2 {
		t.Errorf("second tool block index = %v, want 2", bs2["index"])
	}
	// Also should have input_json_delta for second tool's arguments.
	bd := findEvent(t, lines3, "content_block_delta")
	if bd == nil {
		t.Fatal("expected content_block_delta for second tool arguments")
	}

	// Finish.
	chunk4 := makeOpenAIChunk("chatcmpl-3", "gpt-4", map[string]any{}, strPtr("tool_calls"),
		map[string]any{"prompt_tokens": 30, "completion_tokens": 20})
	lines4 := convert(chunk4)

	// Should close both tool blocks.
	stops := findAllEvents(t, lines4, "content_block_stop")
	if len(stops) != 2 {
		t.Errorf("expected 2 content_block_stop events, got %d", len(stops))
	}

	// message_delta with tool_use stop reason.
	msgDelta := findEvent(t, lines4, "message_delta")
	dd, _ := msgDelta["delta"].(map[string]any)
	if dd["stop_reason"] != "tool_use" {
		t.Errorf("stop_reason = %v, want tool_use", dd["stop_reason"])
	}
}

func TestCreateOpenAIToAnthropicSSEConverter_TextThenToolCall(t *testing.T) {
	convert := createOpenAIToAnthropicSSEConverter()

	// Text content first.
	chunk1 := makeOpenAIChunk("chatcmpl-4", "gpt-4", map[string]any{"content": "Let me check"}, nil, nil)
	lines1 := convert(chunk1)
	if findEvent(t, lines1, "content_block_start") == nil {
		t.Fatal("expected text content_block_start")
	}

	// Then tool call: should close text block first.
	chunk2 := makeOpenAIChunk("chatcmpl-4", "gpt-4", map[string]any{
		"tool_calls": []any{
			map[string]any{
				"index": 0,
				"id":    "call_mixed",
				"type":  "function",
				"function": map[string]any{
					"name":      "search",
					"arguments": "",
				},
			},
		},
	}, nil, nil)
	lines2 := convert(chunk2)

	// Should have a content_block_stop (for text) before tool content_block_start.
	textStop := findEvent(t, lines2, "content_block_stop")
	if textStop == nil {
		t.Fatal("expected content_block_stop for text block before tool call")
	}
	if int(textStop["index"].(float64)) != 0 {
		t.Errorf("text block stop index = %v, want 0", textStop["index"])
	}

	toolStart := findEvent(t, lines2, "content_block_start")
	if toolStart == nil {
		t.Fatal("expected content_block_start for tool")
	}
	tcb, _ := toolStart["content_block"].(map[string]any)
	if tcb["type"] != "tool_use" {
		t.Errorf("content_block type = %v, want tool_use", tcb["type"])
	}

	// Finish.
	chunk3 := makeOpenAIChunk("chatcmpl-4", "gpt-4", map[string]any{}, strPtr("tool_calls"), nil)
	lines3 := convert(chunk3)
	if findEvent(t, lines3, "message_stop") == nil {
		t.Fatal("expected message_stop")
	}
}

func TestCreateOpenAIToAnthropicSSEConverter_DoneOnly(t *testing.T) {
	convert := createOpenAIToAnthropicSSEConverter()

	// If no chunks were sent before [DONE], should still produce message_delta + message_stop.
	lines := convert("[DONE]")
	if findEvent(t, lines, "message_delta") == nil {
		t.Error("expected message_delta from [DONE]")
	}
	if findEvent(t, lines, "message_stop") == nil {
		t.Error("expected message_stop from [DONE]")
	}
}
