package handler

import (
	"strings"
	"testing"
)

func TestCursorParseToolCalls(t *testing.T) {
	raw := "Hello\n\n```json action\n" + `{
  "tool": "Read",
  "parameters": { "file_path": "a.go" }
}
` + "```\n"
	clean, tools := cursorParseToolCalls(raw)
	if len(tools) != 1 || tools[0].Name != "Read" {
		t.Fatalf("tools=%v", tools)
	}
	fp, _ := tools[0].Args["file_path"].(string)
	if fp != "a.go" {
		t.Fatalf("args=%v", tools[0].Args)
	}
	if clean != "Hello" {
		t.Fatalf("clean=%q", clean)
	}
}

func TestCursorComChatAPIModel(t *testing.T) {
	if got := cursorComChatAPIModel("claude-4.5-sonnet"); got != "anthropic/claude-4.5-sonnet" {
		t.Fatal(got)
	}
	if got := cursorComChatAPIModel("anthropic/foo"); got != "anthropic/foo" {
		t.Fatal(got)
	}
}

func TestOpenAIToolChoiceFunctionMapsToAnthropicToolChoice(t *testing.T) {
	body := map[string]any{
		"tool_choice": map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": "Read",
			},
		},
	}
	anth := openAIChatBodyToAnthropicShape(body)
	tc, ok := anth["tool_choice"].(map[string]any)
	if !ok {
		t.Fatalf("tool_choice=%#v", anth["tool_choice"])
	}
	if tc["type"] != "tool" || tc["name"] != "Read" {
		t.Fatalf("tool_choice=%#v", tc)
	}
	if got := cursorToolChoiceConstraint(tc); got == "" {
		t.Fatal("expected named tool_choice to produce a constraint")
	}
}

func TestAnthropicBodyToCursorComChatTreatsOpenAIToolRoleAsUserResult(t *testing.T) {
	anth := map[string]any{
		"messages": []any{
			map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{
				"function": map[string]any{"name": "Read", "arguments": `{"file_path":"a.go"}`},
			}}},
			map[string]any{"role": "tool", "content": "file contents"},
		},
	}
	chat, err := anthropicBodyToCursorComChat(anth, "anthropic/test")
	if err != nil {
		t.Fatal(err)
	}
	msgs, _ := chat["messages"].([]map[string]any)
	if len(msgs) != 2 {
		t.Fatalf("messages=%#v", chat["messages"])
	}
	if msgs[1]["role"] != "user" {
		t.Fatalf("tool result should be sent as user message, got %#v", msgs[1])
	}
	parts, _ := msgs[1]["parts"].([]any)
	part, _ := parts[0].(map[string]any)
	text, _ := part["text"].(string)
	if text == "" || !strings.Contains(text, "Action output") || !strings.Contains(text, "file contents") {
		t.Fatalf("tool result text=%q", text)
	}
}
