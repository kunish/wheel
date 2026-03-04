package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

func makeChannelWithType(id int, channelType types.OutboundType, baseURL string, keyID int, key string) types.Channel {
	ch := makeChannel(id, baseURL, keyID, key)
	ch.Type = channelType
	return ch
}

func newAuthJSONContext(t *testing.T, method string, path string, payload any, supportedModel string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("supportedModels", supportedModel)
	c.Set("apiKeyId", 1)

	return c, w
}

func TestHandleRerank_RetryThenSuccess_RewritesModel(t *testing.T) {
	inboundModel := "inbound-model"
	targetModel := "provider-model"

	var firstHits int32
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&firstHits, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer first.Close()

	var secondHits int32
	var capturedModel string
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&secondHits, 1)
		defer r.Body.Close()
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		if model, _ := req["model"].(string); model != "" {
			capturedModel = model
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    "rr1",
			"model": targetModel,
			"results": []map[string]any{{
				"index":           0,
				"relevance_score": 0.9,
			}},
		})
	}))
	defer second.Close()

	h := newTestRelayHandler(t, inboundModel,
		[]types.Channel{
			makeChannelWithType(1, types.OutboundOpenAI, first.URL, 1001, "k1"),
			makeChannelWithType(2, types.OutboundOpenAI, second.URL, 1002, "k2"),
		},
		[]types.GroupItem{
			{GroupID: 1, ChannelID: 1, ModelName: targetModel, Priority: 1, Enabled: true},
			{GroupID: 1, ChannelID: 2, ModelName: targetModel, Priority: 2, Enabled: true},
		},
	)

	c, w := newAuthJSONContext(t, http.MethodPost, "/v1/rerank", map[string]any{
		"model":     inboundModel,
		"query":     "hello",
		"documents": []string{"a", "b"},
	}, inboundModel)

	h.HandleRerank(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp relay.RerankResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal rerank response: %v", err)
	}
	if resp.Model != targetModel {
		t.Fatalf("expected response model %q, got %q", targetModel, resp.Model)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected one rerank result, got %d", len(resp.Results))
	}
	if capturedModel != targetModel {
		t.Fatalf("expected upstream model rewrite to %q, got %q", targetModel, capturedModel)
	}
	if atomic.LoadInt32(&firstHits) == 0 || atomic.LoadInt32(&secondHits) == 0 {
		t.Fatalf("expected both channels attempted, first=%d second=%d", firstHits, secondHits)
	}
}

func TestHandleCountTokens_AnthropicRetryThenSuccess_RewritesModel(t *testing.T) {
	inboundModel := "inbound-model"
	targetModel := "provider-model"

	var firstHits int32
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&firstHits, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer first.Close()

	var secondHits int32
	var capturedModel string
	var capturedPath string
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&secondHits, 1)
		capturedPath = r.URL.Path
		defer r.Body.Close()
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		if model, _ := req["model"].(string); model != "" {
			capturedModel = model
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"input_tokens": 12})
	}))
	defer second.Close()

	h := newTestRelayHandler(t, inboundModel,
		[]types.Channel{
			makeChannelWithType(1, types.OutboundAnthropic, first.URL, 1101, "k1"),
			makeChannelWithType(2, types.OutboundAnthropic, second.URL, 1102, "k2"),
		},
		[]types.GroupItem{
			{GroupID: 1, ChannelID: 1, ModelName: targetModel, Priority: 1, Enabled: true},
			{GroupID: 1, ChannelID: 2, ModelName: targetModel, Priority: 2, Enabled: true},
		},
	)

	c, w := newAuthJSONContext(t, http.MethodPost, "/v1/count-tokens", map[string]any{
		"model": inboundModel,
		"input": "hello",
	}, inboundModel)

	h.HandleCountTokens(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp relay.CountTokensResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal count-tokens response: %v", err)
	}
	if resp.InputTokens != 12 {
		t.Fatalf("expected input tokens 12, got %d", resp.InputTokens)
	}
	if resp.Model != targetModel {
		t.Fatalf("expected response model %q, got %q", targetModel, resp.Model)
	}
	if capturedModel != targetModel {
		t.Fatalf("expected upstream model rewrite to %q, got %q", targetModel, capturedModel)
	}
	if capturedPath != "/v1/messages/count_tokens" {
		t.Fatalf("expected anthropic endpoint /v1/messages/count_tokens, got %q", capturedPath)
	}
	if atomic.LoadInt32(&firstHits) == 0 || atomic.LoadInt32(&secondHits) == 0 {
		t.Fatalf("expected both channels attempted, first=%d second=%d", firstHits, secondHits)
	}
}

func TestHandleCountTokens_OpenAIFallbackWithoutUpstreamCall(t *testing.T) {
	inboundModel := "inbound-model"
	targetModel := "provider-model"

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"input_tokens":999}`))
	}))
	defer server.Close()

	h := newTestRelayHandler(t, inboundModel,
		[]types.Channel{makeChannelWithType(1, types.OutboundOpenAI, server.URL, 1201, "k1")},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: targetModel, Priority: 1, Enabled: true}},
	)

	c, w := newAuthJSONContext(t, http.MethodPost, "/v1/count-tokens", map[string]any{
		"model": inboundModel,
		"input": "hello world",
	}, inboundModel)

	h.HandleCountTokens(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp relay.CountTokensResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal count-tokens response: %v", err)
	}
	if resp.InputTokens <= 0 {
		t.Fatalf("expected estimated token count > 0, got %d", resp.InputTokens)
	}
	if resp.Model != targetModel {
		t.Fatalf("expected rewritten model %q, got %q", targetModel, resp.Model)
	}
	if atomic.LoadInt32(&hits) != 0 {
		t.Fatalf("expected no upstream call for OpenAI fallback path, hits=%d", hits)
	}
}

func TestHandleRerank_AllRetriesFailedReturnsBadGateway(t *testing.T) {
	model := "inbound-model"

	var hits int32
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer failing.Close()

	h := newTestRelayHandler(t, model,
		[]types.Channel{makeChannelWithType(1, types.OutboundOpenAI, failing.URL, 1301, "k1")},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: model, Priority: 1, Enabled: true}},
	)

	c, w := newAuthJSONContext(t, http.MethodPost, "/v1/rerank", map[string]any{
		"model":     model,
		"query":     "hello",
		"documents": []string{"a"},
	}, model)

	h.HandleRerank(c)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d body=%s", w.Code, w.Body.String())
	}
	if atomic.LoadInt32(&hits) != int32(maxRetryRounds) {
		t.Fatalf("expected %d attempts, got %d", maxRetryRounds, hits)
	}
}

func TestHandleCountTokens_AnthropicAllRetriesFailedReturnsBadGateway(t *testing.T) {
	model := "inbound-model"

	var hits int32
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer failing.Close()

	h := newTestRelayHandler(t, model,
		[]types.Channel{makeChannelWithType(1, types.OutboundAnthropic, failing.URL, 1401, "k1")},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: model, Priority: 1, Enabled: true}},
	)

	c, w := newAuthJSONContext(t, http.MethodPost, "/v1/count-tokens", map[string]any{
		"model": model,
		"input": "hello",
	}, model)

	h.HandleCountTokens(c)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d body=%s", w.Code, w.Body.String())
	}
	if atomic.LoadInt32(&hits) != int32(maxRetryRounds) {
		t.Fatalf("expected %d attempts, got %d", maxRetryRounds, hits)
	}
}
