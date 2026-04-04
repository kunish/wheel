package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/kunish/wheel/apps/worker/internal/codexruntime"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/runtimeauth"
	"github.com/kunish/wheel/apps/worker/internal/types"
	"github.com/tidwall/gjson"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
)

// fakeCopilotRelay is a test helper that overrides ResolveAccessToken to avoid DB.
type fakeCopilotRelay struct {
	CopilotRelay
	accessTokens map[string]string // channelKey -> accessToken
}

func TestCopilotRelay_ResolveAccessToken_MatchesAuthIndex(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	sqldb, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer sqldb.Close()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT version()")).WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow("8.0.36"))
	db := bun.NewDB(sqldb, mysqldialect.New())

	channelID := 42
	fileName := "github-copilot-testuser.json"
	managedName := codexruntime.ManagedAuthFileName(channelID, fileName)
	authIndex := runtimeauth.EnsureAuthIndex(managedName, "", "")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `codex_auth_file`.`id`, `codex_auth_file`.`channel_id`, `codex_auth_file`.`name`, `codex_auth_file`.`provider`, `codex_auth_file`.`email`, `codex_auth_file`.`disabled`, `codex_auth_file`.`content`, `codex_auth_file`.`created_at`, `codex_auth_file`.`updated_at` FROM `codex_auth_files` AS `codex_auth_file` WHERE (channel_id = ") + "[0-9]+" + regexp.QuoteMeta(") ORDER BY name ASC")).WillReturnRows(
		sqlmock.NewRows([]string{"id", "channel_id", "name", "provider", "email", "disabled", "content", "created_at", "updated_at"}).
			AddRow(1, channelID, fileName, "copilot", "copilot@example.com", false, `{"type":"github-copilot","access_token":"copilot-token"}`, "2026-03-06 00:00:00", "2026-03-06 00:00:00"),
	)

	relay := NewCopilotRelay(db)
	token, err := relay.ResolveAccessToken(context.Background(), channelID, authIndex)
	if err != nil {
		t.Fatalf("ResolveAccessToken() error = %v", err)
	}
	if token != "copilot-token" {
		t.Fatalf("token = %q, want copilot-token", token)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
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
		model := req["model"].(string)
		if model != "claude-opus-4.6" {
			t.Errorf("model = %q, want claude-opus-4.6", model)
		}
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

// ---------------------------------------------------------------------------
// New tests for CLIProxyAPIPlus optimizations
// ---------------------------------------------------------------------------

func TestIsCopilotAgentInitiated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want bool
	}{
		{"empty body", "", false},
		{"user message only", `{"messages":[{"role":"user","content":"hi"}]}`, false},
		{"last role assistant", `{"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"hello"}]}`, true},
		{"last role tool", `{"messages":[{"role":"user","content":"hi"},{"role":"tool","content":"result"}]}`, true},
		{"user with tool_result content", `{"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"ok"}]}]}`, true},
		{"user after assistant with tool_use", `{"messages":[{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"f"}]},{"role":"user","content":"result"}]}`, true},
		{"simple multi-turn follow-up", `{"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"hello"},{"role":"user","content":"thanks"}]}`, false},
		{"responses api function_call_output", `{"input":[{"type":"function_call_output","call_id":"c1","output":"ok"}]}`, true},
		{"responses api function_call", `{"input":[{"type":"function_call","call_id":"c1","name":"f"}]}`, true},
		{"responses api user only", `{"input":[{"type":"message","role":"user","content":"hi"}]}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isCopilotAgentInitiated([]byte(tt.body))
			if got != tt.want {
				t.Errorf("isCopilotAgentInitiated() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectCopilotVisionContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want bool
	}{
		{"no vision", `{"messages":[{"role":"user","content":"hi"}]}`, false},
		{"image_url type", `{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,abc"}}]}]}`, true},
		{"image type", `{"messages":[{"role":"user","content":[{"type":"image","source":{"data":"abc"}}]}]}`, true},
		{"text only array", `{"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := detectCopilotVisionContent([]byte(tt.body))
			if got != tt.want {
				t.Errorf("detectCopilotVisionContent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyCopilotHeaders_NewHeaders(t *testing.T) {
	t.Parallel()

	// Test with agent-initiated body
	agentBody := []byte(`{"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"hello"},{"role":"tool","content":"result"}]}`)
	req := httptest.NewRequest("POST", "https://api.githubcopilot.com/chat/completions", nil)
	applyCopilotHeaders(req, "test-token", agentBody)

	if got := req.Header.Get("X-Request-Id"); got == "" {
		t.Error("X-Request-Id should not be empty")
	}
	if got := req.Header.Get("X-Initiator"); got != "agent" {
		t.Errorf("X-Initiator = %q, want 'agent'", got)
	}
	if got := req.Header.Get("Openai-Intent"); got != "conversation-edits" {
		t.Errorf("Openai-Intent = %q, want 'conversation-edits'", got)
	}

	// Test with user-initiated body
	userBody := []byte(`{"messages":[{"role":"user","content":"hi"}]}`)
	req2 := httptest.NewRequest("POST", "https://api.githubcopilot.com/chat/completions", nil)
	applyCopilotHeaders(req2, "test-token", userBody)

	if got := req2.Header.Get("X-Initiator"); got != "user" {
		t.Errorf("X-Initiator = %q, want 'user'", got)
	}
}

func TestShouldUseCopilotResponsesEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		body  map[string]any
		model string
		want  bool
	}{
		{"chat completions model", map[string]any{"messages": []any{}}, "gpt-4o", false},
		{"codex model", map[string]any{"messages": []any{}}, "codex-mini-latest", true},
		{"body has input field", map[string]any{"input": []any{}}, "gpt-4o", true},
		{"claude model", map[string]any{"messages": []any{}}, "claude-sonnet-4", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := shouldUseCopilotResponsesEndpoint(tt.body, tt.model)
			if got != tt.want {
				t.Errorf("shouldUseCopilotResponsesEndpoint() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeCopilotReasoningField(t *testing.T) {
	t.Parallel()

	t.Run("maps reasoning_text to reasoning_content", func(t *testing.T) {
		t.Parallel()
		input := `{"choices":[{"delta":{"reasoning_text":"thinking..."}}]}`
		result := normalizeCopilotReasoningField([]byte(input))
		var obj map[string]any
		_ = json.Unmarshal(result, &obj)

		choices := obj["choices"].([]any)
		delta := choices[0].(map[string]any)["delta"].(map[string]any)
		if delta["reasoning_content"] != "thinking..." {
			t.Errorf("reasoning_content = %v, want 'thinking...'", delta["reasoning_content"])
		}
	})

	t.Run("preserves existing reasoning_content", func(t *testing.T) {
		t.Parallel()
		input := `{"choices":[{"delta":{"reasoning_text":"old","reasoning_content":"existing"}}]}`
		result := normalizeCopilotReasoningField([]byte(input))
		var obj map[string]any
		_ = json.Unmarshal(result, &obj)

		choices := obj["choices"].([]any)
		delta := choices[0].(map[string]any)["delta"].(map[string]any)
		if delta["reasoning_content"] != "existing" {
			t.Errorf("reasoning_content = %v, want 'existing'", delta["reasoning_content"])
		}
	})
}

func TestFlattenCopilotAssistantContent(t *testing.T) {
	t.Parallel()

	t.Run("flattens text-only array", func(t *testing.T) {
		t.Parallel()
		input := `{"messages":[{"role":"assistant","content":[{"type":"text","text":"hello"},{"type":"text","text":" world"}]}]}`
		result := flattenCopilotAssistantContent([]byte(input))
		content := gjson.GetBytes(result, "messages.0.content").String()
		if content != "hello world" {
			t.Errorf("content = %q, want 'hello world'", content)
		}
	})

	t.Run("skips non-text content", func(t *testing.T) {
		t.Parallel()
		input := `{"messages":[{"role":"assistant","content":[{"type":"text","text":"hi"},{"type":"tool_use","id":"t1"}]}]}`
		result := flattenCopilotAssistantContent([]byte(input))
		content := gjson.GetBytes(result, "messages.0.content")
		if !content.IsArray() {
			t.Error("content should remain as array when tool_use is present")
		}
	})
}

func TestNormalizeCopilotChatTools(t *testing.T) {
	t.Parallel()

	t.Run("filters non-function tools", func(t *testing.T) {
		t.Parallel()
		input := `{"tools":[{"type":"function","function":{"name":"f1"}},{"type":"computer","name":"c1"}],"tool_choice":"auto"}`
		result := normalizeCopilotChatTools([]byte(input))
		tools := gjson.GetBytes(result, "tools")
		if len(tools.Array()) != 1 {
			t.Errorf("tools count = %d, want 1", len(tools.Array()))
		}
	})

	t.Run("invalid tool_choice falls back to auto", func(t *testing.T) {
		t.Parallel()
		input := `{"tool_choice":"invalid"}`
		result := normalizeCopilotChatTools([]byte(input))
		tc := gjson.GetBytes(result, "tool_choice").String()
		if tc != "auto" {
			t.Errorf("tool_choice = %q, want 'auto'", tc)
		}
	})
}

// ---------------------------------------------------------------------------
// Tests for stripCopilotUnsupportedBetas
// ---------------------------------------------------------------------------

func TestStripCopilotUnsupportedBetas_RemovesContext1M(t *testing.T) {
	t.Parallel()
	body := []byte(`{"model":"claude-opus-4.6","betas":["interleaved-thinking-2025-05-14","context-1m-2025-08-07","claude-code-20250219"],"messages":[]}`)
	result := stripCopilotUnsupportedBetas(body)

	betas := gjson.GetBytes(result, "betas")
	if !betas.Exists() {
		t.Fatal("betas field should still exist after stripping")
	}
	for _, item := range betas.Array() {
		if item.String() == "context-1m-2025-08-07" {
			t.Fatal("context-1m-2025-08-07 should have been stripped")
		}
	}
	found := false
	for _, item := range betas.Array() {
		if item.String() == "interleaved-thinking-2025-05-14" {
			found = true
		}
	}
	if !found {
		t.Fatal("other betas should be preserved")
	}
}

func TestStripCopilotUnsupportedBetas_NoBetasField(t *testing.T) {
	t.Parallel()
	body := []byte(`{"model":"gpt-4o","messages":[]}`)
	result := stripCopilotUnsupportedBetas(body)
	if string(result) != string(body) {
		t.Fatalf("body should be unchanged when no betas field exists, got %s", string(result))
	}
}

func TestStripCopilotUnsupportedBetas_MetadataBetas(t *testing.T) {
	t.Parallel()
	body := []byte(`{"model":"claude-opus-4.6","metadata":{"betas":["context-1m-2025-08-07","other-beta"]},"messages":[]}`)
	result := stripCopilotUnsupportedBetas(body)

	betas := gjson.GetBytes(result, "metadata.betas")
	if !betas.Exists() {
		t.Fatal("metadata.betas field should still exist after stripping")
	}
	for _, item := range betas.Array() {
		if item.String() == "context-1m-2025-08-07" {
			t.Fatal("context-1m-2025-08-07 should have been stripped from metadata.betas")
		}
	}
	if betas.Array()[0].String() != "other-beta" {
		t.Fatal("other betas in metadata.betas should be preserved")
	}
}

func TestStripCopilotUnsupportedBetas_AllBetasStripped(t *testing.T) {
	t.Parallel()
	body := []byte(`{"model":"claude-opus-4.6","betas":["context-1m-2025-08-07"],"messages":[]}`)
	result := stripCopilotUnsupportedBetas(body)

	betas := gjson.GetBytes(result, "betas")
	if betas.Exists() {
		t.Fatal("betas field should be deleted when all betas are stripped")
	}
}

// ---------------------------------------------------------------------------
// Tests for applyCopilotResponsesDefaults
// ---------------------------------------------------------------------------

func TestApplyCopilotResponsesDefaults_SetsAllDefaults(t *testing.T) {
	t.Parallel()
	body := []byte(`{"input":"hello","reasoning":{"effort":"medium"}}`)
	got := applyCopilotResponsesDefaults(body)

	if gjson.GetBytes(got, "store").Bool() != false {
		t.Fatalf("store = %v, want false", gjson.GetBytes(got, "store").Raw)
	}
	inc := gjson.GetBytes(got, "include")
	if !inc.IsArray() || inc.Array()[0].String() != "reasoning.encrypted_content" {
		t.Fatalf("include = %s, want [\"reasoning.encrypted_content\"]", inc.Raw)
	}
	if gjson.GetBytes(got, "reasoning.summary").String() != "auto" {
		t.Fatalf("reasoning.summary = %q, want auto", gjson.GetBytes(got, "reasoning.summary").String())
	}
}

func TestApplyCopilotResponsesDefaults_DoesNotOverrideExisting(t *testing.T) {
	t.Parallel()
	body := []byte(`{"input":"hello","store":true,"include":["other"],"reasoning":{"effort":"high","summary":"concise"}}`)
	got := applyCopilotResponsesDefaults(body)

	if gjson.GetBytes(got, "store").Bool() != true {
		t.Fatalf("store should not be overridden, got %s", gjson.GetBytes(got, "store").Raw)
	}
	if gjson.GetBytes(got, "include").Array()[0].String() != "other" {
		t.Fatalf("include should not be overridden, got %s", gjson.GetBytes(got, "include").Raw)
	}
	if gjson.GetBytes(got, "reasoning.summary").String() != "concise" {
		t.Fatalf("reasoning.summary should not be overridden, got %q", gjson.GetBytes(got, "reasoning.summary").String())
	}
}

func TestApplyCopilotResponsesDefaults_NoReasoningEffort(t *testing.T) {
	t.Parallel()
	body := []byte(`{"input":"hello"}`)
	got := applyCopilotResponsesDefaults(body)

	if gjson.GetBytes(got, "store").Bool() != false {
		t.Fatalf("store = %v, want false", gjson.GetBytes(got, "store").Raw)
	}
	if gjson.GetBytes(got, "reasoning.summary").Exists() {
		t.Fatalf("reasoning.summary should not be set when reasoning.effort is absent, got %q", gjson.GetBytes(got, "reasoning.summary").String())
	}
}

// ---------------------------------------------------------------------------
// Additional normalizeCopilotReasoningField tests
// ---------------------------------------------------------------------------

func TestNormalizeCopilotReasoningField_MultiChoice(t *testing.T) {
	t.Parallel()
	data := []byte(`{"choices":[{"message":{"reasoning_text":"thought-0"}},{"message":{"reasoning_text":"thought-1"}}]}`)
	got := normalizeCopilotReasoningField(data)
	rc0 := gjson.GetBytes(got, "choices.0.message.reasoning_content").String()
	rc1 := gjson.GetBytes(got, "choices.1.message.reasoning_content").String()
	if rc0 != "thought-0" {
		t.Fatalf("choices[0].reasoning_content = %q, want %q", rc0, "thought-0")
	}
	if rc1 != "thought-1" {
		t.Fatalf("choices[1].reasoning_content = %q, want %q", rc1, "thought-1")
	}
}

func TestNormalizeCopilotReasoningField_NoChoices(t *testing.T) {
	t.Parallel()
	data := []byte(`{"id":"chatcmpl-123"}`)
	got := normalizeCopilotReasoningField(data)
	if string(got) != string(data) {
		t.Fatalf("expected no change, got %s", string(got))
	}
}

func TestNormalizeCopilotReasoningField_NonStreaming(t *testing.T) {
	t.Parallel()
	data := []byte(`{"choices":[{"message":{"content":"hello","reasoning_text":"I think..."}}]}`)
	got := normalizeCopilotReasoningField(data)
	rc := gjson.GetBytes(got, "choices.0.message.reasoning_content").String()
	if rc != "I think..." {
		t.Fatalf("reasoning_content = %q, want %q", rc, "I think...")
	}
}

func TestNormalizeCopilotReasoningField_StreamingDelta(t *testing.T) {
	t.Parallel()
	data := []byte(`{"choices":[{"delta":{"reasoning_text":"thinking delta"}}]}`)
	got := normalizeCopilotReasoningField(data)
	rc := gjson.GetBytes(got, "choices.0.delta.reasoning_content").String()
	if rc != "thinking delta" {
		t.Fatalf("reasoning_content = %q, want %q", rc, "thinking delta")
	}
}

// ---------------------------------------------------------------------------
// Tests for normalizeCopilotReasoningFieldSSE
// ---------------------------------------------------------------------------

func TestNormalizeCopilotReasoningFieldSSE_Normalizes(t *testing.T) {
	t.Parallel()
	line := []byte(`data: {"choices":[{"delta":{"reasoning_text":"thinking..."}}]}`)
	got := normalizeCopilotReasoningFieldSSE(line)
	if !strings.Contains(string(got), "reasoning_content") {
		t.Fatalf("expected reasoning_content in output, got %s", string(got))
	}
	if !strings.HasPrefix(string(got), "data: ") {
		t.Fatalf("expected data: prefix, got %s", string(got))
	}
}

func TestNormalizeCopilotReasoningFieldSSE_PassthroughNonData(t *testing.T) {
	t.Parallel()
	line := []byte("event: ping")
	got := normalizeCopilotReasoningFieldSSE(line)
	if string(got) != string(line) {
		t.Fatalf("non-data line should pass through unchanged, got %s", string(got))
	}
}

func TestNormalizeCopilotReasoningFieldSSE_PassthroughDone(t *testing.T) {
	t.Parallel()
	line := []byte("data: [DONE]")
	got := normalizeCopilotReasoningFieldSSE(line)
	if string(got) != string(line) {
		t.Fatalf("[DONE] should pass through unchanged, got %s", string(got))
	}
}

func TestNormalizeCopilotReasoningFieldSSE_NoChangeWhenNoReasoningText(t *testing.T) {
	t.Parallel()
	line := []byte(`data: {"choices":[{"delta":{"content":"hello"}}]}`)
	got := normalizeCopilotReasoningFieldSSE(line)
	if string(got) != string(line) {
		t.Fatalf("line without reasoning_text should be unchanged, got %s", string(got))
	}
}

// ---------------------------------------------------------------------------
// Additional isCopilotAgentInitiated tests
// ---------------------------------------------------------------------------

func TestIsCopilotAgentInitiated_UserFollowUpAfterToolHistory(t *testing.T) {
	t.Parallel()
	// User follow-up after a completed tool-use conversation.
	// The last message is a genuine user question — should be "user", not "agent".
	body := []byte(`{"messages":[{"role":"user","content":"hello"},{"role":"assistant","content":[{"type":"tool_use","id":"tu1","name":"Read","input":{}}]},{"role":"tool","tool_call_id":"tu1","content":"file data"},{"role":"assistant","content":"I read the file."},{"role":"user","content":"What did we do so far?"}]}`)
	got := isCopilotAgentInitiated(body)
	if got != false {
		t.Fatalf("isCopilotAgentInitiated() = %v, want false (genuine follow-up after tool history)", got)
	}
}

func TestIsCopilotAgentInitiated_ResponsesAPIHistoryHasAssistant(t *testing.T) {
	t.Parallel()
	// Responses API: last item is user-role but history contains assistant → agent.
	body := []byte(`{"input":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"I can help"}]},{"type":"message","role":"user","content":[{"type":"input_text","text":"Do X"}]}]}`)
	got := isCopilotAgentInitiated(body)
	if got != true {
		t.Fatalf("isCopilotAgentInitiated() = %v, want true (history has assistant)", got)
	}
}

// ---------------------------------------------------------------------------
// Test for OpenAI-Intent header value
// ---------------------------------------------------------------------------

func TestApplyCopilotHeaders_OpenAIIntentValue(t *testing.T) {
	t.Parallel()
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	applyCopilotHeaders(req, "token", nil)
	if got := req.Header.Get("Openai-Intent"); got != "conversation-edits" {
		t.Fatalf("Openai-Intent = %q, want conversation-edits", got)
	}
}

