package openai

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/runtime/sdk/api/handlers"
	sdkopenai "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/api/handlers/openai"
	"github.com/tidwall/gjson"
)

func (h *APIHandler) ChatCompletions(c *gin.Context) {
	rawJSON, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, handlers.ErrorResponse{
			Error: handlers.ErrorDetail{
				Message: fmt.Sprintf("Invalid request: %v", err),
				Type:    "invalid_request_error",
			},
		})
		return
	}
	stream := gjson.GetBytes(rawJSON, "stream").Type == gjson.True
	modelName, ok := requireOpenAIModel(c, rawJSON)
	if !ok {
		return
	}
	if overrideEndpoint, ok := h.resolveChatOverride(modelName); ok && overrideEndpoint == sdkopenai.OpenAIResponsesEndpoint {
		if stream {
			h.handleChatStreamingResponseViaResponses(c, rawJSON)
			return
		}
		h.handleChatNonStreamingResponseViaResponses(c, rawJSON)
		return
	}
	if shouldTreatAsResponsesFormat(rawJSON) {
		rawJSON = sdkopenai.ConvertResponsesRequestToChatCompletions(modelName, rawJSON, stream)
		stream = gjson.GetBytes(rawJSON, "stream").Bool()
	}
	if stream {
		h.handleChatStreamingResponse(c, rawJSON)
		return
	}
	c.Header("Content-Type", "application/json")
	cliCtx, cliCancel := h.getContext(c, context.Background())
	resp, upstreamHeaders, errMsg := h.executeRequest(cliCtx, modelName, rawJSON, h.alt(c))
	if errMsg != nil {
		writeRuntimeErrorResponse(c, errMsg)
		cliCancel(errMsg.Error)
		return
	}
	handlers.WriteUpstreamHeaders(c.Writer.Header(), upstreamHeaders)
	_, _ = c.Writer.Write(resp)
	cliCancel(nil)
}

func (h *APIHandler) handleChatStreamingResponse(c *gin.Context, rawJSON []byte) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, handlers.ErrorResponse{
			Error: handlers.ErrorDetail{
				Message: "Streaming not supported",
				Type:    "server_error",
			},
		})
		return
	}

	modelName := gjson.GetBytes(rawJSON, "model").String()
	cliCtx, cliCancel := h.getContext(c, context.Background())
	dataChan, upstreamHeaders, errChan := h.executeStreamRequest(cliCtx, modelName, rawJSON, h.alt(c))

	setSSEHeaders := func() {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")
	}

	for {
		select {
		case <-c.Request.Context().Done():
			cliCancel(c.Request.Context().Err())
			return
		case errMsg, ok := <-errChan:
			if !ok {
				errChan = nil
				continue
			}
			writeRuntimeErrorResponse(c, errMsg)
			if errMsg != nil {
				cliCancel(errMsg.Error)
			} else {
				cliCancel(nil)
			}
			return
		case chunk, ok := <-dataChan:
			if !ok {
				setSSEHeaders()
				handlers.WriteUpstreamHeaders(c.Writer.Header(), upstreamHeaders)
				_, _ = fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
				flusher.Flush()
				cliCancel(nil)
				return
			}

			setSSEHeaders()
			handlers.WriteUpstreamHeaders(c.Writer.Header(), upstreamHeaders)
			_, _ = c.Writer.Write([]byte(formatStreamChunk(chunk)))
			flusher.Flush()

			keepAliveInterval := time.Duration(0)
			if h != nil && h.inner != nil {
				keepAliveInterval = handlers.StreamingKeepAliveInterval(h.inner.Cfg)
			}
			var keepAlive *time.Ticker
			var keepAliveC <-chan time.Time
			if keepAliveInterval > 0 {
				keepAlive = time.NewTicker(keepAliveInterval)
				defer keepAlive.Stop()
				keepAliveC = keepAlive.C
			}

			for {
				select {
				case <-c.Request.Context().Done():
					cliCancel(c.Request.Context().Err())
					return
				case chunk, ok := <-dataChan:
					if !ok {
						_, _ = fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
						flusher.Flush()
						cliCancel(nil)
						return
					}
					_, _ = c.Writer.Write([]byte(formatStreamChunk(chunk)))
					flusher.Flush()
				case errMsg, ok := <-errChan:
					if !ok {
						continue
					}
					if errMsg != nil {
						body := handlers.BuildErrorResponseBody(defaultStatus(errMsg.StatusCode), runtimeErrorText(errMsg))
						_, _ = c.Writer.Write([]byte(formatStreamChunk(body)))
						flusher.Flush()
						cliCancel(errMsg.Error)
						return
					}
					cliCancel(nil)
					return
				case <-keepAliveC:
					_, _ = c.Writer.Write([]byte(": keep-alive\n\n"))
					flusher.Flush()
				}
			}
		}
	}
}

func (h *APIHandler) handleChatNonStreamingResponseViaResponses(c *gin.Context, originalChatJSON []byte) {
	c.Header("Content-Type", "application/json")
	modelName := gjson.GetBytes(originalChatJSON, "model").String()
	responsesRequestJSON := sdkopenai.ConvertChatRequestToResponses(modelName, originalChatJSON, false)
	cliCtx, cliCancel := h.getContext(c, context.Background())
	resp, upstreamHeaders, errMsg := h.executeResponsesRequest(cliCtx, modelName, responsesRequestJSON, h.alt(c))
	if errMsg != nil {
		writeRuntimeErrorResponse(c, errMsg)
		cliCancel(errMsg.Error)
		return
	}
	handlers.WriteUpstreamHeaders(c.Writer.Header(), upstreamHeaders)
	converted := sdkopenai.ConvertResponsesObjectToChatCompletion(cliCtx, modelName, originalChatJSON, responsesRequestJSON, resp)
	if len(converted) == 0 {
		writeRuntimeErrorResponse(c, &runtimeError{StatusCode: http.StatusInternalServerError, Error: fmt.Errorf("failed to convert responses payload to chat completion")})
		cliCancel(fmt.Errorf("response conversion failed"))
		return
	}
	_, _ = c.Writer.Write(converted)
	cliCancel(nil)
}

func writeResponsesAsChatChunk(w io.Writer, ctx context.Context, modelName string, originalChatJSON, responsesRequestJSON, chunk []byte, param *any) {
	outputs := sdkopenai.ConvertResponsesChunkToChatCompletions(ctx, modelName, originalChatJSON, responsesRequestJSON, chunk, param)
	for _, out := range outputs {
		if out == "" {
			continue
		}
		_, _ = w.Write([]byte(formatStreamChunk([]byte(out))))
	}
}

func (h *APIHandler) handleChatStreamingResponseViaResponses(c *gin.Context, originalChatJSON []byte) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, handlers.ErrorResponse{
			Error: handlers.ErrorDetail{
				Message: "Streaming not supported",
				Type:    "server_error",
			},
		})
		return
	}

	modelName := gjson.GetBytes(originalChatJSON, "model").String()
	responsesRequestJSON := sdkopenai.ConvertChatRequestToResponses(modelName, originalChatJSON, true)
	cliCtx, cliCancel := h.getContext(c, context.Background())
	dataChan, upstreamHeaders, errChan := h.executeResponsesStreamRequest(cliCtx, modelName, responsesRequestJSON, h.alt(c))
	var param any

	setSSEHeaders := func() {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")
	}

	for {
		select {
		case <-c.Request.Context().Done():
			cliCancel(c.Request.Context().Err())
			return
		case errMsg, ok := <-errChan:
			if !ok {
				errChan = nil
				continue
			}
			writeRuntimeErrorResponse(c, errMsg)
			if errMsg != nil {
				cliCancel(errMsg.Error)
			} else {
				cliCancel(nil)
			}
			return
		case chunk, ok := <-dataChan:
			if !ok {
				setSSEHeaders()
				handlers.WriteUpstreamHeaders(c.Writer.Header(), upstreamHeaders)
				_, _ = fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
				flusher.Flush()
				cliCancel(nil)
				return
			}

			setSSEHeaders()
			handlers.WriteUpstreamHeaders(c.Writer.Header(), upstreamHeaders)
			writeResponsesAsChatChunk(c.Writer, cliCtx, modelName, originalChatJSON, responsesRequestJSON, chunk, &param)
			flusher.Flush()

			keepAliveInterval := time.Duration(0)
			if h != nil && h.inner != nil {
				keepAliveInterval = handlers.StreamingKeepAliveInterval(h.inner.Cfg)
			}
			var keepAlive *time.Ticker
			var keepAliveC <-chan time.Time
			if keepAliveInterval > 0 {
				keepAlive = time.NewTicker(keepAliveInterval)
				defer keepAlive.Stop()
				keepAliveC = keepAlive.C
			}

			for {
				select {
				case <-c.Request.Context().Done():
					cliCancel(c.Request.Context().Err())
					return
				case chunk, ok := <-dataChan:
					if !ok {
						_, _ = fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
						flusher.Flush()
						cliCancel(nil)
						return
					}
					writeResponsesAsChatChunk(c.Writer, cliCtx, modelName, originalChatJSON, responsesRequestJSON, chunk, &param)
					flusher.Flush()
				case errMsg, ok := <-errChan:
					if !ok {
						continue
					}
					if errMsg != nil {
						body := handlers.BuildErrorResponseBody(defaultStatus(errMsg.StatusCode), runtimeErrorText(errMsg))
						_, _ = c.Writer.Write([]byte(formatStreamChunk(body)))
						flusher.Flush()
						cliCancel(errMsg.Error)
						return
					}
					cliCancel(nil)
					return
				case <-keepAliveC:
					_, _ = c.Writer.Write([]byte(": keep-alive\n\n"))
					flusher.Flush()
				}
			}
		}
	}
}

func (h *APIHandler) getContext(c *gin.Context, parent context.Context) (context.Context, func(error)) {
	if h != nil && h.getContextWithCancel != nil {
		return h.getContextWithCancel(c, parent)
	}
	if parent == nil {
		parent = context.Background()
	}
	return parent, func(error) {}
}

func (h *APIHandler) alt(c *gin.Context) string {
	if h != nil && h.getAlt != nil {
		return h.getAlt(c)
	}
	if c == nil {
		return ""
	}
	alt, hasAlt := c.GetQuery("alt")
	if !hasAlt {
		alt, _ = c.GetQuery("$alt")
	}
	if alt == "sse" {
		return ""
	}
	return alt
}

func (h *APIHandler) resolveChatOverride(modelName string) (string, bool) {
	if h != nil && h.resolveEndpointOverride != nil {
		return h.resolveEndpointOverride(modelName, sdkopenai.OpenAIChatEndpoint)
	}
	return "", false
}

func (h *APIHandler) startKeepAlive(c *gin.Context, ctx context.Context) func() {
	if h != nil && h.startNonStreamingKeepAlive != nil {
		return h.startNonStreamingKeepAlive(c, ctx)
	}
	return func() {}
}

func (h *APIHandler) executeRequest(ctx context.Context, model string, rawJSON []byte, alt string) ([]byte, http.Header, *runtimeError) {
	if h != nil && h.execute != nil {
		return h.execute(ctx, model, rawJSON, alt)
	}
	return nil, nil, &runtimeError{StatusCode: http.StatusInternalServerError, Error: fmt.Errorf("execute runtime not configured")}
}

func (h *APIHandler) executeStreamRequest(ctx context.Context, model string, rawJSON []byte, alt string) (<-chan []byte, http.Header, <-chan *runtimeError) {
	if h != nil && h.streamExecute != nil {
		return h.streamExecute(ctx, model, rawJSON, alt)
	}
	errChan := make(chan *runtimeError, 1)
	errChan <- &runtimeError{StatusCode: http.StatusInternalServerError, Error: fmt.Errorf("stream runtime not configured")}
	close(errChan)
	return nil, nil, errChan
}

func (h *APIHandler) executeResponsesRequest(ctx context.Context, model string, rawJSON []byte, alt string) ([]byte, http.Header, *runtimeError) {
	if h != nil && h.responsesExecute != nil {
		return h.responsesExecute(ctx, model, rawJSON, alt)
	}
	return nil, nil, &runtimeError{StatusCode: http.StatusInternalServerError, Error: fmt.Errorf("responses execute runtime not configured")}
}

func (h *APIHandler) executeResponsesStreamRequest(ctx context.Context, model string, rawJSON []byte, alt string) (<-chan []byte, http.Header, <-chan *runtimeError) {
	if h != nil && h.responsesStreamExecute != nil {
		return h.responsesStreamExecute(ctx, model, rawJSON, alt)
	}
	errChan := make(chan *runtimeError, 1)
	errChan <- &runtimeError{StatusCode: http.StatusInternalServerError, Error: fmt.Errorf("responses stream runtime not configured")}
	close(errChan)
	return nil, nil, errChan
}
