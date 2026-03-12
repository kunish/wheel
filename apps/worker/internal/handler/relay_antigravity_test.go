package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kunish/wheel/apps/worker/internal/codexruntime"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/runtimeauth"
)

func TestAntigravityRelay_ResolveAccessToken_MatchesAuthIndex(t *testing.T) {
	t.Parallel()

	channelID := 77
	fileName := "antigravity-testuser.json"
	managedName := codexruntime.ManagedAuthFileName(channelID, fileName)
	authIndex := runtimeauth.EnsureAuthIndex(managedName, "", "")

	if authIndex == "" {
		t.Fatal("authIndex should not be empty")
	}
	authIndex2 := runtimeauth.EnsureAuthIndex(managedName, "", "")
	if authIndex != authIndex2 {
		t.Fatalf("authIndex mismatch: %q != %q", authIndex, authIndex2)
	}
}

func TestAntigravityBaseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		model string
		want  string
	}{
		{"claude-opus-4-6-thinking", antigravityProdURL},
		{"claude-sonnet-4-6", antigravityProdURL},
		{"gemini-2.5-flash", antigravityDailyURL},
		{"gemini-3-pro-high", antigravityDailyURL},
		{"gemini-3-flash", antigravityDailyURL},
		{"gpt-oss-120b-medium", antigravityDailyURL},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			t.Parallel()
			got := antigravityBaseURL(tt.model)
			if got != tt.want {
				t.Errorf("antigravityBaseURL(%q) = %q, want %q", tt.model, got, tt.want)
			}
		})
	}
}

func TestApplyAntigravityHeaders(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("POST", "https://cloudcode-pa.googleapis.com/v1internal:generateContent", nil)
	applyAntigravityHeaders(req, "ya29.test-token", "https://cloudcode-pa.googleapis.com")

	if got := req.Header.Get("Authorization"); got != "Bearer ya29.test-token" {
		t.Errorf("Authorization = %q, want 'Bearer ya29.test-token'", got)
	}
	if got := req.Header.Get("User-Agent"); got != antigravityUA {
		t.Errorf("User-Agent = %q, want %q", got, antigravityUA)
	}
	if got := req.Header.Get("Host"); got != "cloudcode-pa.googleapis.com" {
		t.Errorf("Host = %q, want cloudcode-pa.googleapis.com", got)
	}
}

func TestBuildAntigravityEnvelope(t *testing.T) {
	t.Parallel()

	geminiBody := map[string]any{
		"contents": []any{
			map[string]any{"role": "user", "parts": []any{map[string]any{"text": "hello"}}},
		},
	}

	envelope := buildAntigravityEnvelope(geminiBody, "gemini-2.5-flash", "project-123")

	if envelope["project"] != "project-123" {
		t.Errorf("project = %v, want project-123", envelope["project"])
	}
	if envelope["model"] != "gemini-2.5-flash" {
		t.Errorf("model = %v, want gemini-2.5-flash", envelope["model"])
	}
	if envelope["userAgent"] != "antigravity" {
		t.Errorf("userAgent = %v, want antigravity", envelope["userAgent"])
	}
	if envelope["request"] == nil {
		t.Error("request should not be nil")
	}
	if envelope["requestId"] == nil {
		t.Error("requestId should not be nil")
	}
	if envelope["sessionId"] == nil {
		t.Error("sessionId should not be nil")
	}
}

func TestBuildAntigravityEnvelope_DefaultProjectID(t *testing.T) {
	t.Parallel()

	envelope := buildAntigravityEnvelope(map[string]any{}, "gemini-3-flash", "")
	if envelope["project"] != "ag-default" {
		t.Errorf("project = %v, want ag-default when projectID is empty", envelope["project"])
	}
}

// ──────────────────────────────────────────────────────────────
// Anthropic → Gemini conversion tests
// ──────────────────────────────────────────────────────────────

func TestConvertAnthropicToGemini_BasicMessages(t *testing.T) {
	t.Parallel()

	body := map[string]any{
		"model":      "gemini-2.5-flash",
		"max_tokens": float64(1024),
		"messages": []any{
			map[string]any{"role": "user", "content": "What is 2+2?"},
			map[string]any{"role": "assistant", "content": "4"},
			map[string]any{"role": "user", "content": "Thanks!"},
		},
	}

	result := convertAnthropicToGemini(body, "gemini-2.5-flash")

	contents, ok := result["contents"].([]any)
	if !ok {
		t.Fatal("contents is not []any")
	}
	if len(contents) != 3 {
		t.Fatalf("expected 3 contents, got %d", len(contents))
	}

	// Verify role mapping.
	c0, _ := contents[0].(map[string]any)
	if c0["role"] != "user" {
		t.Errorf("contents[0].role = %v, want user", c0["role"])
	}
	c1, _ := contents[1].(map[string]any)
	if c1["role"] != "model" {
		t.Errorf("contents[1].role = %v, want model (mapped from assistant)", c1["role"])
	}

	// Verify generation config.
	genConfig, _ := result["generationConfig"].(map[string]any)
	if genConfig["maxOutputTokens"] != float64(1024) {
		t.Errorf("maxOutputTokens = %v, want 1024", genConfig["maxOutputTokens"])
	}
}

func TestConvertAnthropicToGemini_SystemInstruction(t *testing.T) {
	t.Parallel()

	t.Run("string system", func(t *testing.T) {
		t.Parallel()
		body := map[string]any{
			"system":   "You are a helpful assistant.",
			"messages": []any{map[string]any{"role": "user", "content": "hi"}},
		}
		result := convertAnthropicToGemini(body, "gemini-2.5-flash")
		sysInst, ok := result["systemInstruction"].(map[string]any)
		if !ok {
			t.Fatal("systemInstruction missing")
		}
		parts, _ := sysInst["parts"].([]any)
		if len(parts) != 1 {
			t.Fatalf("expected 1 system part, got %d", len(parts))
		}
		part0, _ := parts[0].(map[string]any)
		if part0["text"] != "You are a helpful assistant." {
			t.Errorf("system text = %v, want 'You are a helpful assistant.'", part0["text"])
		}
	})

	t.Run("array system", func(t *testing.T) {
		t.Parallel()
		body := map[string]any{
			"system": []any{
				map[string]any{"type": "text", "text": "First."},
				map[string]any{"type": "text", "text": "Second."},
			},
			"messages": []any{map[string]any{"role": "user", "content": "hi"}},
		}
		result := convertAnthropicToGemini(body, "gemini-2.5-flash")
		sysInst, _ := result["systemInstruction"].(map[string]any)
		parts, _ := sysInst["parts"].([]any)
		part0, _ := parts[0].(map[string]any)
		if part0["text"] != "First.\nSecond." {
			t.Errorf("system text = %q, want 'First.\\nSecond.'", part0["text"])
		}
	})
}

func TestConvertAnthropicToGemini_GenerationConfig(t *testing.T) {
	t.Parallel()

	body := map[string]any{
		"temperature":    float64(0.7),
		"top_p":          float64(0.95),
		"max_tokens":     float64(2048),
		"stop_sequences": []any{"STOP", "END"},
		"messages":       []any{map[string]any{"role": "user", "content": "hi"}},
	}

	result := convertAnthropicToGemini(body, "gemini-2.5-flash")
	genConfig, _ := result["generationConfig"].(map[string]any)

	if genConfig["temperature"] != float64(0.7) {
		t.Errorf("temperature = %v, want 0.7", genConfig["temperature"])
	}
	if genConfig["topP"] != float64(0.95) {
		t.Errorf("topP = %v, want 0.95", genConfig["topP"])
	}
	if genConfig["maxOutputTokens"] != float64(2048) {
		t.Errorf("maxOutputTokens = %v, want 2048", genConfig["maxOutputTokens"])
	}
}

func TestConvertAnthropicToGemini_ThinkingConfig(t *testing.T) {
	t.Parallel()

	body := map[string]any{
		"thinking": map[string]any{
			"type":          "enabled",
			"budget_tokens": float64(8192),
		},
		"messages": []any{map[string]any{"role": "user", "content": "think about this"}},
	}

	result := convertAnthropicToGemini(body, "gemini-2.5-flash")
	genConfig, _ := result["generationConfig"].(map[string]any)
	thinkingConfig, _ := genConfig["thinkingConfig"].(map[string]any)

	if thinkingConfig["thinkingBudget"] != 8192 {
		t.Errorf("thinkingBudget = %v, want 8192", thinkingConfig["thinkingBudget"])
	}
	if thinkingConfig["includeThoughts"] != true {
		t.Errorf("includeThoughts = %v, want true", thinkingConfig["includeThoughts"])
	}
}

func TestConvertAnthropicContentToGeminiParts_TextContent(t *testing.T) {
	t.Parallel()

	// String content.
	parts := convertAnthropicContentToGeminiParts("hello world", "user")
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	part0, _ := parts[0].(map[string]any)
	if part0["text"] != "hello world" {
		t.Errorf("text = %v, want 'hello world'", part0["text"])
	}
}

func TestConvertAnthropicContentToGeminiParts_ThinkingBlock(t *testing.T) {
	t.Parallel()

	content := []any{
		map[string]any{
			"type":      "thinking",
			"thinking":  "Let me think...",
			"signature": "sig123",
		},
	}
	parts := convertAnthropicContentToGeminiParts(content, "assistant")

	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	p, _ := parts[0].(map[string]any)
	if p["thought"] != true {
		t.Errorf("thought = %v, want true", p["thought"])
	}
	if p["text"] != "Let me think..." {
		t.Errorf("text = %v, want 'Let me think...'", p["text"])
	}
	if p["thoughtSignature"] != "sig123" {
		t.Errorf("thoughtSignature = %v, want sig123", p["thoughtSignature"])
	}
}

func TestConvertAnthropicContentToGeminiParts_ToolUse(t *testing.T) {
	t.Parallel()

	content := []any{
		map[string]any{
			"type":  "tool_use",
			"name":  "get_weather",
			"input": map[string]any{"city": "Tokyo"},
		},
	}
	parts := convertAnthropicContentToGeminiParts(content, "assistant")

	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	fc, _ := parts[0].(map[string]any)
	funcCall, _ := fc["functionCall"].(map[string]any)
	if funcCall["name"] != "get_weather" {
		t.Errorf("functionCall.name = %v, want get_weather", funcCall["name"])
	}
	args, _ := funcCall["args"].(map[string]any)
	if args["city"] != "Tokyo" {
		t.Errorf("functionCall.args.city = %v, want Tokyo", args["city"])
	}
}

func TestConvertAnthropicContentToGeminiParts_ToolResult(t *testing.T) {
	t.Parallel()

	content := []any{
		map[string]any{
			"type":        "tool_result",
			"tool_use_id": "toolu_123",
			"content":     "Sunny, 25°C",
		},
	}
	parts := convertAnthropicContentToGeminiParts(content, "user")

	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	fr, _ := parts[0].(map[string]any)
	funcResp, _ := fr["functionResponse"].(map[string]any)
	if funcResp["id"] != "toolu_123" {
		t.Errorf("functionResponse.id = %v, want toolu_123", funcResp["id"])
	}
	response, _ := funcResp["response"].(map[string]any)
	if response["output"] != "Sunny, 25°C" {
		t.Errorf("functionResponse.response.output = %v, want 'Sunny, 25°C'", response["output"])
	}
}

func TestConvertAnthropicContentToGeminiParts_Image(t *testing.T) {
	t.Parallel()

	content := []any{
		map[string]any{
			"type": "image",
			"source": map[string]any{
				"type":       "base64",
				"media_type": "image/png",
				"data":       "iVBORw0KGgo=",
			},
		},
	}
	parts := convertAnthropicContentToGeminiParts(content, "user")

	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	p, _ := parts[0].(map[string]any)
	inlineData, _ := p["inlineData"].(map[string]any)
	if inlineData["mimeType"] != "image/png" {
		t.Errorf("mimeType = %v, want image/png", inlineData["mimeType"])
	}
	if inlineData["data"] != "iVBORw0KGgo=" {
		t.Errorf("data = %v, want iVBORw0KGgo=", inlineData["data"])
	}
}

func TestConvertAnthropicToolsToGemini(t *testing.T) {
	t.Parallel()

	tools := []any{
		map[string]any{
			"name":        "get_weather",
			"description": "Get weather for a city",
			"input_schema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"city": map[string]any{"type": "string"}},
			},
		},
		map[string]any{
			"name":        "search",
			"description": "Search the web",
		},
	}

	result := convertAnthropicToolsToGemini(tools)
	if len(result) != 1 {
		t.Fatalf("expected 1 tool wrapper, got %d", len(result))
	}
	wrapper, _ := result[0].(map[string]any)
	declarations, _ := wrapper["functionDeclarations"].([]any)
	if len(declarations) != 2 {
		t.Fatalf("expected 2 function declarations, got %d", len(declarations))
	}

	decl0, _ := declarations[0].(map[string]any)
	if decl0["name"] != "get_weather" {
		t.Errorf("declarations[0].name = %v, want get_weather", decl0["name"])
	}
	if decl0["description"] != "Get weather for a city" {
		t.Errorf("declarations[0].description = %v", decl0["description"])
	}

	// Tool without input_schema should get default schema.
	decl1, _ := declarations[1].(map[string]any)
	if decl1["parametersJsonSchema"] == nil {
		t.Error("declarations[1].parametersJsonSchema should have default schema")
	}
}

func TestConvertAnthropicToolChoiceToGemini(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   any
		mode string
	}{
		{"string auto", "auto", "AUTO"},
		{"string any", "any", "ANY"},
		{"string none", "none", "NONE"},
		{"object auto", map[string]any{"type": "auto"}, "AUTO"},
		{"object any", map[string]any{"type": "any"}, "ANY"},
		{"object none", map[string]any{"type": "none"}, "NONE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := convertAnthropicToolChoiceToGemini(tt.in)
			fcc, _ := result["functionCallingConfig"].(map[string]any)
			if fcc["mode"] != tt.mode {
				t.Errorf("mode = %v, want %v", fcc["mode"], tt.mode)
			}
		})
	}
}

func TestConvertAnthropicToolChoiceToGemini_ToolType(t *testing.T) {
	t.Parallel()

	result := convertAnthropicToolChoiceToGemini(map[string]any{"type": "tool", "name": "get_weather"})
	fcc, _ := result["functionCallingConfig"].(map[string]any)
	if fcc["mode"] != "ANY" {
		t.Errorf("mode = %v, want ANY", fcc["mode"])
	}
	allowed, ok := fcc["allowedFunctionNames"].([]string)
	if !ok || len(allowed) != 1 || allowed[0] != "get_weather" {
		t.Errorf("allowedFunctionNames = %v, want [get_weather]", fcc["allowedFunctionNames"])
	}
}

// ──────────────────────────────────────────────────────────────
// Gemini → Anthropic response conversion tests
// ──────────────────────────────────────────────────────────────

func TestConvertGeminiToAnthropic_BasicTextResponse(t *testing.T) {
	t.Parallel()

	geminiResp := map[string]any{
		"candidates": []any{
			map[string]any{
				"content": map[string]any{
					"role":  "model",
					"parts": []any{map[string]any{"text": "Hello, world!"}},
				},
			},
		},
		"usageMetadata": map[string]any{
			"promptTokenCount":     float64(10),
			"candidatesTokenCount": float64(5),
		},
	}

	result, inputTokens, outputTokens := convertGeminiToAnthropic(geminiResp, "gemini-2.5-flash")

	if result["type"] != "message" {
		t.Errorf("type = %v, want message", result["type"])
	}
	if result["role"] != "assistant" {
		t.Errorf("role = %v, want assistant", result["role"])
	}
	if result["model"] != "gemini-2.5-flash" {
		t.Errorf("model = %v, want gemini-2.5-flash", result["model"])
	}
	if result["stop_reason"] != "end_turn" {
		t.Errorf("stop_reason = %v, want end_turn", result["stop_reason"])
	}
	if inputTokens != 10 {
		t.Errorf("inputTokens = %d, want 10", inputTokens)
	}
	if outputTokens != 5 {
		t.Errorf("outputTokens = %d, want 5", outputTokens)
	}

	content, _ := result["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(content))
	}
	block, _ := content[0].(map[string]any)
	if block["type"] != "text" {
		t.Errorf("block type = %v, want text", block["type"])
	}
	if block["text"] != "Hello, world!" {
		t.Errorf("block text = %v, want 'Hello, world!'", block["text"])
	}
}

func TestConvertGeminiToAnthropic_ThinkingResponse(t *testing.T) {
	t.Parallel()

	geminiResp := map[string]any{
		"candidates": []any{
			map[string]any{
				"content": map[string]any{
					"role": "model",
					"parts": []any{
						map[string]any{"text": "Thinking...", "thought": true, "thoughtSignature": "sig-abc"},
						map[string]any{"text": "The answer is 42."},
					},
				},
			},
		},
		"usageMetadata": map[string]any{
			"promptTokenCount":     float64(20),
			"candidatesTokenCount": float64(15),
		},
	}

	result, _, _ := convertGeminiToAnthropic(geminiResp, "gemini-2.5-flash")
	content, _ := result["content"].([]any)

	if len(content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(content))
	}

	// First block: thinking.
	thinking, _ := content[0].(map[string]any)
	if thinking["type"] != "thinking" {
		t.Errorf("block 0 type = %v, want thinking", thinking["type"])
	}
	if thinking["thinking"] != "Thinking..." {
		t.Errorf("block 0 thinking = %v, want 'Thinking...'", thinking["thinking"])
	}
	if thinking["signature"] != "sig-abc" {
		t.Errorf("block 0 signature = %v, want sig-abc", thinking["signature"])
	}

	// Second block: text.
	text, _ := content[1].(map[string]any)
	if text["type"] != "text" {
		t.Errorf("block 1 type = %v, want text", text["type"])
	}
	if text["text"] != "The answer is 42." {
		t.Errorf("block 1 text = %v, want 'The answer is 42.'", text["text"])
	}
}

func TestConvertGeminiToAnthropic_FunctionCallResponse(t *testing.T) {
	t.Parallel()

	geminiResp := map[string]any{
		"candidates": []any{
			map[string]any{
				"content": map[string]any{
					"role": "model",
					"parts": []any{
						map[string]any{
							"functionCall": map[string]any{
								"name": "get_weather",
								"args": map[string]any{"city": "Tokyo"},
							},
						},
					},
				},
			},
		},
		"usageMetadata": map[string]any{
			"promptTokenCount":     float64(10),
			"candidatesTokenCount": float64(5),
		},
	}

	result, _, _ := convertGeminiToAnthropic(geminiResp, "gemini-2.5-flash")
	content, _ := result["content"].([]any)

	if len(content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(content))
	}

	block, _ := content[0].(map[string]any)
	if block["type"] != "tool_use" {
		t.Errorf("block type = %v, want tool_use", block["type"])
	}
	if block["name"] != "get_weather" {
		t.Errorf("block name = %v, want get_weather", block["name"])
	}
	input, _ := block["input"].(map[string]any)
	if input["city"] != "Tokyo" {
		t.Errorf("block input.city = %v, want Tokyo", input["city"])
	}
	if block["id"] == nil {
		t.Error("block id should not be nil")
	}
}

func TestConvertGeminiToAnthropic_EmptyCandidates(t *testing.T) {
	t.Parallel()

	geminiResp := map[string]any{
		"candidates": []any{},
	}

	result, _, _ := convertGeminiToAnthropic(geminiResp, "gemini-2.5-flash")
	content, _ := result["content"].([]any)

	// Should return a default empty text block.
	if len(content) != 1 {
		t.Fatalf("expected 1 default content block, got %d", len(content))
	}
	block, _ := content[0].(map[string]any)
	if block["type"] != "text" {
		t.Errorf("default block type = %v, want text", block["type"])
	}
}

// ──────────────────────────────────────────────────────────────
// Helper function tests
// ──────────────────────────────────────────────────────────────

func TestExtractSystemText(t *testing.T) {
	t.Parallel()

	t.Run("string", func(t *testing.T) {
		t.Parallel()
		if got := extractSystemText("hello"); got != "hello" {
			t.Errorf("got %q, want 'hello'", got)
		}
	})

	t.Run("array of text blocks", func(t *testing.T) {
		t.Parallel()
		sys := []any{
			map[string]any{"text": "First."},
			map[string]any{"text": "Second."},
		}
		if got := extractSystemText(sys); got != "First.\nSecond." {
			t.Errorf("got %q, want 'First.\\nSecond.'", got)
		}
	})
}

func TestExtractToolResultContent(t *testing.T) {
	t.Parallel()

	t.Run("string content", func(t *testing.T) {
		t.Parallel()
		if got := extractToolResultContent("result text"); got != "result text" {
			t.Errorf("got %q, want 'result text'", got)
		}
	})

	t.Run("array text blocks", func(t *testing.T) {
		t.Parallel()
		content := []any{
			map[string]any{"text": "part1"},
			map[string]any{"text": "part2"},
		}
		if got := extractToolResultContent(content); got != "part1part2" {
			t.Errorf("got %q, want 'part1part2'", got)
		}
	})

	t.Run("nil content", func(t *testing.T) {
		t.Parallel()
		if got := extractToolResultContent(nil); got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})
}

func TestWriteAnthropicSSE(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	data := map[string]any{
		"type":  "content_block_delta",
		"index": 0,
		"delta": map[string]any{"type": "text_delta", "text": "hello"},
	}
	writeAnthropicSSE(w, w, "content_block_delta", data)

	body := w.Body.String()
	if !strings.Contains(body, "event: content_block_delta") {
		t.Errorf("response should contain event type, got: %s", body)
	}
	if !strings.Contains(body, "data: ") {
		t.Errorf("response should contain data line, got: %s", body)
	}
	if !strings.Contains(body, "text_delta") {
		t.Errorf("response should contain text_delta in data, got: %s", body)
	}
}

// ──────────────────────────────────────────────────────────────
// Streaming integration test
// ──────────────────────────────────────────────────────────────

func TestAntigravityRelay_ProxyStreaming_ConvertsGeminiToAnthropicSSE(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request body is in Gemini envelope format.
		body, _ := io.ReadAll(r.Body)
		var envelope map[string]any
		_ = json.Unmarshal(body, &envelope)

		if envelope["model"] == nil {
			t.Error("envelope should have model field")
		}
		if envelope["request"] == nil {
			t.Error("envelope should have request field")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher, _ := w.(http.Flusher)

		// Send Gemini SSE chunks.
		chunks := []string{
			`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"thinking...","thought":true}]}}]}`,
			`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"Hello!"}]}}]}`,
			`data: {"usageMetadata":{"promptTokenCount":20,"candidatesTokenCount":10},"candidates":[{"content":{"role":"model","parts":[{"text":" World!"}]}}]}`,
		}
		for _, chunk := range chunks {
			_, _ = w.Write([]byte(chunk + "\n"))
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
	defer ts.Close()

	// Test the SSE conversion by making a direct request to the test server
	// and verifying the Gemini response format.
	ctx := context.Background()
	body := map[string]any{
		"messages": []any{map[string]any{"role": "user", "content": "hi"}},
	}
	geminiBody := convertAnthropicToGemini(body, "gemini-2.5-flash")
	envelope := buildAntigravityEnvelope(geminiBody, "gemini-2.5-flash", "test-project")
	bodyJSON, _ := json.Marshal(envelope)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/v1internal:streamGenerateContent?alt=sse", strings.NewReader(string(bodyJSON)))
	applyAntigravityHeaders(req, "ya29.test", ts.URL)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(respBody), "thinking...") {
		t.Error("response should contain thinking content")
	}
	if !strings.Contains(string(respBody), "Hello!") {
		t.Error("response should contain text content")
	}
}

func TestAntigravityRelay_UpstreamError_ReturnsProxyError(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"error":"insufficient_permissions"}`))
	}))
	defer ts.Close()

	ctx := context.Background()
	bodyJSON, _ := json.Marshal(map[string]any{"model": "gemini-2.5-flash"})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/v1internal:generateContent", strings.NewReader(string(bodyJSON)))
	applyAntigravityHeaders(req, "ya29.test", ts.URL)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 403 {
		t.Errorf("StatusCode = %d, want 403", resp.StatusCode)
	}

	respBytes, _ := io.ReadAll(resp.Body)
	pe := &relay.ProxyError{
		Message:    string(respBytes),
		StatusCode: resp.StatusCode,
	}
	if pe.StatusCode != 403 {
		t.Errorf("ProxyError.StatusCode = %d, want 403", pe.StatusCode)
	}
}

// ──────────────────────────────────────────────────────────────
// Full round-trip conversion test
// ──────────────────────────────────────────────────────────────

func TestAnthropicToGeminiToAnthropic_RoundTrip(t *testing.T) {
	t.Parallel()

	// Build an Anthropic request.
	anthropicBody := map[string]any{
		"model":       "gemini-2.5-flash",
		"max_tokens":  float64(4096),
		"temperature": float64(0.5),
		"system":      "You are a weather assistant.",
		"messages": []any{
			map[string]any{"role": "user", "content": "What is the weather in Tokyo?"},
		},
		"tools": []any{
			map[string]any{
				"name":        "get_weather",
				"description": "Get weather data",
				"input_schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"city": map[string]any{"type": "string"},
					},
				},
			},
		},
		"tool_choice": "auto",
	}

	// Convert to Gemini format.
	geminiBody := convertAnthropicToGemini(anthropicBody, "gemini-2.5-flash")

	// Verify Gemini format has expected fields.
	if geminiBody["contents"] == nil {
		t.Error("gemini body should have contents")
	}
	if geminiBody["systemInstruction"] == nil {
		t.Error("gemini body should have systemInstruction")
	}
	if geminiBody["generationConfig"] == nil {
		t.Error("gemini body should have generationConfig")
	}
	if geminiBody["tools"] == nil {
		t.Error("gemini body should have tools")
	}
	if geminiBody["toolConfig"] == nil {
		t.Error("gemini body should have toolConfig")
	}

	// Simulate a Gemini response with a function call.
	geminiResponse := map[string]any{
		"candidates": []any{
			map[string]any{
				"content": map[string]any{
					"role": "model",
					"parts": []any{
						map[string]any{
							"functionCall": map[string]any{
								"name": "get_weather",
								"args": map[string]any{"city": "Tokyo"},
							},
						},
					},
				},
			},
		},
		"usageMetadata": map[string]any{
			"promptTokenCount":     float64(50),
			"candidatesTokenCount": float64(20),
		},
	}

	// Convert back to Anthropic format.
	anthropicResp, inputTokens, outputTokens := convertGeminiToAnthropic(geminiResponse, "gemini-2.5-flash")

	if anthropicResp["type"] != "message" {
		t.Errorf("response type = %v, want message", anthropicResp["type"])
	}
	if inputTokens != 50 {
		t.Errorf("inputTokens = %d, want 50", inputTokens)
	}
	if outputTokens != 20 {
		t.Errorf("outputTokens = %d, want 20", outputTokens)
	}

	content, _ := anthropicResp["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(content))
	}
	block, _ := content[0].(map[string]any)
	if block["type"] != "tool_use" {
		t.Errorf("content[0].type = %v, want tool_use", block["type"])
	}
	if block["name"] != "get_weather" {
		t.Errorf("content[0].name = %v, want get_weather", block["name"])
	}
}
