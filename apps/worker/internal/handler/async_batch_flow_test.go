package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kunish/wheel/apps/worker/internal/cache"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

func newTestRelayHandler(t *testing.T, model string, channels []types.Channel, items []types.GroupItem) *RelayHandler {
	t.Helper()

	kv := cache.New()
	t.Cleanup(kv.Close)

	groups := []types.Group{{
		ID:    1,
		Name:  model,
		Mode:  types.GroupModeFailover,
		Items: items,
	}}

	kv.Put("channels", channels, 300)
	kv.Put("groups", groups, 300)
	kv.Put("cb_config", map[string]any{"threshold": 5, "baseSec": 60, "maxSec": 600}, 300)

	cbm := relay.NewCircuitBreakerManager(nil, kv)

	return &RelayHandler{
		Handler: Handler{
			Cache: kv,
		},
		CircuitBreakers: cbm,
		Sessions:        relay.NewSessionManager(),
		Balancer:        relay.NewBalancerState(),
		HTTPClient: &http.Client{
			Timeout: 3 * time.Second,
		},
		BatchStore: relay.NewBatchStore(),
		AsyncStore: relay.NewAsyncStore(),
	}
}

func makeChannel(id int, baseURL string, keyID int, key string) types.Channel {
	return types.Channel{
		ID:      id,
		Name:    "ch",
		Type:    types.OutboundOpenAI,
		Enabled: true,
		BaseUrls: types.BaseUrlList{
			{URL: baseURL, Delay: 0},
		},
		Keys: []types.ChannelKey{{
			ID:         keyID,
			ChannelID:  id,
			Enabled:    true,
			ChannelKey: key,
		}},
	}
}

func successCompletionResponse(model string) map[string]any {
	return map[string]any{
		"id":     "chatcmpl-test",
		"object": "chat.completion",
		"model":  model,
		"choices": []any{map[string]any{
			"index": 0,
			"message": map[string]any{
				"role":    "assistant",
				"content": "ok",
			},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{
			"prompt_tokens":     3,
			"completion_tokens": 2,
			"total_tokens":      5,
		},
	}
}

func TestProcessAsyncJob_RetryThenSuccess(t *testing.T) {
	model := "test-model"

	var firstHits int32
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&firstHits, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer first.Close()

	var secondHits int32
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&secondHits, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(successCompletionResponse(model))
	}))
	defer second.Close()

	h := newTestRelayHandler(t, model,
		[]types.Channel{
			makeChannel(1, first.URL, 101, "k1"),
			makeChannel(2, second.URL, 102, "k2"),
		},
		[]types.GroupItem{
			{GroupID: 1, ChannelID: 1, ModelName: model, Priority: 1, Enabled: true},
			{GroupID: 1, ChannelID: 2, ModelName: model, Priority: 2, Enabled: true},
		},
	)

	job := h.AsyncStore.CreateJob(model, 1, map[string]any{
		"model":    model,
		"messages": []any{map[string]any{"role": "user", "content": "hello"}},
	})

	h.processAsyncJob(job.ID)

	got := h.AsyncStore.GetJob(job.ID)
	if got == nil {
		t.Fatal("expected async job to exist")
	}
	if got.Status != relay.AsyncStatusCompleted {
		t.Fatalf("expected async status completed, got %s", got.Status)
	}
	if got.Response == nil {
		t.Fatal("expected async response to be populated")
	}
	if atomic.LoadInt32(&firstHits) == 0 || atomic.LoadInt32(&secondHits) == 0 {
		t.Fatalf("expected both channels to be attempted, first=%d second=%d", firstHits, secondHits)
	}
}

func TestProcessBatchJob_RetryThenSuccess(t *testing.T) {
	model := "test-model"

	var firstHits int32
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&firstHits, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer first.Close()

	var secondHits int32
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&secondHits, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(successCompletionResponse(model))
	}))
	defer second.Close()

	h := newTestRelayHandler(t, model,
		[]types.Channel{
			makeChannel(1, first.URL, 201, "k1"),
			makeChannel(2, second.URL, 202, "k2"),
		},
		[]types.GroupItem{
			{GroupID: 1, ChannelID: 1, ModelName: model, Priority: 1, Enabled: true},
			{GroupID: 1, ChannelID: 2, ModelName: model, Priority: 2, Enabled: true},
		},
	)

	job := h.BatchStore.CreateJob([]relay.BatchRequest{{
		CustomID: "req-1",
		Method:   http.MethodPost,
		URL:      "/v1/chat/completions",
		Body: map[string]any{
			"model":    model,
			"messages": []any{map[string]any{"role": "user", "content": "hello"}},
		},
	}}, model, 1)

	h.processBatchJob(job.ID)

	got := h.BatchStore.GetJob(job.ID)
	if got == nil {
		t.Fatal("expected batch job to exist")
	}
	if got.Status != relay.BatchStatusCompleted {
		t.Fatalf("expected batch status completed, got %s", got.Status)
	}
	if len(got.Responses) != 1 || got.Responses[0].Response == nil {
		t.Fatalf("expected one successful batch response, got %#v", got.Responses)
	}
	if atomic.LoadInt32(&firstHits) == 0 || atomic.LoadInt32(&secondHits) == 0 {
		t.Fatalf("expected both channels to be attempted, first=%d second=%d", firstHits, secondHits)
	}
}

func TestProcessBatchJob_CancelStopsFurtherRequests(t *testing.T) {
	model := "test-model"

	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	var hits int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&hits, 1)
		if call == 1 {
			close(firstStarted)
			<-releaseFirst
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(successCompletionResponse(model))
	}))
	defer srv.Close()

	h := newTestRelayHandler(t, model,
		[]types.Channel{makeChannel(1, srv.URL, 301, "k1")},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: model, Priority: 1, Enabled: true}},
	)

	job := h.BatchStore.CreateJob([]relay.BatchRequest{
		{
			CustomID: "req-1",
			Method:   http.MethodPost,
			URL:      "/v1/chat/completions",
			Body: map[string]any{
				"model":    model,
				"messages": []any{map[string]any{"role": "user", "content": "a"}},
			},
		},
		{
			CustomID: "req-2",
			Method:   http.MethodPost,
			URL:      "/v1/chat/completions",
			Body: map[string]any{
				"model":    model,
				"messages": []any{map[string]any{"role": "user", "content": "b"}},
			},
		},
	}, model, 1)

	done := make(chan struct{})
	go func() {
		h.processBatchJob(job.ID)
		close(done)
	}()

	select {
	case <-firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first batch request to start")
	}

	if ok := h.BatchStore.CancelJob(job.ID); !ok {
		t.Fatal("expected cancel to succeed")
	}
	close(releaseFirst)

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for batch processor to stop")
	}

	got := h.BatchStore.GetJob(job.ID)
	if got == nil {
		t.Fatal("expected batch job to exist")
	}
	if got.Status != relay.BatchStatusCancelled {
		t.Fatalf("expected cancelled batch status, got %s", got.Status)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("expected only first request to execute after cancel, hits=%d", hits)
	}
}

func TestProcessAsyncJob_AllRetriesFailed(t *testing.T) {
	model := "test-model"

	var hits int32
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer failing.Close()

	h := newTestRelayHandler(t, model,
		[]types.Channel{makeChannel(1, failing.URL, 401, "k1")},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: model, Priority: 1, Enabled: true}},
	)

	job := h.AsyncStore.CreateJob(model, 1, map[string]any{
		"model":    model,
		"messages": []any{map[string]any{"role": "user", "content": "hello"}},
	})

	h.processAsyncJob(job.ID)

	got := h.AsyncStore.GetJob(job.ID)
	if got == nil {
		t.Fatal("expected async job to exist")
	}
	if got.Status != relay.AsyncStatusFailed {
		t.Fatalf("expected async status failed, got %s", got.Status)
	}
	if got.Error == nil || *got.Error == "" {
		t.Fatal("expected async error to be recorded")
	}
	if atomic.LoadInt32(&hits) != int32(maxRetryRounds) {
		t.Fatalf("expected %d attempts, got %d", maxRetryRounds, hits)
	}
}

func TestProcessBatchJob_AllRetriesFailed(t *testing.T) {
	model := "test-model"

	var hits int32
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer failing.Close()

	h := newTestRelayHandler(t, model,
		[]types.Channel{makeChannel(1, failing.URL, 501, "k1")},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: model, Priority: 1, Enabled: true}},
	)

	job := h.BatchStore.CreateJob([]relay.BatchRequest{{
		CustomID: "req-1",
		Method:   http.MethodPost,
		URL:      "/v1/chat/completions",
		Body: map[string]any{
			"model":    model,
			"messages": []any{map[string]any{"role": "user", "content": "hello"}},
		},
	}}, model, 1)

	h.processBatchJob(job.ID)

	got := h.BatchStore.GetJob(job.ID)
	if got == nil {
		t.Fatal("expected batch job to exist")
	}
	if got.Status != relay.BatchStatusCompleted {
		t.Fatalf("expected batch status completed with item errors, got %s", got.Status)
	}
	if len(got.Responses) != 1 || got.Responses[0].Error == nil {
		t.Fatalf("expected one failed batch item response, got %#v", got.Responses)
	}
	if got.RequestCounts.Failed != 1 {
		t.Fatalf("expected failed count 1, got %d", got.RequestCounts.Failed)
	}
	if atomic.LoadInt32(&hits) != int32(maxRetryRounds) {
		t.Fatalf("expected %d attempts, got %d", maxRetryRounds, hits)
	}
}

func TestExecuteBackgroundNonStream_Proxy429WithNilDBDoesNotPanic(t *testing.T) {
	model := "test-model"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer server.Close()

	h := newTestRelayHandler(t, model,
		[]types.Channel{makeChannel(1, server.URL, 901, "k1")},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: model, Priority: 1, Enabled: true}},
	)
	h.DB = nil

	_, err := h.executeBackgroundNonStream(
		"/v1/chat/completions",
		map[string]any{"model": model, "messages": []any{map[string]any{"role": "user", "content": "hello"}}},
		model,
		1,
	)
	if err == nil {
		t.Fatal("expected error when upstream responds with 429")
	}
}

func TestExecuteBackgroundNonStream_SupportsEndpointAwareJSONRoutes(t *testing.T) {
	model := "dall-e-3"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/v1/images/generations")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"https://example.com/image.png"}]}`))
	}))
	defer server.Close()

	h := newTestRelayHandler(t, model,
		[]types.Channel{makeChannel(1, server.URL, 901, "k1")},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: model, Priority: 1, Enabled: true}},
	)

	result, err := h.executeBackgroundNonStream(
		"/v1/images/generations",
		map[string]any{"model": model, "prompt": "hello"},
		model,
		1,
	)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result for endpoint-aware json route")
	}
	data, ok := result.Response["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("unexpected image response: %#v", result.Response)
	}
}

func TestExecuteBackgroundNonStream_RejectsUnsupportedAudioEndpoints(t *testing.T) {
	h := newTestRelayHandler(t, "whisper-1",
		[]types.Channel{makeChannel(1, "http://example.invalid", 901, "k1")},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: "whisper-1", Priority: 1, Enabled: true}},
	)

	_, err := h.executeBackgroundNonStream(
		"/v1/audio/transcriptions",
		map[string]any{"model": "whisper-1"},
		"whisper-1",
		1,
	)
	if err == nil {
		t.Fatal("expected explicit error for unsupported audio background execution")
	}
	if got := err.Error(); got != "background execution does not support audio endpoint /v1/audio/transcriptions" {
		t.Fatalf("error = %q, want %q", got, "background execution does not support audio endpoint /v1/audio/transcriptions")
	}
}
