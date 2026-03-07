package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/relay"
)

const relayParseSupportedModels = "tts-1,whisper-1,gpt-4o-mini-transcribe,gpt-4o-mini"

type multipartRequest struct {
	fields   map[string]string
	fieldsKV [][2]string
	fileName string
	fileBody []byte
}

func newRelayParseTestContext(t *testing.T, method, path, contentType string, body []byte, supportedModels string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	c.Request = req
	c.Set("supportedModels", supportedModels)
	c.Set("apiKeyId", 1)
	return c, w
}

func mustJSONBody(t *testing.T, payload any) []byte {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal json payload: %v", err)
	}
	return b
}

func mustMultipartBody(t *testing.T, req multipartRequest) ([]byte, string) {
	t.Helper()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for key, value := range req.fields {
		if err := w.WriteField(key, value); err != nil {
			t.Fatalf("failed to write field %q: %v", key, err)
		}
	}
	for _, pair := range req.fieldsKV {
		if err := w.WriteField(pair[0], pair[1]); err != nil {
			t.Fatalf("failed to write field %q: %v", pair[0], err)
		}
	}
	if req.fileName != "" {
		part, err := w.CreateFormFile("file", req.fileName)
		if err != nil {
			t.Fatalf("failed to create form file: %v", err)
		}
		if _, err := part.Write(req.fileBody); err != nil {
			t.Fatalf("failed to write form file: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}
	return body.Bytes(), w.FormDataContentType()
}

func newMultipartAudioContext(t *testing.T, path string, req multipartRequest) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	body, contentType := mustMultipartBody(t, req)
	return newRelayParseTestContext(t, http.MethodPost, path, contentType, body, relayParseSupportedModels)
}

func parseErrorMessage(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to unmarshal error payload: %v body=%s", err, w.Body.String())
	}
	errObj, _ := payload["error"].(map[string]any)
	msg, _ := errObj["message"].(string)
	return msg
}

func TestParseRelayRequest(t *testing.T) {
	h := &RelayHandler{}

	t.Run("classifies json audio speech and applies default model", func(t *testing.T) {
		body := mustJSONBody(t, map[string]any{
			"input": "hello",
			"voice": "alloy",
		})
		c, w := newRelayParseTestContext(t, http.MethodPost, "/v1/audio/speech", "application/json", body, relayParseSupportedModels)

		req := h.parseRelayRequest(c)
		if req == nil {
			t.Fatalf("expected parsed request, got nil with status %d body=%s", w.Code, w.Body.String())
		}
		if req.RequestType != relay.RequestTypeAudioSpeech {
			t.Fatalf("request type = %q, want %q", req.RequestType, relay.RequestTypeAudioSpeech)
		}
		if req.Model != "tts-1" {
			t.Fatalf("model = %q, want %q", req.Model, "tts-1")
		}
	})

	t.Run("multipart audio transcription preserves repeated form values", func(t *testing.T) {
		c, w := newMultipartAudioContext(t, "/v1/audio/transcriptions", multipartRequest{
			fields: map[string]string{"model": "gpt-4o-mini-transcribe"},
			fieldsKV: [][2]string{
				{"timestamp_granularities[]", "word"},
				{"timestamp_granularities[]", "segment"},
			},
			fileName: "sample.wav",
			fileBody: []byte("audio-bytes"),
		})

		req := h.parseRelayRequest(c)
		if req == nil {
			t.Fatalf("expected parsed request, got nil with status %d body=%s", w.Code, w.Body.String())
		}
		values, ok := req.Body["timestamp_granularities[]"].([]any)
		if !ok {
			t.Fatalf("timestamp_granularities[] = %#v, want []any", req.Body["timestamp_granularities[]"])
		}
		if len(values) != 2 || values[0] != "word" || values[1] != "segment" {
			t.Fatalf("timestamp_granularities[] = %#v, want [word segment]", values)
		}
	})

	t.Run("rejects missing model on standard json request", func(t *testing.T) {
		body := mustJSONBody(t, map[string]any{
			"messages": []map[string]any{{"role": "user", "content": "hello"}},
		})
		c, w := newRelayParseTestContext(t, http.MethodPost, "/v1/chat/completions", "application/json", body, relayParseSupportedModels)

		req := h.parseRelayRequest(c)
		if req != nil {
			t.Fatalf("expected nil request for missing model, got %#v", req)
		}
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		if got := parseErrorMessage(t, w); got != "Model is required" {
			t.Fatalf("error message = %q, want %q", got, "Model is required")
		}
	})

	t.Run("rejects oversized request body with clear validation error", func(t *testing.T) {
		body := bytes.Repeat([]byte("a"), maxRelayRequestBodyBytes+1)
		c, w := newRelayParseTestContext(t, http.MethodPost, "/v1/chat/completions", "application/json", body, relayParseSupportedModels)

		req := h.parseRelayRequest(c)
		if req != nil {
			t.Fatalf("expected nil request for oversized body, got %#v", req)
		}
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		if got := parseErrorMessage(t, w); got != "Request body too large" {
			t.Fatalf("error message = %q, want %q", got, "Request body too large")
		}
	})

	t.Run("multipart audio transcription parses form fields and file metadata", func(t *testing.T) {
		c, w := newMultipartAudioContext(t, "/v1/audio/transcriptions", multipartRequest{
			fields:   map[string]string{"model": "gpt-4o-mini-transcribe", "language": "en"},
			fileName: "sample.wav",
			fileBody: []byte("audio-bytes"),
		})

		req := h.parseRelayRequest(c)
		if req == nil {
			t.Fatalf("expected parsed request, got nil with status %d body=%s", w.Code, w.Body.String())
		}
		if req.RequestType != relay.RequestTypeAudioTranscribe {
			t.Fatalf("request type = %q, want %q", req.RequestType, relay.RequestTypeAudioTranscribe)
		}
		if req.Model != "gpt-4o-mini-transcribe" {
			t.Fatalf("model = %q, want %q", req.Model, "gpt-4o-mini-transcribe")
		}
		if got, _ := req.Body["language"].(string); got != "en" {
			t.Fatalf("language = %q, want %q", got, "en")
		}
		if _, ok := req.Body["file"].(map[string]any); !ok {
			t.Fatalf("expected file metadata in parsed body, got %#v", req.Body["file"])
		}
	})

	t.Run("multipart audio translation requires explicit model", func(t *testing.T) {
		c, w := newMultipartAudioContext(t, "/v1/audio/translations", multipartRequest{
			fields:   map[string]string{"prompt": "translate to english"},
			fileName: "sample.wav",
			fileBody: []byte("audio-bytes"),
		})

		req := h.parseRelayRequest(c)
		if req != nil {
			t.Fatalf("expected nil request for missing multipart model, got %#v", req)
		}
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		if got := parseErrorMessage(t, w); got != "Model is required" {
			t.Fatalf("error message = %q, want %q", got, "Model is required")
		}
	})

	t.Run("multipart audio transcription missing file fails validation", func(t *testing.T) {
		c, w := newMultipartAudioContext(t, "/v1/audio/transcriptions", multipartRequest{
			fields: map[string]string{"model": "gpt-4o-mini-transcribe"},
		})

		req := h.parseRelayRequest(c)
		if req != nil {
			t.Fatalf("expected nil request for missing multipart file, got %#v", req)
		}
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		if got := parseErrorMessage(t, w); got != "File is required" {
			t.Fatalf("error message = %q, want %q", got, "File is required")
		}
	})
}
