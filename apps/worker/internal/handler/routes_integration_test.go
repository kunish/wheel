package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

func newRelayTestEngine(h *RelayHandler, supportedModels string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("supportedModels", supportedModels)
		c.Set("apiKeyId", 1)
		c.Next()
	})

	v1 := r.Group("/v1")
	v1.POST("/rerank", h.HandleRerank)
	v1.POST("/count-tokens", h.HandleCountTokens)
	v1.POST("/async/chat/completions", h.HandleCreateAsyncInference)
	v1.GET("/async", h.HandleListAsyncJobs)
	v1.GET("/async/:id", h.HandleGetAsyncJob)
	v1.POST("/batch", h.HandleCreateBatch)
	v1.GET("/batch", h.HandleListBatches)
	v1.GET("/batch/:id", h.HandleGetBatch)
	v1.POST("/batch/:id/cancel", h.HandleCancelBatch)

	return r
}

func doJSONRequest(t *testing.T, r http.Handler, method string, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()

	var bodyReader *bytes.Reader
	if payload == nil {
		bodyReader = bytes.NewReader(nil)
	} else {
		b, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("failed to marshal payload: %v", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func extractID(t *testing.T, body []byte) string {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	id, _ := obj["id"].(string)
	if id == "" {
		t.Fatalf("missing id in response: %s", string(body))
	}
	return id
}

func waitForStatus(t *testing.T, fn func() (string, map[string]any)) map[string]any {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		status, obj := fn()
		if status == "completed" || status == "cancelled" {
			return obj
		}
		if status == "failed" {
			t.Fatalf("job entered failed state: %#v", obj)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timeout waiting for terminal status")
	return nil
}

func TestAsyncRoutes_EndToEndSuccessFlow(t *testing.T) {
	model := "test-model"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(successCompletionResponse(model))
	}))
	defer server.Close()

	h := newTestRelayHandler(t, model,
		[]types.Channel{makeChannelWithType(1, types.OutboundOpenAI, server.URL, 2001, "k1")},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: model, Priority: 1, Enabled: true}},
	)
	r := newRelayTestEngine(h, model)

	create := doJSONRequest(t, r, http.MethodPost, "/v1/async/chat/completions", map[string]any{
		"model":    model,
		"messages": []any{map[string]any{"role": "user", "content": "hello"}},
	})
	if create.Code != http.StatusAccepted {
		t.Fatalf("expected 202 on create async, got %d body=%s", create.Code, create.Body.String())
	}
	jobID := extractID(t, create.Body.Bytes())

	final := waitForStatus(t, func() (string, map[string]any) {
		get := doJSONRequest(t, r, http.MethodGet, fmt.Sprintf("/v1/async/%s", jobID), nil)
		if get.Code != http.StatusOK {
			return "", nil
		}
		obj := map[string]any{}
		_ = json.Unmarshal(get.Body.Bytes(), &obj)
		status, _ := obj["status"].(string)
		return status, obj
	})

	if final["response"] == nil {
		t.Fatalf("expected async response in terminal payload: %#v", final)
	}

	list := doJSONRequest(t, r, http.MethodGet, "/v1/async?limit=10&offset=0", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("expected 200 on list async, got %d body=%s", list.Code, list.Body.String())
	}
}

func TestBatchRoutes_EndToEndSuccessFlow(t *testing.T) {
	model := "test-model"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(successCompletionResponse(model))
	}))
	defer server.Close()

	h := newTestRelayHandler(t, model,
		[]types.Channel{makeChannelWithType(1, types.OutboundOpenAI, server.URL, 3001, "k1")},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: model, Priority: 1, Enabled: true}},
	)
	r := newRelayTestEngine(h, model)

	create := doJSONRequest(t, r, http.MethodPost, "/v1/batch", map[string]any{
		"model": model,
		"requests": []map[string]any{{
			"custom_id": "req-1",
			"method":    "POST",
			"url":       "/v1/chat/completions",
			"body": map[string]any{
				"model":    model,
				"messages": []any{map[string]any{"role": "user", "content": "hello"}},
			},
		}},
	})
	if create.Code != http.StatusOK {
		t.Fatalf("expected 200 on create batch, got %d body=%s", create.Code, create.Body.String())
	}
	batchID := extractID(t, create.Body.Bytes())

	final := waitForStatus(t, func() (string, map[string]any) {
		get := doJSONRequest(t, r, http.MethodGet, fmt.Sprintf("/v1/batch/%s", batchID), nil)
		if get.Code != http.StatusOK {
			return "", nil
		}
		obj := map[string]any{}
		_ = json.Unmarshal(get.Body.Bytes(), &obj)
		status, _ := obj["status"].(string)
		return status, obj
	})

	rc, _ := final["request_counts"].(map[string]any)
	if rc == nil {
		t.Fatalf("missing request_counts in batch payload: %#v", final)
	}
	if int(rc["completed"].(float64)) != 1 || int(rc["failed"].(float64)) != 0 {
		t.Fatalf("unexpected request counts: %#v", rc)
	}

	list := doJSONRequest(t, r, http.MethodGet, "/v1/batch", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("expected 200 on list batch, got %d body=%s", list.Code, list.Body.String())
	}
}

func TestBatchRoutes_CancelStopsFurtherRequests(t *testing.T) {
	model := "test-model"

	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	var hits int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&hits, 1)
		if call == 1 {
			close(firstStarted)
			<-releaseFirst
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(successCompletionResponse(model))
	}))
	defer server.Close()

	h := newTestRelayHandler(t, model,
		[]types.Channel{makeChannelWithType(1, types.OutboundOpenAI, server.URL, 4001, "k1")},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: model, Priority: 1, Enabled: true}},
	)
	r := newRelayTestEngine(h, model)

	create := doJSONRequest(t, r, http.MethodPost, "/v1/batch", map[string]any{
		"model": model,
		"requests": []map[string]any{
			{
				"custom_id": "req-1",
				"method":    "POST",
				"url":       "/v1/chat/completions",
				"body": map[string]any{
					"model":    model,
					"messages": []any{map[string]any{"role": "user", "content": "a"}},
				},
			},
			{
				"custom_id": "req-2",
				"method":    "POST",
				"url":       "/v1/chat/completions",
				"body": map[string]any{
					"model":    model,
					"messages": []any{map[string]any{"role": "user", "content": "b"}},
				},
			},
		},
	})
	if create.Code != http.StatusOK {
		t.Fatalf("expected 200 on create batch, got %d body=%s", create.Code, create.Body.String())
	}
	batchID := extractID(t, create.Body.Bytes())

	select {
	case <-firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first batch request to start")
	}

	cancel := doJSONRequest(t, r, http.MethodPost, fmt.Sprintf("/v1/batch/%s/cancel", batchID), map[string]any{})
	if cancel.Code != http.StatusOK {
		t.Fatalf("expected 200 on cancel batch, got %d body=%s", cancel.Code, cancel.Body.String())
	}
	close(releaseFirst)

	final := waitForStatus(t, func() (string, map[string]any) {
		get := doJSONRequest(t, r, http.MethodGet, fmt.Sprintf("/v1/batch/%s", batchID), nil)
		if get.Code != http.StatusOK {
			return "", nil
		}
		obj := map[string]any{}
		_ = json.Unmarshal(get.Body.Bytes(), &obj)
		status, _ := obj["status"].(string)
		return status, obj
	})

	status, _ := final["status"].(string)
	if status != "cancelled" {
		t.Fatalf("expected cancelled batch status, got %s", status)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("expected only first request to run before cancellation, hits=%d", hits)
	}
}

func TestRoutes_ModelAccessDenied(t *testing.T) {
	model := "test-model"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(successCompletionResponse(model))
	}))
	defer server.Close()

	h := newTestRelayHandler(t, model,
		[]types.Channel{makeChannelWithType(1, types.OutboundOpenAI, server.URL, 5001, "k1")},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: model, Priority: 1, Enabled: true}},
	)
	// supportedModels does not include test-model
	r := newRelayTestEngine(h, "another-model")

	asyncCreate := doJSONRequest(t, r, http.MethodPost, "/v1/async/chat/completions", map[string]any{
		"model":    model,
		"messages": []any{map[string]any{"role": "user", "content": "hello"}},
	})
	if asyncCreate.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for async model denial, got %d body=%s", asyncCreate.Code, asyncCreate.Body.String())
	}

	batchCreate := doJSONRequest(t, r, http.MethodPost, "/v1/batch", map[string]any{
		"model": model,
		"requests": []map[string]any{{
			"custom_id": "req-1",
			"method":    "POST",
			"url":       "/v1/chat/completions",
			"body": map[string]any{
				"model":    model,
				"messages": []any{map[string]any{"role": "user", "content": "hello"}},
			},
		}},
	})
	if batchCreate.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for batch model denial, got %d body=%s", batchCreate.Code, batchCreate.Body.String())
	}

	rerank := doJSONRequest(t, r, http.MethodPost, "/v1/rerank", map[string]any{
		"model":     model,
		"query":     "q",
		"documents": []string{"a"},
	})
	if rerank.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for rerank model denial, got %d body=%s", rerank.Code, rerank.Body.String())
	}

	countTokens := doJSONRequest(t, r, http.MethodPost, "/v1/count-tokens", map[string]any{
		"model": model,
		"input": "hello",
	})
	if countTokens.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for count-tokens model denial, got %d body=%s", countTokens.Code, countTokens.Body.String())
	}
}
