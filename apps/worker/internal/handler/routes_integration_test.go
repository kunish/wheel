package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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
	v1.POST("/chat/completions", h.handleRelay)
	v1.POST("/completions", h.handleRelay)
	v1.POST("/embeddings", h.handleRelay)
	v1.POST("/responses", h.handleRelay)
	v1.POST("/images/generations", h.handleRelay)
	v1.POST("/audio/transcriptions", h.handleRelay)
	v1.POST("/audio/speech", h.handleRelay)
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

func successResponsesAPIResponse(model string) map[string]any {
	return map[string]any{
		"id":     "resp-test",
		"object": "response",
		"model":  model,
		"output": []any{map[string]any{
			"id":      "msg-test",
			"type":    "message",
			"role":    "assistant",
			"content": []any{map[string]any{"type": "output_text", "text": "ok"}},
		}},
	}
}

func successTextCompletionResponse(model string) map[string]any {
	return map[string]any{
		"id":     "cmpl-test",
		"object": "text_completion",
		"model":  model,
		"choices": []any{map[string]any{
			"index":         0,
			"text":          "ok",
			"finish_reason": "stop",
		}},
		"usage": map[string]any{
			"prompt_tokens":     3,
			"completion_tokens": 2,
			"total_tokens":      5,
		},
	}
}

func successEmbeddingsResponse(model string) map[string]any {
	return map[string]any{
		"object": "list",
		"model":  model,
		"data": []any{map[string]any{
			"object":    "embedding",
			"embedding": []any{0.1, 0.2},
			"index":     0,
		}},
		"usage": map[string]any{
			"prompt_tokens": 2,
			"total_tokens":  2,
		},
	}
}

func successImageGenerationResponse() map[string]any {
	return map[string]any{
		"created": 123,
		"data":    []any{map[string]any{"url": "https://example.com/generated.png"}},
	}
}

func lookupBatchResponse(t *testing.T, responses []any, customID string) map[string]any {
	t.Helper()
	for _, item := range responses {
		obj, _ := item.(map[string]any)
		if gotID, _ := obj["custom_id"].(string); gotID == customID {
			return obj
		}
	}
	t.Fatalf("missing batch response for %q in %#v", customID, responses)
	return nil
}

func putRelayTestGroups(h *RelayHandler, groups []types.Group) {
	h.Cache.Put("groups", groups, 300)
}

func assertListContainsID(t *testing.T, body []byte, wantID string) {
	t.Helper()

	payload := decodeJSONBody(t, body)
	data, ok := payload["data"].([]any)
	if !ok {
		t.Fatalf("expected list data array, got %#v", payload["data"])
	}
	for _, item := range data {
		obj, _ := item.(map[string]any)
		if gotID, _ := obj["id"].(string); gotID == wantID {
			return
		}
	}
	t.Fatalf("expected id %q in list payload: %#v", wantID, payload)
}

const (
	openAICompatInferenceModel = "gpt-4.1-mini"
	openAICompatImageModel     = "dall-e-3"
	openAICompatAudioModel     = "whisper-1"
	openAICompatSpeechModel    = "tts-1"
)

type openAICompatFixture struct {
	router *gin.Engine
	server *httptest.Server
}

func newOpenAICompatFixture(t *testing.T, models ...string) *openAICompatFixture {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(successCompletionResponse(openAICompatInferenceModel))
		case "/v1/completions":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(successTextCompletionResponse(openAICompatInferenceModel))
		case "/v1/responses":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(successResponsesAPIResponse(openAICompatInferenceModel))
		case "/v1/embeddings":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(successEmbeddingsResponse(openAICompatInferenceModel))
		case "/v1/images/generations":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(successImageGenerationResponse())
		case "/v1/audio/speech":
			w.Header().Set("Content-Type", "audio/mpeg")
			_, _ = w.Write([]byte("mp3-bytes"))
		case "/v1/audio/transcriptions":
			if err := r.ParseMultipartForm(10 << 20); err != nil {
				t.Fatalf("failed to parse multipart request: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"text":"hello world"}`))
		default:
			t.Fatalf("unexpected upstream path %q", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	channels := make([]types.Channel, 0, len(models))
	groups := make([]types.Group, 0, len(models))
	items := make([]types.GroupItem, 0, len(models))
	for idx, model := range models {
		groupID := idx + 1
		channelID := idx + 1
		keyID := 8200 + idx + 1
		item := types.GroupItem{GroupID: groupID, ChannelID: channelID, ModelName: model, Priority: 1, Enabled: true}
		channels = append(channels, makeChannelWithType(channelID, types.OutboundOpenAI, server.URL, keyID, fmt.Sprintf("k%d", channelID)))
		groups = append(groups, types.Group{ID: groupID, Name: model, Mode: types.GroupModeFailover, Items: []types.GroupItem{item}})
		items = append(items, item)
	}

	h := newTestRelayHandler(t, models[0], channels, items)
	putRelayTestGroups(h, groups)

	return &openAICompatFixture{
		router: newRelayTestEngine(h, strings.Join(models, ",")),
		server: server,
	}
}

func TestRelayRoutes_EndToEndOpenAICompatibleEndpoints(t *testing.T) {
	fixture := newOpenAICompatFixture(t,
		openAICompatInferenceModel,
		openAICompatImageModel,
		openAICompatAudioModel,
		openAICompatSpeechModel,
	)
	r := fixture.router

	t.Run("chat completions", func(t *testing.T) {
		resp := doJSONRequest(t, r, http.MethodPost, "/v1/chat/completions", map[string]any{
			"model":    openAICompatInferenceModel,
			"messages": []any{map[string]any{"role": "user", "content": "hello"}},
		})
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", resp.Code, resp.Body.String())
		}
		body := decodeJSONBody(t, resp.Body.Bytes())
		if got := body["object"]; got != "chat.completion" {
			t.Fatalf("object = %#v, want %q", got, "chat.completion")
		}
	})

	t.Run("completions", func(t *testing.T) {
		resp := doJSONRequest(t, r, http.MethodPost, "/v1/completions", map[string]any{
			"model":  openAICompatInferenceModel,
			"prompt": "hello",
		})
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", resp.Code, resp.Body.String())
		}
		body := decodeJSONBody(t, resp.Body.Bytes())
		if got := body["object"]; got != "text_completion" {
			t.Fatalf("object = %#v, want %q", got, "text_completion")
		}
	})

	t.Run("responses", func(t *testing.T) {
		resp := doJSONRequest(t, r, http.MethodPost, "/v1/responses", map[string]any{
			"model": openAICompatInferenceModel,
			"input": "hello",
		})
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", resp.Code, resp.Body.String())
		}
		body := decodeJSONBody(t, resp.Body.Bytes())
		if got := body["object"]; got != "response" {
			t.Fatalf("object = %#v, want %q", got, "response")
		}
	})

	t.Run("embeddings", func(t *testing.T) {
		resp := doJSONRequest(t, r, http.MethodPost, "/v1/embeddings", map[string]any{
			"model": openAICompatInferenceModel,
			"input": "hello",
		})
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", resp.Code, resp.Body.String())
		}
		body := decodeJSONBody(t, resp.Body.Bytes())
		if got := body["object"]; got != "list" {
			t.Fatalf("object = %#v, want %q", got, "list")
		}
	})

	t.Run("image generations", func(t *testing.T) {
		resp := doJSONRequest(t, r, http.MethodPost, "/v1/images/generations", map[string]any{
			"model":  openAICompatImageModel,
			"prompt": "a lighthouse on a cliff",
		})
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", resp.Code, resp.Body.String())
		}
		body := decodeJSONBody(t, resp.Body.Bytes())
		data, _ := body["data"].([]any)
		if len(data) != 1 {
			t.Fatalf("expected one image result, got %#v", body)
		}
	})

	t.Run("audio speech", func(t *testing.T) {
		resp := doJSONRequest(t, r, http.MethodPost, "/v1/audio/speech", map[string]any{
			"model": openAICompatSpeechModel,
			"input": "hello",
			"voice": "alloy",
		})
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", resp.Code, resp.Body.String())
		}
		if got := resp.Header().Get("Content-Type"); got != "audio/mpeg" {
			t.Fatalf("content-type = %q, want %q", got, "audio/mpeg")
		}
		if got := resp.Body.String(); got != "mp3-bytes" {
			t.Fatalf("body = %q, want %q", got, "mp3-bytes")
		}
	})

	t.Run("audio transcriptions", func(t *testing.T) {
		resp := doMultipartRequest(t, r, "/v1/audio/transcriptions", map[string]string{
			"model":    openAICompatAudioModel,
			"language": "en",
		}, "sample.wav", []byte("audio-bytes"))
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", resp.Code, resp.Body.String())
		}
		body := decodeJSONBody(t, resp.Body.Bytes())
		if got := body["text"]; got != "hello world" {
			t.Fatalf("text = %#v, want %q", got, "hello world")
		}
	})
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
	assertListContainsID(t, list.Body.Bytes(), jobID)
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
	assertListContainsID(t, list.Body.Bytes(), batchID)
}

func TestBatchRoutes_EndToEndSupportsOpenAICompatibleEndpoints(t *testing.T) {
	fixture := newOpenAICompatFixture(t, openAICompatInferenceModel, openAICompatImageModel)
	r := fixture.router

	create := doJSONRequest(t, r, http.MethodPost, "/v1/batch", map[string]any{
		"requests": []map[string]any{
			{
				"custom_id": "chat",
				"method":    "POST",
				"url":       "/v1/chat/completions",
				"body": map[string]any{
					"model":    openAICompatInferenceModel,
					"messages": []any{map[string]any{"role": "user", "content": "hello"}},
				},
			},
			{
				"custom_id": "responses",
				"method":    "POST",
				"url":       "/v1/responses",
				"body": map[string]any{
					"model": openAICompatInferenceModel,
					"input": "hello",
				},
			},
			{
				"custom_id": "embeddings",
				"method":    "POST",
				"url":       "/v1/embeddings",
				"body": map[string]any{
					"model": openAICompatInferenceModel,
					"input": "hello",
				},
			},
			{
				"custom_id": "images",
				"method":    "POST",
				"url":       "/v1/images/generations",
				"body": map[string]any{
					"model":  openAICompatImageModel,
					"prompt": "a lighthouse on a cliff",
				},
			},
		},
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
	if int(rc["completed"].(float64)) != 4 || int(rc["failed"].(float64)) != 0 {
		t.Fatalf("unexpected request counts: %#v", rc)
	}

	responses, _ := final["responses"].([]any)
	if len(responses) != 4 {
		t.Fatalf("expected four batch responses, got %#v", final["responses"])
	}

	chat := lookupBatchResponse(t, responses, "chat")
	chatResult, _ := chat["response"].(map[string]any)
	chatBody, _ := chatResult["body"].(map[string]any)
	if got := chatBody["object"]; got != "chat.completion" {
		t.Fatalf("chat object = %#v, want %q", got, "chat.completion")
	}

	responsesItem := lookupBatchResponse(t, responses, "responses")
	responsesResult, _ := responsesItem["response"].(map[string]any)
	responsesBody, _ := responsesResult["body"].(map[string]any)
	if got := responsesBody["object"]; got != "response" {
		t.Fatalf("responses object = %#v, want %q", got, "response")
	}

	embeddings := lookupBatchResponse(t, responses, "embeddings")
	embeddingsResult, _ := embeddings["response"].(map[string]any)
	embeddingsBody, _ := embeddingsResult["body"].(map[string]any)
	if got := embeddingsBody["object"]; got != "list" {
		t.Fatalf("embeddings object = %#v, want %q", got, "list")
	}

	images := lookupBatchResponse(t, responses, "images")
	imagesResult, _ := images["response"].(map[string]any)
	imagesBody, _ := imagesResult["body"].(map[string]any)
	data, _ := imagesBody["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("expected one image batch result, got %#v", imagesBody)
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

func TestBatchRoutes_RejectsUnsupportedAudioEndpointsAtCreateTime(t *testing.T) {
	model := "whisper-1"
	h := newTestRelayHandler(t, model,
		[]types.Channel{makeChannelWithType(1, types.OutboundOpenAI, "http://example.invalid", 4001, "k1")},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: model, Priority: 1, Enabled: true}},
	)
	r := newRelayTestEngine(h, model)

	create := doJSONRequest(t, r, http.MethodPost, "/v1/batch", map[string]any{
		"model": model,
		"requests": []map[string]any{{
			"custom_id": "req-1",
			"method":    "POST",
			"url":       "/v1/audio/transcriptions",
			"body": map[string]any{
				"model": model,
			},
		}},
	})
	if create.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on unsupported batch endpoint, got %d body=%s", create.Code, create.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(create.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal create error: %v", err)
	}
	errObj, _ := body["error"].(map[string]any)
	if got, _ := errObj["message"].(string); got != "Batch API does not support audio endpoint /v1/audio/transcriptions" {
		t.Fatalf("error message = %q, want %q", got, "Batch API does not support audio endpoint /v1/audio/transcriptions")
	}
}

func TestBatchRoutes_RejectsUnknownEndpointsAtCreateTime(t *testing.T) {
	model := "gpt-4o-mini"
	h := newTestRelayHandler(t, model,
		[]types.Channel{makeChannelWithType(1, types.OutboundOpenAI, "http://example.invalid", 4001, "k1")},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: model, Priority: 1, Enabled: true}},
	)
	r := newRelayTestEngine(h, model)

	create := doJSONRequest(t, r, http.MethodPost, "/v1/batch", map[string]any{
		"model": model,
		"requests": []map[string]any{{
			"custom_id": "req-1",
			"method":    "POST",
			"url":       "/v1/not-a-real-endpoint",
			"body": map[string]any{
				"model": model,
			},
		}},
	})
	if create.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on unknown batch endpoint, got %d body=%s", create.Code, create.Body.String())
	}

	assertStableOpenAIErrorEnvelope(t, create.Body.Bytes(), "invalid_request_error", "Unsupported batch endpoint /v1/not-a-real-endpoint")
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

func TestAsyncRoutes_ErrorEnvelopeConsistency(t *testing.T) {
	h := newTestRelayHandler(t, "test-model",
		[]types.Channel{makeChannelWithType(1, types.OutboundOpenAI, "http://example.invalid", 6001, "k1")},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: "test-model", Priority: 1, Enabled: true}},
	)
	r := newRelayTestEngine(h, "test-model")

	t.Run("missing model uses stable openai envelope", func(t *testing.T) {
		resp := doJSONRequest(t, r, http.MethodPost, "/v1/async/chat/completions", map[string]any{
			"messages": []any{map[string]any{"role": "user", "content": "hello"}},
		})
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", resp.Code, resp.Body.String())
		}
		assertStableOpenAIErrorEnvelope(t, resp.Body.Bytes(), "invalid_request_error", "Model is required")
	})

	t.Run("model access denied uses stable openai envelope", func(t *testing.T) {
		resp := doJSONRequest(t, r, http.MethodPost, "/v1/async/chat/completions", map[string]any{
			"model":    "forbidden-model",
			"messages": []any{map[string]any{"role": "user", "content": "hello"}},
		})
		if resp.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d body=%s", resp.Code, resp.Body.String())
		}
		assertStableOpenAIErrorEnvelope(t, resp.Body.Bytes(), "invalid_request_error", "Model not allowed for this API key")
	})
}

func TestBatchRoutes_ErrorEnvelopeConsistency(t *testing.T) {
	h := newTestRelayHandler(t, "test-model",
		[]types.Channel{makeChannelWithType(1, types.OutboundOpenAI, "http://example.invalid", 7001, "k1")},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: "test-model", Priority: 1, Enabled: true}},
	)
	r := newRelayTestEngine(h, "test-model")

	t.Run("missing per-request model uses stable openai envelope", func(t *testing.T) {
		resp := doJSONRequest(t, r, http.MethodPost, "/v1/batch", map[string]any{
			"requests": []map[string]any{{
				"custom_id": "req-1",
				"method":    "POST",
				"url":       "/v1/chat/completions",
				"body": map[string]any{
					"messages": []any{map[string]any{"role": "user", "content": "hello"}},
				},
			}},
		})
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", resp.Code, resp.Body.String())
		}
		assertStableOpenAIErrorEnvelope(t, resp.Body.Bytes(), "invalid_request_error", "Model is required for each batch request")
	})

	t.Run("model access denied uses stable openai envelope", func(t *testing.T) {
		resp := doJSONRequest(t, r, http.MethodPost, "/v1/batch", map[string]any{
			"model": "forbidden-model",
			"requests": []map[string]any{{
				"custom_id": "req-1",
				"method":    "POST",
				"url":       "/v1/chat/completions",
				"body": map[string]any{
					"model":    "forbidden-model",
					"messages": []any{map[string]any{"role": "user", "content": "hello"}},
				},
			}},
		})
		if resp.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d body=%s", resp.Code, resp.Body.String())
		}
		assertStableOpenAIErrorEnvelope(t, resp.Body.Bytes(), "invalid_request_error", "Model not allowed for this API key")
	})
}
