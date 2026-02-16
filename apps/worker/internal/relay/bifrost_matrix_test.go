package relay

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	schemas "github.com/maximhq/bifrost/core/schemas"
)

func TestNormalizeInboundToOpenAIRequest_Matrix(t *testing.T) {
	tests := []struct {
		name        string
		requestType string
		in          map[string]any
		assert      func(t *testing.T, out map[string]any)
	}{
		{
			name:        "openai passthrough",
			requestType: "openai-chat",
			in: map[string]any{
				"model": "gpt-5",
				"messages": []any{
					map[string]any{"role": "user", "content": "hi"},
				},
				"temperature": 0.1,
			},
			assert: func(t *testing.T, out map[string]any) {
				if out["model"] != "gpt-5" {
					t.Fatalf("model mismatch: %v", out["model"])
				}
				if out["temperature"] != 0.1 {
					t.Fatalf("temperature mismatch: %v", out["temperature"])
				}
			},
		},
		{
			name:        "anthropic with tools and tool_result",
			requestType: "anthropic-messages",
			in: map[string]any{
				"model":       "claude-sonnet",
				"stream":      true,
				"max_tokens":  512,
				"temperature": 0.3,
				"top_p":       0.8,
				"stop_sequences": []any{
					"stop1",
				},
				"system": "system prompt",
				"messages": []any{
					map[string]any{
						"role": "assistant",
						"content": []any{
							map[string]any{"type": "text", "text": "answer"},
							map[string]any{
								"type":  "tool_use",
								"id":    "call_a",
								"name":  "lookup",
								"input": map[string]any{"q": "x"},
							},
						},
					},
					map[string]any{
						"role": "user",
						"content": []any{
							map[string]any{"type": "text", "text": "user msg"},
							map[string]any{"type": "tool_result", "tool_use_id": "call_a", "content": "tool output"},
						},
					},
				},
				"tools": []any{
					map[string]any{
						"name":        "lookup",
						"description": "lookup docs",
						"input_schema": map[string]any{
							"type": "object",
						},
					},
				},
			},
			assert: func(t *testing.T, out map[string]any) {
				if out["model"] != "claude-sonnet" {
					t.Fatalf("model mismatch: %v", out["model"])
				}
				if stream, _ := out["stream"].(bool); !stream {
					t.Fatalf("expected stream=true")
				}
				if mt, _ := readInt(out["max_tokens"]); mt != 512 {
					t.Fatalf("max_tokens mismatch: %v", out["max_tokens"])
				}
				msgs, _ := out["messages"].([]any)
				if len(msgs) < 3 {
					t.Fatalf("expected >=3 converted messages, got %d", len(msgs))
				}
				first, _ := msgs[0].(map[string]any)
				if first["role"] != "system" {
					t.Fatalf("first message should be system")
				}
				assistant, _ := msgs[1].(map[string]any)
				if assistant["role"] != "assistant" {
					t.Fatalf("second message should be assistant")
				}
				toolCalls, _ := assistant["tool_calls"].([]any)
				if len(toolCalls) != 1 {
					t.Fatalf("expected tool_calls=1 got %d", len(toolCalls))
				}
				tools, _ := out["tools"].([]any)
				if len(tools) != 1 {
					t.Fatalf("expected tools=1 got %d", len(tools))
				}
			},
		},
		{
			name:        "gemini with function call and function response",
			requestType: "gemini-generate-content",
			in: map[string]any{
				"system_instruction": map[string]any{
					"parts": []any{
						map[string]any{"text": "sys"},
					},
				},
				"contents": []any{
					map[string]any{
						"role": "user",
						"parts": []any{
							map[string]any{"text": "hi"},
						},
					},
					map[string]any{
						"role": "model",
						"parts": []any{
							map[string]any{
								"functionCall": map[string]any{
									"id":   "fc_1",
									"name": "lookup",
									"args": map[string]any{"k": "v"},
								},
							},
						},
					},
					map[string]any{
						"role": "user",
						"parts": []any{
							map[string]any{
								"functionResponse": map[string]any{
									"name":     "lookup",
									"response": map[string]any{"result": "ok"},
								},
							},
						},
					},
				},
				"generationConfig": map[string]any{
					"temperature":     0.5,
					"topP":            0.7,
					"maxOutputTokens": 256,
					"stopSequences":   []any{"stop"},
				},
				"tools": []any{
					map[string]any{
						"functionDeclarations": []any{
							map[string]any{
								"name":        "lookup",
								"description": "lookup",
								"parameters": map[string]any{
									"type": "object",
								},
							},
						},
					},
				},
			},
			assert: func(t *testing.T, out map[string]any) {
				msgs, _ := out["messages"].([]any)
				if len(msgs) < 4 {
					t.Fatalf("expected >=4 messages, got %d", len(msgs))
				}
				if mt, _ := readInt(out["max_tokens"]); mt != 256 {
					t.Fatalf("max_tokens mismatch: %v", out["max_tokens"])
				}
				if tp, _ := readFloat(out["top_p"]); tp != 0.7 {
					t.Fatalf("top_p mismatch: %v", out["top_p"])
				}
				tools, _ := out["tools"].([]any)
				if len(tools) != 1 {
					t.Fatalf("expected tools=1 got %d", len(tools))
				}
				assistant := msgs[2].(map[string]any)
				toolCalls, _ := assistant["tool_calls"].([]any)
				if len(toolCalls) != 1 {
					t.Fatalf("expected assistant tool_calls=1, got %d", len(toolCalls))
				}
			},
		},
		{
			name:        "gemini stream flag",
			requestType: "gemini-stream-generate-content",
			in: map[string]any{
				"contents": []any{
					map[string]any{
						"role":  "user",
						"parts": []any{map[string]any{"text": "hello"}},
					},
				},
			},
			assert: func(t *testing.T, out map[string]any) {
				if stream, _ := out["stream"].(bool); !stream {
					t.Fatalf("expected stream=true")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := normalizeInboundToOpenAIRequest(tc.requestType, tc.in)
			if err != nil {
				t.Fatalf("normalizeInboundToOpenAIRequest: %v", err)
			}
			tc.assert(t, out)
		})
	}
}

func TestOpenAIPivotToBifrostChatRequest(t *testing.T) {
	openAIBody := map[string]any{
		"messages": []any{
			map[string]any{"role": "system", "content": "sys"},
			map[string]any{"role": "user", "content": "hi"},
		},
		"max_tokens":  1024,
		"temperature": 0.2,
		"top_p":       0.9,
		"stop":        []any{"END"},
		"tool_choice": map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": "lookup",
			},
		},
		"tools": []any{
			map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        "lookup",
					"description": "lookup docs",
					"parameters": map[string]any{
						"type": "object",
					},
				},
			},
		},
		"user": "u-1",
	}

	req, err := openAIToBifrostChatRequest(openAIBody, schemas.OpenAI, "gpt-5.3-codex")
	if err != nil {
		t.Fatalf("openAIToBifrostChatRequest: %v", err)
	}
	if req.Provider != schemas.OpenAI {
		t.Fatalf("provider mismatch: %s", req.Provider)
	}
	if req.Model != "gpt-5.3-codex" {
		t.Fatalf("model mismatch: %s", req.Model)
	}
	if len(req.Input) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(req.Input))
	}
	if req.Params == nil || req.Params.MaxCompletionTokens == nil || *req.Params.MaxCompletionTokens != 1024 {
		t.Fatalf("max_completion_tokens not set correctly")
	}
	if req.Params.ToolChoice == nil {
		t.Fatalf("tool_choice expected")
	}
	if len(req.Params.Tools) != 1 {
		t.Fatalf("expected tools=1, got %d", len(req.Params.Tools))
	}
	if req.Params.User == nil || *req.Params.User != "u-1" {
		t.Fatalf("user mismatch")
	}
}

func TestOpenAIPivotToBifrostChatRequest_Errors(t *testing.T) {
	if _, err := openAIToBifrostChatRequest(map[string]any{}, schemas.OpenAI, "m"); err == nil {
		t.Fatalf("expected error when messages missing")
	}
}

func TestNormalizedToOutboundResponseMatrix(t *testing.T) {
	openAI := map[string]any{
		"id":    "chatcmpl-1",
		"model": "gpt-5.3-codex",
		"choices": []any{
			map[string]any{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": "hello",
					"tool_calls": []any{
						map[string]any{
							"id":   "call_1",
							"type": "function",
							"function": map[string]any{
								"name":      "lookup",
								"arguments": `{"q":"x"}`,
							},
						},
					},
				},
				"finish_reason": "tool_calls",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     11,
			"completion_tokens": 7,
			"total_tokens":      18,
		},
	}

	anth := ConvertToAnthropicResponseFromOpenAI(openAI)
	if anth["type"] != "message" {
		t.Fatalf("anthropic type mismatch: %v", anth["type"])
	}
	content, _ := anth["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("anthropic content expected")
	}
	usage, _ := anth["usage"].(map[string]any)
	if toInt(usage["input_tokens"]) != 11 || toInt(usage["output_tokens"]) != 7 {
		t.Fatalf("anthropic usage mismatch: %v", usage)
	}

	gem := ConvertToGeminiResponseFromOpenAI(openAI)
	candidates, _ := gem["candidates"].([]any)
	if len(candidates) != 1 {
		t.Fatalf("gemini candidates mismatch: %d", len(candidates))
	}
	c0 := candidates[0].(map[string]any)
	if c0["finishReason"] != "STOP" {
		t.Fatalf("gemini finishReason mismatch: %v", c0["finishReason"])
	}
	gUsage, _ := gem["usageMetadata"].(map[string]any)
	if toInt(gUsage["totalTokenCount"]) != 18 {
		t.Fatalf("gemini usage mismatch: %v", gUsage)
	}
}

func TestBifrostChatResponseToOpenAI(t *testing.T) {
	text := "hello"
	role := "assistant"
	finish := "stop"
	resp := &schemas.BifrostChatResponse{
		ID:      "chatcmpl-x",
		Model:   "gpt-5.3-codex",
		Object:  "chat.completion",
		Created: 123,
		Choices: []schemas.BifrostResponseChoice{
			{
				Index:        0,
				FinishReason: &finish,
				ChatNonStreamResponseChoice: &schemas.ChatNonStreamResponseChoice{
					Message: &schemas.ChatMessage{
						Role: schemas.ChatMessageRoleAssistant,
						Content: &schemas.ChatMessageContent{
							ContentStr: &text,
						},
					},
				},
			},
			{
				Index: 1,
				ChatStreamResponseChoice: &schemas.ChatStreamResponseChoice{
					Delta: &schemas.ChatStreamResponseChoiceDelta{
						Role:    &role,
						Content: &text,
					},
				},
			},
		},
		Usage: &schemas.BifrostLLMUsage{
			PromptTokens:     1,
			CompletionTokens: 2,
			TotalTokens:      3,
		},
	}
	out := bifrostChatResponseToOpenAI(resp)
	if out["id"] != "chatcmpl-x" {
		t.Fatalf("id mismatch")
	}
	choices, _ := out["choices"].([]any)
	if len(choices) != 2 {
		t.Fatalf("choices mismatch: %d", len(choices))
	}
	usage, _ := out["usage"].(map[string]any)
	if toInt(usage["total_tokens"]) != 3 {
		t.Fatalf("usage mismatch: %v", usage)
	}
}

func TestStreamRenderers_Matrix(t *testing.T) {
	openAIChunk := map[string]any{
		"id":      "chatcmpl-stream",
		"object":  "chat.completion.chunk",
		"created": 1,
		"model":   "gpt-5.3-codex",
		"choices": []any{
			map[string]any{
				"index": 0,
				"delta": map[string]any{
					"content": "hello",
				},
				"finish_reason": nil,
			},
		},
	}

	// openai stream output
	{
		rec := httptest.NewRecorder()
		if err := writeStreamChunk(rec, rec, "openai-chat", openAIChunk, createOpenAIToAnthropicSSEConverter()); err != nil {
			t.Fatalf("writeStreamChunk openai: %v", err)
		}
		if !strings.Contains(rec.Body.String(), "data: ") {
			t.Fatalf("expected sse data for openai, got %q", rec.Body.String())
		}
		if err := writeStreamDone(rec, rec, "openai-chat", createOpenAIToAnthropicSSEConverter()); err != nil {
			t.Fatalf("writeStreamDone openai: %v", err)
		}
		if !strings.Contains(rec.Body.String(), "[DONE]") {
			t.Fatalf("expected [DONE] marker, got %q", rec.Body.String())
		}
	}

	// anthropic stream output
	{
		rec := httptest.NewRecorder()
		conv := createOpenAIToAnthropicSSEConverter()
		if err := writeStreamChunk(rec, rec, "anthropic-messages", openAIChunk, conv); err != nil {
			t.Fatalf("writeStreamChunk anthropic: %v", err)
		}
		if !strings.Contains(rec.Body.String(), "event: message_start") {
			t.Fatalf("expected anthropic message_start event, got %q", rec.Body.String())
		}
		if err := writeStreamDone(rec, rec, "anthropic-messages", conv); err != nil {
			t.Fatalf("writeStreamDone anthropic: %v", err)
		}
		if !strings.Contains(rec.Body.String(), "event: message_stop") {
			t.Fatalf("expected anthropic message_stop event, got %q", rec.Body.String())
		}
	}

	// anthropic stream output (finish chunk + done) should stop exactly once
	{
		rec := httptest.NewRecorder()
		conv := createOpenAIToAnthropicSSEConverter()

		first := map[string]any{
			"id":      "chatcmpl-stream",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "gpt-5.3-codex",
			"choices": []any{
				map[string]any{
					"index": 0,
					"delta": map[string]any{
						"content": "hello",
					},
					"finish_reason": nil,
				},
			},
		}
		last := map[string]any{
			"id":      "chatcmpl-stream",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "gpt-5.3-codex",
			"choices": []any{
				map[string]any{
					"index":         0,
					"delta":         map[string]any{},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
			},
		}

		if err := writeStreamChunk(rec, rec, "anthropic-messages", first, conv); err != nil {
			t.Fatalf("writeStreamChunk anthropic first: %v", err)
		}
		if err := writeStreamChunk(rec, rec, "anthropic-messages", last, conv); err != nil {
			t.Fatalf("writeStreamChunk anthropic last: %v", err)
		}
		if err := writeStreamDone(rec, rec, "anthropic-messages", conv); err != nil {
			t.Fatalf("writeStreamDone anthropic: %v", err)
		}

		if got := strings.Count(rec.Body.String(), "event: message_stop"); got != 1 {
			t.Fatalf("expected exactly 1 message_stop, got %d, body=%q", got, rec.Body.String())
		}
	}

	// gemini stream output
	{
		rec := httptest.NewRecorder()
		if err := writeStreamChunk(rec, rec, "gemini-stream-generate-content", openAIChunk, createOpenAIToAnthropicSSEConverter()); err != nil {
			t.Fatalf("writeStreamChunk gemini: %v", err)
		}
		if !strings.Contains(rec.Body.String(), "\"candidates\"") {
			t.Fatalf("expected gemini candidates payload, got %q", rec.Body.String())
		}
		if err := writeStreamDone(rec, rec, "gemini-stream-generate-content", createOpenAIToAnthropicSSEConverter()); err != nil {
			t.Fatalf("writeStreamDone gemini: %v", err)
		}
		if !strings.Contains(rec.Body.String(), "{\"done\":true}") {
			t.Fatalf("expected gemini done marker, got %q", rec.Body.String())
		}
	}

	// anthropic tool-use stream output
	{
		rec := httptest.NewRecorder()
		conv := createOpenAIToAnthropicSSEConverter()

		toolStart := map[string]any{
			"id":      "chatcmpl-stream",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "gpt-5.3-codex",
			"choices": []any{
				map[string]any{
					"index": 0,
					"delta": map[string]any{
						"tool_calls": []any{
							map[string]any{
								"index": 0,
								"id":    "call_1",
								"type":  "function",
								"function": map[string]any{
									"name":      "skill",
									"arguments": "{\"x\":",
								},
							},
						},
					},
					"finish_reason": nil,
				},
			},
		}
		toolEnd := map[string]any{
			"id":      "chatcmpl-stream",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "gpt-5.3-codex",
			"choices": []any{
				map[string]any{
					"index": 0,
					"delta": map[string]any{
						"tool_calls": []any{
							map[string]any{
								"index": 0,
								"function": map[string]any{
									"arguments": "1}",
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
			},
		}

		if err := writeStreamChunk(rec, rec, "anthropic-messages", toolStart, conv); err != nil {
			t.Fatalf("writeStreamChunk anthropic tool start: %v", err)
		}
		if err := writeStreamChunk(rec, rec, "anthropic-messages", toolEnd, conv); err != nil {
			t.Fatalf("writeStreamChunk anthropic tool end: %v", err)
		}
		if err := writeStreamDone(rec, rec, "anthropic-messages", conv); err != nil {
			t.Fatalf("writeStreamDone anthropic tool: %v", err)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "\"type\":\"tool_use\"") {
			t.Fatalf("expected tool_use block in anthropic stream, got %q", body)
		}
		if !strings.Contains(body, "\"type\":\"input_json_delta\"") {
			t.Fatalf("expected input_json_delta in anthropic stream, got %q", body)
		}
		if got := strings.Count(body, "event: message_stop"); got != 1 {
			t.Fatalf("expected exactly 1 message_stop, got %d, body=%q", got, body)
		}
	}

	// anthropic stream should degrade to end_turn when tool_calls are malformed (missing name)
	{
		rec := httptest.NewRecorder()
		conv := createOpenAIToAnthropicSSEConverter()
		badTool := map[string]any{
			"id":      "chatcmpl-stream",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "gpt-5.3-codex",
			"choices": []any{
				map[string]any{
					"index": 0,
					"delta": map[string]any{
						"tool_calls": []any{
							map[string]any{
								"index": 0,
								"id":    "call_1",
								"type":  "function",
								"function": map[string]any{
									"arguments": `{"x":1}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
		}
		if err := writeStreamChunk(rec, rec, "anthropic-messages", badTool, conv); err != nil {
			t.Fatalf("writeStreamChunk anthropic malformed tool: %v", err)
		}
		if err := writeStreamDone(rec, rec, "anthropic-messages", conv); err != nil {
			t.Fatalf("writeStreamDone anthropic malformed tool: %v", err)
		}
		body := rec.Body.String()
		if strings.Contains(body, "\"type\":\"tool_use\"") {
			t.Fatalf("did not expect tool_use block for malformed tool chunk, got %q", body)
		}
		if !strings.Contains(body, "\"stop_reason\":\"end_turn\"") {
			t.Fatalf("expected stop_reason end_turn fallback, got %q", body)
		}
	}
}

func TestToGeminiStreamEventFromOpenAI_WithToolCalls(t *testing.T) {
	chunk := map[string]any{
		"choices": []any{
			map[string]any{
				"delta": map[string]any{
					"tool_calls": []any{
						map[string]any{
							"function": map[string]any{
								"name":      "lookup",
								"arguments": `{"q":"x"}`,
							},
						},
					},
				},
				"finish_reason": "tool_calls",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     9,
			"completion_tokens": 3,
			"total_tokens":      12,
		},
	}
	event := toGeminiStreamEventFromOpenAI(chunk)
	if event == nil {
		t.Fatalf("expected gemini event")
	}
	candidates, _ := event["candidates"].([]any)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate")
	}
}

func TestToProxyErrorFromBifrost(t *testing.T) {
	status := 429
	msg := "rate limited"
	err := &schemas.BifrostError{
		StatusCode: &status,
		Error: &schemas.ErrorField{
			Message: msg,
		},
	}
	pe := toProxyErrorFromBifrost(err)
	if pe.StatusCode != 429 || pe.Message != msg {
		t.Fatalf("unexpected proxy error: %+v", pe)
	}
}

func TestExtractRawPayloadForLog(t *testing.T) {
	raw := extractRawPayloadForLog(map[string]any{"a": 1}, map[string]any{"b": 2})
	if raw == nil {
		t.Fatalf("expected raw payload")
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(*raw), &parsed); err != nil {
		t.Fatalf("raw payload is invalid json: %v", err)
	}
	if _, ok := parsed["raw_request"]; !ok {
		t.Fatalf("expected raw_request field")
	}
	if _, ok := parsed["raw_response"]; !ok {
		t.Fatalf("expected raw_response field")
	}
}
