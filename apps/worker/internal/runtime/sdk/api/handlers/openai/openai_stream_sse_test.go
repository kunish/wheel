package openai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/interfaces"
	"github.com/kunish/wheel/apps/worker/internal/runtime/sdk/api/handlers"
	sdkconfig "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/config"
)

func TestFormatOpenAIStreamChunkWrapsRawJSONOnce(t *testing.T) {
	got := formatOpenAIStreamChunk([]byte(`{"id":"chatcmpl-1"}`))
	want := "data: {\"id\":\"chatcmpl-1\"}\n\n"
	if got != want {
		t.Fatalf("formatOpenAIStreamChunk() = %q, want %q", got, want)
	}
}

func TestFormatOpenAIStreamChunkPreservesExistingSSEPayload(t *testing.T) {
	got := formatOpenAIStreamChunk([]byte("data: {\"id\":\"chatcmpl-1\"}"))
	want := "data: {\"id\":\"chatcmpl-1\"}\n\n"
	if got != want {
		t.Fatalf("formatOpenAIStreamChunk() = %q, want %q", got, want)
	}
}

func TestHandleStreamResultDoesNotDoubleWrapSSEChunk(t *testing.T) {
	gin.SetMode(gin.TestMode)
	base := handlers.NewBaseAPIHandlers(&sdkconfig.SDKConfig{}, nil)
	h := NewOpenAIAPIHandler(base)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		t.Fatalf("expected gin writer to implement http.Flusher")
	}

	data := make(chan []byte, 1)
	errChan := make(chan *interfaces.ErrorMessage)
	data <- []byte("data: {\"id\":\"chatcmpl-1\"}")
	close(data)
	close(errChan)

	h.handleStreamResult(c, flusher, func(error) {}, data, errChan)
	body := recorder.Body.String()
	if strings.Count(body, "data: ") != 2 {
		t.Fatalf("expected one chunk plus done marker, got: %q", body)
	}
	if strings.Contains(body, "data: data: {") {
		t.Fatalf("expected existing SSE chunk to be preserved, got: %q", body)
	}
	if !strings.Contains(body, "data: [DONE]") {
		t.Fatalf("expected done marker, got: %q", body)
	}
}

func TestConvertChatCompletionsStreamChunkToCompletionsAcceptsSSEPayload(t *testing.T) {
	chunk := []byte("data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"gpt-4o\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"OK\"}}]}")
	converted := convertChatCompletionsStreamChunkToCompletions(chunk)
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
