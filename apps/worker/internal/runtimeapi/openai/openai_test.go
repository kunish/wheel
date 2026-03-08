package openai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/sdk/api/handlers"
	sdkopenai "github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/sdk/api/handlers/openai"
	sdkconfig "github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/sdk/config"
	"github.com/tidwall/gjson"
)

func TestOwnedOpenAIHandlersRejectMissingModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	base := handlers.NewBaseAPIHandlers(&sdkconfig.SDKConfig{}, nil)
	chatHandler := NewAPIHandler(base)
	responsesHandler := NewResponsesHandler(base)

	router := gin.New()
	router.POST("/v1/chat/completions", chatHandler.ChatCompletions)
	router.POST("/v1/completions", chatHandler.Completions)
	router.POST("/v1/responses", responsesHandler.Responses)

	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "chat completions", path: "/v1/chat/completions", body: `{"messages":[{"role":"user","content":"hello"}]}`},
		{name: "completions", path: "/v1/completions", body: `{"prompt":"hello"}`},
		{name: "responses", path: "/v1/responses", body: `{"input":"hello"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d body=%s", resp.Code, http.StatusBadRequest, resp.Body.String())
			}
			if !strings.Contains(resp.Body.String(), `"message":"Model is required"`) {
				t.Fatalf("expected missing model message, got: %s", resp.Body.String())
			}
			if !strings.Contains(resp.Body.String(), `"type":"invalid_request_error"`) {
				t.Fatalf("expected invalid_request_error, got: %s", resp.Body.String())
			}
		})
	}
}

func TestNewAPIHandlerReturnsOwnedType(t *testing.T) {
	base := handlers.NewBaseAPIHandlers(&sdkconfig.SDKConfig{}, nil)
	got := reflect.TypeOf(NewAPIHandler(base)).String()
	if got != "*openai.APIHandler" {
		t.Fatalf("handler type = %q, want %q", got, "*openai.APIHandler")
	}
}

func TestFilterOpenAIModels(t *testing.T) {
	filtered := FilterOpenAIModels([]map[string]any{{
		"id":          "gpt-4o",
		"object":      "model",
		"created":     123,
		"owned_by":    "openai",
		"extra_field": "drop-me",
	}})
	if len(filtered) != 1 {
		t.Fatalf("filtered len = %d, want 1", len(filtered))
	}
	if filtered[0]["id"] != "gpt-4o" || filtered[0]["object"] != "model" {
		t.Fatalf("unexpected filtered model: %#v", filtered[0])
	}
	if filtered[0]["created"] != 123 || filtered[0]["owned_by"] != "openai" {
		t.Fatalf("missing required passthrough fields: %#v", filtered[0])
	}
	if _, exists := filtered[0]["extra_field"]; exists {
		t.Fatalf("extra field must be removed: %#v", filtered[0])
	}
}

func TestOpenAIModelsUsesOwnedFiltering(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &APIHandler{models: func() []map[string]any {
		return []map[string]any{{
			"id":          "gpt-4o",
			"object":      "model",
			"created":     123,
			"owned_by":    "openai",
			"extra_field": "drop-me",
		}}
	}}

	router := gin.New()
	router.GET("/v1/models", handler.OpenAIModels)
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if gjson.Get(resp.Body.String(), "data.0.id").String() != "gpt-4o" {
		t.Fatalf("unexpected id: %s", resp.Body.String())
	}
	if gjson.Get(resp.Body.String(), "data.0.extra_field").Exists() {
		t.Fatalf("extra field must be removed: %s", resp.Body.String())
	}
}

func TestCompletionsUsesOwnedNonStreamingExecution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotModel string
	var gotRaw string
	handler := &APIHandler{
		handlerType: func() string { return "openai" },
		getContextWithCancel: func(*gin.Context, context.Context) (context.Context, func(error)) {
			return context.Background(), func(error) {}
		},
		startNonStreamingKeepAlive: func(*gin.Context, context.Context) func() { return func() {} },
		execute: func(_ context.Context, model string, rawJSON []byte, alt string) ([]byte, http.Header, *runtimeError) {
			gotModel = model
			gotRaw = string(rawJSON)
			if alt != "" {
				t.Fatalf("alt = %q, want empty", alt)
			}
			return []byte(`{"id":"chatcmpl-1","object":"chat.completion","created":1,"model":"gpt-4o","choices":[{"index":0,"message":{"content":"done"},"finish_reason":"stop"}]}`), http.Header{"X-Upstream": {"1"}}, nil
		},
		streamExecute: func(context.Context, string, []byte, string) (<-chan []byte, http.Header, <-chan *runtimeError) {
			t.Fatal("unexpected stream execution")
			return nil, nil, nil
		},
	}

	router := gin.New()
	router.POST("/v1/completions", handler.Completions)
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", strings.NewReader(`{"model":"gpt-4o","prompt":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if gotModel != "gpt-4o" {
		t.Fatalf("model = %q, want %q", gotModel, "gpt-4o")
	}
	if gjson.Get(gotRaw, "messages.0.content").String() != "hello" {
		t.Fatalf("converted request = %s", gotRaw)
	}
	if gjson.Get(resp.Body.String(), "object").String() != "text_completion" {
		t.Fatalf("response = %s", resp.Body.String())
	}
	if gjson.Get(resp.Body.String(), "choices.0.text").String() != "done" {
		t.Fatalf("response = %s", resp.Body.String())
	}
	if resp.Header().Get("X-Upstream") != "1" {
		t.Fatalf("missing passthrough header: %#v", resp.Header())
	}
}

func TestCompletionsUsesOwnedStreamingExecution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &APIHandler{
		handlerType: func() string { return "openai" },
		getContextWithCancel: func(*gin.Context, context.Context) (context.Context, func(error)) {
			return context.Background(), func(error) {}
		},
		streamExecute: func(_ context.Context, model string, rawJSON []byte, alt string) (<-chan []byte, http.Header, <-chan *runtimeError) {
			if model != "gpt-4o" {
				t.Fatalf("model = %q, want %q", model, "gpt-4o")
			}
			if alt != "" {
				t.Fatalf("alt = %q, want empty", alt)
			}
			data := make(chan []byte, 1)
			data <- []byte(`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"OK"}}]}`)
			close(data)
			errs := make(chan *runtimeError)
			close(errs)
			return data, http.Header{"X-Upstream": {"1"}}, errs
		},
	}

	router := gin.New()
	router.POST("/v1/completions", handler.Completions)
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", strings.NewReader(`{"model":"gpt-4o","prompt":"hello","stream":true}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"object":"text_completion"`) {
		t.Fatalf("response = %s", resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"text":"OK"`) {
		t.Fatalf("response = %s", resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `data: [DONE]`) {
		t.Fatalf("response = %s", resp.Body.String())
	}
	if resp.Header().Get("X-Upstream") != "1" {
		t.Fatalf("missing passthrough header: %#v", resp.Header())
	}
}

func TestChatCompletionsUsesOwnedNonStreamingExecution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotAlt string
	var gotRaw string
	handler := &APIHandler{
		getContextWithCancel: func(*gin.Context, context.Context) (context.Context, func(error)) {
			return context.Background(), func(error) {}
		},
		execute: func(_ context.Context, model string, rawJSON []byte, alt string) ([]byte, http.Header, *runtimeError) {
			gotAlt = alt
			gotRaw = string(rawJSON)
			return []byte(`{"id":"chatcmpl-1","object":"chat.completion","choices":[{"index":0,"message":{"content":"done"}}]}`), http.Header{"X-Upstream": {"1"}}, nil
		},
		streamExecute: func(context.Context, string, []byte, string) (<-chan []byte, http.Header, <-chan *runtimeError) {
			t.Fatal("unexpected stream execution")
			return nil, nil, nil
		},
		resolveEndpointOverride: func(string, string) (string, bool) { return "", false },
	}

	router := gin.New()
	router.POST("/v1/chat/completions", handler.ChatCompletions)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions?alt=demo", strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if gotAlt != "demo" {
		t.Fatalf("alt = %q, want %q", gotAlt, "demo")
	}
	if gjson.Get(gotRaw, "messages.0.content").String() != "hello" {
		t.Fatalf("request = %s", gotRaw)
	}
	if gjson.Get(resp.Body.String(), "choices.0.message.content").String() != "done" {
		t.Fatalf("response = %s", resp.Body.String())
	}
	if resp.Header().Get("X-Upstream") != "1" {
		t.Fatalf("missing passthrough header: %#v", resp.Header())
	}
}

func TestChatCompletionsUsesOwnedStreamingExecution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &APIHandler{
		getContextWithCancel: func(*gin.Context, context.Context) (context.Context, func(error)) {
			return context.Background(), func(error) {}
		},
		streamExecute: func(_ context.Context, model string, rawJSON []byte, alt string) (<-chan []byte, http.Header, <-chan *runtimeError) {
			data := make(chan []byte, 1)
			data <- []byte(`{"id":"chatcmpl-1","choices":[{"index":0,"delta":{"content":"OK"}}]}`)
			close(data)
			errs := make(chan *runtimeError)
			close(errs)
			return data, http.Header{"X-Upstream": {"1"}}, errs
		},
		resolveEndpointOverride: func(string, string) (string, bool) { return "", false },
	}

	router := gin.New()
	router.POST("/v1/chat/completions", handler.ChatCompletions)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"stream":true}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"content":"OK"`) {
		t.Fatalf("response = %s", resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `data: [DONE]`) {
		t.Fatalf("response = %s", resp.Body.String())
	}
	if resp.Header().Get("X-Upstream") != "1" {
		t.Fatalf("missing passthrough header: %#v", resp.Header())
	}
}

func TestChatCompletionsUsesOwnedExecutionForResponsesPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotRaw string
	handler := &APIHandler{
		getContextWithCancel: func(*gin.Context, context.Context) (context.Context, func(error)) {
			return context.Background(), func(error) {}
		},
		execute: func(_ context.Context, model string, rawJSON []byte, alt string) ([]byte, http.Header, *runtimeError) {
			gotRaw = string(rawJSON)
			if model != "gpt-4o" {
				t.Fatalf("model = %q, want %q", model, "gpt-4o")
			}
			if alt != "" {
				t.Fatalf("alt = %q, want empty", alt)
			}
			return []byte(`{"id":"chatcmpl-1","object":"chat.completion","choices":[{"index":0,"message":{"content":"done"}}]}`), http.Header{"X-Upstream": {"1"}}, nil
		},
		resolveEndpointOverride: func(string, string) (string, bool) { return "", false },
	}

	router := gin.New()
	router.POST("/v1/chat/completions", handler.ChatCompletions)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","input":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if gjson.Get(gotRaw, "messages.0.content").String() != "hello" {
		t.Fatalf("request = %s", gotRaw)
	}
	if gjson.Get(gotRaw, "input").Exists() {
		t.Fatalf("responses-only field leaked into chat request: %s", gotRaw)
	}
	if gjson.Get(resp.Body.String(), "choices.0.message.content").String() != "done" {
		t.Fatalf("response = %s", resp.Body.String())
	}
	if resp.Header().Get("X-Upstream") != "1" {
		t.Fatalf("missing passthrough header: %#v", resp.Header())
	}
}

func TestChatCompletionsUsesOwnedResponsesOverrideNonStreamingExecution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotModel string
	var gotRaw string
	handler := &APIHandler{
		getContextWithCancel: func(*gin.Context, context.Context) (context.Context, func(error)) {
			return context.Background(), func(error) {}
		},
		responsesExecute: func(_ context.Context, model string, rawJSON []byte, alt string) ([]byte, http.Header, *runtimeError) {
			gotModel = model
			gotRaw = string(rawJSON)
			if alt != "" {
				t.Fatalf("alt = %q, want empty", alt)
			}
			return []byte(`{"id":"resp_1","object":"response","output":[{"type":"message","status":"completed","content":[{"type":"output_text","text":"done"}],"role":"assistant"}]}`), http.Header{"X-Upstream": {"1"}}, nil
		},
		resolveEndpointOverride: func(string, string) (string, bool) { return sdkopenai.OpenAIResponsesEndpoint, true },
	}

	router := gin.New()
	router.POST("/v1/chat/completions", handler.ChatCompletions)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if gotModel != "gpt-4o" {
		t.Fatalf("model = %q, want %q", gotModel, "gpt-4o")
	}
	if gjson.Get(gotRaw, "input.0.role").String() != "user" {
		t.Fatalf("request = %s", gotRaw)
	}
	if gjson.Get(resp.Body.String(), "choices.0.message.content").String() != "done" {
		t.Fatalf("response = %s", resp.Body.String())
	}
	if resp.Header().Get("X-Upstream") != "1" {
		t.Fatalf("missing passthrough header: %#v", resp.Header())
	}
}

func TestChatCompletionsUsesOwnedResponsesOverrideStreamingExecution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &APIHandler{
		getContextWithCancel: func(*gin.Context, context.Context) (context.Context, func(error)) {
			return context.Background(), func(error) {}
		},
		responsesStreamExecute: func(_ context.Context, model string, rawJSON []byte, alt string) (<-chan []byte, http.Header, <-chan *runtimeError) {
			if model != "gpt-4o" {
				t.Fatalf("model = %q, want %q", model, "gpt-4o")
			}
			if alt != "" {
				t.Fatalf("alt = %q, want empty", alt)
			}
			if gjson.GetBytes(rawJSON, "input.0.role").String() != "user" {
				t.Fatalf("request = %s", string(rawJSON))
			}
			data := make(chan []byte, 1)
			data <- []byte(`data: {"type":"response.output_text.delta","delta":"OK"}`)
			close(data)
			errs := make(chan *runtimeError)
			close(errs)
			return data, http.Header{"X-Upstream": {"1"}}, errs
		},
		resolveEndpointOverride: func(string, string) (string, bool) { return sdkopenai.OpenAIResponsesEndpoint, true },
	}

	router := gin.New()
	router.POST("/v1/chat/completions", handler.ChatCompletions)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"stream":true}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"content":"OK"`) {
		t.Fatalf("response = %s", resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `data: [DONE]`) {
		t.Fatalf("response = %s", resp.Body.String())
	}
	if resp.Header().Get("X-Upstream") != "1" {
		t.Fatalf("missing passthrough header: %#v", resp.Header())
	}
}

func TestNewResponsesHandlerReturnsOwnedType(t *testing.T) {
	base := handlers.NewBaseAPIHandlers(&sdkconfig.SDKConfig{}, nil)
	got := reflect.TypeOf(NewResponsesHandler(base)).String()
	if got != "*openai.ResponsesHandler" {
		t.Fatalf("handler type = %q, want %q", got, "*openai.ResponsesHandler")
	}
}

func TestResponsesUsesOwnedNonStreamingExecution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotModel string
	var gotRaw string
	handler := &ResponsesHandler{
		getContextWithCancel: func(*gin.Context, context.Context) (context.Context, func(error)) {
			return context.Background(), func(error) {}
		},
		startNonStreamingKeepAlive: func(*gin.Context, context.Context) func() { return func() {} },
		execute: func(_ context.Context, model string, rawJSON []byte, alt string) ([]byte, http.Header, *runtimeError) {
			gotModel = model
			gotRaw = string(rawJSON)
			if alt != "" {
				t.Fatalf("alt = %q, want empty", alt)
			}
			return []byte(`{"id":"resp_1","object":"response","output":[{"type":"output_text","text":"done"}]}`), http.Header{"X-Upstream": {"1"}}, nil
		},
		streamExecute: func(context.Context, string, []byte, string) (<-chan []byte, http.Header, <-chan *runtimeError) {
			t.Fatal("unexpected stream execution")
			return nil, nil, nil
		},
		resolveEndpointOverride: func(string, string) (string, bool) { return "", false },
	}

	router := gin.New()
	router.POST("/v1/responses", handler.Responses)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-4o","input":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if gotModel != "gpt-4o" {
		t.Fatalf("model = %q, want %q", gotModel, "gpt-4o")
	}
	if gjson.Get(gotRaw, "input").String() != "hello" {
		t.Fatalf("request = %s", gotRaw)
	}
	if gjson.Get(resp.Body.String(), "output.0.text").String() != "done" {
		t.Fatalf("response = %s", resp.Body.String())
	}
	if resp.Header().Get("X-Upstream") != "1" {
		t.Fatalf("missing passthrough header: %#v", resp.Header())
	}
}

func TestResponsesUsesOwnedStreamingExecution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &ResponsesHandler{
		getContextWithCancel: func(*gin.Context, context.Context) (context.Context, func(error)) {
			return context.Background(), func(error) {}
		},
		streamExecute: func(_ context.Context, model string, rawJSON []byte, alt string) (<-chan []byte, http.Header, <-chan *runtimeError) {
			if model != "gpt-4o" {
				t.Fatalf("model = %q, want %q", model, "gpt-4o")
			}
			if alt != "" {
				t.Fatalf("alt = %q, want empty", alt)
			}
			if gjson.GetBytes(rawJSON, "input").String() != "hello" {
				t.Fatalf("request = %s", string(rawJSON))
			}
			data := make(chan []byte, 1)
			data <- []byte("event: response.output_text.delta\ndata: {\"delta\":\"OK\"}\n\n")
			close(data)
			errs := make(chan *runtimeError)
			close(errs)
			return data, http.Header{"X-Upstream": {"1"}}, errs
		},
		resolveEndpointOverride: func(string, string) (string, bool) { return "", false },
	}

	router := gin.New()
	router.POST("/v1/responses", handler.Responses)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-4o","input":"hello","stream":true}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `event: response.output_text.delta`) {
		t.Fatalf("response = %s", resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"delta":"OK"`) {
		t.Fatalf("response = %s", resp.Body.String())
	}
	if resp.Header().Get("X-Upstream") != "1" {
		t.Fatalf("missing passthrough header: %#v", resp.Header())
	}
}

func TestResponsesUsesOwnedChatOverrideNonStreamingExecution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotModel string
	var gotRaw string
	handler := &ResponsesHandler{
		getContextWithCancel: func(*gin.Context, context.Context) (context.Context, func(error)) {
			return context.Background(), func(error) {}
		},
		chatExecute: func(_ context.Context, model string, rawJSON []byte, alt string) ([]byte, http.Header, *runtimeError) {
			gotModel = model
			gotRaw = string(rawJSON)
			if alt != "" {
				t.Fatalf("alt = %q, want empty", alt)
			}
			return []byte(`{"id":"chatcmpl-1","object":"chat.completion","created":1,"model":"gpt-4o","choices":[{"index":0,"message":{"content":"done"},"finish_reason":"stop"}]}`), http.Header{"X-Upstream": {"1"}}, nil
		},
		resolveEndpointOverride: func(string, string) (string, bool) { return sdkopenai.OpenAIChatEndpoint, true },
	}

	router := gin.New()
	router.POST("/v1/responses", handler.Responses)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-4o","input":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if gotModel != "gpt-4o" {
		t.Fatalf("model = %q, want %q", gotModel, "gpt-4o")
	}
	if gjson.Get(gotRaw, "messages.0.content").String() != "hello" {
		t.Fatalf("request = %s", gotRaw)
	}
	if gjson.Get(resp.Body.String(), "object").String() != "response" {
		t.Fatalf("response = %s", resp.Body.String())
	}
	if gjson.Get(resp.Body.String(), "output.0.content.0.text").String() != "done" {
		t.Fatalf("response = %s", resp.Body.String())
	}
	if resp.Header().Get("X-Upstream") != "1" {
		t.Fatalf("missing passthrough header: %#v", resp.Header())
	}
}

func TestResponsesUsesOwnedChatOverrideStreamingExecution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &ResponsesHandler{
		getContextWithCancel: func(*gin.Context, context.Context) (context.Context, func(error)) {
			return context.Background(), func(error) {}
		},
		chatStreamExecute: func(_ context.Context, model string, rawJSON []byte, alt string) (<-chan []byte, http.Header, <-chan *runtimeError) {
			if model != "gpt-4o" {
				t.Fatalf("model = %q, want %q", model, "gpt-4o")
			}
			if alt != "" {
				t.Fatalf("alt = %q, want empty", alt)
			}
			if gjson.GetBytes(rawJSON, "messages.0.content").String() != "hello" {
				t.Fatalf("request = %s", string(rawJSON))
			}
			data := make(chan []byte, 1)
			data <- []byte(`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"OK"}}]}`)
			close(data)
			errs := make(chan *runtimeError)
			close(errs)
			return data, http.Header{"X-Upstream": {"1"}}, errs
		},
		resolveEndpointOverride: func(string, string) (string, bool) { return sdkopenai.OpenAIChatEndpoint, true },
	}

	router := gin.New()
	router.POST("/v1/responses", handler.Responses)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-4o","input":"hello","stream":true}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `event: response.output_text.delta`) {
		t.Fatalf("response = %s", resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"delta":"OK"`) {
		t.Fatalf("response = %s", resp.Body.String())
	}
	if resp.Header().Get("X-Upstream") != "1" {
		t.Fatalf("missing passthrough header: %#v", resp.Header())
	}
}

func TestCompactUsesOwnedExecution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotModel string
	var gotRaw string
	handler := &ResponsesHandler{
		getContextWithCancel: func(*gin.Context, context.Context) (context.Context, func(error)) {
			return context.Background(), func(error) {}
		},
		startNonStreamingKeepAlive: func(*gin.Context, context.Context) func() { return func() {} },
		execute: func(_ context.Context, model string, rawJSON []byte, alt string) ([]byte, http.Header, *runtimeError) {
			gotModel = model
			gotRaw = string(rawJSON)
			if alt != "responses/compact" {
				t.Fatalf("alt = %q, want %q", alt, "responses/compact")
			}
			return []byte(`{"id":"resp_1","object":"response","output":[{"type":"output_text","text":"compact"}]}`), http.Header{"X-Upstream": {"1"}}, nil
		},
	}

	router := gin.New()
	router.POST("/v1/responses/compact", handler.Compact)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses/compact", strings.NewReader(`{"model":"gpt-4o","stream":false,"input":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if gotModel != "gpt-4o" {
		t.Fatalf("model = %q, want %q", gotModel, "gpt-4o")
	}
	if gjson.Get(gotRaw, "stream").Exists() {
		t.Fatalf("stream flag must be removed: %s", gotRaw)
	}
	if gjson.Get(resp.Body.String(), "output.0.text").String() != "compact" {
		t.Fatalf("response = %s", resp.Body.String())
	}
	if resp.Header().Get("X-Upstream") != "1" {
		t.Fatalf("missing passthrough header: %#v", resp.Header())
	}
}

func TestResponsesWebsocketUsesOwnedStreamingExecution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotModel string
	var gotRaw string
	handler := &ResponsesHandler{
		getContextWithCancel: func(*gin.Context, context.Context) (context.Context, func(error)) {
			return context.Background(), func(error) {}
		},
		streamExecute: func(_ context.Context, model string, rawJSON []byte, alt string) (<-chan []byte, http.Header, <-chan *runtimeError) {
			gotModel = model
			gotRaw = string(rawJSON)
			if alt != "" {
				t.Fatalf("alt = %q, want empty", alt)
			}
			data := make(chan []byte, 1)
			data <- []byte("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp-1\",\"output\":[{\"type\":\"message\",\"id\":\"out-1\"}]}}\n\n")
			close(data)
			errs := make(chan *runtimeError)
			close(errs)
			return data, nil, errs
		},
		websocketSupportsIncrementalInputForModel: func(string) bool { return true },
		newWebsocketSessionID:                     func() string { return "session-1" },
		withDownstreamWebsocketContext:            func(ctx context.Context) context.Context { return ctx },
		withExecutionSessionID:                    func(ctx context.Context, sessionID string) context.Context { return ctx },
		withPinnedAuthID:                          func(ctx context.Context, authID string) context.Context { return ctx },
		withSelectedAuthIDCallback: func(ctx context.Context, callback func(string)) context.Context {
			return ctx
		},
		closeExecutionSession: func(string) {},
	}

	router := gin.New()
	router.GET("/v1/responses/ws", handler.ResponsesWebsocket)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/responses/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer func() {
		if errClose := conn.Close(); errClose != nil {
			t.Fatalf("close websocket: %v", errClose)
		}
	}()

	if errWrite := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","model":"gpt-4o"}`)); errWrite != nil {
		t.Fatalf("write websocket message: %v", errWrite)
	}

	_, payload, errReadMessage := conn.ReadMessage()
	if errReadMessage != nil {
		t.Fatalf("read websocket message: %v", errReadMessage)
	}
	if gjson.GetBytes(payload, "type").String() != "response.completed" {
		t.Fatalf("payload type = %s, want response.completed", gjson.GetBytes(payload, "type").String())
	}
	if gjson.GetBytes(payload, "response.output.0.id").String() != "out-1" {
		t.Fatalf("payload = %s", payload)
	}
	if gotModel != "gpt-4o" {
		t.Fatalf("model = %q, want %q", gotModel, "gpt-4o")
	}
	if gjson.Get(gotRaw, "type").Exists() {
		t.Fatalf("type leaked upstream: %s", gotRaw)
	}
	if !gjson.Get(gotRaw, "stream").Bool() {
		t.Fatalf("stream must be forced true: %s", gotRaw)
	}
}

func TestResponsesWebsocketPrewarmHandledLocally(t *testing.T) {
	gin.SetMode(gin.TestMode)
	streamCalls := 0
	var gotRaw string
	handler := &ResponsesHandler{
		getContextWithCancel: func(*gin.Context, context.Context) (context.Context, func(error)) {
			return context.Background(), func(error) {}
		},
		streamExecute: func(_ context.Context, model string, rawJSON []byte, alt string) (<-chan []byte, http.Header, <-chan *runtimeError) {
			streamCalls++
			gotRaw = string(rawJSON)
			data := make(chan []byte, 1)
			data <- []byte(`{"type":"response.completed","response":{"id":"resp-upstream","output":[{"type":"message","id":"out-1"}]}}`)
			close(data)
			errs := make(chan *runtimeError)
			close(errs)
			return data, nil, errs
		},
		websocketSupportsIncrementalInputForModel: func(string) bool { return false },
		newWebsocketSessionID:                     func() string { return "session-1" },
		withDownstreamWebsocketContext:            func(ctx context.Context) context.Context { return ctx },
		withExecutionSessionID:                    func(ctx context.Context, sessionID string) context.Context { return ctx },
		withPinnedAuthID:                          func(ctx context.Context, authID string) context.Context { return ctx },
		withSelectedAuthIDCallback: func(ctx context.Context, callback func(string)) context.Context {
			return ctx
		},
		closeExecutionSession: func(string) {},
	}

	router := gin.New()
	router.GET("/v1/responses/ws", handler.ResponsesWebsocket)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/responses/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer func() {
		if errClose := conn.Close(); errClose != nil {
			t.Fatalf("close websocket: %v", errClose)
		}
	}()

	if errWrite := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","model":"gpt-4o","generate":false}`)); errWrite != nil {
		t.Fatalf("write prewarm websocket message: %v", errWrite)
	}

	_, createdPayload, errReadMessage := conn.ReadMessage()
	if errReadMessage != nil {
		t.Fatalf("read prewarm created message: %v", errReadMessage)
	}
	if gjson.GetBytes(createdPayload, "type").String() != "response.created" {
		t.Fatalf("created payload type = %s, want response.created", gjson.GetBytes(createdPayload, "type").String())
	}
	prewarmResponseID := gjson.GetBytes(createdPayload, "response.id").String()
	if prewarmResponseID == "" {
		t.Fatal("prewarm response id is empty")
	}
	if streamCalls != 0 {
		t.Fatalf("stream calls after prewarm = %d, want 0", streamCalls)
	}

	_, completedPayload, errReadMessage := conn.ReadMessage()
	if errReadMessage != nil {
		t.Fatalf("read prewarm completed message: %v", errReadMessage)
	}
	if gjson.GetBytes(completedPayload, "type").String() != "response.completed" {
		t.Fatalf("completed payload type = %s, want response.completed", gjson.GetBytes(completedPayload, "type").String())
	}
	if gjson.GetBytes(completedPayload, "response.id").String() != prewarmResponseID {
		t.Fatalf("completed response id = %s, want %s", gjson.GetBytes(completedPayload, "response.id").String(), prewarmResponseID)
	}

	secondRequest := fmt.Sprintf(`{"type":"response.create","previous_response_id":%q,"input":[{"type":"message","id":"msg-1"}]}`, prewarmResponseID)
	if errWrite := conn.WriteMessage(websocket.TextMessage, []byte(secondRequest)); errWrite != nil {
		t.Fatalf("write follow-up websocket message: %v", errWrite)
	}

	_, upstreamPayload, errReadMessage := conn.ReadMessage()
	if errReadMessage != nil {
		t.Fatalf("read upstream completed message: %v", errReadMessage)
	}
	if gjson.GetBytes(upstreamPayload, "type").String() != "response.completed" {
		t.Fatalf("upstream payload type = %s, want response.completed", gjson.GetBytes(upstreamPayload, "type").String())
	}
	if streamCalls != 1 {
		t.Fatalf("stream calls after follow-up = %d, want 1", streamCalls)
	}
	if gjson.Get(gotRaw, "previous_response_id").Exists() {
		t.Fatalf("previous_response_id leaked upstream: %s", gotRaw)
	}
	if gjson.Get(gotRaw, "generate").Exists() {
		t.Fatalf("generate leaked upstream: %s", gotRaw)
	}
	if gjson.Get(gotRaw, "model").String() != "gpt-4o" {
		t.Fatalf("forwarded model = %s, want gpt-4o", gjson.Get(gotRaw, "model").String())
	}
	input := gjson.Get(gotRaw, "input").Array()
	if len(input) != 1 || input[0].Get("id").String() != "msg-1" {
		t.Fatalf("unexpected forwarded input: %s", gotRaw)
	}
}

func TestFormatStreamChunkWrapsRawJSONOnce(t *testing.T) {
	got := FormatStreamChunk([]byte(`{"id":"chatcmpl-1"}`))
	want := "data: {\"id\":\"chatcmpl-1\"}\n\n"
	if got != want {
		t.Fatalf("FormatStreamChunk() = %q, want %q", got, want)
	}
}

func TestFormatStreamChunkPreservesExistingSSEPayload(t *testing.T) {
	got := FormatStreamChunk([]byte("data: {\"id\":\"chatcmpl-1\"}"))
	want := "data: {\"id\":\"chatcmpl-1\"}\n\n"
	if got != want {
		t.Fatalf("FormatStreamChunk() = %q, want %q", got, want)
	}
}

func TestConvertChatCompletionsStreamChunkToCompletionsAcceptsSSEPayload(t *testing.T) {
	chunk := []byte("data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"gpt-4o\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"OK\"}}]}")
	converted := ConvertChatCompletionsStreamChunkToCompletions(chunk)
	if converted == nil {
		t.Fatal("expected converted chunk, got nil")
	}
	if !strings.Contains(string(converted), "\"object\":\"text_completion\"") {
		t.Fatalf("expected text completion object, got: %s", string(converted))
	}
	if !strings.Contains(string(converted), "\"text\":\"OK\"") {
		t.Fatalf("expected converted text, got: %s", string(converted))
	}
}

func TestFormatCompletionsStreamChunk(t *testing.T) {
	chunk := []byte(`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"OK"}}]}`)
	formatted := FormatCompletionsStreamChunk(chunk)
	if formatted == nil {
		t.Fatal("expected formatted chunk, got nil")
	}
	if !strings.Contains(string(formatted), "data: ") {
		t.Fatalf("expected SSE prefix, got: %s", string(formatted))
	}
	if !strings.Contains(string(formatted), `"object":"text_completion"`) {
		t.Fatalf("expected text completion object, got: %s", string(formatted))
	}
	if !strings.Contains(string(formatted), `"text":"OK"`) {
		t.Fatalf("expected converted text, got: %s", string(formatted))
	}
}

func TestFormatCompletionsStreamChunkSkipsDone(t *testing.T) {
	if formatted := FormatCompletionsStreamChunk([]byte("data: [DONE]")); formatted != nil {
		t.Fatalf("expected nil for DONE, got: %s", string(formatted))
	}
}

func TestNormalizeResponsesWebsocketRequestCreate(t *testing.T) {
	raw := []byte(`{"type":"response.create","model":"test-model","stream":false,"input":[{"type":"message","id":"msg-1"}]}`)

	normalized, last, status, err := NormalizeResponsesWebsocketRequest(raw, nil, nil, true)
	if err != nil {
		t.Fatalf("unexpected error: status=%d err=%v", status, err)
	}
	if gjson.GetBytes(normalized, "type").Exists() {
		t.Fatalf("normalized create request must not include type field")
	}
	if !gjson.GetBytes(normalized, "stream").Bool() {
		t.Fatalf("normalized create request must force stream=true")
	}
	if gjson.GetBytes(normalized, "model").String() != "test-model" {
		t.Fatalf("unexpected model: %s", gjson.GetBytes(normalized, "model").String())
	}
	if !reflect.DeepEqual(last, normalized) {
		t.Fatalf("last request snapshot should match normalized request")
	}
}

func TestNormalizeResponsesWebsocketRequestWithPreviousResponseIDIncremental(t *testing.T) {
	lastRequest := []byte(`{"model":"test-model","stream":true,"instructions":"be helpful","input":[{"type":"message","id":"msg-1"}]}`)
	lastResponseOutput := []byte(`[{"type":"function_call","id":"fc-1","call_id":"call-1"},{"type":"message","id":"assistant-1"}]`)
	raw := []byte(`{"type":"response.create","previous_response_id":"resp-1","input":[{"type":"function_call_output","call_id":"call-1","id":"tool-out-1"}]}`)

	normalized, next, status, err := NormalizeResponsesWebsocketRequest(raw, lastRequest, lastResponseOutput, true)
	if err != nil {
		t.Fatalf("unexpected error: status=%d err=%v", status, err)
	}
	if gjson.GetBytes(normalized, "type").Exists() {
		t.Fatalf("normalized request must not include type field")
	}
	if gjson.GetBytes(normalized, "previous_response_id").String() != "resp-1" {
		t.Fatalf("previous_response_id must be preserved in incremental mode")
	}
	input := gjson.GetBytes(normalized, "input").Array()
	if len(input) != 1 {
		t.Fatalf("incremental input len = %d, want 1", len(input))
	}
	if input[0].Get("id").String() != "tool-out-1" {
		t.Fatalf("unexpected incremental input item id: %s", input[0].Get("id").String())
	}
	if gjson.GetBytes(normalized, "model").String() != "test-model" {
		t.Fatalf("unexpected model: %s", gjson.GetBytes(normalized, "model").String())
	}
	if gjson.GetBytes(normalized, "instructions").String() != "be helpful" {
		t.Fatalf("unexpected instructions: %s", gjson.GetBytes(normalized, "instructions").String())
	}
	if !reflect.DeepEqual(next, normalized) {
		t.Fatalf("next request snapshot should match normalized request")
	}
}

func TestWebsocketJSONPayloadsFromChunk(t *testing.T) {
	chunk := []byte("event: response.created\n\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp-1\"}}\n\ndata: [DONE]\n")

	payloads := WebsocketJSONPayloadsFromChunk(chunk)
	if len(payloads) != 1 {
		t.Fatalf("payloads len = %d, want 1", len(payloads))
	}
	if gjson.GetBytes(payloads[0], "type").String() != "response.created" {
		t.Fatalf("unexpected payload type: %s", gjson.GetBytes(payloads[0], "type").String())
	}
}

func TestResponseCompletedOutputFromPayload(t *testing.T) {
	payload := []byte(`{"type":"response.completed","response":{"id":"resp-1","output":[{"type":"message","id":"out-1"}]}}`)

	output := ResponseCompletedOutputFromPayload(payload)
	items := gjson.ParseBytes(output).Array()
	if len(items) != 1 {
		t.Fatalf("output len = %d, want 1", len(items))
	}
	if items[0].Get("id").String() != "out-1" {
		t.Fatalf("unexpected output id: %s", items[0].Get("id").String())
	}
}

func TestAppendWebsocketEvent(t *testing.T) {
	var builder strings.Builder

	AppendWebsocketEvent(&builder, "request", []byte("  {\"type\":\"response.create\"}\n"))
	AppendWebsocketEvent(&builder, "response", []byte("{\"type\":\"response.created\"}"))

	got := builder.String()
	if !strings.Contains(got, "websocket.request\n{\"type\":\"response.create\"}\n") {
		t.Fatalf("request event not found in body: %s", got)
	}
	if !strings.Contains(got, "websocket.response\n{\"type\":\"response.created\"}\n") {
		t.Fatalf("response event not found in body: %s", got)
	}
}

func TestWebsocketPayloadEventType(t *testing.T) {
	if got := WebsocketPayloadEventType([]byte(`{"type":"response.completed"}`)); got != "response.completed" {
		t.Fatalf("event type = %q, want %q", got, "response.completed")
	}
	if got := WebsocketPayloadEventType([]byte(`{"ok":true}`)); got != "-" {
		t.Fatalf("event type = %q, want %q", got, "-")
	}
}

func TestWebsocketPayloadPreview(t *testing.T) {
	if got := WebsocketPayloadPreview([]byte(" \n ")); got != "<empty>" {
		t.Fatalf("preview = %q, want %q", got, "<empty>")
	}
	if got := WebsocketPayloadPreview([]byte("hello\r\nworld")); got != "hello\\r\\nworld" {
		t.Fatalf("preview = %q, want escaped newlines", got)
	}
}

func TestSetWebsocketRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	SetWebsocketRequestBody(c, " \n ")
	if _, exists := c.Get(WebsocketRequestBodyKey); exists {
		t.Fatalf("request body key should not be set for empty body")
	}

	SetWebsocketRequestBody(c, "event body")
	value, exists := c.Get(WebsocketRequestBodyKey)
	if !exists {
		t.Fatalf("request body key not set")
	}
	bodyBytes, ok := value.([]byte)
	if !ok {
		t.Fatalf("request body key type mismatch")
	}
	if string(bodyBytes) != "event body" {
		t.Fatalf("request body = %q, want %q", string(bodyBytes), "event body")
	}
}

func TestMarkAPIResponseTimestamp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	MarkAPIResponseTimestamp(c)
	value, exists := c.Get(APIResponseTimestampKey)
	if !exists {
		t.Fatalf("timestamp key not set")
	}
	first, ok := value.(time.Time)
	if !ok {
		t.Fatalf("timestamp key type mismatch")
	}

	expected := first.Add(-time.Minute)
	c.Set(APIResponseTimestampKey, expected)
	MarkAPIResponseTimestamp(c)
	value, exists = c.Get(APIResponseTimestampKey)
	if !exists {
		t.Fatalf("timestamp key missing after second mark")
	}
	if got, ok := value.(time.Time); !ok || !got.Equal(expected) {
		t.Fatalf("timestamp = %#v, want %#v", value, expected)
	}
}

func TestBuildResponsesWebsocketErrorPayload(t *testing.T) {
	payload, err := BuildResponsesWebsocketErrorPayload(http.StatusTooManyRequests, "rate limited", map[string][]string{
		"retry.after": {"5"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gjson.GetBytes(payload, "type").String() != "error" {
		t.Fatalf("type = %q, want %q", gjson.GetBytes(payload, "type").String(), "error")
	}
	if gjson.GetBytes(payload, "status").Int() != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", gjson.GetBytes(payload, "status").Int(), http.StatusTooManyRequests)
	}
	if gjson.GetBytes(payload, `headers.retry\.after`).String() != "5" {
		t.Fatalf("headers.retry.after = %q, want %q payload=%s", gjson.GetBytes(payload, `headers.retry\.after`).String(), "5", string(payload))
	}
	if gjson.GetBytes(payload, "error.message").String() != "rate limited" {
		t.Fatalf("error.message = %q, want %q", gjson.GetBytes(payload, "error.message").String(), "rate limited")
	}
	if gjson.GetBytes(payload, "error.type").String() == "" {
		t.Fatalf("error.type must not be empty")
	}
}

func TestNormalizeResponsesCompactRequestRejectsStream(t *testing.T) {
	_, status, err := NormalizeResponsesCompactRequest([]byte(`{"model":"gpt-4o","stream":true}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
	}
	if err.Error() != "Streaming not supported for compact responses" {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestNormalizeResponsesCompactRequestDeletesStreamFlag(t *testing.T) {
	normalized, status, err := NormalizeResponsesCompactRequest([]byte(`{"model":"gpt-4o","stream":false,"input":"hello"}`))
	if err != nil {
		t.Fatalf("unexpected error: status=%d err=%v", status, err)
	}
	if gjson.GetBytes(normalized, "stream").Exists() {
		t.Fatalf("stream field must be removed: %s", string(normalized))
	}
	if gjson.GetBytes(normalized, "model").String() != "gpt-4o" {
		t.Fatalf("model = %q, want %q", gjson.GetBytes(normalized, "model").String(), "gpt-4o")
	}
	if gjson.GetBytes(normalized, "input").String() != "hello" {
		t.Fatalf("input = %q, want %q", gjson.GetBytes(normalized, "input").String(), "hello")
	}
}

func TestRequireOpenAIModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	if modelName, ok := RequireOpenAIModel(c, []byte(`{"model":"gpt-4o"}`)); !ok || modelName != "gpt-4o" {
		t.Fatalf("modelName=%q ok=%t", modelName, ok)
	}

	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	if _, ok := RequireOpenAIModel(c, []byte(`{"prompt":"hello"}`)); ok {
		t.Fatal("expected missing model to fail")
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if !strings.Contains(recorder.Body.String(), `"message":"Model is required"`) {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestConvertCompletionsRequestToChatCompletions(t *testing.T) {
	converted := ConvertCompletionsRequestToChatCompletions([]byte(`{"model":"gpt-4o","prompt":"hello","max_tokens":42,"stream":true,"stop":["END"]}`))
	if gjson.GetBytes(converted, "model").String() != "gpt-4o" {
		t.Fatalf("model = %q", gjson.GetBytes(converted, "model").String())
	}
	if gjson.GetBytes(converted, "messages.0.role").String() != "user" {
		t.Fatalf("role = %q", gjson.GetBytes(converted, "messages.0.role").String())
	}
	if gjson.GetBytes(converted, "messages.0.content").String() != "hello" {
		t.Fatalf("content = %q", gjson.GetBytes(converted, "messages.0.content").String())
	}
	if gjson.GetBytes(converted, "max_tokens").Int() != 42 {
		t.Fatalf("max_tokens = %d", gjson.GetBytes(converted, "max_tokens").Int())
	}
	if !gjson.GetBytes(converted, "stream").Bool() {
		t.Fatalf("stream = false, want true")
	}
	if len(gjson.GetBytes(converted, "stop").Array()) != 1 {
		t.Fatalf("stop = %s", gjson.GetBytes(converted, "stop").Raw)
	}
}

func TestConvertChatCompletionsResponseToCompletions(t *testing.T) {
	converted := ConvertChatCompletionsResponseToCompletions([]byte(`{"id":"chatcmpl-1","object":"chat.completion","created":1,"model":"gpt-4o","choices":[{"index":0,"message":{"content":"done"},"finish_reason":"stop"}],"usage":{"total_tokens":10}}`))
	if gjson.GetBytes(converted, "object").String() != "text_completion" {
		t.Fatalf("object = %q", gjson.GetBytes(converted, "object").String())
	}
	if gjson.GetBytes(converted, "choices.0.text").String() != "done" {
		t.Fatalf("text = %q", gjson.GetBytes(converted, "choices.0.text").String())
	}
	if gjson.GetBytes(converted, "choices.0.finish_reason").String() != "stop" {
		t.Fatalf("finish_reason = %q", gjson.GetBytes(converted, "choices.0.finish_reason").String())
	}
	if gjson.GetBytes(converted, "usage.total_tokens").Int() != 10 {
		t.Fatalf("usage.total_tokens = %d", gjson.GetBytes(converted, "usage.total_tokens").Int())
	}
}

func TestShouldTreatAsResponsesFormat(t *testing.T) {
	if ShouldTreatAsResponsesFormat([]byte(`{"messages":[{"role":"user","content":"hi"}]}`)) {
		t.Fatal("chat payload must not be treated as responses format")
	}
	if !ShouldTreatAsResponsesFormat([]byte(`{"input":"hi"}`)) {
		t.Fatal("responses input payload must be treated as responses format")
	}
	if !ShouldTreatAsResponsesFormat([]byte(`{"instructions":"be helpful"}`)) {
		t.Fatal("responses instructions payload must be treated as responses format")
	}
}

func TestWrapResponsesPayloadAsCompleted(t *testing.T) {
	wrapped := WrapResponsesPayloadAsCompleted([]byte(`{"object":"response","id":"resp-1"}`))
	if gjson.GetBytes(wrapped, "type").String() != "response.completed" {
		t.Fatalf("type = %q", gjson.GetBytes(wrapped, "type").String())
	}
	if gjson.GetBytes(wrapped, "response.id").String() != "resp-1" {
		t.Fatalf("response.id = %q", gjson.GetBytes(wrapped, "response.id").String())
	}

	alreadyWrapped := []byte(`{"type":"response.completed","response":{"id":"resp-2"}}`)
	if string(WrapResponsesPayloadAsCompleted(alreadyWrapped)) != string(alreadyWrapped) {
		t.Fatalf("already wrapped payload should be preserved")
	}
}
