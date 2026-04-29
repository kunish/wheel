package handler

import "testing"

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
