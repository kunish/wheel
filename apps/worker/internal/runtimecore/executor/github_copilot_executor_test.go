package executor

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	runtimeconfig "github.com/kunish/wheel/apps/worker/internal/runtimecore/config"
	cliproxyauth "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy/executor"
	sdktranslator "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/translator"
	"github.com/tidwall/gjson"
)

var _ cliproxyauth.ProviderExecutor = (*GitHubCopilotExecutor)(nil)

func TestGitHubCopilotExecutorExecuteNormalizesModelAndUsesChatPath(t *testing.T) {
	t.Parallel()

	var gotPath string
	var gotAuth string
	var gotInitiator string
	var gotModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotInitiator = r.Header.Get("X-Initiator")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		gotModel = gjson.GetBytes(body, "model").String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl_1","object":"chat.completion","usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2},"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	e := NewGitHubCopilotExecutor(&runtimeconfig.Config{})
	accessToken := "ghu_test"
	e.cache[accessToken] = &cachedAPIToken{
		token:       "copilot_api_token",
		apiEndpoint: server.URL,
		expiresAt:   time.Now().Add(time.Hour),
	}

	auth := &cliproxyauth.Auth{
		ID:       "copilot-auth",
		Provider: "github-copilot",
		Metadata: map[string]any{"access_token": accessToken},
	}
	resp, err := e.Execute(context.Background(), auth, cliproxyexecutor.Request{
		Model:   "claude-opus-4-6",
		Payload: []byte(`{"model":"claude-opus-4-6","messages":[{"role":"user","content":"hello"}]}`),
	}, cliproxyexecutor.Options{SourceFormat: sdktranslator.FromString("openai")})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if gotPath != "/chat/completions" {
		t.Fatalf("path = %q, want /chat/completions", gotPath)
	}
	if gotAuth != "Bearer copilot_api_token" {
		t.Fatalf("authorization = %q, want bearer token", gotAuth)
	}
	if gotInitiator != "user" {
		t.Fatalf("X-Initiator = %q, want user", gotInitiator)
	}
	if gotModel != "claude-opus-4.6" {
		t.Fatalf("model = %q, want claude-opus-4.6", gotModel)
	}
	if got := gjson.GetBytes(resp.Payload, "choices.0.message.content").String(); got != "ok" {
		t.Fatalf("response content = %q, want ok", got)
	}
}

func TestGitHubCopilotExecutorCountTokensReturnsNotImplemented(t *testing.T) {
	t.Parallel()

	e := NewGitHubCopilotExecutor(&runtimeconfig.Config{})
	_, err := e.CountTokens(context.Background(), nil, cliproxyexecutor.Request{}, cliproxyexecutor.Options{})
	if err == nil {
		t.Fatal("expected error")
	}
	status, ok := err.(interface{ StatusCode() int })
	if !ok {
		t.Fatalf("expected status error, got %T", err)
	}
	if got, want := status.StatusCode(), http.StatusNotImplemented; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

func TestGitHubCopilotExecutorHttpRequestAddsAuthorization(t *testing.T) {
	t.Parallel()

	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	e := NewGitHubCopilotExecutor(&runtimeconfig.Config{})
	accessToken := "ghu_test"
	e.cache[accessToken] = &cachedAPIToken{
		token:       "copilot_api_token",
		apiEndpoint: server.URL,
		expiresAt:   time.Now().Add(time.Hour),
	}
	auth := &cliproxyauth.Auth{Provider: "github-copilot", Metadata: map[string]any{"access_token": accessToken}}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}

	resp, err := e.HttpRequest(context.Background(), auth, req)
	if err != nil {
		t.Fatalf("HttpRequest() error = %v", err)
	}
	defer resp.Body.Close()
	if got, want := gotAuth, "Bearer copilot_api_token"; got != want {
		t.Fatalf("authorization = %q, want %q", got, want)
	}
}

func TestGitHubCopilotExecutorExecuteStreamUsesChatPath(t *testing.T) {
	t.Parallel()

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	e := NewGitHubCopilotExecutor(&runtimeconfig.Config{})
	accessToken := "ghu_test"
	e.cache[accessToken] = &cachedAPIToken{
		token:       "copilot_api_token",
		apiEndpoint: server.URL,
		expiresAt:   time.Now().Add(time.Hour),
	}
	auth := &cliproxyauth.Auth{Provider: "github-copilot", Metadata: map[string]any{"access_token": accessToken}}

	stream, err := e.ExecuteStream(context.Background(), auth, cliproxyexecutor.Request{
		Model:   "claude-opus-4-6",
		Payload: []byte(`{"model":"claude-opus-4-6","messages":[{"role":"user","content":"hello"}]}`),
	}, cliproxyexecutor.Options{SourceFormat: sdktranslator.FromString("openai")})
	if err != nil {
		t.Fatalf("ExecuteStream() error = %v", err)
	}
	if stream == nil {
		t.Fatal("expected stream result")
	}
	if got, want := gotPath, "/chat/completions"; got != want {
		t.Fatalf("path = %q, want %q", got, want)
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
	joined := strings.Join(chunks, "")
	if !strings.Contains(joined, "ok") {
		t.Fatalf("joined chunks = %q, want translated content", joined)
	}
	if ct := stream.Headers.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content-type = %q, want text/event-stream", ct)
	}
	_ = bufio.ErrAdvanceTooFar
}
