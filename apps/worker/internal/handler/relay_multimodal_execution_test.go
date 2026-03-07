package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

func newMultimodalRelayTestEngine(h *RelayHandler, supportedModels string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("supportedModels", supportedModels)
		c.Set("apiKeyId", 1)
		c.Next()
	})

	v1 := r.Group("/v1")
	v1.POST("/images/generations", h.handleRelay)
	v1.POST("/moderations", h.handleRelay)
	v1.POST("/audio/speech", h.handleRelay)
	v1.POST("/audio/transcriptions", h.handleRelay)
	v1.POST("/audio/translations", h.handleRelay)

	return r
}

func doMultipartRequest(t *testing.T, r http.Handler, path string, fields map[string]string, fileName string, fileBody []byte) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := w.WriteField(key, value); err != nil {
			t.Fatalf("failed to write field %q: %v", key, err)
		}
	}
	part, err := w.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := part.Write(fileBody); err != nil {
		t.Fatalf("failed to write form file: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func doMultipartRequestWithPairs(t *testing.T, r http.Handler, path string, pairs [][2]string, fileName string, fileBody []byte) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for _, pair := range pairs {
		if err := w.WriteField(pair[0], pair[1]); err != nil {
			t.Fatalf("failed to write field %q: %v", pair[0], err)
		}
	}
	part, err := w.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := part.Write(fileBody); err != nil {
		t.Fatalf("failed to write form file: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestHandleRelay_ModerationsUsesEndpointAwareJSONExecution(t *testing.T) {
	var gotPath string
	var gotQuery string
	var gotAPIKey string
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAPIKey = r.Header.Get("api-key")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"modr-1","results":[{"flagged":false}]}`))
	}))
	defer server.Close()

	h := newTestRelayHandler(t, "omni-moderation-latest",
		[]types.Channel{makeChannelWithType(1, types.OutboundAzureOpenAI, server.URL, 1001, "k1")},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: "omni-moderation-latest", Priority: 1, Enabled: true}},
	)
	h.Cache.Put("channels", []types.Channel{{
		ID:      1,
		Name:    "ch",
		Type:    types.OutboundAzureOpenAI,
		Enabled: true,
		BaseUrls: types.BaseUrlList{{
			URL:   server.URL,
			Delay: 0,
		}},
		CustomHeader: types.CustomHeaderList{{Key: "api-version", Value: "2025-02-01-preview"}},
		Keys: []types.ChannelKey{{
			ID:         1001,
			ChannelID:  1,
			Enabled:    true,
			ChannelKey: "k1",
		}},
	}}, 300)

	r := newMultimodalRelayTestEngine(h, "omni-moderation-latest")
	resp := doJSONRequest(t, r, http.MethodPost, "/v1/moderations", map[string]any{"input": "hello"})

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", resp.Code, resp.Body.String())
	}
	if gotPath != "/openai/deployments/omni-moderation-latest/moderations" {
		t.Fatalf("upstream path = %q, want %q", gotPath, "/openai/deployments/omni-moderation-latest/moderations")
	}
	if gotQuery != "api-version=2025-02-01-preview" {
		t.Fatalf("upstream query = %q, want %q", gotQuery, "api-version=2025-02-01-preview")
	}
	if gotAPIKey != "k1" {
		t.Fatalf("api-key = %q, want %q", gotAPIKey, "k1")
	}
	if _, ok := gotBody["model"]; ok {
		t.Fatalf("expected azure multimodal body to omit model, got %#v", gotBody)
	}
}

func TestHandleRelay_ImageGenerationUsesEndpointAwareJSONExecution(t *testing.T) {
	var gotPath string
	var gotQuery string
	var gotAPIKey string
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAPIKey = r.Header.Get("api-key")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"https://example.com/image.png"}]}`))
	}))
	defer server.Close()

	h := newTestRelayHandler(t, "dall-e-3",
		[]types.Channel{makeChannelWithType(1, types.OutboundAzureOpenAI, server.URL, 1001, "k1")},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: "dall-e-3", Priority: 1, Enabled: true}},
	)

	r := newMultimodalRelayTestEngine(h, "dall-e-3")
	resp := doJSONRequest(t, r, http.MethodPost, "/v1/images/generations", map[string]any{
		"prompt": "a lighthouse on a cliff",
	})

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", resp.Code, resp.Body.String())
	}
	if gotPath != "/openai/deployments/dall-e-3/images/generations" {
		t.Fatalf("upstream path = %q, want %q", gotPath, "/openai/deployments/dall-e-3/images/generations")
	}
	if gotQuery != "api-version=2024-12-01-preview" {
		t.Fatalf("upstream query = %q, want %q", gotQuery, "api-version=2024-12-01-preview")
	}
	if gotAPIKey != "k1" {
		t.Fatalf("api-key = %q, want %q", gotAPIKey, "k1")
	}
	if _, ok := gotBody["model"]; ok {
		t.Fatalf("expected azure multimodal body to omit model, got %#v", gotBody)
	}
}

func TestHandleRelay_AudioSpeechPassesThroughBinaryResponse(t *testing.T) {
	var gotPath string
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("mp3-bytes"))
	}))
	defer server.Close()

	h := newTestRelayHandler(t, "tts-1",
		[]types.Channel{makeChannel(1, server.URL, 1001, "k1")},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: "tts-1", Priority: 1, Enabled: true}},
	)

	r := newMultimodalRelayTestEngine(h, relayParseSupportedModels)
	resp := doJSONRequest(t, r, http.MethodPost, "/v1/audio/speech", map[string]any{
		"input": "hello",
		"voice": "alloy",
	})

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", resp.Code, resp.Body.String())
	}
	if gotPath != "/v1/audio/speech" {
		t.Fatalf("upstream path = %q, want %q", gotPath, "/v1/audio/speech")
	}
	if got, _ := gotBody["model"].(string); got != "tts-1" {
		t.Fatalf("upstream model = %q, want %q", got, "tts-1")
	}
	if resp.Header().Get("Content-Type") != "audio/mpeg" {
		t.Fatalf("content-type = %q, want %q", resp.Header().Get("Content-Type"), "audio/mpeg")
	}
	if body := strings.TrimSpace(resp.Body.String()); body != "mp3-bytes" {
		t.Fatalf("response body = %q, want %q", body, "mp3-bytes")
	}
}

func TestHandleRelay_AudioTranscriptionProxiesMultipartUpstream(t *testing.T) {
	var gotPath string
	var gotContentType string
	var gotModel string
	var gotFileName string
	var gotFileBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("failed to parse multipart request: %v", err)
		}
		gotModel = r.FormValue("model")
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("failed to read multipart file: %v", err)
		}
		defer file.Close()
		body, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("failed to read multipart file body: %v", err)
		}
		gotFileName = header.Filename
		gotFileBody = string(body)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"hello"}`))
	}))
	defer server.Close()

	h := newTestRelayHandler(t, "whisper-1",
		[]types.Channel{makeChannel(1, server.URL, 1001, "k1")},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: "whisper-1", Priority: 1, Enabled: true}},
	)

	r := newMultimodalRelayTestEngine(h, relayParseSupportedModels)
	resp := doMultipartRequest(t, r, "/v1/audio/transcriptions", map[string]string{
		"model":    "whisper-1",
		"language": "en",
	}, "sample.wav", []byte("audio-bytes"))

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", resp.Code, resp.Body.String())
	}
	if gotPath != "/v1/audio/transcriptions" {
		t.Fatalf("upstream path = %q, want %q", gotPath, "/v1/audio/transcriptions")
	}
	if !strings.HasPrefix(gotContentType, "multipart/form-data;") {
		t.Fatalf("content-type = %q, want multipart/form-data", gotContentType)
	}
	if gotModel != "whisper-1" {
		t.Fatalf("multipart model = %q, want %q", gotModel, "whisper-1")
	}
	if gotFileName != "sample.wav" {
		t.Fatalf("file name = %q, want %q", gotFileName, "sample.wav")
	}
	if gotFileBody != "audio-bytes" {
		t.Fatalf("file body = %q, want %q", gotFileBody, "audio-bytes")
	}
}

func TestHandleRelay_AudioTranscriptionMultipartUpstreamHonorsModelRewriteAndOverrides(t *testing.T) {
	var gotPath string
	var gotContentType string
	var gotModel string
	var gotResponseFormat string
	var gotTemperature string
	var gotFileName string
	var gotFileBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("failed to parse multipart request: %v", err)
		}
		gotModel = r.FormValue("model")
		gotResponseFormat = r.FormValue("response_format")
		gotTemperature = r.FormValue("temperature")
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("failed to read multipart file: %v", err)
		}
		defer file.Close()
		body, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("failed to read multipart file body: %v", err)
		}
		gotFileName = header.Filename
		gotFileBody = string(body)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"hello"}`))
	}))
	defer server.Close()

	overrides := `{"response_format":"verbose_json","temperature":0}`
	channel := makeChannel(1, server.URL, 1001, "k1")
	channel.ParamOverride = &overrides

	h := newTestRelayHandler(t, "client-whisper",
		[]types.Channel{channel},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: "provider-whisper", Priority: 1, Enabled: true}},
	)

	r := newMultimodalRelayTestEngine(h, "client-whisper")
	resp := doMultipartRequest(t, r, "/v1/audio/transcriptions", map[string]string{
		"model":    "client-whisper",
		"language": "en",
	}, "sample.wav", []byte("audio-bytes"))

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", resp.Code, resp.Body.String())
	}
	if gotPath != "/v1/audio/transcriptions" {
		t.Fatalf("upstream path = %q, want %q", gotPath, "/v1/audio/transcriptions")
	}
	if !strings.HasPrefix(gotContentType, "multipart/form-data;") {
		t.Fatalf("content-type = %q, want multipart/form-data", gotContentType)
	}
	if gotModel != "provider-whisper" {
		t.Fatalf("multipart model = %q, want %q", gotModel, "provider-whisper")
	}
	if gotResponseFormat != "verbose_json" {
		t.Fatalf("response_format = %q, want %q", gotResponseFormat, "verbose_json")
	}
	if gotTemperature != "0" {
		t.Fatalf("temperature = %q, want %q", gotTemperature, "0")
	}
	if gotFileName != "sample.wav" {
		t.Fatalf("file name = %q, want %q", gotFileName, "sample.wav")
	}
	if gotFileBody != "audio-bytes" {
		t.Fatalf("file body = %q, want %q", gotFileBody, "audio-bytes")
	}
}

func TestHandleRelay_AudioTranslationProxiesMultipartUpstream(t *testing.T) {
	var gotPath string
	var gotContentType string
	var gotModel string
	var gotFileName string
	var gotFileBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("failed to parse multipart request: %v", err)
		}
		gotModel = r.FormValue("model")
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("failed to read multipart file: %v", err)
		}
		defer file.Close()
		body, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("failed to read multipart file body: %v", err)
		}
		gotFileName = header.Filename
		gotFileBody = string(body)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"translated hello"}`))
	}))
	defer server.Close()

	h := newTestRelayHandler(t, "whisper-1",
		[]types.Channel{makeChannel(1, server.URL, 1001, "k1")},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: "whisper-1", Priority: 1, Enabled: true}},
	)

	r := newMultimodalRelayTestEngine(h, relayParseSupportedModels)
	resp := doMultipartRequest(t, r, "/v1/audio/translations", map[string]string{
		"model":  "whisper-1",
		"prompt": "translate to english",
	}, "sample.wav", []byte("audio-bytes"))

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", resp.Code, resp.Body.String())
	}
	if gotPath != "/v1/audio/translations" {
		t.Fatalf("upstream path = %q, want %q", gotPath, "/v1/audio/translations")
	}
	if !strings.HasPrefix(gotContentType, "multipart/form-data;") {
		t.Fatalf("content-type = %q, want multipart/form-data", gotContentType)
	}
	if gotModel != "whisper-1" {
		t.Fatalf("multipart model = %q, want %q", gotModel, "whisper-1")
	}
	if gotFileName != "sample.wav" {
		t.Fatalf("file name = %q, want %q", gotFileName, "sample.wav")
	}
	if gotFileBody != "audio-bytes" {
		t.Fatalf("file body = %q, want %q", gotFileBody, "audio-bytes")
	}
}

func TestHandleRelay_AudioTranscriptionMultipartUpstreamPreservesRepeatedFields(t *testing.T) {
	var gotGranularities []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("failed to parse multipart request: %v", err)
		}
		gotGranularities = append(gotGranularities, r.MultipartForm.Value["timestamp_granularities[]"]...)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"hello"}`))
	}))
	defer server.Close()

	channel := makeChannel(1, server.URL, 1001, "k1")
	h := newTestRelayHandler(t, "whisper-1",
		[]types.Channel{channel},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: "whisper-1", Priority: 1, Enabled: true}},
	)

	r := newMultimodalRelayTestEngine(h, relayParseSupportedModels)
	resp := doMultipartRequestWithPairs(t, r, "/v1/audio/transcriptions", [][2]string{
		{"model", "whisper-1"},
		{"timestamp_granularities[]", "word"},
		{"timestamp_granularities[]", "segment"},
	}, "sample.wav", []byte("audio-bytes"))

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", resp.Code, resp.Body.String())
	}
	if len(gotGranularities) != 2 || gotGranularities[0] != "word" || gotGranularities[1] != "segment" {
		t.Fatalf("timestamp_granularities[] = %#v, want [word segment]", gotGranularities)
	}
}
