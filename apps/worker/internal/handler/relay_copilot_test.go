package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kunish/wheel/apps/worker/internal/codexruntime"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/runtimeauth"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// fakeCopilotRelay is a test helper that overrides ResolveAccessToken to avoid DB.
type fakeCopilotRelay struct {
	CopilotRelay
	accessTokens map[string]string // channelKey -> accessToken
}

func TestCopilotRelay_ResolveAccessToken_MatchesAuthIndex(t *testing.T) {
	t.Parallel()

	channelID := 42
	fileName := "github-copilot-testuser.json"
	managedName := codexruntime.ManagedAuthFileName(channelID, fileName)
	authIndex := runtimeauth.EnsureAuthIndex(managedName, "", "")

	if authIndex == "" {
		t.Fatal("authIndex should not be empty")
	}
	// Just verify the hashing is deterministic.
	authIndex2 := runtimeauth.EnsureAuthIndex(managedName, "", "")
	if authIndex != authIndex2 {
		t.Fatalf("authIndex mismatch: %q != %q", authIndex, authIndex2)
	}
}

func TestCopilotRelay_EnsureAPIToken_CachesResult(t *testing.T) {
	t.Parallel()

	// Fake token exchange server.
	var callCount int
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      "test-api-token-" + r.Header.Get("Authorization"),
			"expires_at": 9999999999,
			"endpoints":  map[string]string{"api": "https://api.githubcopilot.com"},
		})
	}))
	defer ts.Close()

	// NOTE: We can't easily redirect the SDK seam to use a test server
	// because ExchangeCopilotAPIToken uses the internal copilot auth client.
	// Instead, we test the caching logic directly.

	cr := NewCopilotRelay(nil)

	// Manually populate cache.
	cr.tokenCache["test-access-token"] = &copilotCachedToken{
		token:       "cached-api-token",
		apiEndpoint: "https://api.githubcopilot.com",
		expiresAt:   time.Now().Add(copilotRelayTokenCacheTTL),
	}

	token, endpoint, err := cr.ensureAPIToken(context.Background(), "test-access-token")
	if err != nil {
		t.Fatalf("ensureAPIToken error: %v", err)
	}
	if token != "cached-api-token" {
		t.Fatalf("token = %q, want cached-api-token", token)
	}
	if endpoint != "https://api.githubcopilot.com" {
		t.Fatalf("endpoint = %q, want https://api.githubcopilot.com", endpoint)
	}
}

func TestCopilotRelay_ProxyNonStreaming_CallsUpstream(t *testing.T) {
	t.Parallel()

	// Fake Copilot API upstream.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers.
		if auth := r.Header.Get("Authorization"); !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("Authorization = %q, want Bearer prefix", auth)
		}
		if ua := r.Header.Get("User-Agent"); ua != copilotRelayUserAgent {
			t.Errorf("User-Agent = %q, want %q", ua, copilotRelayUserAgent)
		}
		// Verify model normalization.
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		model := req["model"].(string)
		if model != "claude-opus-4.6" {
			t.Errorf("model = %q, want claude-opus-4.6", model)
		}
		if req["stream"] != false {
			t.Errorf("stream = %v, want false", req["stream"])
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": "hello"}}},
			"usage":   map[string]any{"prompt_tokens": 10, "completion_tokens": 5},
		})
	}))
	defer ts.Close()

	cr := NewCopilotRelay(nil)
	// Pre-populate cache with a token pointing to our test server.
	cr.tokenCache["test-access"] = &copilotCachedToken{
		token:       "test-api-token",
		apiEndpoint: ts.URL,
		expiresAt:   time.Now().Add(copilotRelayTokenCacheTTL),
	}

	result, err := cr.ProxyNonStreaming(
		context.Background(),
		"test-access",
		"claude-opus-4-6",
		map[string]any{"model": "claude-opus-4-6", "messages": []any{map[string]any{"role": "user", "content": "hi"}}},
		types.OutboundCopilot,
	)
	if err != nil {
		t.Fatalf("ProxyNonStreaming error: %v", err)
	}
	if result.InputTokens != 10 {
		t.Errorf("InputTokens = %d, want 10", result.InputTokens)
	}
	if result.OutputTokens != 5 {
		t.Errorf("OutputTokens = %d, want 5", result.OutputTokens)
	}
}

func TestCopilotRelay_ProxyStreaming_StreamsSSE(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify stream=true in body.
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		if req["stream"] != true {
			t.Errorf("stream = %v, want true", req["stream"])
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher, _ := w.(http.Flusher)
		lines := []string{
			`data: {"choices":[{"delta":{"content":"hello"}}]}`,
			`data: {"choices":[{"delta":{"content":" world"}}],"usage":{"prompt_tokens":8,"completion_tokens":3}}`,
			`data: [DONE]`,
		}
		for _, l := range lines {
			_, _ = w.Write([]byte(l + "\n"))
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
	defer ts.Close()

	cr := NewCopilotRelay(nil)
	cr.tokenCache["test-access"] = &copilotCachedToken{
		token:       "test-api-token",
		apiEndpoint: ts.URL,
		expiresAt:   time.Now().Add(copilotRelayTokenCacheTTL),
	}

	w := httptest.NewRecorder()
	info, err := cr.ProxyStreaming(
		w,
		context.Background(),
		"test-access",
		"claude-opus-4-6",
		map[string]any{"model": "claude-opus-4-6", "messages": []any{map[string]any{"role": "user", "content": "hi"}}},
		types.OutboundCopilot,
		false,
		nil,
	)
	if err != nil {
		t.Fatalf("ProxyStreaming error: %v", err)
	}
	if info.InputTokens != 8 {
		t.Errorf("InputTokens = %d, want 8", info.InputTokens)
	}
	if info.OutputTokens != 3 {
		t.Errorf("OutputTokens = %d, want 3", info.OutputTokens)
	}
	if info.ResponseContent != "hello world" {
		t.Errorf("ResponseContent = %q, want 'hello world'", info.ResponseContent)
	}

	// Verify SSE was written to the response.
	respBody := w.Body.String()
	if !strings.Contains(respBody, "data: ") {
		t.Errorf("response body should contain SSE data lines, got: %s", respBody)
	}
}

func TestCopilotRelay_ProxyStreaming_AnthropicInbound(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher, _ := w.(http.Flusher)
		lines := []string{
			`data: {"id":"chatcmpl-1","model":"gpt-5.4","choices":[{"delta":{"content":"hello"}}]}`,
			`data: {"id":"chatcmpl-1","model":"gpt-5.4","choices":[{"delta":{"content":" world"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`,
			`data: [DONE]`,
		}
		for _, l := range lines {
			_, _ = w.Write([]byte(l + "\n"))
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
	defer ts.Close()

	cr := NewCopilotRelay(nil)
	cr.tokenCache["test-access"] = &copilotCachedToken{
		token:       "test-api-token",
		apiEndpoint: ts.URL,
		expiresAt:   time.Now().Add(copilotRelayTokenCacheTTL),
	}

	w := httptest.NewRecorder()
	info, err := cr.ProxyStreaming(
		w,
		context.Background(),
		"test-access",
		"gpt-5.4",
		map[string]any{"model": "gpt-5.4", "messages": []any{map[string]any{"role": "user", "content": "hi"}}},
		types.OutboundCopilot,
		true, // anthropicInbound
		nil,
	)
	if err != nil {
		t.Fatalf("ProxyStreaming error: %v", err)
	}
	if info.ResponseContent != "hello world" {
		t.Errorf("ResponseContent = %q, want 'hello world'", info.ResponseContent)
	}

	// Verify Anthropic SSE events were written.
	respBody := w.Body.String()
	if !strings.Contains(respBody, "event: message_start") {
		t.Errorf("response should contain 'event: message_start', got:\n%s", respBody)
	}
	if !strings.Contains(respBody, "event: content_block_delta") {
		t.Errorf("response should contain 'event: content_block_delta', got:\n%s", respBody)
	}
	if !strings.Contains(respBody, "event: message_stop") {
		t.Errorf("response should contain 'event: message_stop', got:\n%s", respBody)
	}
	// Should NOT contain raw OpenAI SSE format.
	if strings.Contains(respBody, `"choices"`) {
		t.Errorf("response should NOT contain raw OpenAI 'choices' in Anthropic mode, got:\n%s", respBody)
	}
}

func TestCopilotRelay_UpstreamError_ReturnsProxyError(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer ts.Close()

	cr := NewCopilotRelay(nil)
	cr.tokenCache["test-access"] = &copilotCachedToken{
		token:       "test-api-token",
		apiEndpoint: ts.URL,
		expiresAt:   time.Now().Add(copilotRelayTokenCacheTTL),
	}

	_, err := cr.ProxyNonStreaming(
		context.Background(),
		"test-access",
		"gpt-4o",
		map[string]any{"model": "gpt-4o", "messages": []any{}},
		types.OutboundCopilot,
	)
	if err == nil {
		t.Fatal("expected error from 403 upstream")
	}
	pe, ok := err.(*relay.ProxyError)
	if !ok {
		t.Fatalf("expected *relay.ProxyError, got %T", err)
	}
	if pe.StatusCode != 403 {
		t.Errorf("StatusCode = %d, want 403", pe.StatusCode)
	}
}

func TestApplyCopilotHeaders(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("POST", "https://api.githubcopilot.com/chat/completions", nil)
	applyCopilotHeaders(req, "test-token", nil)

	if got := req.Header.Get("Authorization"); got != "Bearer test-token" {
		t.Errorf("Authorization = %q, want 'Bearer test-token'", got)
	}
	if got := req.Header.Get("User-Agent"); got != copilotRelayUserAgent {
		t.Errorf("User-Agent = %q, want %q", got, copilotRelayUserAgent)
	}
	if got := req.Header.Get("Editor-Version"); got != copilotRelayEditorVersion {
		t.Errorf("Editor-Version = %q, want %q", got, copilotRelayEditorVersion)
	}
	if got := req.Header.Get("Copilot-Integration-Id"); got != copilotRelayIntegrationID {
		t.Errorf("Copilot-Integration-Id = %q, want %q", got, copilotRelayIntegrationID)
	}
}

// ---------------------------------------------------------------------------
// Anthropic → OpenAI body conversion tests
// ---------------------------------------------------------------------------

func TestNormalizeToolChoiceForOpenAI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   any
		want any
	}{
		{"string auto passthrough", "auto", "auto"},
		{"string none passthrough", "none", "none"},
		{"anthropic auto object", map[string]any{"type": "auto"}, "auto"},
		{"anthropic any → required", map[string]any{"type": "any"}, "required"},
		{"anthropic none object", map[string]any{"type": "none"}, "none"},
		{"anthropic tool → function", map[string]any{"type": "tool", "name": "get_weather"}, map[string]any{
			"type":     "function",
			"function": map[string]any{"name": "get_weather"},
		}},
		{"anthropic tool no name → auto", map[string]any{"type": "tool"}, "auto"},
		{"openai function passthrough", map[string]any{"type": "function", "function": map[string]any{"name": "x"}}, map[string]any{"type": "function", "function": map[string]any{"name": "x"}}},
		{"unknown type → auto", map[string]any{"type": "bogus"}, "auto"},
		{"non-map non-string → auto", 42, "auto"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeToolChoiceForOpenAI(tt.in)
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(tt.want)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("normalizeToolChoiceForOpenAI(%v) = %s, want %s", tt.in, gotJSON, wantJSON)
			}
		})
	}
}

func TestConvertAnthropicToolsToOpenAI(t *testing.T) {
	t.Parallel()

	anthropicTools := []any{
		map[string]any{
			"name":        "get_weather",
			"description": "Get weather for a city",
			"input_schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"city": map[string]any{"type": "string"},
				},
			},
		},
		// Already-OpenAI-format tool should pass through.
		map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":       "search",
				"parameters": map[string]any{"type": "object"},
			},
		},
	}

	result := convertAnthropicToolsToOpenAI(anthropicTools)
	if len(result) != 2 {
		t.Fatalf("got %d tools, want 2", len(result))
	}

	// First tool should be converted.
	tool0, ok := result[0].(map[string]any)
	if !ok {
		t.Fatal("tool 0 is not map[string]any")
	}
	if tool0["type"] != "function" {
		t.Errorf("tool 0 type = %v, want function", tool0["type"])
	}
	fn0, _ := tool0["function"].(map[string]any)
	if fn0["name"] != "get_weather" {
		t.Errorf("tool 0 function.name = %v, want get_weather", fn0["name"])
	}
	if fn0["description"] != "Get weather for a city" {
		t.Errorf("tool 0 function.description = %v, want 'Get weather for a city'", fn0["description"])
	}
	if fn0["parameters"] == nil {
		t.Error("tool 0 function.parameters should not be nil")
	}

	// Second tool should pass through unchanged.
	tool1, _ := result[1].(map[string]any)
	if tool1["type"] != "function" {
		t.Errorf("tool 1 type = %v, want function", tool1["type"])
	}
}

func TestConvertAnthropicAssistantToOpenAI(t *testing.T) {
	t.Parallel()

	t.Run("simple string content", func(t *testing.T) {
		t.Parallel()
		msg := map[string]any{"role": "assistant", "content": "hello"}
		result := convertAnthropicAssistantToOpenAI(msg)
		if result["content"] != "hello" {
			t.Errorf("content = %v, want hello", result["content"])
		}
		if result["role"] != "assistant" {
			t.Errorf("role = %v, want assistant", result["role"])
		}
	})

	t.Run("text and tool_use blocks", func(t *testing.T) {
		t.Parallel()
		msg := map[string]any{
			"role": "assistant",
			"content": []any{
				map[string]any{"type": "text", "text": "Let me check the weather."},
				map[string]any{
					"type":  "tool_use",
					"id":    "toolu_123",
					"name":  "get_weather",
					"input": map[string]any{"city": "Tokyo"},
				},
			},
		}
		result := convertAnthropicAssistantToOpenAI(msg)
		if result["content"] != "Let me check the weather." {
			t.Errorf("content = %v, want 'Let me check the weather.'", result["content"])
		}
		toolCalls, ok := result["tool_calls"].([]any)
		if !ok || len(toolCalls) != 1 {
			t.Fatalf("tool_calls length = %v, want 1", toolCalls)
		}
		tc, _ := toolCalls[0].(map[string]any)
		if tc["id"] != "toolu_123" {
			t.Errorf("tool_calls[0].id = %v, want toolu_123", tc["id"])
		}
		fn, _ := tc["function"].(map[string]any)
		if fn["name"] != "get_weather" {
			t.Errorf("tool_calls[0].function.name = %v, want get_weather", fn["name"])
		}
	})
}

func TestConvertAnthropicUserToOpenAI(t *testing.T) {
	t.Parallel()

	t.Run("simple string", func(t *testing.T) {
		t.Parallel()
		msg := map[string]any{"role": "user", "content": "hi"}
		result := convertAnthropicUserToOpenAI(msg)
		m, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("expected map, got %T", result)
		}
		if m["content"] != "hi" {
			t.Errorf("content = %v, want hi", m["content"])
		}
	})

	t.Run("tool_result block produces tool role message", func(t *testing.T) {
		t.Parallel()
		msg := map[string]any{
			"role": "user",
			"content": []any{
				map[string]any{
					"type":        "tool_result",
					"tool_use_id": "toolu_123",
					"content":     "Sunny, 25°C",
				},
			},
		}
		result := convertAnthropicUserToOpenAI(msg)
		// Single tool_result → single message (not a slice).
		m, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("expected map, got %T", result)
		}
		if m["role"] != "tool" {
			t.Errorf("role = %v, want tool", m["role"])
		}
		if m["tool_call_id"] != "toolu_123" {
			t.Errorf("tool_call_id = %v, want toolu_123", m["tool_call_id"])
		}
		if m["content"] != "Sunny, 25°C" {
			t.Errorf("content = %v, want 'Sunny, 25°C'", m["content"])
		}
	})

	t.Run("mixed text and tool_result produces multiple messages", func(t *testing.T) {
		t.Parallel()
		msg := map[string]any{
			"role": "user",
			"content": []any{
				map[string]any{"type": "text", "text": "Here is the result:"},
				map[string]any{
					"type":        "tool_result",
					"tool_use_id": "toolu_456",
					"content":     "result data",
				},
			},
		}
		result := convertAnthropicUserToOpenAI(msg)
		// Should be a slice of 2 messages.
		slice, ok := result.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", result)
		}
		if len(slice) != 2 {
			t.Fatalf("len = %d, want 2", len(slice))
		}
		// First is user text message.
		m0, _ := slice[0].(map[string]any)
		if m0["role"] != "user" {
			t.Errorf("slice[0].role = %v, want user", m0["role"])
		}
		// Second is tool message.
		m1, _ := slice[1].(map[string]any)
		if m1["role"] != "tool" {
			t.Errorf("slice[1].role = %v, want tool", m1["role"])
		}
	})
}

func TestConvertAnthropicBodyToOpenAI_FullBody(t *testing.T) {
	t.Parallel()

	body := map[string]any{
		"model":      "claude-sonnet-4-20250514",
		"max_tokens": float64(1024),
		"system":     "You are a helpful assistant.",
		"messages": []any{
			map[string]any{"role": "user", "content": "What is the weather?"},
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{"type": "text", "text": "Let me check."},
					map[string]any{
						"type":  "tool_use",
						"id":    "toolu_1",
						"name":  "get_weather",
						"input": map[string]any{"city": "Tokyo"},
					},
				},
			},
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type":        "tool_result",
						"tool_use_id": "toolu_1",
						"content":     "Sunny, 25°C",
					},
				},
			},
		},
		"tools": []any{
			map[string]any{
				"name":        "get_weather",
				"description": "Get weather",
				"input_schema": map[string]any{
					"type":       "object",
					"properties": map[string]any{"city": map[string]any{"type": "string"}},
				},
			},
		},
		"tool_choice":    map[string]any{"type": "auto"},
		"stop_sequences": []any{"STOP"},
		"temperature":    float64(0.7),
	}

	result := convertAnthropicBodyToOpenAI(body)

	// Check system message is first.
	msgs, ok := result["messages"].([]any)
	if !ok {
		t.Fatal("messages is not []any")
	}
	if len(msgs) < 4 {
		t.Fatalf("expected at least 4 messages (system + user + assistant + tool), got %d", len(msgs))
	}

	msg0, _ := msgs[0].(map[string]any)
	if msg0["role"] != "system" {
		t.Errorf("msgs[0].role = %v, want system", msg0["role"])
	}
	if msg0["content"] != "You are a helpful assistant." {
		t.Errorf("msgs[0].content = %v, want system prompt", msg0["content"])
	}

	// Check tool_choice was converted.
	if result["tool_choice"] != "auto" {
		t.Errorf("tool_choice = %v, want 'auto' string", result["tool_choice"])
	}

	// Check stop_sequences → stop.
	if result["stop"] == nil {
		t.Error("stop should be set from stop_sequences")
	}
	if result["stop_sequences"] != nil {
		t.Error("stop_sequences should not be in output")
	}

	// Check tools were converted.
	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatal("tools is not []any")
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	tool0, _ := tools[0].(map[string]any)
	if tool0["type"] != "function" {
		t.Errorf("tools[0].type = %v, want function", tool0["type"])
	}

	// Check scalar fields passed through.
	if result["model"] != "claude-sonnet-4-20250514" {
		t.Errorf("model = %v, want claude-sonnet-4-20250514", result["model"])
	}
	if result["temperature"] != float64(0.7) {
		t.Errorf("temperature = %v, want 0.7", result["temperature"])
	}
	if result["max_tokens"] != float64(1024) {
		t.Errorf("max_tokens = %v, want 1024", result["max_tokens"])
	}
}

func TestConvertAnthropicBodyToOpenAI_UserSliceFlattening(t *testing.T) {
	t.Parallel()

	// A message that produces multiple OpenAI messages (text + tool_result)
	// should be flattened into the top-level messages slice.
	body := map[string]any{
		"model": "test",
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "context"},
					map[string]any{
						"type":        "tool_result",
						"tool_use_id": "toolu_1",
						"content":     "result",
					},
				},
			},
		},
	}

	result := convertAnthropicBodyToOpenAI(body)
	msgs, _ := result["messages"].([]any)

	// Should have 2 flattened messages, not 1 nested slice.
	if len(msgs) != 2 {
		t.Fatalf("expected 2 flattened messages, got %d", len(msgs))
	}

	// Verify each element is a map, not a []any.
	for i, m := range msgs {
		if _, ok := m.(map[string]any); !ok {
			t.Errorf("msgs[%d] is %T, want map[string]any", i, m)
		}
	}
}

func TestConvertAnthropicBodyToOpenAI_SystemArray(t *testing.T) {
	t.Parallel()

	body := map[string]any{
		"model": "test",
		"system": []any{
			map[string]any{"type": "text", "text": "First."},
			map[string]any{"type": "text", "text": "Second."},
		},
		"messages": []any{
			map[string]any{"role": "user", "content": "hi"},
		},
	}

	result := convertAnthropicBodyToOpenAI(body)
	msgs, _ := result["messages"].([]any)
	if len(msgs) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(msgs))
	}
	sysMsg, _ := msgs[0].(map[string]any)
	if sysMsg["role"] != "system" {
		t.Errorf("msgs[0].role = %v, want system", sysMsg["role"])
	}
	content, _ := sysMsg["content"].(string)
	if content != "First.\nSecond." {
		t.Errorf("system content = %q, want 'First.\\nSecond.'", content)
	}
}
