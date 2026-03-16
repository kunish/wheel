package handler

import (
	"encoding/json"
	"testing"
)

func TestConvertResponsesBodyToChatCompletions_StringInput(t *testing.T) {
	body := map[string]any{
		"model":  "claude-3-5-sonnet",
		"input":  "hello world",
		"stream": true,
	}
	result := convertResponsesBodyToChatCompletions(body)
	messages, ok := result["messages"].([]any)
	if !ok {
		t.Fatal("expected messages to be []any")
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	msg := messages[0].(map[string]any)
	if msg["role"] != "user" {
		t.Errorf("expected role=user, got %v", msg["role"])
	}
	if msg["content"] != "hello world" {
		t.Errorf("expected content='hello world', got %v", msg["content"])
	}
	// model should be preserved
	if result["model"] != "claude-3-5-sonnet" {
		t.Errorf("expected model preserved, got %v", result["model"])
	}
	// input should NOT be in output
	if _, exists := result["input"]; exists {
		t.Error("input field should not be in converted output")
	}
}

func TestConvertResponsesBodyToChatCompletions_ArrayInput(t *testing.T) {
	body := map[string]any{
		"model": "claude-opus-4-6",
		"input": []any{
			map[string]any{
				"role":    "system",
				"content": "You are a helpful assistant.",
			},
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type": "input_text",
						"text": "hello world",
					},
				},
			},
		},
		"stream": true,
	}
	result := convertResponsesBodyToChatCompletions(body)
	messages, ok := result["messages"].([]any)
	if !ok {
		t.Fatal("expected messages to be []any")
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	// System message
	sysMsg := messages[0].(map[string]any)
	if sysMsg["role"] != "system" {
		t.Errorf("expected first message role=system, got %v", sysMsg["role"])
	}
	if sysMsg["content"] != "You are a helpful assistant." {
		t.Errorf("expected system content, got %v", sysMsg["content"])
	}

	// User message - input_text should be converted to plain text
	userMsg := messages[1].(map[string]any)
	if userMsg["role"] != "user" {
		t.Errorf("expected second message role=user, got %v", userMsg["role"])
	}
	if userMsg["content"] != "hello world" {
		t.Errorf("expected content='hello world', got %v", userMsg["content"])
	}
}

func TestConvertResponsesBodyToChatCompletions_WithInstructions(t *testing.T) {
	body := map[string]any{
		"model":        "claude-3-5-sonnet",
		"instructions": "Be concise.",
		"input":        "hello",
		"stream":       false,
	}
	result := convertResponsesBodyToChatCompletions(body)
	messages, ok := result["messages"].([]any)
	if !ok {
		t.Fatal("expected messages to be []any")
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d", len(messages))
	}

	sysMsg := messages[0].(map[string]any)
	if sysMsg["role"] != "system" {
		t.Errorf("expected first message role=system, got %v", sysMsg["role"])
	}
	if sysMsg["content"] != "Be concise." {
		t.Errorf("expected instructions content, got %v", sysMsg["content"])
	}

	userMsg := messages[1].(map[string]any)
	if userMsg["role"] != "user" {
		t.Errorf("expected second message role=user, got %v", userMsg["role"])
	}
}

func TestConvertResponsesBodyToChatCompletions_FunctionCalls(t *testing.T) {
	body := map[string]any{
		"model": "claude-3-5-sonnet",
		"input": []any{
			map[string]any{
				"role":    "user",
				"content": "Search for repos",
			},
			map[string]any{
				"type":      "function_call",
				"call_id":   "call_123",
				"name":      "search_repos",
				"arguments": `{"query":"mcp"}`,
			},
			map[string]any{
				"type":    "function_call_output",
				"call_id": "call_123",
				"output":  `[{"name":"repo1"}]`,
			},
		},
	}
	result := convertResponsesBodyToChatCompletions(body)
	messages, ok := result["messages"].([]any)
	if !ok {
		t.Fatal("expected messages to be []any")
	}
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(messages))
	}

	// User message
	userMsg := messages[0].(map[string]any)
	if userMsg["role"] != "user" {
		t.Errorf("expected role=user, got %v", userMsg["role"])
	}

	// Assistant with tool_calls
	asstMsg := messages[1].(map[string]any)
	if asstMsg["role"] != "assistant" {
		t.Errorf("expected role=assistant, got %v", asstMsg["role"])
	}
	toolCalls, ok := asstMsg["tool_calls"].([]any)
	if !ok || len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool_call, got %v", asstMsg["tool_calls"])
	}
	tc := toolCalls[0].(map[string]any)
	if tc["id"] != "call_123" {
		t.Errorf("expected call_id=call_123, got %v", tc["id"])
	}

	// Tool response
	toolMsg := messages[2].(map[string]any)
	if toolMsg["role"] != "tool" {
		t.Errorf("expected role=tool, got %v", toolMsg["role"])
	}
	if toolMsg["tool_call_id"] != "call_123" {
		t.Errorf("expected tool_call_id=call_123, got %v", toolMsg["tool_call_id"])
	}
}

func TestConvertResponsesBodyToChatCompletions_ToolsPreserved(t *testing.T) {
	body := map[string]any{
		"model": "claude-3-5-sonnet",
		"input": "hello",
		"tools": []any{
			map[string]any{
				"type": "function",
				"name": "search",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{"type": "string"},
					},
				},
			},
		},
	}
	result := convertResponsesBodyToChatCompletions(body)
	tools, ok := result["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("expected tools to be preserved, got %v", result["tools"])
	}
}

func TestConvertResponsesBodyToChatCompletions_ProducesValidJSON(t *testing.T) {
	body := map[string]any{
		"model": "claude-3-5-sonnet",
		"input": []any{
			map[string]any{
				"role":    "system",
				"content": "You are helpful.",
			},
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "input_text", "text": "hello"},
				},
			},
		},
		"stream": true,
		"tools": []any{
			map[string]any{
				"type": "function",
				"name": "test",
				"parameters": map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
	}
	result := convertResponsesBodyToChatCompletions(body)
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}
	var check map[string]any
	if err := json.Unmarshal(b, &check); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
}
