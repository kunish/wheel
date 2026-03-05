package dal

import "testing"

func TestExtractLastMessagePreview(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    string
	}{
		{
			name:    "openai messages picks last text",
			payload: `{"messages":[{"role":"user","content":"hello"},{"role":"user","content":"what is the weather in tokyo today?"}]}`,
			want:    "what is the weather in tokyo today?",
		},
		{
			name:    "multi-part content picks text and image marker",
			payload: `{"messages":[{"role":"user","content":[{"type":"text","text":"describe this"},{"type":"image_url","image_url":{"url":"https://example.com/a.png"}}]}]}`,
			want:    "describe this [image]",
		},
		{
			name:    "tool calls fallback",
			payload: `{"messages":[{"role":"assistant","content":"","tool_calls":[{"id":"1"},{"id":"2"}]}]}`,
			want:    "[2 tool calls]",
		},
		{
			name:    "invalid payload returns empty",
			payload: `not-json`,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractLastMessagePreview(tt.payload)
			if got != tt.want {
				t.Fatalf("extractLastMessagePreview() = %q, want %q", got, tt.want)
			}
		})
	}
}
