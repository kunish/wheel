package relay

import (
	"encoding/json"
	"testing"
)

// ── Helper ─────────────────────────────────────────────────────

func parseBody(t *testing.T, raw string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	return m
}

// ── ConvertAnthropicResponse → OpenAI ──────────────────────────

func TestConvertAnthropicResponse_TextOnly(t *testing.T) {
	resp := parseBody(t, `{
		"id": "msg_123",
		"model": "claude-3-opus",
		"stop_reason": "end_turn",
		"content": [{"type": "text", "text": "Hello there!"}],
		"usage": {"input_tokens": 10, "output_tokens": 5}
	}`)

	result := ConvertAnthropicResponse(resp)

	if result["id"] != "msg_123" {
		t.Errorf("id = %v", result["id"])
	}
	if result["object"] != "chat.completion" {
		t.Errorf("object = %v", result["object"])
	}
	if result["model"] != "claude-3-opus" {
		t.Errorf("model = %v", result["model"])
	}

	choices := result["choices"].([]any)
	choice := choices[0].(map[string]any)
	msg := choice["message"].(map[string]any)
	if msg["role"] != "assistant" {
		t.Errorf("role = %q", msg["role"])
	}
	if msg["content"] != "Hello there!" {
		t.Errorf("content = %q", msg["content"])
	}
	if choice["finish_reason"] != "stop" {
		t.Errorf("finish_reason = %q", choice["finish_reason"])
	}

	usage := result["usage"].(map[string]any)
	if usage["prompt_tokens"] != 10 {
		t.Errorf("prompt_tokens = %v", usage["prompt_tokens"])
	}
	if usage["completion_tokens"] != 5 {
		t.Errorf("completion_tokens = %v", usage["completion_tokens"])
	}
	if usage["total_tokens"] != 15 {
		t.Errorf("total_tokens = %v", usage["total_tokens"])
	}
}

func TestConvertAnthropicResponse_ToolUse(t *testing.T) {
	resp := parseBody(t, `{
		"id": "msg_456",
		"model": "claude-3-opus",
		"stop_reason": "tool_use",
		"content": [
			{"type": "text", "text": "I'll search for that."},
			{"type": "tool_use", "id": "toolu_1", "name": "search", "input": {"q": "test"}}
		],
		"usage": {"input_tokens": 20, "output_tokens": 15}
	}`)

	result := ConvertAnthropicResponse(resp)

	choices := result["choices"].([]any)
	choice := choices[0].(map[string]any)
	msg := choice["message"].(map[string]any)

	if msg["content"] != "I'll search for that." {
		t.Errorf("content = %q", msg["content"])
	}

	toolCalls := msg["tool_calls"].([]any)
	if len(toolCalls) != 1 {
		t.Fatalf("tool_calls len = %d", len(toolCalls))
	}

	tc := toolCalls[0].(map[string]any)
	if tc["id"] != "toolu_1" {
		t.Errorf("id = %q", tc["id"])
	}
	if tc["type"] != "function" {
		t.Errorf("type = %q", tc["type"])
	}
	fn := tc["function"].(map[string]any)
	if fn["name"] != "search" {
		t.Errorf("name = %q", fn["name"])
	}

	// arguments should be JSON string
	var args map[string]any
	if err := json.Unmarshal([]byte(fn["arguments"].(string)), &args); err != nil {
		t.Fatalf("arguments not valid JSON: %v", err)
	}
	if args["q"] != "test" {
		t.Errorf("args = %v", args)
	}

	if choice["finish_reason"] != "tool_calls" {
		t.Errorf("finish_reason = %q", choice["finish_reason"])
	}
}

func TestConvertAnthropicResponse_ToolUseOnlyNoText(t *testing.T) {
	resp := parseBody(t, `{
		"id": "msg_789",
		"model": "claude-3-opus",
		"stop_reason": "tool_use",
		"content": [
			{"type": "tool_use", "id": "toolu_1", "name": "calc", "input": {"x": 1}}
		],
		"usage": {"input_tokens": 5, "output_tokens": 3}
	}`)

	result := ConvertAnthropicResponse(resp)
	choices := result["choices"].([]any)
	msg := choices[0].(map[string]any)["message"].(map[string]any)

	// When only tool_use, content should be nil
	if msg["content"] != nil {
		t.Errorf("content = %v, want nil", msg["content"])
	}
	if msg["tool_calls"] == nil {
		t.Error("tool_calls missing")
	}
}

func TestConvertAnthropicResponse_EmptyContent(t *testing.T) {
	resp := parseBody(t, `{
		"id": "msg_empty",
		"model": "claude-3-opus",
		"stop_reason": "end_turn",
		"content": [],
		"usage": {"input_tokens": 5, "output_tokens": 0}
	}`)

	result := ConvertAnthropicResponse(resp)
	choices := result["choices"].([]any)
	msg := choices[0].(map[string]any)["message"].(map[string]any)

	if msg["content"] != "" {
		t.Errorf("content = %v, want empty string", msg["content"])
	}
}

func TestConvertAnthropicResponse_MissingID(t *testing.T) {
	resp := parseBody(t, `{
		"model": "claude-3-opus",
		"stop_reason": "end_turn",
		"content": [{"type": "text", "text": "hi"}],
		"usage": {"input_tokens": 1, "output_tokens": 1}
	}`)

	result := ConvertAnthropicResponse(resp)
	if result["id"] != "chatcmpl-unknown" {
		t.Errorf("id = %q, want chatcmpl-unknown", result["id"])
	}
}

func TestConvertAnthropicResponse_MultipleToolUses(t *testing.T) {
	resp := parseBody(t, `{
		"id": "msg_multi",
		"model": "claude-3-opus",
		"stop_reason": "tool_use",
		"content": [
			{"type": "tool_use", "id": "t1", "name": "fn1", "input": {}},
			{"type": "tool_use", "id": "t2", "name": "fn2", "input": {"a": 1}}
		],
		"usage": {"input_tokens": 10, "output_tokens": 10}
	}`)

	result := ConvertAnthropicResponse(resp)
	choices := result["choices"].([]any)
	msg := choices[0].(map[string]any)["message"].(map[string]any)
	toolCalls := msg["tool_calls"].([]any)

	if len(toolCalls) != 2 {
		t.Fatalf("tool_calls len = %d, want 2", len(toolCalls))
	}

	tc0 := toolCalls[0].(map[string]any)
	tc1 := toolCalls[1].(map[string]any)
	// index is stored as int in the map (not JSON roundtripped)
	if tc0["index"] != 0 || tc1["index"] != 1 {
		t.Errorf("indices = %v (%T), %v (%T)", tc0["index"], tc0["index"], tc1["index"], tc1["index"])
	}
}

// ── ConvertToAnthropicResponse (OpenAI → Anthropic) ────────────

func TestConvertToAnthropicResponse_Basic(t *testing.T) {
	resp := parseBody(t, `{
		"id": "chatcmpl-abc",
		"object": "chat.completion",
		"model": "gpt-4o",
		"choices": [{
			"index": 0,
			"message": {"role": "assistant", "content": "Hi!"},
			"finish_reason": "stop"
		}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
	}`)

	result := ConvertToAnthropicResponse(resp)

	if result["id"] != "chatcmpl-abc" {
		t.Errorf("id = %q", result["id"])
	}
	if result["type"] != "message" {
		t.Errorf("type = %q", result["type"])
	}
	if result["role"] != "assistant" {
		t.Errorf("role = %q", result["role"])
	}
	if result["model"] != "gpt-4o" {
		t.Errorf("model = %q", result["model"])
	}

	content := result["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("content blocks = %d", len(content))
	}
	block := content[0].(map[string]any)
	if block["type"] != "text" || block["text"] != "Hi!" {
		t.Errorf("content block = %v", block)
	}

	if result["stop_reason"] != "end_turn" {
		t.Errorf("stop_reason = %q", result["stop_reason"])
	}

	usage := result["usage"].(map[string]any)
	if usage["input_tokens"] != 10 {
		t.Errorf("input_tokens = %v", usage["input_tokens"])
	}
	if usage["output_tokens"] != 5 {
		t.Errorf("output_tokens = %v", usage["output_tokens"])
	}
}

func TestConvertToAnthropicResponse_EmptyChoices(t *testing.T) {
	resp := parseBody(t, `{
		"id": "chatcmpl-abc",
		"choices": [],
		"usage": {"prompt_tokens": 0, "completion_tokens": 0}
	}`)

	result := ConvertToAnthropicResponse(resp)
	content := result["content"].([]any)
	block := content[0].(map[string]any)
	if block["text"] != "" {
		t.Errorf("text = %q, want empty", block["text"])
	}
}

func TestConvertToAnthropicResponse_MissingID(t *testing.T) {
	resp := parseBody(t, `{
		"choices": [{"message": {"content": "hi"}, "finish_reason": "stop"}],
		"usage": {"prompt_tokens": 1, "completion_tokens": 1}
	}`)

	result := ConvertToAnthropicResponse(resp)
	if result["id"] != "msg_unknown" {
		t.Errorf("id = %q, want msg_unknown", result["id"])
	}
}

func TestConvertToAnthropicResponse_WithToolCalls(t *testing.T) {
	resp := parseBody(t, `{
		"id": "chatcmpl-abc",
		"model": "gpt-4o",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": "Let me check that.",
				"tool_calls": [{
					"index": 0,
					"id": "call_123",
					"type": "function",
					"function": {
						"name": "Bash",
						"arguments": "{\"command\": \"ls -la\"}"
					}
				}]
			},
			"finish_reason": "tool_calls"
		}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 20}
	}`)

	result := ConvertToAnthropicResponse(resp)

	if result["id"] != "chatcmpl-abc" {
		t.Errorf("id = %q", result["id"])
	}
	if result["stop_reason"] != "tool_use" {
		t.Errorf("stop_reason = %q, want tool_use", result["stop_reason"])
	}

	content := result["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("content blocks = %d, want 2", len(content))
	}

	// First block should be text
	textBlock := content[0].(map[string]any)
	if textBlock["type"] != "text" {
		t.Errorf("first block type = %q, want text", textBlock["type"])
	}
	if textBlock["text"] != "Let me check that." {
		t.Errorf("text = %q", textBlock["text"])
	}

	// Second block should be tool_use
	toolBlock := content[1].(map[string]any)
	if toolBlock["type"] != "tool_use" {
		t.Errorf("second block type = %q, want tool_use", toolBlock["type"])
	}
	if toolBlock["id"] != "call_123" {
		t.Errorf("tool id = %q", toolBlock["id"])
	}
	if toolBlock["name"] != "Bash" {
		t.Errorf("tool name = %q", toolBlock["name"])
	}
	input := toolBlock["input"].(map[string]any)
	if input["command"] != "ls -la" {
		t.Errorf("command = %q, want ls -la", input["command"])
	}
}

func TestConvertToAnthropicResponse_ToolCallsOnlyNoText(t *testing.T) {
	resp := parseBody(t, `{
		"id": "chatcmpl-xyz",
		"model": "gpt-4o",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": null,
				"tool_calls": [{
					"index": 0,
					"id": "call_456",
					"type": "function",
					"function": {
						"name": "Read",
						"arguments": "{\"file_path\": \"/test.txt\"}"
					}
				}]
			},
			"finish_reason": "tool_calls"
		}],
		"usage": {"prompt_tokens": 5, "completion_tokens": 10}
	}`)

	result := ConvertToAnthropicResponse(resp)

	content := result["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("content blocks = %d, want 1", len(content))
	}

	toolBlock := content[0].(map[string]any)
	if toolBlock["type"] != "tool_use" {
		t.Errorf("block type = %q, want tool_use", toolBlock["type"])
	}
	if toolBlock["name"] != "Read" {
		t.Errorf("tool name = %q", toolBlock["name"])
	}
}

// ── Stop reason mapping ────────────────────────────────────────

func TestMapAnthropicStopReason(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"end_turn", "stop"},
		{"stop_sequence", "stop"},
		{"max_tokens", "length"},
		{"tool_use", "tool_calls"},
		{"unknown_reason", "stop"},
		{"", "stop"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := mapAnthropicStopReason(tt.input); got != tt.want {
				t.Errorf("mapAnthropicStopReason(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMapOpenAIFinishReason(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"stop", "end_turn"},
		{"length", "max_tokens"},
		{"tool_calls", "tool_use"},
		{"content_filter", "end_turn"},
		{"", "end_turn"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := mapOpenAIFinishReason(tt.input); got != tt.want {
				t.Errorf("mapOpenAIFinishReason(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ── toInt ──────────────────────────────────────────────────────

func TestToInt(t *testing.T) {
	tests := []struct {
		name string
		v    any
		want int
	}{
		{"nil", nil, 0},
		{"float64", float64(42), 42},
		{"int", int(7), 7},
		{"int64", int64(99), 99},
		{"string", "not a number", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toInt(tt.v); got != tt.want {
				t.Errorf("toInt(%v) = %d, want %d", tt.v, got, tt.want)
			}
		})
	}
}

// ── copyBody ───────────────────────────────────────────────────

func TestCopyBody_DoesNotMutateOriginal(t *testing.T) {
	original := map[string]any{"model": "gpt-4o", "temperature": 0.5}
	copied := copyBody(original)
	copied["model"] = "changed"
	copied["extra"] = "new"

	if original["model"] != "gpt-4o" {
		t.Error("original mutated")
	}
	if original["extra"] != nil {
		t.Error("original should not have extra key")
	}
}
