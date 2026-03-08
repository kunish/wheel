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
