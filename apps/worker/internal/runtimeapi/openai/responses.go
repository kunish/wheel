package openai

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/runtime/sdk/api/handlers"
	sdkopenai "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/api/handlers/openai"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func (h *ResponsesHandler) Responses(c *gin.Context) {
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
	modelName, ok := RequireOpenAIModel(c, rawJSON)
	if !ok {
		return
	}
	if overrideEndpoint, ok := h.resolveResponsesOverride(modelName); ok && overrideEndpoint == sdkopenai.OpenAIChatEndpoint {
		if stream {
			h.handleResponsesStreamingResponseViaChat(c, rawJSON)
			return
		}
		h.handleResponsesNonStreamingResponseViaChat(c, rawJSON)
		return
	}
	if stream {
		h.handleResponsesStreamingResponse(c, rawJSON)
		return
	}
	h.handleResponsesNonStreamingResponse(c, rawJSON)
}

func (h *ResponsesHandler) Compact(c *gin.Context) {
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
	rawJSON, status, err := NormalizeResponsesCompactRequest(rawJSON)
	if err != nil {
		c.JSON(status, handlers.ErrorResponse{
			Error: handlers.ErrorDetail{
				Message: err.Error(),
				Type:    "invalid_request_error",
			},
		})
		return
	}

	c.Header("Content-Type", "application/json")
	modelName := gjson.GetBytes(rawJSON, "model").String()
	cliCtx, cliCancel := h.getContext(c, context.Background())
	stopKeepAlive := h.startKeepAlive(c, cliCtx)
	resp, upstreamHeaders, errMsg := h.executeRequest(cliCtx, modelName, rawJSON, "responses/compact")
	stopKeepAlive()
	if errMsg != nil {
		writeRuntimeErrorResponse(c, errMsg)
		cliCancel(errMsg.Error)
		return
	}
	handlers.WriteUpstreamHeaders(c.Writer.Header(), upstreamHeaders)
	_, _ = c.Writer.Write(resp)
	cliCancel(nil)
}

func (h *ResponsesHandler) getContext(c *gin.Context, parent context.Context) (context.Context, func(error)) {
	if h != nil && h.getContextWithCancel != nil {
		return h.getContextWithCancel(c, parent)
	}
	if parent == nil {
		parent = context.Background()
	}
	return parent, func(error) {}
}

func (h *ResponsesHandler) startKeepAlive(c *gin.Context, ctx context.Context) func() {
	if h != nil && h.startNonStreamingKeepAlive != nil {
		return h.startNonStreamingKeepAlive(c, ctx)
	}
	return func() {}
}

func (h *ResponsesHandler) executeRequest(ctx context.Context, model string, rawJSON []byte, alt string) ([]byte, http.Header, *runtimeError) {
	if h != nil && h.execute != nil {
		return h.execute(ctx, model, rawJSON, alt)
	}
	return nil, nil, &runtimeError{StatusCode: http.StatusInternalServerError, Error: fmt.Errorf("execute runtime not configured")}
}

func (h *ResponsesHandler) executeStreamRequest(ctx context.Context, model string, rawJSON []byte, alt string) (<-chan []byte, http.Header, <-chan *runtimeError) {
	if h != nil && h.streamExecute != nil {
		return h.streamExecute(ctx, model, rawJSON, alt)
	}
	errChan := make(chan *runtimeError, 1)
	errChan <- &runtimeError{StatusCode: http.StatusInternalServerError, Error: fmt.Errorf("stream runtime not configured")}
	close(errChan)
	return nil, nil, errChan
}

func (h *ResponsesHandler) executeChatRequest(ctx context.Context, model string, rawJSON []byte, alt string) ([]byte, http.Header, *runtimeError) {
	if h != nil && h.chatExecute != nil {
		return h.chatExecute(ctx, model, rawJSON, alt)
	}
	return nil, nil, &runtimeError{StatusCode: http.StatusInternalServerError, Error: fmt.Errorf("chat execute runtime not configured")}
}

func (h *ResponsesHandler) executeChatStreamRequest(ctx context.Context, model string, rawJSON []byte, alt string) (<-chan []byte, http.Header, <-chan *runtimeError) {
	if h != nil && h.chatStreamExecute != nil {
		return h.chatStreamExecute(ctx, model, rawJSON, alt)
	}
	errChan := make(chan *runtimeError, 1)
	errChan <- &runtimeError{StatusCode: http.StatusInternalServerError, Error: fmt.Errorf("chat stream runtime not configured")}
	close(errChan)
	return nil, nil, errChan
}

func (h *ResponsesHandler) resolveResponsesOverride(modelName string) (string, bool) {
	if h != nil && h.resolveEndpointOverride != nil {
		return h.resolveEndpointOverride(modelName, sdkopenai.OpenAIResponsesEndpoint)
	}
	return "", false
}

func (h *ResponsesHandler) handleResponsesNonStreamingResponse(c *gin.Context, rawJSON []byte) {
	c.Header("Content-Type", "application/json")
	modelName := gjson.GetBytes(rawJSON, "model").String()
	cliCtx, cliCancel := h.getContext(c, context.Background())
	stopKeepAlive := h.startKeepAlive(c, cliCtx)
	resp, upstreamHeaders, errMsg := h.executeRequest(cliCtx, modelName, rawJSON, "")
	stopKeepAlive()
	if errMsg != nil {
		writeRuntimeErrorResponse(c, errMsg)
		cliCancel(errMsg.Error)
		return
	}
	handlers.WriteUpstreamHeaders(c.Writer.Header(), upstreamHeaders)
	_, _ = c.Writer.Write(resp)
	cliCancel(nil)
}

func (h *ResponsesHandler) handleResponsesNonStreamingResponseViaChat(c *gin.Context, originalResponsesJSON []byte) {
	c.Header("Content-Type", "application/json")
	modelName := gjson.GetBytes(originalResponsesJSON, "model").String()
	chatJSON := sdkopenai.ConvertResponsesRequestToChatCompletions(modelName, originalResponsesJSON, false)
	cliCtx, cliCancel := h.getContext(c, context.Background())
	resp, upstreamHeaders, errMsg := h.executeChatRequest(cliCtx, modelName, chatJSON, "")
	if errMsg != nil {
		writeRuntimeErrorResponse(c, errMsg)
		cliCancel(errMsg.Error)
		return
	}
	handlers.WriteUpstreamHeaders(c.Writer.Header(), upstreamHeaders)
	var param any
	converted := sdkopenai.ConvertChatCompletionsResponseToResponsesNonStream(cliCtx, modelName, originalResponsesJSON, originalResponsesJSON, resp, &param)
	if converted == "" {
		writeRuntimeErrorResponse(c, &runtimeError{StatusCode: http.StatusInternalServerError, Error: fmt.Errorf("failed to convert chat completion response to responses format")})
		cliCancel(fmt.Errorf("response conversion failed"))
		return
	}
	_, _ = c.Writer.Write([]byte(converted))
	cliCancel(nil)
}

func writeChatAsResponsesChunk(w io.Writer, ctx context.Context, modelName string, originalResponsesJSON, chunk []byte, param *any) {
	outputs := sdkopenai.ConvertChatCompletionsResponseToResponses(ctx, modelName, originalResponsesJSON, originalResponsesJSON, chunk, param)
	for _, out := range outputs {
		if out == "" {
			continue
		}
		if bytes.HasPrefix([]byte(out), []byte("event:")) {
			_, _ = w.Write([]byte("\n"))
		}
		_, _ = w.Write([]byte(out))
		_, _ = w.Write([]byte("\n"))
	}
}

func (h *ResponsesHandler) handleResponsesStreamingResponseViaChat(c *gin.Context, originalResponsesJSON []byte) {
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

	modelName := gjson.GetBytes(originalResponsesJSON, "model").String()
	chatJSON := sdkopenai.ConvertResponsesRequestToChatCompletions(modelName, originalResponsesJSON, true)
	cliCtx, cliCancel := h.getContext(c, context.Background())
	dataChan, upstreamHeaders, errChan := h.executeChatStreamRequest(cliCtx, modelName, chatJSON, "")
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
				_, _ = c.Writer.Write([]byte("\n"))
				flusher.Flush()
				cliCancel(nil)
				return
			}

			setSSEHeaders()
			handlers.WriteUpstreamHeaders(c.Writer.Header(), upstreamHeaders)
			writeChatAsResponsesChunk(c.Writer, cliCtx, modelName, originalResponsesJSON, chunk, &param)
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

			var terminalErr *runtimeError
			for {
				select {
				case <-c.Request.Context().Done():
					cliCancel(c.Request.Context().Err())
					return
				case chunk, ok := <-dataChan:
					if !ok {
						if terminalErr == nil {
							select {
							case errMsg, ok := <-errChan:
								if ok && errMsg != nil {
									terminalErr = errMsg
								}
							default:
							}
						}
						if terminalErr != nil {
							status := defaultStatus(terminalErr.StatusCode)
							errText := runtimeErrorText(terminalErr)
							body := handlers.BuildErrorResponseBody(status, errText)
							_, _ = fmt.Fprintf(c.Writer, "\nevent: error\ndata: %s\n\n", string(body))
							flusher.Flush()
							cliCancel(terminalErr.Error)
							return
						}
						_, _ = c.Writer.Write([]byte("\n"))
						flusher.Flush()
						cliCancel(nil)
						return
					}
					writeChatAsResponsesChunk(c.Writer, cliCtx, modelName, originalResponsesJSON, chunk, &param)
					flusher.Flush()
				case errMsg, ok := <-errChan:
					if !ok {
						continue
					}
					if errMsg != nil {
						status := defaultStatus(errMsg.StatusCode)
						errText := runtimeErrorText(errMsg)
						body := handlers.BuildErrorResponseBody(status, errText)
						_, _ = fmt.Fprintf(c.Writer, "\nevent: error\ndata: %s\n\n", string(body))
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

func (h *ResponsesHandler) handleResponsesStreamingResponse(c *gin.Context, rawJSON []byte) {
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
	dataChan, upstreamHeaders, errChan := h.executeStreamRequest(cliCtx, modelName, rawJSON, "")

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
				_, _ = c.Writer.Write([]byte("\n"))
				flusher.Flush()
				cliCancel(nil)
				return
			}

			setSSEHeaders()
			handlers.WriteUpstreamHeaders(c.Writer.Header(), upstreamHeaders)
			if bytes.HasPrefix(chunk, []byte("event:")) {
				_, _ = c.Writer.Write([]byte("\n"))
			}
			_, _ = c.Writer.Write(chunk)
			_, _ = c.Writer.Write([]byte("\n"))
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

			var terminalErr *runtimeError
			for {
				select {
				case <-c.Request.Context().Done():
					cliCancel(c.Request.Context().Err())
					return
				case chunk, ok := <-dataChan:
					if !ok {
						if terminalErr == nil {
							select {
							case errMsg, ok := <-errChan:
								if ok && errMsg != nil {
									terminalErr = errMsg
								}
							default:
							}
						}
						if terminalErr != nil {
							status := defaultStatus(terminalErr.StatusCode)
							errText := runtimeErrorText(terminalErr)
							chunk := handlers.BuildOpenAIResponsesStreamErrorChunk(status, errText, 0)
							_, _ = fmt.Fprintf(c.Writer, "\nevent: error\ndata: %s\n\n", string(chunk))
							flusher.Flush()
							cliCancel(terminalErr.Error)
							return
						}
						_, _ = c.Writer.Write([]byte("\n"))
						flusher.Flush()
						cliCancel(nil)
						return
					}
					if bytes.HasPrefix(chunk, []byte("event:")) {
						_, _ = c.Writer.Write([]byte("\n"))
					}
					_, _ = c.Writer.Write(chunk)
					_, _ = c.Writer.Write([]byte("\n"))
					flusher.Flush()
				case errMsg, ok := <-errChan:
					if !ok {
						continue
					}
					if errMsg != nil {
						chunk := handlers.BuildOpenAIResponsesStreamErrorChunk(defaultStatus(errMsg.StatusCode), runtimeErrorText(errMsg), 0)
						_, _ = fmt.Fprintf(c.Writer, "\nevent: error\ndata: %s\n\n", string(chunk))
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

func NormalizeResponsesCompactRequest(rawJSON []byte) ([]byte, int, error) {
	streamResult := gjson.GetBytes(rawJSON, "stream")
	if streamResult.Type == gjson.True {
		return nil, http.StatusBadRequest, fmt.Errorf("Streaming not supported for compact responses")
	}
	if streamResult.Exists() {
		if updated, err := sjson.DeleteBytes(rawJSON, "stream"); err == nil {
			rawJSON = updated
		}
	}
	return rawJSON, 0, nil
}
