package relay

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kunish/wheel/apps/worker/internal/types"
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

// ── SelectBaseUrl ──────────────────────────────────────────────

func TestSelectBaseUrl_Empty(t *testing.T) {
	got := SelectBaseUrl(nil)
	if got != "https://api.openai.com" {
		t.Errorf("got %q, want default", got)
	}
}

func TestSelectBaseUrl_PicksLowestDelay(t *testing.T) {
	urls := []types.BaseUrl{
		{URL: "https://slow.example.com", Delay: 500},
		{URL: "https://fast.example.com", Delay: 50},
		{URL: "https://mid.example.com", Delay: 200},
	}
	got := SelectBaseUrl(urls)
	if got != "https://fast.example.com" {
		t.Errorf("got %q, want fast", got)
	}
}

func TestSelectBaseUrl_TrimsTrailingSlash(t *testing.T) {
	urls := []types.BaseUrl{{URL: "https://api.example.com/", Delay: 0}}
	got := SelectBaseUrl(urls)
	if got != "https://api.example.com" {
		t.Errorf("got %q, want trimmed", got)
	}
}

// ── BuildUpstreamRequest routing ───────────────────────────────

func TestBuildUpstreamRequest_RoutesToOpenAI(t *testing.T) {
	ch := ChannelConfig{
		Type:     types.OutboundOpenAI,
		BaseUrls: []types.BaseUrl{{URL: "https://api.openai.com", Delay: 0}},
	}
	body := map[string]any{"model": "gpt-4o", "messages": []any{}}
	req := BuildUpstreamRequest(ch, "sk-test", body, "/v1/chat/completions", "gpt-4o", false)

	if req.URL != "https://api.openai.com/v1/chat/completions" {
		t.Errorf("URL = %q", req.URL)
	}
	if req.Headers["Authorization"] != "Bearer sk-test" {
		t.Error("missing Bearer auth")
	}
}

func TestBuildUpstreamRequest_RoutesToAnthropic(t *testing.T) {
	ch := ChannelConfig{
		Type:     types.OutboundAnthropic,
		BaseUrls: []types.BaseUrl{{URL: "https://api.anthropic.com", Delay: 0}},
	}
	body := map[string]any{
		"model":    "claude-3-opus",
		"messages": []any{map[string]any{"role": "user", "content": "hello"}},
	}
	req := BuildUpstreamRequest(ch, "sk-ant-test", body, "/v1/chat/completions", "claude-3-opus", false)

	if req.URL != "https://api.anthropic.com/v1/messages" {
		t.Errorf("URL = %q", req.URL)
	}
	if req.Headers["x-api-key"] != "sk-ant-test" {
		t.Error("missing x-api-key")
	}
	if req.Headers["anthropic-version"] != "2023-06-01" {
		t.Error("missing anthropic-version")
	}
}

func TestBuildUpstreamRequest_AnthropicPassthrough(t *testing.T) {
	ch := ChannelConfig{
		Type:     types.OutboundAnthropic,
		BaseUrls: []types.BaseUrl{{URL: "https://api.anthropic.com", Delay: 0}},
	}
	body := map[string]any{
		"model":    "claude-3-opus",
		"messages": []any{map[string]any{"role": "user", "content": "hello"}},
	}
	req := BuildUpstreamRequest(ch, "sk-ant-test", body, "/v1/messages", "claude-3-opus", true)

	if req.URL != "https://api.anthropic.com/v1/messages" {
		t.Errorf("URL = %q", req.URL)
	}
	// Passthrough should preserve original body structure
	var parsed map[string]any
	if err := json.Unmarshal([]byte(req.Body), &parsed); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}
	if parsed["model"] != "claude-3-opus" {
		t.Error("model not set in body")
	}
}

// ── buildOpenAIRequest path routing ────────────────────────────

func TestBuildOpenAIRequest_ChatPath(t *testing.T) {
	ch := ChannelConfig{
		Type:     types.OutboundOpenAIChat,
		BaseUrls: []types.BaseUrl{{URL: "https://api.openai.com", Delay: 0}},
	}
	body := map[string]any{"model": "gpt-4o"}
	req := BuildUpstreamRequest(ch, "key", body, "/v1/chat/completions", "gpt-4o", false)

	if req.URL != "https://api.openai.com/v1/chat/completions" {
		t.Errorf("URL = %q", req.URL)
	}
}

func TestBuildOpenAIRequest_EmbeddingsPath(t *testing.T) {
	ch := ChannelConfig{
		Type:     types.OutboundOpenAIEmbedding,
		BaseUrls: []types.BaseUrl{{URL: "https://api.openai.com", Delay: 0}},
	}
	body := map[string]any{"model": "text-embedding-3-small", "input": "hello"}
	req := BuildUpstreamRequest(ch, "key", body, "/v1/embeddings", "text-embedding-3-small", false)

	if req.URL != "https://api.openai.com/v1/embeddings" {
		t.Errorf("URL = %q", req.URL)
	}
}

func TestBuildOpenAIRequest_ResponsesPath(t *testing.T) {
	ch := ChannelConfig{
		Type:     types.OutboundOpenAIResponses,
		BaseUrls: []types.BaseUrl{{URL: "https://api.openai.com", Delay: 0}},
	}
	body := map[string]any{"model": "gpt-4o", "input": "hello"}
	req := BuildUpstreamRequest(ch, "key", body, "/v1/responses", "gpt-4o", false)

	if req.URL != "https://api.openai.com/v1/responses" {
		t.Errorf("URL = %q", req.URL)
	}
}

func TestBuildGeminiRequest_NativeFormat(t *testing.T) {
	ch := ChannelConfig{
		Type:     types.OutboundGemini,
		BaseUrls: []types.BaseUrl{{URL: "https://generativelanguage.googleapis.com", Delay: 0}},
	}
	body := map[string]any{
		"model":    "gemini-pro",
		"messages": []any{map[string]any{"role": "user", "content": "hello"}},
	}
	req := BuildUpstreamRequest(ch, "test-key", body, "/v1/chat/completions", "gemini-pro", false)

	if !strings.Contains(req.URL, "/v1beta/models/gemini-pro:generateContent") {
		t.Errorf("URL = %q, want native Gemini endpoint", req.URL)
	}
	if !strings.Contains(req.URL, "key=test-key") {
		t.Errorf("URL = %q, want API key in URL", req.URL)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(req.Body), &parsed); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}
	if parsed["contents"] == nil {
		t.Error("expected 'contents' field in Gemini body")
	}
	if parsed["model"] != nil {
		t.Error("model should not be in Gemini body")
	}
}

func TestBuildOpenAIRequest_CustomHeaders(t *testing.T) {
	ch := ChannelConfig{
		Type:         types.OutboundOpenAI,
		BaseUrls:     []types.BaseUrl{{URL: "https://api.openai.com", Delay: 0}},
		CustomHeader: []types.CustomHeader{{Key: "X-Custom", Value: "test"}},
	}
	body := map[string]any{"model": "gpt-4o"}
	req := BuildUpstreamRequest(ch, "key", body, "/v1/chat/completions", "gpt-4o", false)

	if req.Headers["X-Custom"] != "test" {
		t.Error("custom header not applied")
	}
}

func TestBuildOpenAIRequest_ParamOverride(t *testing.T) {
	override := `{"temperature": 0.7}`
	ch := ChannelConfig{
		Type:          types.OutboundOpenAI,
		BaseUrls:      []types.BaseUrl{{URL: "https://api.openai.com", Delay: 0}},
		ParamOverride: &override,
	}
	body := map[string]any{"model": "gpt-4o"}
	req := BuildUpstreamRequest(ch, "key", body, "/v1/chat/completions", "gpt-4o", false)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(req.Body), &parsed); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}
	if parsed["temperature"] != 0.7 {
		t.Errorf("temperature = %v, want 0.7", parsed["temperature"])
	}
}

func TestBuildOpenAIRequest_OverridesModel(t *testing.T) {
	ch := ChannelConfig{
		Type:     types.OutboundOpenAI,
		BaseUrls: []types.BaseUrl{{URL: "https://api.openai.com", Delay: 0}},
	}
	body := map[string]any{"model": "original-model"}
	req := BuildUpstreamRequest(ch, "key", body, "/v1/chat/completions", "target-model", false)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(req.Body), &parsed); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}
	if parsed["model"] != "target-model" {
		t.Errorf("model = %q, want target-model", parsed["model"])
	}
	// Original body should not be mutated
	if body["model"] != "original-model" {
		t.Error("original body was mutated")
	}
}

// ── OpenAI → Anthropic request conversion ──────────────────────

func TestBuildAnthropicRequest_SystemMessage(t *testing.T) {
	ch := ChannelConfig{
		Type:     types.OutboundAnthropic,
		BaseUrls: []types.BaseUrl{{URL: "https://api.anthropic.com", Delay: 0}},
	}
	body := map[string]any{
		"model": "claude-3-opus",
		"messages": []any{
			map[string]any{"role": "system", "content": "You are helpful."},
			map[string]any{"role": "user", "content": "Hi"},
		},
	}
	req := BuildUpstreamRequest(ch, "key", body, "/v1/chat/completions", "claude-3-opus", false)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(req.Body), &parsed); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}

	if parsed["system"] != "You are helpful." {
		t.Errorf("system = %v", parsed["system"])
	}

	messages := parsed["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("messages len = %d, want 1 (system extracted)", len(messages))
	}
	msg := messages[0].(map[string]any)
	if msg["role"] != "user" {
		t.Errorf("role = %q, want user", msg["role"])
	}
}

func TestBuildAnthropicRequest_ToolCalls(t *testing.T) {
	ch := ChannelConfig{
		Type:     types.OutboundAnthropic,
		BaseUrls: []types.BaseUrl{{URL: "https://api.anthropic.com", Delay: 0}},
	}
	body := parseBody(t, `{
		"model": "claude-3-opus",
		"messages": [
			{"role": "user", "content": "What's the weather?"},
			{
				"role": "assistant",
				"content": "Let me check.",
				"tool_calls": [{
					"id": "call_1",
					"type": "function",
					"function": {
						"name": "get_weather",
						"arguments": "{\"city\":\"Tokyo\"}"
					}
				}]
			},
			{
				"role": "tool",
				"tool_call_id": "call_1",
				"content": "Sunny, 25°C"
			}
		],
		"tools": [{
			"type": "function",
			"function": {
				"name": "get_weather",
				"description": "Get weather",
				"parameters": {"type": "object", "properties": {"city": {"type": "string"}}}
			}
		}]
	}`)
	req := BuildUpstreamRequest(ch, "key", body, "/v1/chat/completions", "claude-3-opus", false)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(req.Body), &parsed); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}

	messages := parsed["messages"].([]any)
	if len(messages) != 3 {
		t.Fatalf("messages len = %d, want 3", len(messages))
	}

	// Assistant message with tool_use blocks
	assistantMsg := messages[1].(map[string]any)
	content := assistantMsg["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("assistant content blocks = %d, want 2 (text + tool_use)", len(content))
	}
	textBlock := content[0].(map[string]any)
	if textBlock["type"] != "text" {
		t.Errorf("first block type = %q", textBlock["type"])
	}
	toolBlock := content[1].(map[string]any)
	if toolBlock["type"] != "tool_use" {
		t.Errorf("second block type = %q", toolBlock["type"])
	}
	if toolBlock["name"] != "get_weather" {
		t.Errorf("tool name = %q", toolBlock["name"])
	}

	// Tool result converted to user with tool_result
	toolMsg := messages[2].(map[string]any)
	if toolMsg["role"] != "user" {
		t.Errorf("tool msg role = %q, want user", toolMsg["role"])
	}
	toolContent := toolMsg["content"].([]any)
	toolResult := toolContent[0].(map[string]any)
	if toolResult["type"] != "tool_result" {
		t.Errorf("tool result type = %q", toolResult["type"])
	}

	// Tools converted to Anthropic format
	tools := parsed["tools"].([]any)
	tool := tools[0].(map[string]any)
	if tool["name"] != "get_weather" {
		t.Errorf("tool name = %q", tool["name"])
	}
	if tool["input_schema"] == nil {
		t.Error("input_schema missing")
	}
	if tool["function"] != nil {
		t.Error("OpenAI function wrapper should not be present")
	}
}

func TestBuildAnthropicRequest_MaxTokensFallback(t *testing.T) {
	ch := ChannelConfig{
		Type:     types.OutboundAnthropic,
		BaseUrls: []types.BaseUrl{{URL: "https://api.anthropic.com", Delay: 0}},
	}
	body := map[string]any{
		"model":    "claude-3-opus",
		"messages": []any{map[string]any{"role": "user", "content": "Hi"}},
	}
	req := BuildUpstreamRequest(ch, "key", body, "/v1/chat/completions", "claude-3-opus", false)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(req.Body), &parsed); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}

	// Should default to 4096 then ensureMaxTokens may adjust
	mt, ok := parsed["max_tokens"].(float64)
	if !ok || mt < 1 {
		t.Errorf("max_tokens = %v, expected positive number", parsed["max_tokens"])
	}
}

func TestBuildAnthropicRequest_ForwardsStreamAndParams(t *testing.T) {
	ch := ChannelConfig{
		Type:     types.OutboundAnthropic,
		BaseUrls: []types.BaseUrl{{URL: "https://api.anthropic.com", Delay: 0}},
	}
	body := map[string]any{
		"model":       "claude-3-opus",
		"messages":    []any{map[string]any{"role": "user", "content": "Hi"}},
		"stream":      true,
		"temperature": 0.5,
		"top_p":       0.9,
		"stop":        []any{"END"},
	}
	req := BuildUpstreamRequest(ch, "key", body, "/v1/chat/completions", "claude-3-opus", false)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(req.Body), &parsed); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}

	if parsed["stream"] != true {
		t.Error("stream not forwarded")
	}
	if parsed["temperature"] != 0.5 {
		t.Errorf("temperature = %v", parsed["temperature"])
	}
	if parsed["top_p"] != 0.9 {
		t.Errorf("top_p = %v", parsed["top_p"])
	}
	if parsed["stop_sequences"] == nil {
		t.Error("stop_sequences missing")
	}
	// OpenAI field should not be present
	if parsed["stop"] != nil {
		t.Error("OpenAI 'stop' field should not be present in Anthropic body")
	}
}

// ── convertAssistantMessage ────────────────────────────────────

func TestConvertAssistantMessage_PlainText(t *testing.T) {
	msg := map[string]any{"role": "assistant", "content": "Hello!"}
	result := convertAssistantMessage(msg)

	if result["role"] != "assistant" {
		t.Errorf("role = %q", result["role"])
	}
	if result["content"] != "Hello!" {
		t.Errorf("content = %v", result["content"])
	}
}

func TestConvertAssistantMessage_WithToolCalls(t *testing.T) {
	msg := parseBody(t, `{
		"role": "assistant",
		"content": "Let me check.",
		"tool_calls": [{
			"id": "call_1",
			"type": "function",
			"function": {"name": "search", "arguments": "{\"q\":\"test\"}"}
		}]
	}`)

	result := convertAssistantMessage(msg)
	content := result["content"].([]any)

	if len(content) != 2 {
		t.Fatalf("content blocks = %d, want 2", len(content))
	}

	text := content[0].(map[string]any)
	if text["type"] != "text" || text["text"] != "Let me check." {
		t.Errorf("text block = %v", text)
	}

	toolUse := content[1].(map[string]any)
	if toolUse["type"] != "tool_use" {
		t.Errorf("type = %q", toolUse["type"])
	}
	if toolUse["id"] != "call_1" {
		t.Errorf("id = %q", toolUse["id"])
	}
	if toolUse["name"] != "search" {
		t.Errorf("name = %q", toolUse["name"])
	}
	input := toolUse["input"].(map[string]any)
	if input["q"] != "test" {
		t.Errorf("input = %v", input)
	}
}

func TestConvertAssistantMessage_ToolCallsWithoutText(t *testing.T) {
	msg := parseBody(t, `{
		"role": "assistant",
		"content": "",
		"tool_calls": [{
			"id": "call_1",
			"type": "function",
			"function": {"name": "search", "arguments": "{}"}
		}]
	}`)

	result := convertAssistantMessage(msg)
	content := result["content"].([]any)

	// Empty string content should not produce a text block
	if len(content) != 1 {
		t.Fatalf("content blocks = %d, want 1 (tool_use only)", len(content))
	}
	if content[0].(map[string]any)["type"] != "tool_use" {
		t.Error("expected tool_use block only")
	}
}

// ── convertToolResultMessage ───────────────────────────────────

func TestConvertToolResultMessage_String(t *testing.T) {
	msg := map[string]any{
		"role":         "tool",
		"tool_call_id": "call_1",
		"content":      "Result text",
	}
	result := convertToolResultMessage(msg)

	if result["role"] != "user" {
		t.Errorf("role = %q, want user", result["role"])
	}
	content := result["content"].([]any)
	block := content[0].(map[string]any)
	if block["type"] != "tool_result" {
		t.Errorf("type = %q", block["type"])
	}
	if block["tool_use_id"] != "call_1" {
		t.Errorf("tool_use_id = %q", block["tool_use_id"])
	}
	if block["content"] != "Result text" {
		t.Errorf("content = %q", block["content"])
	}
}

func TestConvertToolResultMessage_ObjectContent(t *testing.T) {
	msg := map[string]any{
		"role":         "tool",
		"tool_call_id": "call_1",
		"content":      map[string]any{"status": "ok"},
	}
	result := convertToolResultMessage(msg)

	content := result["content"].([]any)
	block := content[0].(map[string]any)
	// Object content should be JSON-serialized
	contentStr, ok := block["content"].(string)
	if !ok {
		t.Fatal("content should be string")
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(contentStr), &parsed); err != nil {
		t.Fatalf("content is not valid JSON: %v", err)
	}
	if parsed["status"] != "ok" {
		t.Errorf("parsed content = %v", parsed)
	}
}

// ── convertOpenAITools ─────────────────────────────────────────

func TestConvertOpenAITools(t *testing.T) {
	tools := []any{
		map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "search",
				"description": "Search the web",
				"parameters": map[string]any{
					"type":       "object",
					"properties": map[string]any{"q": map[string]any{"type": "string"}},
				},
			},
		},
	}

	result := convertOpenAITools(tools)
	if len(result) != 1 {
		t.Fatalf("len = %d, want 1", len(result))
	}

	tool := result[0].(map[string]any)
	if tool["name"] != "search" {
		t.Errorf("name = %q", tool["name"])
	}
	if tool["description"] != "Search the web" {
		t.Errorf("description = %q", tool["description"])
	}
	if tool["input_schema"] == nil {
		t.Error("input_schema missing")
	}
}

func TestConvertOpenAITools_SkipsNonFunction(t *testing.T) {
	tools := []any{
		map[string]any{"type": "retrieval"},
		map[string]any{
			"type":     "function",
			"function": map[string]any{"name": "valid"},
		},
	}
	result := convertOpenAITools(tools)
	if len(result) != 1 {
		t.Fatalf("len = %d, want 1 (skipped retrieval)", len(result))
	}
}

func TestConvertOpenAITools_DefaultParams(t *testing.T) {
	tools := []any{
		map[string]any{
			"type":     "function",
			"function": map[string]any{"name": "noop", "description": "Do nothing"},
		},
	}
	result := convertOpenAITools(tools)
	tool := result[0].(map[string]any)
	schema := tool["input_schema"].(map[string]any)
	if schema["type"] != "object" {
		t.Errorf("default schema type = %q", schema["type"])
	}
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

// ── applyParamOverrides ────────────────────────────────────────

func TestApplyParamOverrides_Nil(t *testing.T) {
	body := map[string]any{"model": "gpt-4o"}
	applyParamOverrides(body, nil)
	if len(body) != 1 {
		t.Error("nil override should not modify body")
	}
}

func TestApplyParamOverrides_Valid(t *testing.T) {
	body := map[string]any{"model": "gpt-4o"}
	override := `{"temperature": 0.5, "top_p": 0.9}`
	applyParamOverrides(body, &override)

	if body["temperature"] != 0.5 {
		t.Errorf("temperature = %v", body["temperature"])
	}
	if body["top_p"] != 0.9 {
		t.Errorf("top_p = %v", body["top_p"])
	}
}

func TestApplyParamOverrides_InvalidJSON(t *testing.T) {
	body := map[string]any{"model": "gpt-4o"}
	invalid := `{invalid`
	applyParamOverrides(body, &invalid)
	// Should not panic or modify body
	if len(body) != 1 {
		t.Error("invalid JSON should not modify body")
	}
}

// ── ensureThinkingParams ───────────────────────────────────────

func TestEnsureThinkingParams_NonThinkingModel(t *testing.T) {
	body := map[string]any{"model": "claude-3-opus"}
	ensureThinkingParams(body, "claude-3-opus")
	if body["thinking"] != nil {
		t.Error("should not add thinking for non-thinking model")
	}
}

func TestEnsureThinkingParams_ThinkingModel(t *testing.T) {
	body := map[string]any{"model": "claude-3-7-sonnet-thinking"}
	ensureThinkingParams(body, "claude-3-7-sonnet-thinking")

	thinking, ok := body["thinking"].(map[string]any)
	if !ok {
		t.Fatal("thinking not set")
	}
	if thinking["type"] != "enabled" {
		t.Errorf("type = %q", thinking["type"])
	}
	budget, _ := thinking["budget_tokens"].(int)
	if budget != defaultThinkingBudget {
		t.Errorf("budget = %d", budget)
	}
}

func TestEnsureThinkingParams_AlreadySet(t *testing.T) {
	body := map[string]any{
		"model":    "claude-3-7-sonnet-thinking",
		"thinking": map[string]any{"type": "enabled", "budget_tokens": 5000},
	}
	ensureThinkingParams(body, "claude-3-7-sonnet-thinking")

	thinking := body["thinking"].(map[string]any)
	if thinking["budget_tokens"] != 5000 {
		t.Error("should not overwrite existing thinking config")
	}
}

func TestEnsureThinkingParams_AdjustsMaxTokens(t *testing.T) {
	body := map[string]any{
		"model":      "claude-thinking",
		"max_tokens": float64(2000),
	}
	ensureThinkingParams(body, "claude-thinking")

	mt := body["max_tokens"].(float64)
	// 2000 <= defaultThinkingBudget(10000), so max_tokens should be bumped
	if mt != float64(defaultThinkingBudget+2000) {
		t.Errorf("max_tokens = %v, want %d", mt, defaultThinkingBudget+2000)
	}
}

// ── ensureMaxTokens ────────────────────────────────────────────

func TestEnsureMaxTokens_NotSet(t *testing.T) {
	body := map[string]any{}
	ensureMaxTokens(body)
	if body["max_tokens"] != float64(8192) {
		t.Errorf("max_tokens = %v, want 8192", body["max_tokens"])
	}
}

func TestEnsureMaxTokens_Zero(t *testing.T) {
	body := map[string]any{"max_tokens": float64(0)}
	ensureMaxTokens(body)
	if body["max_tokens"] != float64(8192) {
		t.Errorf("max_tokens = %v, want 8192 (replaced zero)", body["max_tokens"])
	}
}

func TestEnsureMaxTokens_AlreadySet(t *testing.T) {
	body := map[string]any{"max_tokens": float64(4096)}
	ensureMaxTokens(body)
	if body["max_tokens"] != float64(4096) {
		t.Errorf("max_tokens = %v, want 4096 (unchanged)", body["max_tokens"])
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
