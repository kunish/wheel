package executor

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	translatorbridge "github.com/kunish/wheel/apps/worker/internal/runtimeapi/translatorbridge"
	runtimeconfig "github.com/kunish/wheel/apps/worker/internal/runtimecore/config"
	cliproxyauth "github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/sdk/cliproxy/executor"
	sdktranslator "github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/sdk/translator"
	"github.com/tidwall/gjson"
)

var _ cliproxyauth.ProviderExecutor = (*OpenAICompatExecutor)(nil)

func TestOpenAICompatExecutorExecuteUsesOwnedTranslatorBridge(t *testing.T) {
	t.Parallel()

	var gotPath string
	var gotAuth string
	var gotTrace string
	var gotModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotTrace = r.Header.Get("X-Trace")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		gotModel = gjson.GetBytes(body, "model").String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl_1","choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer server.Close()

	e := NewOpenAICompatExecutor("openai-compatibility", &runtimeconfig.Config{})
	e.trans = translatorbridge.Default()
	auth := &cliproxyauth.Auth{
		Provider: "openai-compatibility",
		Attributes: map[string]string{
			"base_url":       server.URL,
			"api_key":        "test-key",
			"header:x-trace": "trace-123",
		},
	}

	resp, err := e.Execute(context.Background(), auth, cliproxyexecutor.Request{
		Model:   "gpt-4o-mini",
		Payload: []byte(`{"messages":[{"role":"user","content":"hello"}]}`),
	}, cliproxyexecutor.Options{SourceFormat: sdktranslator.FromString("openai")})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got, want := gotPath, "/chat/completions"; got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
	if got, want := gotAuth, "Bearer test-key"; got != want {
		t.Fatalf("authorization = %q, want %q", got, want)
	}
	if got, want := gotTrace, "trace-123"; got != want {
		t.Fatalf("x-trace = %q, want %q", got, want)
	}
	if got, want := gotModel, "gpt-4o-mini"; got != want {
		t.Fatalf("model = %q, want %q", got, want)
	}
	if got := gjson.GetBytes(resp.Payload, "choices.0.message.content").String(); got != "ok" {
		t.Fatalf("response content = %q, want ok", got)
	}
}

func TestOpenAICompatExecutorExecuteUsesCompactResponsesEndpoint(t *testing.T) {
	t.Parallel()

	var gotPath string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		gotBody = body
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_1","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`))
	}))
	defer server.Close()

	e := NewOpenAICompatExecutor("openai-compatibility", &runtimeconfig.Config{})
	auth := &cliproxyauth.Auth{Provider: "openai-compatibility", Attributes: map[string]string{"base_url": server.URL, "api_key": "test-key"}}
	resp, err := e.Execute(context.Background(), auth, cliproxyexecutor.Request{
		Model:   "gpt-4o-mini",
		Payload: []byte(`{"input":"hello"}`),
	}, cliproxyexecutor.Options{SourceFormat: sdktranslator.FromString("openai-response"), Alt: "responses/compact"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got, want := gotPath, "/responses/compact"; got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
	if !gjson.GetBytes(gotBody, "input").Exists() {
		t.Fatalf("expected input in body: %s", string(gotBody))
	}
	if gjson.GetBytes(gotBody, "messages").Exists() {
		t.Fatalf("unexpected messages in body: %s", string(gotBody))
	}
	if got := string(resp.Payload); !strings.Contains(got, `"id":"resp_1"`) {
		t.Fatalf("unexpected compact payload: %s", got)
	}
}

func TestOpenAICompatExecutorPrepareRequestAppliesCustomHeaders(t *testing.T) {
	t.Parallel()

	e := NewOpenAICompatExecutor("openai-compatibility", &runtimeconfig.Config{})
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	auth := &cliproxyauth.Auth{
		Provider: "openai-compatibility",
		Attributes: map[string]string{
			"api_key":         "test-key",
			"header:x-trace":  "trace-123",
			"header:x-client": "wheel",
		},
	}

	if err := e.PrepareRequest(req, auth); err != nil {
		t.Fatalf("PrepareRequest() error = %v", err)
	}
	if got, want := req.Header.Get("X-Trace"), "trace-123"; got != want {
		t.Fatalf("X-Trace = %q, want %q", got, want)
	}
	if got, want := req.Header.Get("X-Client"), "wheel"; got != want {
		t.Fatalf("X-Client = %q, want %q", got, want)
	}
}

func TestOpenAICompatExecutorExecuteStreamTranslatesSSE(t *testing.T) {
	t.Parallel()

	var gotTrace string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTrace = r.Header.Get("X-Trace")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	e := NewOpenAICompatExecutor("openai-compatibility", &runtimeconfig.Config{})
	auth := &cliproxyauth.Auth{Provider: "openai-compatibility", Attributes: map[string]string{"base_url": server.URL, "api_key": "test-key", "header:x-trace": "trace-stream"}}
	stream, err := e.ExecuteStream(context.Background(), auth, cliproxyexecutor.Request{
		Model:   "gpt-4o-mini",
		Payload: []byte(`{"messages":[{"role":"user","content":"hello"}]}`),
	}, cliproxyexecutor.Options{SourceFormat: sdktranslator.FromString("openai")})
	if err != nil {
		t.Fatalf("ExecuteStream() error = %v", err)
	}
	var chunks []string
	for chunk := range stream.Chunks {
		if chunk.Err != nil {
			t.Fatalf("chunk error = %v", chunk.Err)
		}
		if len(chunk.Payload) > 0 {
			chunks = append(chunks, string(chunk.Payload))
		}
	}
	if joined := strings.Join(chunks, ""); !strings.Contains(joined, "ok") {
		t.Fatalf("joined chunks = %q, want ok", joined)
	}
	if got, want := gotTrace, "trace-stream"; got != want {
		t.Fatalf("x-trace = %q, want %q", got, want)
	}
}

func TestOpenAICompatExecutorCountTokensReturnsUsagePayload(t *testing.T) {
	t.Parallel()

	e := NewOpenAICompatExecutor("openai-compatibility", &runtimeconfig.Config{})
	resp, err := e.CountTokens(context.Background(), nil, cliproxyexecutor.Request{
		Model:   "gpt-4o-mini",
		Payload: []byte(`{"messages":[{"role":"user","content":"hello world"}]}`),
	}, cliproxyexecutor.Options{SourceFormat: sdktranslator.FromString("openai")})
	if err != nil {
		t.Fatalf("CountTokens() error = %v", err)
	}
	if total := gjson.GetBytes(resp.Payload, "usage.total_tokens").Int(); total <= 0 {
		t.Fatalf("total_tokens = %d, want > 0", total)
	}
}
