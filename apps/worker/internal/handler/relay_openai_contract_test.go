package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

const openAIContractTestModel = "test-model"

func newOpenAIContractTestEngine(h *RelayHandler, supportedModels string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("supportedModels", supportedModels)
		c.Set("apiKeyId", 1)
		c.Next()
	})

	v1 := r.Group("/v1")
	v1.GET("/models", h.handleModels)
	v1.POST("/chat/completions", h.handleRelay)

	return r
}

func newOpenAIContractTestRouter(t *testing.T) *gin.Engine {
	t.Helper()

	h := newTestRelayHandler(t, openAIContractTestModel,
		[]types.Channel{makeChannelWithType(1, types.OutboundOpenAI, "http://example.invalid", 1001, "k1")},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: openAIContractTestModel, Priority: 1, Enabled: true}},
	)

	return newOpenAIContractTestEngine(h, openAIContractTestModel)
}

func doJSONRequestWithHeaders(t *testing.T, r http.Handler, method string, path string, payload any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	var bodyReader *bytes.Reader
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("failed to marshal payload: %v", err)
		}
		bodyReader = bytes.NewReader(body)
	} else {
		bodyReader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestOpenAIContract(t *testing.T) {
	t.Run("invalid requests include null param and code fields", func(t *testing.T) {
		r := newOpenAIContractTestRouter(t)

		resp := doJSONRequest(t, r, http.MethodPost, "/v1/chat/completions", map[string]any{
			"messages": []map[string]any{{"role": "user", "content": "hello"}},
		})
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", resp.Code, resp.Body.String())
		}
		assertStableOpenAIErrorEnvelope(t, resp.Body.Bytes(), "invalid_request_error", "Model is required")
	})

	t.Run("models returns openai list shape", func(t *testing.T) {
		r := newOpenAIContractTestRouter(t)

		resp := doJSONRequest(t, r, http.MethodGet, "/v1/models", nil)
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", resp.Code, resp.Body.String())
		}

		body := decodeJSONBody(t, resp.Body.Bytes())

		if got := body["object"]; got != "list" {
			t.Fatalf("expected object %q, got %#v", "list", got)
		}
		data, ok := body["data"].([]any)
		if !ok {
			t.Fatalf("expected data array, got %#v", body["data"])
		}
		if len(data) == 0 {
			t.Fatal("expected at least one model in data")
		}
	})

	t.Run("successful relay forwards stable openai headers", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Request-Id", "req_success_123")
			w.Header().Set("X-Ratelimit-Limit-Requests", "500")
			w.Header().Set("X-Ratelimit-Remaining-Requests", "499")
			w.Header().Set("Set-Cookie", "session=secret")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(successCompletionResponse(openAIContractTestModel))
		}))
		defer upstream.Close()

		h := newTestRelayHandler(t, openAIContractTestModel,
			[]types.Channel{makeChannelWithType(1, types.OutboundOpenAI, upstream.URL, 1001, "k1")},
			[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: openAIContractTestModel, Priority: 1, Enabled: true}},
		)
		r := newOpenAIContractTestEngine(h, openAIContractTestModel)

		resp := doJSONRequest(t, r, http.MethodPost, "/v1/chat/completions", map[string]any{
			"model":    openAIContractTestModel,
			"messages": []map[string]any{{"role": "user", "content": "hello"}},
		})
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", resp.Code, resp.Body.String())
		}
		if got := resp.Header().Get("X-Request-Id"); got != "req_success_123" {
			t.Fatalf("expected X-Request-Id to be forwarded, got %q", got)
		}
		if got := resp.Header().Get("X-Ratelimit-Limit-Requests"); got != "500" {
			t.Fatalf("expected rate limit header to be forwarded, got %q", got)
		}
		if got := resp.Header().Get("Set-Cookie"); got != "" {
			t.Fatalf("expected unsafe headers to stay filtered, got %q", got)
		}
	})

	t.Run("rate limit errors preserve openai headers and stable envelope", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Request-Id", "req_rate_limit_123")
			w.Header().Set("X-Ratelimit-Limit-Requests", "500")
			w.Header().Set("X-Ratelimit-Remaining-Requests", "0")
			w.Header().Set("X-Ratelimit-Reset-Requests", "12ms")
			w.Header().Set("Retry-After", "2")
			w.Header().Set("Openai-Processing-Ms", "42")
			w.Header().Set("Cf-Cache-Status", "DYNAMIC")
			w.Header().Set("Set-Cookie", "session=secret")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
		}))
		defer upstream.Close()

		h := newTestRelayHandler(t, openAIContractTestModel,
			[]types.Channel{makeChannelWithType(1, types.OutboundOpenAI, upstream.URL, 1001, "k1")},
			[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: openAIContractTestModel, Priority: 1, Enabled: true}},
		)
		r := newOpenAIContractTestEngine(h, openAIContractTestModel)

		resp := doJSONRequest(t, r, http.MethodPost, "/v1/chat/completions", map[string]any{
			"model":    openAIContractTestModel,
			"messages": []map[string]any{{"role": "user", "content": "hello"}},
		})
		if resp.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429, got %d body=%s", resp.Code, resp.Body.String())
		}
		if got := resp.Header().Get("X-Request-Id"); got != "req_rate_limit_123" {
			t.Fatalf("expected X-Request-Id to be forwarded, got %q", got)
		}
		if got := resp.Header().Get("X-Ratelimit-Remaining-Requests"); got != "0" {
			t.Fatalf("expected rate limit headers to be forwarded, got %q", got)
		}
		if got := resp.Header().Get("Retry-After"); got != "2" {
			t.Fatalf("expected Retry-After to be preserved, got %q", got)
		}
		if got := resp.Header().Get("Openai-Processing-Ms"); got != "" {
			t.Fatalf("expected non-essential success header to be dropped on generated error, got %q", got)
		}
		if got := resp.Header().Get("Cf-Cache-Status"); got != "" {
			t.Fatalf("expected cache status header to be dropped on generated error, got %q", got)
		}
		if got := resp.Header().Get("Set-Cookie"); got != "" {
			t.Fatalf("expected unsafe headers to stay filtered, got %q", got)
		}
		assertStableOpenAIErrorEnvelope(t, resp.Body.Bytes(), "rate_limit_error", "All channels exhausted after 3 rounds. Last error: Upstream error 429: {\"error\":{\"message\":\"rate limited\"}}")
	})

	t.Run("models response is deterministic by fields", func(t *testing.T) {
		h := newTestRelayHandler(t, "ignored",
			[]types.Channel{makeChannelWithType(1, types.OutboundOpenAI, "http://example.invalid", 1001, "k1")},
			[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: "ignored", Priority: 1, Enabled: true}},
		)
		h.Cache.Put("groups", []types.Group{
			{ID: 1, Name: "zeta", Mode: types.GroupModeFailover},
			{ID: 2, Name: "alpha", Mode: types.GroupModeFailover},
		}, 300)

		r := newOpenAIContractTestEngine(h, "alpha,zeta")
		first := doJSONRequest(t, r, http.MethodGet, "/v1/models", nil)
		if first.Code != http.StatusOK {
			t.Fatalf("expected first response 200, got %d body=%s", first.Code, first.Body.String())
		}

		second := doJSONRequest(t, r, http.MethodGet, "/v1/models", nil)
		if second.Code != http.StatusOK {
			t.Fatalf("expected second response 200, got %d body=%s", second.Code, second.Body.String())
		}

		for idx, resp := range []struct {
			name string
			rec  *httptest.ResponseRecorder
		}{{name: "first", rec: first}, {name: "second", rec: second}} {
			_ = idx
			body := decodeJSONBody(t, resp.rec.Body.Bytes())
			data, ok := body["data"].([]any)
			if !ok || len(data) != 2 {
				t.Fatalf("expected two models in %s response, got %#v", resp.name, body["data"])
			}
			firstModel, _ := data[0].(map[string]any)
			secondModel, _ := data[1].(map[string]any)
			if got := firstModel["id"]; got != "alpha" {
				t.Fatalf("expected first model id %q in %s response, got %#v", "alpha", resp.name, got)
			}
			if got := secondModel["id"]; got != "zeta" {
				t.Fatalf("expected second model id %q in %s response, got %#v", "zeta", resp.name, got)
			}
			if got := firstModel["created"]; got != float64(0) {
				t.Fatalf("expected stable created timestamp 0 in %s response, got %#v", resp.name, got)
			}
			if got := secondModel["created"]; got != float64(0) {
				t.Fatalf("expected stable created timestamp 0 in %s response, got %#v", resp.name, got)
			}
		}
	})
}
