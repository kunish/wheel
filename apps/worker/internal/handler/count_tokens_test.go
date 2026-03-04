package handler

import (
	"encoding/json"
	"testing"
)

func TestEstimateInputTokens(t *testing.T) {
	tokens := estimateInputTokens(map[string]any{"input": "hello world"})
	if tokens <= 0 {
		t.Fatalf("expected positive token estimate, got %d", tokens)
	}
}

func TestBuildAnthropicCountTokensBody(t *testing.T) {
	body, err := buildAnthropicCountTokensBody("claude-3-5-sonnet", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if payload["model"] != "claude-3-5-sonnet" {
		t.Fatalf("unexpected model: %v", payload["model"])
	}
	messages, ok := payload["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("expected one message, got: %#v", payload["messages"])
	}
	msg, _ := messages[0].(map[string]any)
	if msg["role"] != "user" || msg["content"] != "hello" {
		t.Fatalf("unexpected message: %#v", msg)
	}
}

func TestBuildGeminiCountTokensBody(t *testing.T) {
	input := []any{
		map[string]any{"role": "user", "content": "hi"},
		map[string]any{"role": "assistant", "content": "there"},
	}
	body, err := buildGeminiCountTokensBody(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	contents, ok := payload["contents"].([]any)
	if !ok || len(contents) != 2 {
		t.Fatalf("expected two contents, got: %#v", payload["contents"])
	}

	first, _ := contents[0].(map[string]any)
	if first["role"] != "user" {
		t.Fatalf("expected first role user, got %v", first["role"])
	}
	second, _ := contents[1].(map[string]any)
	if second["role"] != "model" {
		t.Fatalf("expected second role model, got %v", second["role"])
	}
}
