package openai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/sdk/api/handlers"
	sdkconfig "github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/sdk/config"
)

func TestOpenAIHandlersRejectMissingModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	base := handlers.NewBaseAPIHandlers(&sdkconfig.SDKConfig{}, nil)
	chatHandler := NewOpenAIAPIHandler(base)
	responsesHandler := NewOpenAIResponsesAPIHandler(base)

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
