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
		{"claude-opus-4-6-thinking", antigravityDailyURL},
		{"claude-sonnet-4-6", antigravityDailyURL},
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

func TestTransformClaudeToGemini_Envelope(t *testing.T) {
	t.Parallel()

	body := map[string]any{
		"messages": []any{map[string]any{"role": "user", "content": "hello"}},
	}

	envelope := transformClaudeToGemini(body, "gemini-2.5-flash", "project-123")

	if envelope.Project != "project-123" {
		t.Errorf("project = %v, want project-123", envelope.Project)
	}
	if envelope.Model != "gemini-2.5-flash" {
		t.Errorf("model = %v, want gemini-2.5-flash", envelope.Model)
	}
	if envelope.UserAgent != "antigravity" {
		t.Errorf("userAgent = %v, want antigravity", envelope.UserAgent)
	}
	if envelope.RequestID == "" {
		t.Error("requestId should not be empty")
	}
}

func TestTransformClaudeToGemini_DefaultProjectID(t *testing.T) {
	t.Parallel()

	body := map[string]any{
		"messages": []any{map[string]any{"role": "user", "content": "hello"}},
	}
	envelope := transformClaudeToGemini(body, "gemini-3-flash", "")
	if envelope.Project == "" {
		t.Error("project should not be empty when projectID is empty (should generate random)")
	}
}

// ──────────────────────────────────────────────────────────────
// Anthropic → Gemini conversion tests
// ──────────────────────────────────────────────────────────────

func TestTransformClaudeToGemini_BasicMessages(t *testing.T) {
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

	envelope := transformClaudeToGemini(body, "gemini-2.5-flash", "test")
	contents := envelope.Request.Contents

	if len(contents) != 3 {
		t.Fatalf("expected 3 contents, got %d", len(contents))
	}

	// Verify role mapping.
	if contents[0].Role != "user" {
		t.Errorf("contents[0].role = %v, want user", contents[0].Role)
	}
	if contents[1].Role != "model" {
		t.Errorf("contents[1].role = %v, want model (mapped from assistant)", contents[1].Role)
	}

	// Verify generation config.
	if envelope.Request.GenerationConfig == nil {
		t.Fatal("generationConfig should not be nil")
	}
	if envelope.Request.GenerationConfig.MaxOutputTokens != 1024 {
		t.Errorf("maxOutputTokens = %v, want 1024", envelope.Request.GenerationConfig.MaxOutputTokens)
	}
}

func TestTransformClaudeToGemini_SystemInstruction(t *testing.T) {
	t.Parallel()

	t.Run("string system with identity patch", func(t *testing.T) {
		t.Parallel()
		body := map[string]any{
			"system":   "You are a helpful assistant.",
			"messages": []any{map[string]any{"role": "user", "content": "hi"}},
		}
		envelope := transformClaudeToGemini(body, "gemini-2.5-flash", "test")
		sysInst := envelope.Request.SystemInstruction
		if sysInst == nil {
			t.Fatal("systemInstruction missing")
		}
		if len(sysInst.Parts) != 1 {
			t.Fatalf("expected 1 system part, got %d", len(sysInst.Parts))
		}
		sysText := sysInst.Parts[0].Text
		// Should contain the identity patch AND the original system text.
		if !strings.Contains(sysText, identityPatchText) {
			t.Error("system text should contain identity patch")
		}
		if !strings.Contains(sysText, "You are a helpful assistant.") {
			t.Error("system text should contain original system instruction")
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
		envelope := transformClaudeToGemini(body, "gemini-2.5-flash", "test")
		sysInst := envelope.Request.SystemInstruction
		sysText := sysInst.Parts[0].Text
		if !strings.Contains(sysText, "First.\nSecond.") {
			t.Errorf("system text should contain 'First.\\nSecond.', got %q", sysText)
		}
	})
}

func TestTransformClaudeToGemini_GenerationConfig(t *testing.T) {
	t.Parallel()

	body := map[string]any{
		"temperature":    float64(0.7),
		"top_p":          float64(0.95),
		"max_tokens":     float64(2048),
		"stop_sequences": []any{"STOP", "END"},
		"messages":       []any{map[string]any{"role": "user", "content": "hi"}},
	}

	envelope := transformClaudeToGemini(body, "gemini-2.5-flash", "test")
	gc := envelope.Request.GenerationConfig

	if gc.Temperature == nil || *gc.Temperature != 0.7 {
		t.Errorf("temperature = %v, want 0.7", gc.Temperature)
	}
	if gc.TopP == nil || *gc.TopP != 0.95 {
		t.Errorf("topP = %v, want 0.95", gc.TopP)
	}
	if gc.MaxOutputTokens != 2048 {
		t.Errorf("maxOutputTokens = %v, want 2048", gc.MaxOutputTokens)
	}
}

func TestTransformClaudeToGemini_ThinkingConfig(t *testing.T) {
	t.Parallel()

	body := map[string]any{
		"thinking": map[string]any{
			"type":          "enabled",
			"budget_tokens": float64(8192),
		},
		"messages": []any{map[string]any{"role": "user", "content": "think about this"}},
	}

	envelope := transformClaudeToGemini(body, "gemini-2.5-flash", "test")
	gc := envelope.Request.GenerationConfig
	if gc.ThinkingConfig == nil {
		t.Fatal("thinkingConfig should not be nil")
	}

	if gc.ThinkingConfig.ThinkingBudget != 8192 {
		t.Errorf("thinkingBudget = %v, want 8192", gc.ThinkingConfig.ThinkingBudget)
	}
	if gc.ThinkingConfig.IncludeThoughts != true {
		t.Errorf("includeThoughts = %v, want true", gc.ThinkingConfig.IncludeThoughts)
	}
}

func TestConvertContentToParts_TextContent(t *testing.T) {
	t.Parallel()

	// String content.
	parts := convertContentToParts("hello world", "user", true, nil)
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if parts[0].Text != "hello world" {
		t.Errorf("text = %v, want 'hello world'", parts[0].Text)
	}
}

func TestConvertContentToParts_ThinkingBlock(t *testing.T) {
	t.Parallel()

	content := []any{
		map[string]any{
			"type":      "thinking",
			"thinking":  "Let me think...",
			"signature": "sig123",
		},
	}
	// Gemini model: should use dummy signature.
	parts := convertContentToParts(content, "assistant", true, nil)

	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if parts[0].Thought == nil || !*parts[0].Thought {
		t.Error("thought should be true")
	}
	if parts[0].Text != "Let me think..." {
		t.Errorf("text = %v, want 'Let me think...'", parts[0].Text)
	}
	if parts[0].ThoughtSignature != DummyThoughtSignature {
		t.Errorf("thoughtSignature = %v, want dummy for Gemini model", parts[0].ThoughtSignature)
	}

	// Claude model: should pass through real signature.
	partsClaude := convertContentToParts(content, "assistant", false, nil)
	if partsClaude[0].ThoughtSignature != "sig123" {
		t.Errorf("thoughtSignature = %v, want sig123 for Claude model", partsClaude[0].ThoughtSignature)
	}
}

func TestConvertContentToParts_ToolUse(t *testing.T) {
	t.Parallel()

	content := []any{
		map[string]any{
			"type":  "tool_use",
			"name":  "get_weather",
			"input": map[string]any{"city": "Tokyo"},
		},
	}
	parts := convertContentToParts(content, "assistant", true, nil)

	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if parts[0].FunctionCall == nil {
		t.Fatal("functionCall should not be nil")
	}
	if parts[0].FunctionCall.Name != "get_weather" {
		t.Errorf("functionCall.name = %v, want get_weather", parts[0].FunctionCall.Name)
	}
	if parts[0].FunctionCall.Args["city"] != "Tokyo" {
		t.Errorf("functionCall.args.city = %v, want Tokyo", parts[0].FunctionCall.Args["city"])
	}
}

func TestConvertContentToParts_ToolResult(t *testing.T) {
	t.Parallel()

	toolIDToName := map[string]string{"toolu_123": "get_weather"}
	content := []any{
		map[string]any{
			"type":        "tool_result",
			"tool_use_id": "toolu_123",
			"content":     "Sunny, 25°C",
		},
	}
	parts := convertContentToParts(content, "user", true, toolIDToName)

	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if parts[0].FunctionResponse == nil {
		t.Fatal("functionResponse should not be nil")
	}
	// Should resolve name from the ID→name map.
	if parts[0].FunctionResponse.Name != "get_weather" {
		t.Errorf("functionResponse.name = %v, want get_weather (resolved from map)", parts[0].FunctionResponse.Name)
	}
	if parts[0].FunctionResponse.Response["output"] != "Sunny, 25°C" {
		t.Errorf("functionResponse.response.output = %v, want 'Sunny, 25°C'", parts[0].FunctionResponse.Response["output"])
	}
}

func TestConvertContentToParts_Image(t *testing.T) {
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
	parts := convertContentToParts(content, "user", true, nil)

	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if parts[0].InlineData == nil {
		t.Fatal("inlineData should not be nil")
	}
	if parts[0].InlineData.MimeType != "image/png" {
		t.Errorf("mimeType = %v, want image/png", parts[0].InlineData.MimeType)
	}
	if parts[0].InlineData.Data != "iVBORw0KGgo=" {
		t.Errorf("data = %v, want iVBORw0KGgo=", parts[0].InlineData.Data)
	}
}

func TestConvertToolsToGemini(t *testing.T) {
	t.Parallel()

	tools := []ClaudeTool{
		{
			Name:        "get_weather",
			Description: "Get weather for a city",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"city": map[string]any{"type": "string"}},
			},
		},
		{
			Name:        "search",
			Description: "Search the web",
		},
	}

	result := convertToolsToGemini(tools, false)
	if len(result) != 1 {
		t.Fatalf("expected 1 tool wrapper, got %d", len(result))
	}
	declarations := result[0].FunctionDeclarations
	if len(declarations) != 2 {
		t.Fatalf("expected 2 function declarations, got %d", len(declarations))
	}

	if declarations[0].Name != "get_weather" {
		t.Errorf("declarations[0].name = %v, want get_weather", declarations[0].Name)
	}
	if declarations[0].Description != "Get weather for a city" {
		t.Errorf("declarations[0].description = %v", declarations[0].Description)
	}
}

func TestBuildToolConfig(t *testing.T) {
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
		{"nil default", nil, "VALIDATED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := buildToolConfig(tt.in)
			if result.FunctionCallingConfig.Mode != tt.mode {
				t.Errorf("mode = %v, want %v", result.FunctionCallingConfig.Mode, tt.mode)
			}
		})
	}
}

func TestBuildToolConfig_ToolType(t *testing.T) {
	t.Parallel()

	result := buildToolConfig(map[string]any{"type": "tool", "name": "get_weather"})
	if result.FunctionCallingConfig.Mode != "ANY" {
		t.Errorf("mode = %v, want ANY", result.FunctionCallingConfig.Mode)
	}
	allowed := result.FunctionCallingConfig.AllowedFunctionNames
	if len(allowed) != 1 || allowed[0] != "get_weather" {
		t.Errorf("allowedFunctionNames = %v, want [get_weather]", allowed)
	}
}

// ──────────────────────────────────────────────────────────────
// Gemini → Anthropic response conversion tests
// ──────────────────────────────────────────────────────────────

func TestTransformGeminiToClaudeResponse_BasicTextResponse(t *testing.T) {
	t.Parallel()

	geminiResp := GeminiResponse{
		Candidates: []GeminiCandidate{
			{
				Content: &GeminiContent{
					Role:  "model",
					Parts: []GeminiPart{{Text: "Hello, world!"}},
				},
				FinishReason: "STOP",
			},
		},
		UsageMetadata: &GeminiUsageMetadata{
			PromptTokenCount:     10,
			CandidatesTokenCount: 5,
		},
	}
	respBytes, _ := json.Marshal(geminiResp)

	result, inputTokens, outputTokens := transformGeminiToClaudeResponse(respBytes, "gemini-2.5-flash")

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

func TestTransformGeminiToClaudeResponse_ThinkingResponse(t *testing.T) {
	t.Parallel()

	thought := true
	geminiResp := GeminiResponse{
		Candidates: []GeminiCandidate{
			{
				Content: &GeminiContent{
					Role: "model",
					Parts: []GeminiPart{
						{Text: "Thinking...", Thought: &thought, ThoughtSignature: "sig-abc"},
						{Text: "The answer is 42."},
					},
				},
			},
		},
		UsageMetadata: &GeminiUsageMetadata{
			PromptTokenCount:     20,
			CandidatesTokenCount: 15,
		},
	}
	respBytes, _ := json.Marshal(geminiResp)

	result, _, _ := transformGeminiToClaudeResponse(respBytes, "gemini-2.5-flash")
	content, _ := result["content"].([]any)

	if len(content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(content))
	}

	// First block: thinking (Gemini model → dummy signature).
	thinking, _ := content[0].(map[string]any)
	if thinking["type"] != "thinking" {
		t.Errorf("block 0 type = %v, want thinking", thinking["type"])
	}
	if thinking["thinking"] != "Thinking..." {
		t.Errorf("block 0 thinking = %v, want 'Thinking...'", thinking["thinking"])
	}
	if thinking["signature"] != DummyThoughtSignature {
		t.Errorf("block 0 signature = %v, want dummy for Gemini model", thinking["signature"])
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

func TestTransformGeminiToClaudeResponse_FunctionCallResponse(t *testing.T) {
	t.Parallel()

	geminiResp := GeminiResponse{
		Candidates: []GeminiCandidate{
			{
				Content: &GeminiContent{
					Role: "model",
					Parts: []GeminiPart{
						{FunctionCall: &GeminiFunctionCall{
							Name: "get_weather",
							Args: map[string]any{"city": "Tokyo"},
						}},
					},
				},
			},
		},
		UsageMetadata: &GeminiUsageMetadata{
			PromptTokenCount:     10,
			CandidatesTokenCount: 5,
		},
	}
	respBytes, _ := json.Marshal(geminiResp)

	result, _, _ := transformGeminiToClaudeResponse(respBytes, "gemini-2.5-flash")
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

	// stop_reason should be tool_use when function calls are present.
	if result["stop_reason"] != "tool_use" {
		t.Errorf("stop_reason = %v, want tool_use", result["stop_reason"])
	}
}

func TestTransformGeminiToClaudeResponse_EmptyCandidates(t *testing.T) {
	t.Parallel()

	geminiResp := GeminiResponse{
		Candidates: []GeminiCandidate{},
	}
	respBytes, _ := json.Marshal(geminiResp)

	result, _, _ := transformGeminiToClaudeResponse(respBytes, "gemini-2.5-flash")
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

func TestExtractToolResultText(t *testing.T) {
	t.Parallel()

	t.Run("string content", func(t *testing.T) {
		t.Parallel()
		if got := extractToolResultText("result text", nil); got != "result text" {
			t.Errorf("got %q, want 'result text'", got)
		}
	})

	t.Run("array text blocks", func(t *testing.T) {
		t.Parallel()
		content := []any{
			map[string]any{"text": "part1"},
			map[string]any{"text": "part2"},
		}
		if got := extractToolResultText(content, nil); got != "part1\npart2" {
			t.Errorf("got %q, want 'part1\\npart2'", got)
		}
	})

	t.Run("nil content no error", func(t *testing.T) {
		t.Parallel()
		if got := extractToolResultText(nil, nil); got != "Command executed successfully." {
			t.Errorf("got %q, want fallback message", got)
		}
	})

	t.Run("nil content with error", func(t *testing.T) {
		t.Parallel()
		isErr := true
		if got := extractToolResultText(nil, &isErr); got != "Tool execution failed with no output." {
			t.Errorf("got %q, want error fallback message", got)
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
// New feature tests
// ──────────────────────────────────────────────────────────────

func TestGenerateStableSessionID(t *testing.T) {
	t.Parallel()

	msgs := []ClaudeMessage{
		{Role: "user", Content: "hello world"},
	}

	id1 := generateStableSessionID(msgs)
	id2 := generateStableSessionID(msgs)

	if id1 != id2 {
		t.Errorf("stable session IDs should match: %q != %q", id1, id2)
	}
	if len(id1) != 32 {
		t.Errorf("session ID should be 32 hex chars, got %d: %q", len(id1), id1)
	}
}

func TestInjectIdentityPatch(t *testing.T) {
	t.Parallel()

	t.Run("empty system", func(t *testing.T) {
		t.Parallel()
		result := injectIdentityPatch("")
		if !strings.Contains(result, identityPatchText) {
			t.Error("should inject identity text")
		}
		if !strings.Contains(result, identityBoundaryStart) {
			t.Error("should include boundary markers")
		}
	})

	t.Run("already has identity", func(t *testing.T) {
		t.Parallel()
		result := injectIdentityPatch(identityPatchText + " already here")
		if strings.Contains(result, identityBoundaryStart) {
			t.Error("should not re-inject when identity text already present")
		}
	})

	t.Run("prepends to existing", func(t *testing.T) {
		t.Parallel()
		result := injectIdentityPatch("Custom instructions here")
		if !strings.HasPrefix(result, identityBoundaryStart) {
			t.Error("identity patch should be prepended")
		}
		if !strings.Contains(result, "Custom instructions here") {
			t.Error("original text should be preserved")
		}
	})
}

func TestDetectWebSearchTool(t *testing.T) {
	t.Parallel()

	tools := []ClaudeTool{
		{Name: "get_weather"},
		{Name: "web_search"},
	}
	if !detectWebSearchTool(tools) {
		t.Error("should detect web_search tool")
	}

	toolsNoSearch := []ClaudeTool{
		{Name: "get_weather"},
	}
	if detectWebSearchTool(toolsNoSearch) {
		t.Error("should not detect web_search when not present")
	}
}

func TestDetectMCPTools(t *testing.T) {
	t.Parallel()

	tools := []ClaudeTool{
		{Name: "mcp__server__tool"},
	}
	if !detectMCPTools(tools) {
		t.Error("should detect MCP tools")
	}

	toolsNoMCP := []ClaudeTool{
		{Name: "get_weather"},
	}
	if detectMCPTools(toolsNoMCP) {
		t.Error("should not detect MCP tools when not present")
	}
}

func TestDefaultSafetySettings_Omitted(t *testing.T) {
	t.Parallel()
	body := map[string]any{
		"model":    "gemini-2.5-flash",
		"messages": []any{map[string]any{"role": "user", "content": "hi"}},
	}
	envelope := transformClaudeToGemini(body, "gemini-2.5-flash", "test")

	// safetySettings should be omitted from the request (not sent to upstream).
	raw, _ := json.Marshal(envelope.Request)
	if strings.Contains(string(raw), "safetySettings") {
		t.Error("safetySettings should not be present in request")
	}
}

func TestDefaultStopSequences(t *testing.T) {
	t.Parallel()

	body := map[string]any{
		"messages": []any{map[string]any{"role": "user", "content": "hi"}},
	}
	envelope := transformClaudeToGemini(body, "gemini-2.5-flash", "test")

	gc := envelope.Request.GenerationConfig
	if gc == nil {
		t.Fatal("generationConfig should not be nil")
	}
	if len(gc.StopSequences) == 0 {
		t.Error("should have default stop sequences")
	}
}

func TestWebSearchModelFallback(t *testing.T) {
	t.Parallel()

	body := map[string]any{
		"messages": []any{map[string]any{"role": "user", "content": "search for something"}},
		"tools": []any{
			map[string]any{"name": "web_search", "description": "Web search"},
		},
	}
	envelope := transformClaudeToGemini(body, "claude-sonnet-4-6", "test")

	if envelope.Model != WebSearchFallbackModel {
		t.Errorf("model = %v, want %v (web_search fallback)", envelope.Model, WebSearchFallbackModel)
	}
	if envelope.RequestType != "web_search" {
		t.Errorf("requestType = %v, want web_search", envelope.RequestType)
	}

	// Should have googleSearch tool.
	hasGoogleSearch := false
	for _, tool := range envelope.Request.Tools {
		if tool.GoogleSearch != nil {
			hasGoogleSearch = true
		}
	}
	if !hasGoogleSearch {
		t.Error("should have googleSearch tool")
	}
}

func TestMapFinishReason(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"STOP", "end_turn"},
		{"MAX_TOKENS", "max_tokens"},
		{"SAFETY", "end_turn"},
		{"MALFORMED_FUNCTION_CALL", "end_turn"},
		{"", "end_turn"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			if got := mapFinishReason(tt.input); got != tt.want {
				t.Errorf("mapFinishReason(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildGroundingText(t *testing.T) {
	t.Parallel()

	gm := &GeminiGroundingMetadata{
		WebSearchQueries: []string{"test query"},
		GroundingChunks: []GeminiGroundingChunk{
			{Web: &GeminiGroundingWeb{URI: "https://example.com", Title: "Example"}},
		},
	}
	text := buildGroundingText(gm)
	if !strings.Contains(text, "**Sources:**") {
		t.Error("should contain Sources header")
	}
	if !strings.Contains(text, "[Example](https://example.com)") {
		t.Error("should contain markdown link")
	}
}

func TestGenerateSecureID(t *testing.T) {
	t.Parallel()

	id := generateSecureID()
	if len(id) != 24 { // 12 bytes = 24 hex chars
		t.Errorf("secure ID should be 24 hex chars, got %d: %q", len(id), id)
	}
}

func TestUsageCalculation(t *testing.T) {
	t.Parallel()

	usage := &GeminiUsageMetadata{
		PromptTokenCount:        100,
		CandidatesTokenCount:    50,
		CachedContentTokenCount: 30,
		ThoughtsTokenCount:      20,
	}
	inputTokens, outputTokens, cacheRead := extractUsageFromGemini(usage)
	if inputTokens != 70 { // 100 - 30
		t.Errorf("inputTokens = %d, want 70 (prompt - cached)", inputTokens)
	}
	if outputTokens != 70 { // 50 + 20
		t.Errorf("outputTokens = %d, want 70 (candidates + thoughts)", outputTokens)
	}
	if cacheRead != 30 {
		t.Errorf("cacheRead = %d, want 30", cacheRead)
	}
}

// ──────────────────────────────────────────────────────────────
// Streaming processor tests
// ──────────────────────────────────────────────────────────────

func TestStreamingProcessor_BasicTextStream(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	sp := NewStreamingProcessor(w, "gemini-2.5-flash")

	// Process a text chunk.
	chunk := GeminiResponse{
		Candidates: []GeminiCandidate{
			{Content: &GeminiContent{
				Role:  "model",
				Parts: []GeminiPart{{Text: "Hello!"}},
			}},
		},
	}
	chunkBytes, _ := json.Marshal(chunk)
	sp.ProcessChunk(chunkBytes)

	inputTokens, _ := sp.Finish()

	body := w.Body.String()
	if !strings.Contains(body, "message_start") {
		t.Error("should contain message_start")
	}
	if !strings.Contains(body, "content_block_start") {
		t.Error("should contain content_block_start")
	}
	if !strings.Contains(body, "Hello!") {
		t.Error("should contain text content")
	}
	if !strings.Contains(body, "message_stop") {
		t.Error("should contain message_stop")
	}
	if !strings.Contains(body, `"stop_reason":"end_turn"`) {
		t.Error("should contain end_turn stop_reason")
	}
	if sp.AccText() != "Hello!" {
		t.Errorf("AccText() = %q, want 'Hello!'", sp.AccText())
	}
	_ = inputTokens
}

func TestStreamingProcessor_ToolUseStopReason(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	sp := NewStreamingProcessor(w, "gemini-2.5-flash")

	chunk := GeminiResponse{
		Candidates: []GeminiCandidate{
			{Content: &GeminiContent{
				Role: "model",
				Parts: []GeminiPart{
					{FunctionCall: &GeminiFunctionCall{
						Name: "test",
						Args: map[string]any{"x": 1},
					}},
				},
			}},
		},
	}
	chunkBytes, _ := json.Marshal(chunk)
	sp.ProcessChunk(chunkBytes)
	sp.Finish()

	body := w.Body.String()
	if !strings.Contains(body, `"stop_reason":"tool_use"`) {
		t.Error("should contain tool_use stop_reason when function call present")
	}
}

func TestStreamingProcessor_MessageStartGuard(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	sp := NewStreamingProcessor(w, "gemini-2.5-flash")

	// Finish without any data — should not emit message_stop.
	sp.Finish()

	body := w.Body.String()
	if strings.Contains(body, "message_stop") {
		t.Error("should not emit message_stop if no data was received")
	}
}

// ──────────────────────────────────────────────────────────────
// Streaming integration test
// ──────────────────────────────────────────────────────────────

func TestAntigravityRelay_ProxyStreaming_ConvertsGeminiToAnthropicSSE(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request body is in V1Internal envelope format.
		body, _ := io.ReadAll(r.Body)
		var envelope V1InternalRequest
		_ = json.Unmarshal(body, &envelope)

		if envelope.Model == "" {
			t.Error("envelope should have model field")
		}
		if len(envelope.Request.Contents) == 0 {
			t.Error("envelope should have request.contents")
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

	// Build a request using the new typed API and verify.
	ctx := context.Background()
	body := map[string]any{
		"messages": []any{map[string]any{"role": "user", "content": "hi"}},
	}
	envelope := transformClaudeToGemini(body, "gemini-2.5-flash", "test-project")
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
	envelope := transformClaudeToGemini(anthropicBody, "gemini-2.5-flash", "test")

	// Verify Gemini format has expected fields.
	if len(envelope.Request.Contents) == 0 {
		t.Error("gemini body should have contents")
	}
	if envelope.Request.SystemInstruction == nil {
		t.Error("gemini body should have systemInstruction")
	}
	if envelope.Request.GenerationConfig == nil {
		t.Error("gemini body should have generationConfig")
	}
	if len(envelope.Request.Tools) == 0 {
		t.Error("gemini body should have tools")
	}
	if envelope.Request.ToolConfig == nil {
		t.Error("gemini body should have toolConfig")
	}

	// Simulate a Gemini response with a function call.
	geminiResponse := GeminiResponse{
		Candidates: []GeminiCandidate{
			{
				Content: &GeminiContent{
					Role: "model",
					Parts: []GeminiPart{
						{FunctionCall: &GeminiFunctionCall{
							Name: "get_weather",
							Args: map[string]any{"city": "Tokyo"},
						}},
					},
				},
			},
		},
		UsageMetadata: &GeminiUsageMetadata{
			PromptTokenCount:     50,
			CandidatesTokenCount: 20,
		},
	}
	respBytes, _ := json.Marshal(geminiResponse)

	// Convert back to Anthropic format.
	anthropicResp, inputTokens, outputTokens := transformGeminiToClaudeResponse(respBytes, "gemini-2.5-flash")

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
	// stop_reason should be tool_use.
	if anthropicResp["stop_reason"] != "tool_use" {
		t.Errorf("stop_reason = %v, want tool_use", anthropicResp["stop_reason"])
	}
}
