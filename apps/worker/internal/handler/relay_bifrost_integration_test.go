package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/bifrostx"
	"github.com/kunish/wheel/apps/worker/internal/cache"
	"github.com/kunish/wheel/apps/worker/internal/config"
	"github.com/kunish/wheel/apps/worker/internal/db"
	"github.com/kunish/wheel/apps/worker/internal/observe"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/types"
	"github.com/uptrace/bun"
)

type capturedUpstreamRequest struct {
	Path    string
	Headers http.Header
	Body    map[string]any
}

type upstreamRecorder struct {
	mu   sync.Mutex
	last capturedUpstreamRequest
}

func (r *upstreamRecorder) set(req capturedUpstreamRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.last = req
}

func (r *upstreamRecorder) get() capturedUpstreamRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := r.last
	cp.Headers = cp.Headers.Clone()
	cpBody := make(map[string]any, len(cp.Body))
	for k, v := range cp.Body {
		cpBody[k] = v
	}
	cp.Body = cpBody
	return cp
}

type relayHarness struct {
	router    *gin.Engine
	recorder  *upstreamRecorder
	apiKey    string
	groupName string
}

func TestRelayBifrost_ProtocolInterOp_ModelMapping(t *testing.T) {
	h := newRelayHarness(t)

	tests := []struct {
		name           string
		path           string
		body           map[string]any
		assertResponse func(t *testing.T, body []byte)
	}{
		{
			name: "openai inbound",
			path: "/v1/chat/completions",
			body: map[string]any{
				"model": h.groupName,
				"messages": []any{
					map[string]any{"role": "user", "content": "hi"},
				},
			},
			assertResponse: func(t *testing.T, body []byte) {
				t.Helper()
				var resp map[string]any
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatalf("unmarshal response: %v", err)
				}
				choices, _ := resp["choices"].([]any)
				if len(choices) == 0 {
					t.Fatalf("expected openai choices")
				}
			},
		},
		{
			name: "anthropic inbound",
			path: "/v1/messages",
			body: map[string]any{
				"model": h.groupName,
				"messages": []any{
					map[string]any{
						"role": "user",
						"content": []any{
							map[string]any{"type": "text", "text": "hi"},
						},
					},
				},
				"max_tokens": 128,
			},
			assertResponse: func(t *testing.T, body []byte) {
				t.Helper()
				var resp map[string]any
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatalf("unmarshal response: %v", err)
				}
				if resp["type"] != "message" {
					t.Fatalf("expected anthropic message response, got %v", resp["type"])
				}
			},
		},
		{
			name: "gemini inbound",
			path: "/v1beta/models/" + h.groupName + ":generateContent",
			body: map[string]any{
				"contents": []any{
					map[string]any{
						"role": "user",
						"parts": []any{
							map[string]any{"text": "hi"},
						},
					},
				},
				"generationConfig": map[string]any{
					"maxOutputTokens": 128,
				},
			},
			assertResponse: func(t *testing.T, body []byte) {
				t.Helper()
				var resp map[string]any
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatalf("unmarshal response: %v", err)
				}
				candidates, _ := resp["candidates"].([]any)
				if len(candidates) == 0 {
					t.Fatalf("expected gemini candidates")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			reqBody, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(string(reqBody)))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+h.apiKey)
			h.router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}

			tc.assertResponse(t, rec.Body.Bytes())

			upstream := h.recorder.get()
			if upstream.Path != "/v1/chat/completions" {
				t.Fatalf("expected upstream /v1/chat/completions, got %s", upstream.Path)
			}
			if got := toString(upstream.Body["model"]); got != "gpt-5.3-codex" {
				t.Fatalf("expected mapped model gpt-5.3-codex, got %q", got)
			}
			if auth := upstream.Headers.Get("Authorization"); auth != "Bearer sk-upstream-test" {
				t.Fatalf("expected upstream authorization header, got %q", auth)
			}
		})
	}
}

func TestRelayBifrost_OpenAIStream(t *testing.T) {
	h := newRelayHarness(t)

	body := map[string]any{
		"model":  h.groupName,
		"stream": true,
		"messages": []any{
			map[string]any{"role": "user", "content": "hello stream"},
		},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.apiKey)
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "data:") {
		t.Fatalf("expected sse data in response: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "[DONE]") {
		t.Fatalf("expected [DONE] marker in response: %s", rec.Body.String())
	}

	upstream := h.recorder.get()
	if got := toString(upstream.Body["model"]); got != "gpt-5.3-codex" {
		t.Fatalf("expected mapped model gpt-5.3-codex, got %q", got)
	}
	if stream, _ := upstream.Body["stream"].(bool); !stream {
		t.Fatalf("expected upstream stream=true")
	}
}

func TestRelayBifrost_ResponsesChannel_UsesResponsesStream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := &upstreamRecorder{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var body map[string]any
		if err := json.Unmarshal(bodyBytes, &body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		recorder.set(capturedUpstreamRequest{
			Path:    r.URL.Path,
			Headers: r.Header.Clone(),
			Body:    body,
		})

		if r.URL.Path != "/v1/responses" {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}

		if stream, _ := body["stream"].(bool); stream {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, _ := w.(http.Flusher)
			events := []string{
				`data: {"type":"response.created","sequence_number":0,"response":{"id":"resp_1","created_at":1,"model":"gpt-5.3-codex"}}` + "\n\n",
				`data: {"type":"response.output_text.delta","sequence_number":1,"delta":"hello"}` + "\n\n",
				`data: {"type":"response.completed","sequence_number":2,"response":{"id":"resp_1","created_at":1,"model":"gpt-5.3-codex","usage":{"input_tokens":12,"input_tokens_details":{"cached_tokens":0},"output_tokens":4,"output_tokens_details":{"reasoning_tokens":0,"cached_tokens":0},"total_tokens":16}}}` + "\n\n",
				"data: [DONE]\n\n",
			}
			for _, evt := range events {
				_, _ = io.WriteString(w, evt)
				if flusher != nil {
					flusher.Flush()
				}
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "resp_1",
			"model":   "gpt-5.3-codex",
			"status":  "completed",
			"output":  []any{},
			"usage":   map[string]any{"input_tokens": 12, "output_tokens": 4, "total_tokens": 16},
			"created": 1,
		})
	}))
	t.Cleanup(upstream.Close)

	mainDB, logDB, kv := newTestDatabases(t)
	groupName := "claude-opus-4-6-thinking"
	apiKey := "sk-wheel-integration-test"
	seedRelayData(t, mainDB, upstream.URL, groupName, apiKey, types.OutboundOpenAIResponses)

	bf, err := bifrostx.New(context.Background(), mainDB, false)
	if err != nil {
		t.Fatalf("init bifrost: %v", err)
	}

	logWriter := db.NewLogWriter(logDB, mainDB, nil, nil, nil, kv)
	obs, _ := observe.New(false, false, "", "")
	handler := &RelayHandler{
		Handler: Handler{
			DB:     mainDB,
			LogDB:  logDB,
			Cache:  kv,
			Config: &config.Config{BifrostDebugRaw: false},
		},
		LogWriter:       logWriter,
		Bifrost:         bf,
		Observer:        obs,
		CircuitBreakers: relay.NewCircuitBreakerManager(obs, kv),
		Sessions:        relay.NewSessionManager(),
		Balancer:        relay.NewBalancerState(),
	}
	router := gin.New()
	handler.RegisterRelayRoutes(router)

	reqBody := map[string]any{
		"model":      groupName,
		"stream":     true,
		"max_tokens": 128,
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "hello"},
				},
			},
		},
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages?beta=true", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "event: message_stop") {
		t.Fatalf("expected anthropic stream to stop, body=%s", rec.Body.String())
	}
	upstreamReq := recorder.get()
	if upstreamReq.Path != "/v1/responses" {
		t.Fatalf("expected upstream path /v1/responses, got %s", upstreamReq.Path)
	}
	if stream, _ := upstreamReq.Body["stream"].(bool); !stream {
		t.Fatalf("expected upstream stream=true for responses request")
	}
}

// TestRelayBifrost_ClaudeCodeSimulation simulates Claude Code's actual usage:
// Anthropic Messages inbound → Responses API upstream, with tool_calls.
func TestRelayBifrost_ClaudeCodeSimulation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	requestCount := 0
	var mu sync.Mutex

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		defer r.Body.Close()

		var body map[string]any
		json.Unmarshal(bodyBytes, &body)

		mu.Lock()
		requestCount++
		reqNum := requestCount
		mu.Unlock()

		if r.URL.Path != "/v1/responses" {
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		var events []string
		switch {
		case reqNum == 1:
			// First request: text + tool_call (Claude Code reading a file)
			events = []string{
				`data: {"type":"response.created","sequence_number":0,"response":{"id":"resp_cc1","created_at":1,"model":"gpt-5.3-codex"}}` + "\n\n",
				`data: {"type":"response.output_text.delta","sequence_number":1,"delta":"Let me read that file."}` + "\n\n",
				`data: {"type":"response.output_item.added","sequence_number":2,"item":{"id":"fc_1","type":"function_call","name":"Read","call_id":"call_read1"},"output_index":1}` + "\n\n",
				`data: {"type":"response.function_call_arguments.delta","sequence_number":3,"item_id":"fc_1","delta":"{\"file_path\"","output_index":1}` + "\n\n",
				`data: {"type":"response.function_call_arguments.delta","sequence_number":4,"item_id":"fc_1","delta":": \"/test.go\"}","output_index":1}` + "\n\n",
				`data: {"type":"response.function_call_arguments.done","sequence_number":5,"item_id":"fc_1","arguments":"{\"file_path\": \"/test.go\"}","output_index":1}` + "\n\n",
				`data: {"type":"response.completed","sequence_number":6,"response":{"id":"resp_cc1","created_at":1,"model":"gpt-5.3-codex","stop_reason":"tool_calls","usage":{"input_tokens":500,"input_tokens_details":{"cached_tokens":100},"output_tokens":50,"output_tokens_details":{"reasoning_tokens":0,"cached_tokens":0},"total_tokens":550}}}` + "\n\n",
				"data: [DONE]\n\n",
			}
		case reqNum == 2:
			// Second request: tool_call only, no text (Claude Code running a command)
			events = []string{
				`data: {"type":"response.created","sequence_number":0,"response":{"id":"resp_cc2","created_at":1,"model":"gpt-5.3-codex"}}` + "\n\n",
				`data: {"type":"response.output_item.added","sequence_number":1,"item":{"id":"fc_2","type":"function_call","name":"Bash","call_id":"call_bash1"},"output_index":0}` + "\n\n",
				`data: {"type":"response.function_call_arguments.delta","sequence_number":2,"item_id":"fc_2","delta":"{\"command\": \"go test ./...\"}","output_index":0}` + "\n\n",
				`data: {"type":"response.function_call_arguments.done","sequence_number":3,"item_id":"fc_2","arguments":"{\"command\": \"go test ./...\"}","output_index":0}` + "\n\n",
				`data: {"type":"response.completed","sequence_number":4,"response":{"id":"resp_cc2","created_at":1,"model":"gpt-5.3-codex","stop_reason":"tool_calls","usage":{"input_tokens":800,"input_tokens_details":{"cached_tokens":200},"output_tokens":30,"output_tokens_details":{"reasoning_tokens":0,"cached_tokens":0},"total_tokens":830}}}` + "\n\n",
				"data: [DONE]\n\n",
			}
		default:
			// Third+ request: plain text response (Claude Code answering)
			events = []string{
				`data: {"type":"response.created","sequence_number":0,"response":{"id":"resp_cc3","created_at":1,"model":"gpt-5.3-codex"}}` + "\n\n",
				`data: {"type":"response.output_text.delta","sequence_number":1,"delta":"All tests pass."}` + "\n\n",
				`data: {"type":"response.completed","sequence_number":2,"response":{"id":"resp_cc3","created_at":1,"model":"gpt-5.3-codex","usage":{"input_tokens":1000,"input_tokens_details":{"cached_tokens":500},"output_tokens":10,"output_tokens_details":{"reasoning_tokens":0,"cached_tokens":0},"total_tokens":1010}}}` + "\n\n",
				"data: [DONE]\n\n",
			}
		}

		for _, evt := range events {
			_, _ = io.WriteString(w, evt)
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
	t.Cleanup(upstream.Close)

	mainDB, logDB, kv := newTestDatabases(t)
	groupName := "claude-opus-4-6-thinking"
	apiKey := "sk-wheel-integration-test"
	seedRelayData(t, mainDB, upstream.URL, groupName, apiKey, types.OutboundOpenAIResponses)

	bf, err := bifrostx.New(context.Background(), mainDB, false)
	if err != nil {
		t.Fatalf("init bifrost: %v", err)
	}

	logWriter := db.NewLogWriter(logDB, mainDB, nil, nil, nil, kv)
	obs, _ := observe.New(false, false, "", "")
	handler := &RelayHandler{
		Handler: Handler{
			DB:     mainDB,
			LogDB:  logDB,
			Cache:  kv,
			Config: &config.Config{BifrostDebugRaw: false},
		},
		LogWriter:       logWriter,
		Bifrost:         bf,
		Observer:        obs,
		CircuitBreakers: relay.NewCircuitBreakerManager(obs, kv),
		Sessions:        relay.NewSessionManager(),
		Balancer:        relay.NewBalancerState(),
	}
	router := gin.New()
	handler.RegisterRelayRoutes(router)

	// Helper: send an Anthropic Messages streaming request (like Claude Code does)
	sendRequest := func(t *testing.T, messages []any, tools []any) *httptest.ResponseRecorder {
		t.Helper()
		reqBody := map[string]any{
			"model":      groupName,
			"stream":     true,
			"max_tokens": 32000,
			"messages":   messages,
		}
		if tools != nil {
			reqBody["tools"] = tools
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	claudeCodeTools := []any{
		map[string]any{
			"name":        "Read",
			"description": "Read a file",
			"input_schema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"file_path": map[string]any{"type": "string"}},
				"required":   []any{"file_path"},
			},
		},
		map[string]any{
			"name":        "Bash",
			"description": "Run a bash command",
			"input_schema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"command": map[string]any{"type": "string"}},
				"required":   []any{"command"},
			},
		},
	}

	// ── Request 1: text + tool_call ──
	t.Run("text_plus_tool_call", func(t *testing.T) {
		rec := sendRequest(t, []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "Read /test.go"},
				},
			},
		}, claudeCodeTools)

		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if !strings.Contains(body, "event: message_start") {
			t.Fatalf("missing message_start:\n%s", body)
		}
		if !strings.Contains(body, "event: message_stop") {
			t.Fatalf("missing message_stop:\n%s", body)
		}
		// Should have text content
		if !strings.Contains(body, "text_delta") || !strings.Contains(body, "Let me read that file.") {
			t.Fatalf("missing text content:\n%s", body)
		}
		// Should have tool_use block
		if !strings.Contains(body, "tool_use") {
			t.Fatalf("missing tool_use block:\n%s", body)
		}
		if !strings.Contains(body, "Read") {
			t.Fatalf("missing tool name 'Read':\n%s", body)
		}
		if !strings.Contains(body, "input_json_delta") {
			t.Fatalf("missing input_json_delta:\n%s", body)
		}
		// stop_reason should be tool_use
		if !strings.Contains(body, `"tool_use"`) {
			t.Fatalf("expected stop_reason tool_use:\n%s", body)
		}
	})

	// ── Request 2: tool_call only (no text) ──
	t.Run("tool_call_only", func(t *testing.T) {
		rec := sendRequest(t, []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "run tests"},
				},
			},
		}, claudeCodeTools)

		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if !strings.Contains(body, "event: message_stop") {
			t.Fatalf("missing message_stop:\n%s", body)
		}
		if !strings.Contains(body, "tool_use") {
			t.Fatalf("missing tool_use block:\n%s", body)
		}
		if !strings.Contains(body, "Bash") {
			t.Fatalf("missing tool name 'Bash':\n%s", body)
		}
		if !strings.Contains(body, `"tool_use"`) {
			t.Fatalf("expected stop_reason tool_use:\n%s", body)
		}
	})

	// ── Request 3: plain text (provider still alive after 2 requests) ──
	t.Run("plain_text_after_tool_calls", func(t *testing.T) {
		rec := sendRequest(t, []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "summarize"},
				},
			},
		}, nil)

		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if !strings.Contains(body, "event: message_stop") {
			t.Fatalf("missing message_stop:\n%s", body)
		}
		if !strings.Contains(body, "All tests pass.") {
			t.Fatalf("missing text content:\n%s", body)
		}
	})

	mu.Lock()
	finalCount := requestCount
	mu.Unlock()
	if finalCount != 3 {
		t.Fatalf("expected 3 upstream requests, got %d", finalCount)
	}
}

func newRelayHarness(t *testing.T) *relayHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)

	recorder := &upstreamRecorder{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var body map[string]any
		if err := json.Unmarshal(bodyBytes, &body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		recorder.set(capturedUpstreamRequest{
			Path:    r.URL.Path,
			Headers: r.Header.Clone(),
			Body:    body,
		})

		if stream, _ := body["stream"].(bool); stream {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, _ := w.(http.Flusher)

			model := toString(body["model"])
			chunks := []string{
				fmt.Sprintf(`data: {"id":"chatcmpl-stream","object":"chat.completion.chunk","created":1,"model":%q,"choices":[{"index":0,"delta":{"role":"assistant","content":"hello"},"finish_reason":null}]}`+"\n\n", model),
				fmt.Sprintf(`data: {"id":"chatcmpl-stream","object":"chat.completion.chunk","created":1,"model":%q,"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":4,"total_tokens":16}}`+"\n\n", model),
				"data: [DONE]\n\n",
			}
			for _, chunk := range chunks {
				_, _ = io.WriteString(w, chunk)
				if flusher != nil {
					flusher.Flush()
				}
			}
			return
		}

		model := toString(body["model"])
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": 1,
			"model":   model,
			"choices": []any{
				map[string]any{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "hello from upstream",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     12,
				"completion_tokens": 4,
				"total_tokens":      16,
			},
		})
	}))
	t.Cleanup(upstream.Close)

	mainDB, logDB, kv := newTestDatabases(t)
	groupName := "claude-opus-4-6-thinking"
	apiKey := "sk-wheel-integration-test"
	seedRelayData(t, mainDB, upstream.URL, groupName, apiKey, types.OutboundOpenAIChat)

	bf, err := bifrostx.New(context.Background(), mainDB, false)
	if err != nil {
		t.Fatalf("init bifrost: %v", err)
	}

	logWriter := db.NewLogWriter(logDB, mainDB, nil, nil, nil, kv)
	obs, _ := observe.New(false, false, "", "")
	handler := &RelayHandler{
		Handler: Handler{
			DB:     mainDB,
			LogDB:  logDB,
			Cache:  kv,
			Config: &config.Config{BifrostDebugRaw: false},
		},
		LogWriter:       logWriter,
		Bifrost:         bf,
		Observer:        obs,
		CircuitBreakers: relay.NewCircuitBreakerManager(obs, kv),
		Sessions:        relay.NewSessionManager(),
		Balancer:        relay.NewBalancerState(),
	}

	router := gin.New()
	handler.RegisterRelayRoutes(router)

	return &relayHarness{
		router:    router,
		recorder:  recorder,
		apiKey:    apiKey,
		groupName: groupName,
	}
}

func newTestDatabases(t *testing.T) (*bun.DB, *bun.DB, *cache.MemoryKV) {
	t.Helper()
	tmpDir := t.TempDir()

	mainDB, err := db.Open(filepath.Join(tmpDir, "wheel.db"))
	if err != nil {
		t.Fatalf("open main db: %v", err)
	}
	t.Cleanup(func() { _ = mainDB.Close() })

	logDB, err := db.OpenLogDB(filepath.Join(tmpDir, "wheel-log.db"))
	if err != nil {
		t.Fatalf("open log db: %v", err)
	}
	t.Cleanup(func() { _ = logDB.Close() })

	migrationsDir := testMigrationsDir(t)
	if err := db.Migrate(mainDB.DB, migrationsDir); err != nil {
		t.Fatalf("migrate main db: %v", err)
	}
	if err := db.MigrateLogDB(logDB.DB); err != nil {
		t.Fatalf("migrate log db: %v", err)
	}

	kv := cache.New()
	t.Cleanup(kv.Close)
	return mainDB, logDB, kv
}

func seedRelayData(t *testing.T, database *bun.DB, upstreamBaseURL, groupName, apiKey string, outboundType types.OutboundType) {
	t.Helper()
	ctx := context.Background()

	channel := &types.Channel{
		Name:         "upstream-openai",
		Type:         outboundType,
		Enabled:      true,
		BaseUrls:     types.BaseUrlList{{URL: upstreamBaseURL, Delay: 0}},
		Model:        types.StringList{},
		FetchedModel: types.StringList{},
		CustomHeader: types.CustomHeaderList{},
		Order:        0,
	}
	if _, err := database.NewInsert().Model(channel).Exec(ctx); err != nil {
		t.Fatalf("insert channel: %v", err)
	}

	channelKey := &types.ChannelKey{
		ChannelID:  channel.ID,
		Enabled:    true,
		ChannelKey: "sk-upstream-test",
		Remark:     "integration",
	}
	if _, err := database.NewInsert().Model(channelKey).Exec(ctx); err != nil {
		t.Fatalf("insert channel key: %v", err)
	}

	group := &types.Group{
		Name:              groupName,
		Mode:              types.GroupModeFailover,
		FirstTokenTimeOut: 10,
		SessionKeepTime:   0,
		Order:             0,
	}
	if _, err := database.NewInsert().Model(group).Exec(ctx); err != nil {
		t.Fatalf("insert group: %v", err)
	}

	groupItem := &types.GroupItem{
		GroupID:   group.ID,
		ChannelID: channel.ID,
		ModelName: "gpt-5.3-codex",
		Priority:  0,
		Weight:    1,
		Enabled:   true,
	}
	if _, err := database.NewInsert().Model(groupItem).Exec(ctx); err != nil {
		t.Fatalf("insert group item: %v", err)
	}

	key := &types.APIKey{
		Name:            "integration-key",
		APIKey:          apiKey,
		Enabled:         true,
		ExpireAt:        0,
		MaxCost:         0,
		SupportedModels: "",
	}
	if _, err := database.NewInsert().Model(key).Exec(ctx); err != nil {
		t.Fatalf("insert api key: %v", err)
	}
}

func testMigrationsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	workerRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	return filepath.Join(workerRoot, "drizzle")
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
