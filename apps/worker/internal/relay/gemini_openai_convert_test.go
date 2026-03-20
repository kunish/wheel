package relay

import (
	"encoding/json"
	"testing"
)

func TestConvertOpenAIResponseToGemini_Text(t *testing.T) {
	t.Parallel()
	src, err := json.Marshal(map[string]any{
		"id":      "chatcmpl-x",
		"object":  "chat.completion",
		"created": 1,
		"model":   "gemini-pro",
		"choices": []any{map[string]any{
			"index": 0,
			"message": map[string]any{
				"role":    "assistant",
				"content": "hello",
			},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{
			"prompt_tokens":     3,
			"completion_tokens": 2,
			"total_tokens":      5,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var openai map[string]any
	if err := json.Unmarshal(src, &openai); err != nil {
		t.Fatal(err)
	}
	g := ConvertOpenAIResponseToGemini(openai)
	if g["model"] != "gemini-pro" {
		t.Fatalf("model: %v", g["model"])
	}
	cands, _ := g["candidates"].([]any)
	if len(cands) != 1 {
		t.Fatalf("candidates: %#v", g["candidates"])
	}
	c0, _ := cands[0].(map[string]any)
	content, _ := c0["content"].(map[string]any)
	parts, _ := content["parts"].([]any)
	if len(parts) != 1 {
		t.Fatalf("parts: %#v", parts)
	}
}

func TestConvertOpenAIResponseToGemini_RoundTripStyle(t *testing.T) {
	t.Parallel()
	orig := map[string]any{
		"model": "m1",
		"candidates": []any{map[string]any{
			"index": 0,
			"content": map[string]any{
				"role": "model",
				"parts": []any{map[string]any{
					"text": "hi",
				}},
			},
			"finishReason": "STOP",
		}},
		"usageMetadata": map[string]any{
			"promptTokenCount":     1,
			"candidatesTokenCount": 2,
			"totalTokenCount":      3,
		},
	}
	openai := convertGeminiResponse(orig)
	back := ConvertOpenAIResponseToGemini(openai)
	b, err := json.Marshal(back)
	if err != nil {
		t.Fatal(err)
	}
	var check map[string]any
	if err := json.Unmarshal(b, &check); err != nil {
		t.Fatal(err)
	}
	cands, _ := check["candidates"].([]any)
	if len(cands) != 1 {
		t.Fatalf("expected 1 candidate, got %#v", check["candidates"])
	}
}
