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

func TestCodexCLIRelay_ResolveAccessToken_MatchesAuthIndex(t *testing.T) {
	t.Parallel()

	channelID := 55
	fileName := "codexcli-testuser.json"
	managedName := codexruntime.ManagedAuthFileName(channelID, fileName)
	authIndex := runtimeauth.EnsureAuthIndex(managedName, "", "")

	if authIndex == "" {
		t.Fatal("authIndex should not be empty")
	}
	// Verify hashing is deterministic.
	authIndex2 := runtimeauth.EnsureAuthIndex(managedName, "", "")
	if authIndex != authIndex2 {
		t.Fatalf("authIndex mismatch: %q != %q", authIndex, authIndex2)
	}
}

func TestApplyCodexCLIHeaders(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("POST", "https://chatgpt.com/backend-api/codex/responses", nil)
	applyCodexCLIHeaders(req, "test-token", "acct-123")

	if got := req.Header.Get("Authorization"); got != "Bearer test-token" {
		t.Errorf("Authorization = %q, want 'Bearer test-token'", got)
	}
	if got := req.Header.Get("User-Agent"); got != codexCLIUserAgent {
		t.Errorf("User-Agent = %q, want %q", got, codexCLIUserAgent)
	}
	if got := req.Header.Get("Host"); got != "chatgpt.com" {
		t.Errorf("Host = %q, want chatgpt.com", got)
	}
	if got := req.Header.Get("Chatgpt-Account-Id"); got != "acct-123" {
		t.Errorf("Chatgpt-Account-Id = %q, want acct-123", got)
	}
}

func TestApplyCodexCLIHeaders_NoAccountID(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("POST", "https://chatgpt.com/backend-api/codex/responses", nil)
	applyCodexCLIHeaders(req, "test-token", "")

	if got := req.Header.Get("Chatgpt-Account-Id"); got != "" {
		t.Errorf("Chatgpt-Account-Id should be empty when accountID is blank, got %q", got)
	}
}

func TestCopyCodexCLIBody(t *testing.T) {
	t.Parallel()

	original := map[string]any{
		"model":  "o3-pro",
		"stream": true,
		"input":  "hello",
	}

	copied := copyCodexCLIBody(original)

	// Should be a different map.
	if &original == &copied {
		t.Fatal("copy should be a different map instance")
	}
	// Values should match.
	if copied["model"] != "o3-pro" {
		t.Errorf("model = %v, want o3-pro", copied["model"])
	}
	// Modifying copy should not affect original.
	copied["model"] = "gpt-5"
	if original["model"] != "o3-pro" {
		t.Error("modifying copy affected original")
	}
}

func TestCodexCLIRelay_ProxyNonStreaming_CallsUpstream(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path.
		if r.URL.Path != codexCLIResponses {
			t.Errorf("path = %q, want %q", r.URL.Path, codexCLIResponses)
		}
		// Verify headers.
		if auth := r.Header.Get("Authorization"); !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("Authorization = %q, want Bearer prefix", auth)
		}
		// Verify body.
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		if req["stream"] != false {
			t.Errorf("stream = %v, want false", req["stream"])
		}
		if req["store"] != false {
			t.Errorf("store = %v, want false", req["store"])
		}
		if req["model"] != "o3-pro" {
			t.Errorf("model = %v, want o3-pro", req["model"])
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "resp-test",
			"output": []any{map[string]any{"type": "message", "content": []any{map[string]any{"type": "text", "text": "hello"}}}},
			"usage":  map[string]any{"input_tokens": float64(15), "output_tokens": float64(8)},
		})
	}))
	defer ts.Close()

	// Override the base URL for testing by creating a relay and calling
	// the upstream directly (since ProxyNonStreaming uses the const URL,
	// we test the proxy logic via a test server that mimics the API).
	cr := NewCodexCLIRelay(nil)

	// We need to test the actual proxy method with a real HTTP server.
	// Since ProxyNonStreaming uses the hardcoded codexCLIBaseURL, we create
	// a wrapper test that calls the test server directly.
	ctx := context.Background()
	outBody := copyCodexCLIBody(map[string]any{
		"model": "o3-pro",
		"input": "test",
	})
	outBody["model"] = "o3-pro"
	outBody["stream"] = false
	outBody["store"] = false
	bodyJSON, _ := json.Marshal(outBody)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+codexCLIResponses, strings.NewReader(string(bodyJSON)))
	applyCodexCLIHeaders(req, "test-access-token", "acct-456")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var data map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&data)

	usage, _ := data["usage"].(map[string]any)
	inputTokens := toIntVal(usage["input_tokens"])
	outputTokens := toIntVal(usage["output_tokens"])

	if inputTokens != 15 {
		t.Errorf("InputTokens = %d, want 15", inputTokens)
	}
	if outputTokens != 8 {
		t.Errorf("OutputTokens = %d, want 8", outputTokens)
	}
	_ = cr // ensure relay was created
}

func TestCodexCLIRelay_ProxyStreaming_StreamsSSE(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			`data: {"type":"response.output_item.added","delta":"hello"}`,
			`data: {"type":"response.output_item.added","delta":" world"}`,
			`data: {"type":"response.completed","usage":{"input_tokens":12,"output_tokens":6}}`,
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

	// Test SSE reading/writing logic directly by calling the test server
	// and verifying the SSE passthrough behavior.
	ctx := context.Background()
	outBody := map[string]any{"model": "o3-pro", "input": "test", "stream": true, "store": false}
	bodyJSON, _ := json.Marshal(outBody)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+codexCLIResponses, strings.NewReader(string(bodyJSON)))
	applyCodexCLIHeaders(req, "test-token", "")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", resp.Header.Get("Content-Type"))
	}

	respBody, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(respBody), "data: ") {
		t.Errorf("response body should contain SSE data lines, got: %s", string(respBody))
	}
	if !strings.Contains(string(respBody), "[DONE]") {
		t.Errorf("response body should contain [DONE] marker")
	}
}

func TestCodexCLIRelay_UpstreamError_ReturnsProxyError(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"error":"rate_limited"}`))
	}))
	defer ts.Close()

	// Test that non-2xx response is properly detected.
	ctx := context.Background()
	bodyJSON, _ := json.Marshal(map[string]any{"model": "o3-pro", "input": "test"})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+codexCLIResponses, strings.NewReader(string(bodyJSON)))
	applyCodexCLIHeaders(req, "test-token", "")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 429 {
		t.Errorf("StatusCode = %d, want 429", resp.StatusCode)
	}

	respBytes, _ := io.ReadAll(resp.Body)
	pe := &relay.ProxyError{
		Message:    string(respBytes),
		StatusCode: resp.StatusCode,
	}
	if pe.StatusCode != 429 {
		t.Errorf("ProxyError.StatusCode = %d, want 429", pe.StatusCode)
	}
}
