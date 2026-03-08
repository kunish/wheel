package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/kunish/wheel/apps/worker/internal/runtime/sdk/api/handlers"
	sdkopenai "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/api/handlers/openai"
	cliproxyexecutor "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy/executor"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const wsPayloadLogMaxSize = 2048

const (
	wsRequestTypeCreate  = "response.create"
	wsRequestTypeAppend  = "response.append"
	wsEventTypeError     = "error"
	wsEventTypeCompleted = "response.completed"
	wsDoneMarker         = "[DONE]"
	wsTurnStateHeader    = "x-codex-turn-state"
)

var responsesWebsocketUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

const WebsocketRequestBodyKey = "REQUEST_BODY_OVERRIDE"

const APIResponseTimestampKey = "API_RESPONSE_TIMESTAMP"

type APIHandler struct {
	inner                      *sdkopenai.OpenAIAPIHandler
	models                     func() []map[string]any
	handlerType                func() string
	getAlt                     func(*gin.Context) string
	getContextWithCancel       func(*gin.Context, context.Context) (context.Context, func(error))
	startNonStreamingKeepAlive func(*gin.Context, context.Context) func()
	execute                    func(context.Context, string, []byte, string) ([]byte, http.Header, *runtimeError)
	streamExecute              func(context.Context, string, []byte, string) (<-chan []byte, http.Header, <-chan *runtimeError)
	responsesExecute           func(context.Context, string, []byte, string) ([]byte, http.Header, *runtimeError)
	responsesStreamExecute     func(context.Context, string, []byte, string) (<-chan []byte, http.Header, <-chan *runtimeError)
	resolveEndpointOverride    func(string, string) (string, bool)
}

type ResponsesHandler struct {
	inner                                     *sdkopenai.OpenAIResponsesAPIHandler
	handlerType                               func() string
	getContextWithCancel                      func(*gin.Context, context.Context) (context.Context, func(error))
	startNonStreamingKeepAlive                func(*gin.Context, context.Context) func()
	execute                                   func(context.Context, string, []byte, string) ([]byte, http.Header, *runtimeError)
	streamExecute                             func(context.Context, string, []byte, string) (<-chan []byte, http.Header, <-chan *runtimeError)
	chatExecute                               func(context.Context, string, []byte, string) ([]byte, http.Header, *runtimeError)
	chatStreamExecute                         func(context.Context, string, []byte, string) (<-chan []byte, http.Header, <-chan *runtimeError)
	resolveEndpointOverride                   func(string, string) (string, bool)
	websocketSupportsIncrementalInputForModel func(string) bool
	newWebsocketSessionID                     func() string
	withDownstreamWebsocketContext            func(context.Context) context.Context
	withExecutionSessionID                    func(context.Context, string) context.Context
	withPinnedAuthID                          func(context.Context, string) context.Context
	withSelectedAuthIDCallback                func(context.Context, func(string)) context.Context
	closeExecutionSession                     func(string)
}

type runtimeError struct {
	StatusCode int
	Error      error
	Addon      http.Header
}

func NewAPIHandler(base *handlers.BaseAPIHandler) *APIHandler {
	inner := sdkopenai.NewOpenAIAPIHandler(base)
	return &APIHandler{
		inner: inner,
		models: func() []map[string]any {
			return inner.Models()
		},
		handlerType: func() string {
			return inner.HandlerType()
		},
		getAlt: func(c *gin.Context) string {
			return inner.GetAlt(c)
		},
		getContextWithCancel: func(c *gin.Context, parent context.Context) (context.Context, func(error)) {
			ctx, cancel := inner.GetContextWithCancel(inner, c, parent)
			return ctx, func(err error) { cancel(err) }
		},
		startNonStreamingKeepAlive: func(c *gin.Context, ctx context.Context) func() {
			return inner.StartNonStreamingKeepAlive(c, ctx)
		},
		execute: func(ctx context.Context, model string, rawJSON []byte, alt string) ([]byte, http.Header, *runtimeError) {
			resp, headers, errMsg := inner.ExecuteWithAuthManager(ctx, inner.HandlerType(), model, rawJSON, alt)
			if errMsg == nil {
				return resp, headers, nil
			}
			var addon http.Header
			if errMsg.Addon != nil {
				addon = errMsg.Addon.Clone()
			}
			return nil, nil, &runtimeError{StatusCode: errMsg.StatusCode, Error: errMsg.Error, Addon: addon}
		},
		streamExecute: func(ctx context.Context, model string, rawJSON []byte, alt string) (<-chan []byte, http.Header, <-chan *runtimeError) {
			data, headers, errs := inner.ExecuteStreamWithAuthManager(ctx, inner.HandlerType(), model, rawJSON, alt)
			convertedErrs := make(chan *runtimeError, 1)
			if errs == nil {
				close(convertedErrs)
				return data, headers, convertedErrs
			}
			go func() {
				defer close(convertedErrs)
				for errMsg := range errs {
					if errMsg == nil {
						convertedErrs <- nil
						continue
					}
					var addon http.Header
					if errMsg.Addon != nil {
						addon = errMsg.Addon.Clone()
					}
					convertedErrs <- &runtimeError{StatusCode: errMsg.StatusCode, Error: errMsg.Error, Addon: addon}
				}
			}()
			return data, headers, convertedErrs
		},
		responsesExecute: func(ctx context.Context, model string, rawJSON []byte, alt string) ([]byte, http.Header, *runtimeError) {
			resp, headers, errMsg := inner.ExecuteWithAuthManager(ctx, sdkopenai.OpenAIResponsesHandlerType, model, rawJSON, alt)
			if errMsg == nil {
				return resp, headers, nil
			}
			var addon http.Header
			if errMsg.Addon != nil {
				addon = errMsg.Addon.Clone()
			}
			return nil, nil, &runtimeError{StatusCode: errMsg.StatusCode, Error: errMsg.Error, Addon: addon}
		},
		responsesStreamExecute: func(ctx context.Context, model string, rawJSON []byte, alt string) (<-chan []byte, http.Header, <-chan *runtimeError) {
			data, headers, errs := inner.ExecuteStreamWithAuthManager(ctx, sdkopenai.OpenAIResponsesHandlerType, model, rawJSON, alt)
			convertedErrs := make(chan *runtimeError, 1)
			if errs == nil {
				close(convertedErrs)
				return data, headers, convertedErrs
			}
			go func() {
				defer close(convertedErrs)
				for errMsg := range errs {
					if errMsg == nil {
						convertedErrs <- nil
						continue
					}
					var addon http.Header
					if errMsg.Addon != nil {
						addon = errMsg.Addon.Clone()
					}
					convertedErrs <- &runtimeError{StatusCode: errMsg.StatusCode, Error: errMsg.Error, Addon: addon}
				}
			}()
			return data, headers, convertedErrs
		},
		resolveEndpointOverride: sdkopenai.ResolveEndpointOverride,
	}
}

func NewResponsesHandler(base *handlers.BaseAPIHandler) *ResponsesHandler {
	inner := sdkopenai.NewOpenAIResponsesAPIHandler(base)
	return &ResponsesHandler{
		inner: inner,
		handlerType: func() string {
			return inner.HandlerType()
		},
		getContextWithCancel: func(c *gin.Context, parent context.Context) (context.Context, func(error)) {
			ctx, cancel := inner.GetContextWithCancel(inner, c, parent)
			return ctx, func(err error) { cancel(err) }
		},
		startNonStreamingKeepAlive: func(c *gin.Context, ctx context.Context) func() {
			return inner.StartNonStreamingKeepAlive(c, ctx)
		},
		execute: func(ctx context.Context, model string, rawJSON []byte, alt string) ([]byte, http.Header, *runtimeError) {
			resp, headers, errMsg := inner.ExecuteWithAuthManager(ctx, inner.HandlerType(), model, rawJSON, alt)
			if errMsg == nil {
				return resp, headers, nil
			}
			var addon http.Header
			if errMsg.Addon != nil {
				addon = errMsg.Addon.Clone()
			}
			return nil, nil, &runtimeError{StatusCode: errMsg.StatusCode, Error: errMsg.Error, Addon: addon}
		},
		streamExecute: func(ctx context.Context, model string, rawJSON []byte, alt string) (<-chan []byte, http.Header, <-chan *runtimeError) {
			data, headers, errs := inner.ExecuteStreamWithAuthManager(ctx, inner.HandlerType(), model, rawJSON, alt)
			convertedErrs := make(chan *runtimeError, 1)
			if errs == nil {
				close(convertedErrs)
				return data, headers, convertedErrs
			}
			go func() {
				defer close(convertedErrs)
				for errMsg := range errs {
					if errMsg == nil {
						convertedErrs <- nil
						continue
					}
					var addon http.Header
					if errMsg.Addon != nil {
						addon = errMsg.Addon.Clone()
					}
					convertedErrs <- &runtimeError{StatusCode: errMsg.StatusCode, Error: errMsg.Error, Addon: addon}
				}
			}()
			return data, headers, convertedErrs
		},
		chatExecute: func(ctx context.Context, model string, rawJSON []byte, alt string) ([]byte, http.Header, *runtimeError) {
			resp, headers, errMsg := inner.ExecuteWithAuthManager(ctx, "openai", model, rawJSON, alt)
			if errMsg == nil {
				return resp, headers, nil
			}
			var addon http.Header
			if errMsg.Addon != nil {
				addon = errMsg.Addon.Clone()
			}
			return nil, nil, &runtimeError{StatusCode: errMsg.StatusCode, Error: errMsg.Error, Addon: addon}
		},
		chatStreamExecute: func(ctx context.Context, model string, rawJSON []byte, alt string) (<-chan []byte, http.Header, <-chan *runtimeError) {
			data, headers, errs := inner.ExecuteStreamWithAuthManager(ctx, "openai", model, rawJSON, alt)
			convertedErrs := make(chan *runtimeError, 1)
			if errs == nil {
				close(convertedErrs)
				return data, headers, convertedErrs
			}
			go func() {
				defer close(convertedErrs)
				for errMsg := range errs {
					if errMsg == nil {
						convertedErrs <- nil
						continue
					}
					var addon http.Header
					if errMsg.Addon != nil {
						addon = errMsg.Addon.Clone()
					}
					convertedErrs <- &runtimeError{StatusCode: errMsg.StatusCode, Error: errMsg.Error, Addon: addon}
				}
			}()
			return data, headers, convertedErrs
		},
		resolveEndpointOverride: sdkopenai.ResolveEndpointOverride,
		websocketSupportsIncrementalInputForModel: func(modelName string) bool {
			return sdkopenai.WebsocketUpstreamSupportsIncrementalInputForModel(inner, modelName)
		},
		newWebsocketSessionID:          uuid.NewString,
		withDownstreamWebsocketContext: cliproxyexecutor.WithDownstreamWebsocket,
		withExecutionSessionID:         handlers.WithExecutionSessionID,
		withPinnedAuthID:               handlers.WithPinnedAuthID,
		withSelectedAuthIDCallback:     handlers.WithSelectedAuthIDCallback,
		closeExecutionSession: func(sessionID string) {
			if inner != nil && inner.AuthManager != nil {
				inner.AuthManager.CloseExecutionSession(sessionID)
			}
		},
	}
}

func (h *APIHandler) OpenAIModels(c *gin.Context) {
	models := []map[string]any(nil)
	if h != nil && h.models != nil {
		models = h.models()
	} else if h != nil && h.inner != nil {
		models = h.inner.Models()
	}
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   FilterOpenAIModels(models),
	})
}

func FilterOpenAIModels(allModels []map[string]any) []map[string]any {
	filteredModels := make([]map[string]any, len(allModels))
	for i, model := range allModels {
		filteredModel := map[string]any{
			"id":     model["id"],
			"object": model["object"],
		}
		if created, exists := model["created"]; exists {
			filteredModel["created"] = created
		}
		if ownedBy, exists := model["owned_by"]; exists {
			filteredModel["owned_by"] = ownedBy
		}
		filteredModels[i] = filteredModel
	}
	return filteredModels
}

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
	modelName, ok := RequireOpenAIModel(c, rawJSON)
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
	if ShouldTreatAsResponsesFormat(rawJSON) {
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

func (h *APIHandler) Completions(c *gin.Context) {
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
	if _, ok := RequireOpenAIModel(c, rawJSON); !ok {
		return
	}
	if gjson.GetBytes(rawJSON, "stream").Type == gjson.True {
		h.handleCompletionsStreamingResponse(c, rawJSON)
		return
	}

	c.Header("Content-Type", "application/json")
	chatCompletionsJSON := ConvertCompletionsRequestToChatCompletions(rawJSON)
	modelName := gjson.GetBytes(chatCompletionsJSON, "model").String()
	cliCtx, cliCancel := h.getContext(c, context.Background())
	stopKeepAlive := h.startKeepAlive(c, cliCtx)
	resp, upstreamHeaders, errMsg := h.executeRequest(cliCtx, modelName, chatCompletionsJSON, "")
	stopKeepAlive()
	if errMsg != nil {
		writeRuntimeErrorResponse(c, errMsg)
		cliCancel(errMsg.Error)
		return
	}
	handlers.WriteUpstreamHeaders(c.Writer.Header(), upstreamHeaders)
	completionsResp := ConvertChatCompletionsResponseToCompletions(resp)
	_, _ = c.Writer.Write(completionsResp)
	cliCancel(nil)
}

func FormatCompletionsStreamChunk(chunk []byte) []byte {
	converted := ConvertChatCompletionsStreamChunkToCompletions(chunk)
	if converted == nil {
		return nil
	}
	return []byte(FormatStreamChunk(converted))
}

func (h *APIHandler) handleCompletionsStreamingResponse(c *gin.Context, rawJSON []byte) {
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

	chatCompletionsJSON := ConvertCompletionsRequestToChatCompletions(rawJSON)
	modelName := gjson.GetBytes(chatCompletionsJSON, "model").String()
	cliCtx, cliCancel := h.getContext(c, context.Background())
	dataChan, upstreamHeaders, errChan := h.executeStreamRequest(cliCtx, modelName, chatCompletionsJSON, "")

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
			if formatted := FormatCompletionsStreamChunk(chunk); formatted != nil {
				_, _ = c.Writer.Write(formatted)
				flusher.Flush()
			}

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

			var terminalErr any
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
							status := http.StatusInternalServerError
							errText := http.StatusText(status)
							switch msg := terminalErr.(type) {
							case interface{ GetStatusCode() int }:
								if code := msg.GetStatusCode(); code > 0 {
									status = code
									errText = http.StatusText(code)
								}
							case interface{ StatusCode() int }:
								if code := msg.StatusCode(); code > 0 {
									status = code
									errText = http.StatusText(code)
								}
							}
							if msg, ok := terminalErr.(interface{ GetError() error }); ok && msg.GetError() != nil && msg.GetError().Error() != "" {
								errText = msg.GetError().Error()
							}
							body := handlers.BuildErrorResponseBody(status, errText)
							_, _ = c.Writer.Write([]byte(FormatStreamChunk(body)))
							flusher.Flush()
							if msg, ok := terminalErr.(interface{ GetError() error }); ok {
								cliCancel(msg.GetError())
							} else {
								cliCancel(nil)
							}
							return
						}
						_, _ = fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
						flusher.Flush()
						cliCancel(nil)
						return
					}
					if formatted := FormatCompletionsStreamChunk(chunk); formatted != nil {
						_, _ = c.Writer.Write(formatted)
						flusher.Flush()
					}
				case errMsg, ok := <-errChan:
					if !ok {
						continue
					}
					if errMsg != nil {
						status := http.StatusInternalServerError
						if errMsg.StatusCode > 0 {
							status = errMsg.StatusCode
						}
						errText := http.StatusText(status)
						if errMsg.Error != nil && errMsg.Error.Error() != "" {
							errText = errMsg.Error.Error()
						}
						body := handlers.BuildErrorResponseBody(status, errText)
						_, _ = c.Writer.Write([]byte(FormatStreamChunk(body)))
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
			return
		}
	}
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
			_, _ = c.Writer.Write([]byte(FormatStreamChunk(chunk)))
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
					_, _ = c.Writer.Write([]byte(FormatStreamChunk(chunk)))
					flusher.Flush()
				case errMsg, ok := <-errChan:
					if !ok {
						continue
					}
					if errMsg != nil {
						body := handlers.BuildErrorResponseBody(defaultStatus(errMsg.StatusCode), runtimeErrorText(errMsg))
						_, _ = c.Writer.Write([]byte(FormatStreamChunk(body)))
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
		_, _ = w.Write([]byte(FormatStreamChunk([]byte(out))))
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
						_, _ = c.Writer.Write([]byte(FormatStreamChunk(body)))
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

func writeRuntimeErrorResponse(c *gin.Context, msg *runtimeError) {
	status := defaultStatus(0)
	if msg != nil {
		status = defaultStatus(msg.StatusCode)
	}
	if c != nil && msg != nil && msg.Addon != nil {
		for key, values := range msg.Addon {
			c.Writer.Header().Del(key)
			for _, value := range values {
				c.Writer.Header().Add(key, value)
			}
		}
	}
	errText := runtimeErrorText(msg)
	body := handlers.BuildErrorResponseBody(status, errText)
	if c != nil {
		c.Data(status, "application/json", body)
	}
}

func defaultStatus(code int) int {
	if code > 0 {
		return code
	}
	return http.StatusInternalServerError
}

func runtimeErrorText(msg *runtimeError) string {
	status := defaultStatus(0)
	if msg != nil {
		status = defaultStatus(msg.StatusCode)
	}
	errText := http.StatusText(status)
	if msg != nil && msg.Error != nil && strings.TrimSpace(msg.Error.Error()) != "" {
		errText = msg.Error.Error()
	}
	return errText
}

func (h *ResponsesHandler) ResponsesWebsocket(c *gin.Context) {
	conn, err := responsesWebsocketUpgrader.Upgrade(c.Writer, c.Request, websocketUpgradeHeaders(c.Request))
	if err != nil {
		return
	}
	sessionID := h.websocketSessionID()
	var wsBodyLog strings.Builder
	defer func() {
		h.closeWebsocketSession(sessionID)
		SetWebsocketRequestBody(c, wsBodyLog.String())
		_ = conn.Close()
	}()

	var lastRequest []byte
	lastResponseOutput := []byte("[]")
	pinnedAuthID := ""

	for {
		msgType, payload, errReadMessage := conn.ReadMessage()
		if errReadMessage != nil {
			AppendWebsocketEvent(&wsBodyLog, "disconnect", []byte(errReadMessage.Error()))
			return
		}
		if msgType != websocket.TextMessage && msgType != websocket.BinaryMessage {
			continue
		}
		AppendWebsocketEvent(&wsBodyLog, "request", payload)

		requestModelName := strings.TrimSpace(gjson.GetBytes(payload, "model").String())
		if requestModelName == "" {
			requestModelName = strings.TrimSpace(gjson.GetBytes(lastRequest, "model").String())
		}
		allowIncrementalInputWithPreviousResponseID := h.websocketAllowsIncrementalInput(pinnedAuthID, requestModelName)

		requestJSON, updatedLastRequest, status, err := NormalizeResponsesWebsocketRequest(payload, lastRequest, lastResponseOutput, allowIncrementalInputWithPreviousResponseID)
		if err != nil {
			MarkAPIResponseTimestamp(c)
			errorPayload, errWrite := writeResponsesWebsocketError(conn, &runtimeError{StatusCode: status, Error: err})
			AppendWebsocketEvent(&wsBodyLog, "response", errorPayload)
			if errWrite != nil {
				return
			}
			continue
		}

		if shouldHandleResponsesWebsocketPrewarmLocally(payload, lastRequest, allowIncrementalInputWithPreviousResponseID) {
			if updated, errDelete := sjson.DeleteBytes(requestJSON, "generate"); errDelete == nil {
				requestJSON = updated
			}
			if updated, errDelete := sjson.DeleteBytes(updatedLastRequest, "generate"); errDelete == nil {
				updatedLastRequest = updated
			}
			lastRequest = updatedLastRequest
			lastResponseOutput = []byte("[]")
			if errWrite := writeResponsesWebsocketSyntheticPrewarm(c, conn, requestJSON, &wsBodyLog); errWrite != nil {
				AppendWebsocketEvent(&wsBodyLog, "disconnect", []byte(errWrite.Error()))
				return
			}
			continue
		}

		lastRequest = updatedLastRequest
		modelName := gjson.GetBytes(requestJSON, "model").String()
		cliCtx, cliCancel := h.getContext(c, context.Background())
		cliCtx = h.withDownstreamWebsocket(cliCtx)
		cliCtx = h.withExecutionSession(cliCtx, sessionID)
		if pinnedAuthID != "" {
			cliCtx = h.withPinnedAuth(cliCtx, pinnedAuthID)
		} else {
			cliCtx = h.withSelectedAuthCallback(cliCtx, func(authID string) {
				pinnedAuthID = strings.TrimSpace(authID)
			})
		}
		dataChan, _, errChan := h.executeStreamRequest(cliCtx, modelName, requestJSON, "")
		completedOutput, errForward := h.forwardResponsesWebsocket(c, conn, cliCancel, dataChan, errChan, &wsBodyLog)
		if errForward != nil {
			AppendWebsocketEvent(&wsBodyLog, "disconnect", []byte(errForward.Error()))
			return
		}
		lastResponseOutput = completedOutput
	}
}

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

func websocketUpgradeHeaders(req *http.Request) http.Header {
	headers := http.Header{}
	if req == nil {
		return headers
	}
	turnState := strings.TrimSpace(req.Header.Get(wsTurnStateHeader))
	if turnState != "" {
		headers.Set(wsTurnStateHeader, turnState)
	}
	return headers
}

func (h *ResponsesHandler) websocketSessionID() string {
	if h != nil && h.newWebsocketSessionID != nil {
		return h.newWebsocketSessionID()
	}
	return uuid.NewString()
}

func (h *ResponsesHandler) withDownstreamWebsocket(ctx context.Context) context.Context {
	if h != nil && h.withDownstreamWebsocketContext != nil {
		return h.withDownstreamWebsocketContext(ctx)
	}
	return ctx
}

func (h *ResponsesHandler) withExecutionSession(ctx context.Context, sessionID string) context.Context {
	if h != nil && h.withExecutionSessionID != nil {
		return h.withExecutionSessionID(ctx, sessionID)
	}
	return ctx
}

func (h *ResponsesHandler) withPinnedAuth(ctx context.Context, authID string) context.Context {
	if h != nil && h.withPinnedAuthID != nil {
		return h.withPinnedAuthID(ctx, authID)
	}
	return ctx
}

func (h *ResponsesHandler) withSelectedAuthCallback(ctx context.Context, callback func(string)) context.Context {
	if h != nil && h.withSelectedAuthIDCallback != nil {
		return h.withSelectedAuthIDCallback(ctx, callback)
	}
	return ctx
}

func (h *ResponsesHandler) closeWebsocketSession(sessionID string) {
	if h != nil && h.closeExecutionSession != nil {
		h.closeExecutionSession(sessionID)
	}
}

func (h *ResponsesHandler) websocketAllowsIncrementalInput(pinnedAuthID string, requestModelName string) bool {
	pinnedAuthID = strings.TrimSpace(pinnedAuthID)
	if pinnedAuthID != "" && h != nil && h.inner != nil && h.inner.AuthManager != nil {
		if auth, ok := h.inner.AuthManager.GetByID(pinnedAuthID); ok && auth != nil {
			return WebsocketUpstreamSupportsIncrementalInput(auth.Attributes, auth.Metadata)
		}
	}
	if h != nil && h.websocketSupportsIncrementalInputForModel != nil {
		return h.websocketSupportsIncrementalInputForModel(requestModelName)
	}
	return false
}

func WebsocketUpstreamSupportsIncrementalInput(attributes map[string]string, metadata map[string]any) bool {
	if len(attributes) > 0 {
		if raw := strings.TrimSpace(attributes["websockets"]); raw != "" {
			parsed, err := strconv.ParseBool(raw)
			if err == nil {
				return parsed
			}
		}
	}
	if len(metadata) == 0 {
		return false
	}
	raw, ok := metadata["websockets"]
	if !ok || raw == nil {
		return false
	}
	switch value := raw.(type) {
	case bool:
		return value
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(value))
		if err == nil {
			return parsed
		}
	}
	return false
}

func shouldHandleResponsesWebsocketPrewarmLocally(rawJSON []byte, lastRequest []byte, allowIncrementalInputWithPreviousResponseID bool) bool {
	if allowIncrementalInputWithPreviousResponseID || len(lastRequest) != 0 {
		return false
	}
	if strings.TrimSpace(gjson.GetBytes(rawJSON, "type").String()) != wsRequestTypeCreate {
		return false
	}
	generateResult := gjson.GetBytes(rawJSON, "generate")
	return generateResult.Exists() && !generateResult.Bool()
}

func writeResponsesWebsocketSyntheticPrewarm(c *gin.Context, conn *websocket.Conn, requestJSON []byte, wsBodyLog *strings.Builder) error {
	payloads, err := syntheticResponsesWebsocketPrewarmPayloads(requestJSON)
	if err != nil {
		return err
	}
	for i := 0; i < len(payloads); i++ {
		MarkAPIResponseTimestamp(c)
		AppendWebsocketEvent(wsBodyLog, "response", payloads[i])
		if errWrite := conn.WriteMessage(websocket.TextMessage, payloads[i]); errWrite != nil {
			return errWrite
		}
	}
	return nil
}

func syntheticResponsesWebsocketPrewarmPayloads(requestJSON []byte) ([][]byte, error) {
	responseID := "resp_prewarm_" + uuid.NewString()
	createdAt := time.Now().Unix()
	modelName := strings.TrimSpace(gjson.GetBytes(requestJSON, "model").String())

	createdPayload := []byte(`{"type":"response.created","sequence_number":0,"response":{"id":"","object":"response","created_at":0,"status":"in_progress","background":false,"error":null,"output":[]}}`)
	var err error
	createdPayload, err = sjson.SetBytes(createdPayload, "response.id", responseID)
	if err != nil {
		return nil, err
	}
	createdPayload, err = sjson.SetBytes(createdPayload, "response.created_at", createdAt)
	if err != nil {
		return nil, err
	}
	if modelName != "" {
		createdPayload, err = sjson.SetBytes(createdPayload, "response.model", modelName)
		if err != nil {
			return nil, err
		}
	}

	completedPayload := []byte(`{"type":"response.completed","sequence_number":1,"response":{"id":"","object":"response","created_at":0,"status":"completed","background":false,"error":null,"output":[],"usage":{"input_tokens":0,"output_tokens":0,"total_tokens":0}}}`)
	completedPayload, err = sjson.SetBytes(completedPayload, "response.id", responseID)
	if err != nil {
		return nil, err
	}
	completedPayload, err = sjson.SetBytes(completedPayload, "response.created_at", createdAt)
	if err != nil {
		return nil, err
	}
	if modelName != "" {
		completedPayload, err = sjson.SetBytes(completedPayload, "response.model", modelName)
		if err != nil {
			return nil, err
		}
	}

	return [][]byte{createdPayload, completedPayload}, nil
}

func (h *ResponsesHandler) forwardResponsesWebsocket(c *gin.Context, conn *websocket.Conn, cancel func(error), data <-chan []byte, errs <-chan *runtimeError, wsBodyLog *strings.Builder) ([]byte, error) {
	completed := false
	completedOutput := []byte("[]")

	for {
		select {
		case <-c.Request.Context().Done():
			cancel(c.Request.Context().Err())
			return completedOutput, c.Request.Context().Err()
		case errMsg, ok := <-errs:
			if !ok {
				errs = nil
				continue
			}
			if errMsg != nil {
				MarkAPIResponseTimestamp(c)
				errorPayload, errWrite := writeResponsesWebsocketError(conn, errMsg)
				AppendWebsocketEvent(wsBodyLog, "response", errorPayload)
				if errWrite != nil {
					cancel(errMsg.Error)
					return completedOutput, errWrite
				}
			}
			if errMsg != nil {
				cancel(errMsg.Error)
			} else {
				cancel(nil)
			}
			return completedOutput, nil
		case chunk, ok := <-data:
			if !ok {
				if !completed {
					errMsg := &runtimeError{StatusCode: http.StatusRequestTimeout, Error: fmt.Errorf("stream closed before response.completed")}
					MarkAPIResponseTimestamp(c)
					errorPayload, errWrite := writeResponsesWebsocketError(conn, errMsg)
					AppendWebsocketEvent(wsBodyLog, "response", errorPayload)
					if errWrite != nil {
						cancel(errMsg.Error)
						return completedOutput, errWrite
					}
					cancel(errMsg.Error)
					return completedOutput, nil
				}
				cancel(nil)
				return completedOutput, nil
			}

			payloads := WebsocketJSONPayloadsFromChunk(chunk)
			for i := range payloads {
				if WebsocketPayloadEventType(payloads[i]) == wsEventTypeCompleted {
					completed = true
					completedOutput = ResponseCompletedOutputFromPayload(payloads[i])
				}
				MarkAPIResponseTimestamp(c)
				AppendWebsocketEvent(wsBodyLog, "response", payloads[i])
				if errWrite := conn.WriteMessage(websocket.TextMessage, payloads[i]); errWrite != nil {
					cancel(errWrite)
					return completedOutput, errWrite
				}
			}
		}
	}
}

func writeResponsesWebsocketError(conn *websocket.Conn, errMsg *runtimeError) ([]byte, error) {
	status := defaultStatus(0)
	if errMsg != nil {
		status = defaultStatus(errMsg.StatusCode)
	}
	errText := runtimeErrorText(errMsg)
	var addon map[string][]string
	if errMsg != nil && errMsg.Addon != nil {
		addon = map[string][]string(errMsg.Addon.Clone())
	}
	payload, err := BuildResponsesWebsocketErrorPayload(status, errText, addon)
	if err != nil {
		return nil, err
	}
	return payload, conn.WriteMessage(websocket.TextMessage, payload)
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

func RequireOpenAIModel(c *gin.Context, rawJSON []byte) (string, bool) {
	modelName := gjson.GetBytes(rawJSON, "model").String()
	if modelName == "" {
		c.JSON(http.StatusBadRequest, handlers.ErrorResponse{
			Error: handlers.ErrorDetail{
				Message: "Model is required",
				Type:    "invalid_request_error",
			},
		})
		return "", false
	}
	return modelName, true
}

func ConvertCompletionsRequestToChatCompletions(rawJSON []byte) []byte {
	root := gjson.ParseBytes(rawJSON)
	prompt := root.Get("prompt").String()
	if prompt == "" {
		prompt = "Complete this:"
	}
	out := `{"model":"","messages":[{"role":"user","content":""}]}`
	if model := root.Get("model"); model.Exists() {
		out, _ = sjson.Set(out, "model", model.String())
	}
	out, _ = sjson.Set(out, "messages.0.content", prompt)
	if maxTokens := root.Get("max_tokens"); maxTokens.Exists() {
		out, _ = sjson.Set(out, "max_tokens", maxTokens.Int())
	}
	if temperature := root.Get("temperature"); temperature.Exists() {
		out, _ = sjson.Set(out, "temperature", temperature.Float())
	}
	if topP := root.Get("top_p"); topP.Exists() {
		out, _ = sjson.Set(out, "top_p", topP.Float())
	}
	if frequencyPenalty := root.Get("frequency_penalty"); frequencyPenalty.Exists() {
		out, _ = sjson.Set(out, "frequency_penalty", frequencyPenalty.Float())
	}
	if presencePenalty := root.Get("presence_penalty"); presencePenalty.Exists() {
		out, _ = sjson.Set(out, "presence_penalty", presencePenalty.Float())
	}
	if stop := root.Get("stop"); stop.Exists() {
		out, _ = sjson.SetRaw(out, "stop", stop.Raw)
	}
	if stream := root.Get("stream"); stream.Exists() {
		out, _ = sjson.Set(out, "stream", stream.Bool())
	}
	if logprobs := root.Get("logprobs"); logprobs.Exists() {
		out, _ = sjson.Set(out, "logprobs", logprobs.Bool())
	}
	if topLogprobs := root.Get("top_logprobs"); topLogprobs.Exists() {
		out, _ = sjson.Set(out, "top_logprobs", topLogprobs.Int())
	}
	if echo := root.Get("echo"); echo.Exists() {
		out, _ = sjson.Set(out, "echo", echo.Bool())
	}
	return []byte(out)
}

func ConvertChatCompletionsResponseToCompletions(rawJSON []byte) []byte {
	root := gjson.ParseBytes(rawJSON)
	out := `{"id":"","object":"text_completion","created":0,"model":"","choices":[]}`
	if id := root.Get("id"); id.Exists() {
		out, _ = sjson.Set(out, "id", id.String())
	}
	if created := root.Get("created"); created.Exists() {
		out, _ = sjson.Set(out, "created", created.Int())
	}
	if model := root.Get("model"); model.Exists() {
		out, _ = sjson.Set(out, "model", model.String())
	}
	if usage := root.Get("usage"); usage.Exists() {
		out, _ = sjson.SetRaw(out, "usage", usage.Raw)
	}
	var choices []interface{}
	if chatChoices := root.Get("choices"); chatChoices.Exists() && chatChoices.IsArray() {
		chatChoices.ForEach(func(_, choice gjson.Result) bool {
			completionsChoice := map[string]interface{}{
				"index": choice.Get("index").Int(),
			}
			if message := choice.Get("message"); message.Exists() {
				if content := message.Get("content"); content.Exists() {
					completionsChoice["text"] = content.String()
				}
			} else if delta := choice.Get("delta"); delta.Exists() {
				if content := delta.Get("content"); content.Exists() {
					completionsChoice["text"] = content.String()
				}
			}
			if finishReason := choice.Get("finish_reason"); finishReason.Exists() {
				completionsChoice["finish_reason"] = finishReason.String()
			}
			if logprobs := choice.Get("logprobs"); logprobs.Exists() {
				completionsChoice["logprobs"] = logprobs.Value()
			}
			choices = append(choices, completionsChoice)
			return true
		})
	}
	if len(choices) > 0 {
		choicesJSON, _ := json.Marshal(choices)
		out, _ = sjson.SetRaw(out, "choices", string(choicesJSON))
	}
	return []byte(out)
}

func ShouldTreatAsResponsesFormat(rawJSON []byte) bool {
	if gjson.GetBytes(rawJSON, "messages").Exists() {
		return false
	}
	if gjson.GetBytes(rawJSON, "input").Exists() {
		return true
	}
	if gjson.GetBytes(rawJSON, "instructions").Exists() {
		return true
	}
	return false
}

func WrapResponsesPayloadAsCompleted(payload []byte) []byte {
	if gjson.GetBytes(payload, "type").Exists() {
		return payload
	}
	if gjson.GetBytes(payload, "object").String() != "response" {
		return payload
	}
	wrapped := `{"type":"response.completed","response":{}}`
	wrapped, _ = sjson.SetRaw(wrapped, "response", string(payload))
	return []byte(wrapped)
}

func FormatStreamChunk(chunk []byte) string {
	trimmed := bytes.TrimSpace(chunk)
	if len(trimmed) == 0 {
		return "\n\n"
	}
	if bytes.HasPrefix(trimmed, []byte("data:")) || bytes.HasPrefix(trimmed, []byte("event:")) || bytes.HasPrefix(trimmed, []byte(":")) {
		return string(trimmed) + "\n\n"
	}
	return fmt.Sprintf("data: %s\n\n", trimmed)
}

func ConvertChatCompletionsStreamChunkToCompletions(chunkData []byte) []byte {
	if bytes.HasPrefix(chunkData, []byte("data:")) {
		chunkData = bytes.TrimSpace(chunkData[5:])
	}
	if bytes.Equal(chunkData, []byte("[DONE]")) {
		return nil
	}

	root := gjson.ParseBytes(chunkData)
	hasContent := false
	hasUsage := root.Get("usage").Exists()
	if chatChoices := root.Get("choices"); chatChoices.Exists() && chatChoices.IsArray() {
		chatChoices.ForEach(func(_, choice gjson.Result) bool {
			if delta := choice.Get("delta"); delta.Exists() {
				if content := delta.Get("content"); content.Exists() && content.String() != "" {
					hasContent = true
					return false
				}
			}
			if finishReason := choice.Get("finish_reason"); finishReason.Exists() && finishReason.String() != "" && finishReason.String() != "null" {
				hasContent = true
				return false
			}
			return true
		})
	}
	if !hasContent && !hasUsage {
		return nil
	}

	out := `{"id":"","object":"text_completion","created":0,"model":"","choices":[]}`
	if id := root.Get("id"); id.Exists() {
		out, _ = sjson.Set(out, "id", id.String())
	}
	if created := root.Get("created"); created.Exists() {
		out, _ = sjson.Set(out, "created", created.Int())
	}
	if model := root.Get("model"); model.Exists() {
		out, _ = sjson.Set(out, "model", model.String())
	}

	var choices []interface{}
	if chatChoices := root.Get("choices"); chatChoices.Exists() && chatChoices.IsArray() {
		chatChoices.ForEach(func(_, choice gjson.Result) bool {
			completionsChoice := map[string]interface{}{
				"index": choice.Get("index").Int(),
			}
			if delta := choice.Get("delta"); delta.Exists() {
				if content := delta.Get("content"); content.Exists() && content.String() != "" {
					completionsChoice["text"] = content.String()
				} else {
					completionsChoice["text"] = ""
				}
			} else {
				completionsChoice["text"] = ""
			}
			if finishReason := choice.Get("finish_reason"); finishReason.Exists() && finishReason.String() != "null" {
				completionsChoice["finish_reason"] = finishReason.String()
			}
			if logprobs := choice.Get("logprobs"); logprobs.Exists() {
				completionsChoice["logprobs"] = logprobs.Value()
			}
			choices = append(choices, completionsChoice)
			return true
		})
	}
	if len(choices) > 0 {
		choicesJSON, _ := json.Marshal(choices)
		out, _ = sjson.SetRaw(out, "choices", string(choicesJSON))
	}
	if usage := root.Get("usage"); usage.Exists() {
		out, _ = sjson.SetRaw(out, "usage", usage.Raw)
	}
	return []byte(out)
}

func NormalizeResponsesWebsocketRequest(rawJSON []byte, lastRequest []byte, lastResponseOutput []byte, allowIncrementalInputWithPreviousResponseID bool) ([]byte, []byte, int, error) {
	requestType := strings.TrimSpace(gjson.GetBytes(rawJSON, "type").String())
	switch requestType {
	case "response.create":
		if len(lastRequest) == 0 {
			return normalizeResponseCreateRequest(rawJSON)
		}
		return normalizeResponseSubsequentRequest(rawJSON, lastRequest, lastResponseOutput, allowIncrementalInputWithPreviousResponseID)
	case "response.append":
		return normalizeResponseSubsequentRequest(rawJSON, lastRequest, lastResponseOutput, allowIncrementalInputWithPreviousResponseID)
	default:
		return nil, lastRequest, 400, fmt.Errorf("unsupported websocket request type: %s", requestType)
	}
}

func normalizeResponseCreateRequest(rawJSON []byte) ([]byte, []byte, int, error) {
	normalized, errDelete := sjson.DeleteBytes(rawJSON, "type")
	if errDelete != nil {
		normalized = bytes.Clone(rawJSON)
	}
	normalized, _ = sjson.SetBytes(normalized, "stream", true)
	if !gjson.GetBytes(normalized, "input").Exists() {
		normalized, _ = sjson.SetRawBytes(normalized, "input", []byte("[]"))
	}
	modelName := strings.TrimSpace(gjson.GetBytes(normalized, "model").String())
	if modelName == "" {
		return nil, nil, 400, fmt.Errorf("missing model in response.create request")
	}
	return normalized, bytes.Clone(normalized), 0, nil
}

func normalizeResponseSubsequentRequest(rawJSON []byte, lastRequest []byte, lastResponseOutput []byte, allowIncrementalInputWithPreviousResponseID bool) ([]byte, []byte, int, error) {
	if len(lastRequest) == 0 {
		return nil, lastRequest, 400, fmt.Errorf("websocket request received before response.create")
	}
	nextInput := gjson.GetBytes(rawJSON, "input")
	if !nextInput.Exists() || !nextInput.IsArray() {
		return nil, lastRequest, 400, fmt.Errorf("websocket request requires array field: input")
	}
	if allowIncrementalInputWithPreviousResponseID {
		if prev := strings.TrimSpace(gjson.GetBytes(rawJSON, "previous_response_id").String()); prev != "" {
			normalized, errDelete := sjson.DeleteBytes(rawJSON, "type")
			if errDelete != nil {
				normalized = bytes.Clone(rawJSON)
			}
			if !gjson.GetBytes(normalized, "model").Exists() {
				modelName := strings.TrimSpace(gjson.GetBytes(lastRequest, "model").String())
				if modelName != "" {
					normalized, _ = sjson.SetBytes(normalized, "model", modelName)
				}
			}
			if !gjson.GetBytes(normalized, "instructions").Exists() {
				instructions := gjson.GetBytes(lastRequest, "instructions")
				if instructions.Exists() {
					normalized, _ = sjson.SetRawBytes(normalized, "instructions", []byte(instructions.Raw))
				}
			}
			normalized, _ = sjson.SetBytes(normalized, "stream", true)
			return normalized, bytes.Clone(normalized), 0, nil
		}
	}
	existingInput := gjson.GetBytes(lastRequest, "input")
	mergedInput, err := mergeJSONArrayRaw(existingInput.Raw, normalizeJSONArrayRaw(lastResponseOutput))
	if err != nil {
		return nil, lastRequest, 400, fmt.Errorf("invalid previous response output: %w", err)
	}
	mergedInput, err = mergeJSONArrayRaw(mergedInput, nextInput.Raw)
	if err != nil {
		return nil, lastRequest, 400, fmt.Errorf("invalid request input: %w", err)
	}
	normalized, errDelete := sjson.DeleteBytes(rawJSON, "type")
	if errDelete != nil {
		normalized = bytes.Clone(rawJSON)
	}
	normalized, _ = sjson.DeleteBytes(normalized, "previous_response_id")
	normalized, err = sjson.SetRawBytes(normalized, "input", []byte(mergedInput))
	if err != nil {
		return nil, lastRequest, 400, fmt.Errorf("failed to merge websocket input: %w", err)
	}
	if !gjson.GetBytes(normalized, "model").Exists() {
		modelName := strings.TrimSpace(gjson.GetBytes(lastRequest, "model").String())
		if modelName != "" {
			normalized, _ = sjson.SetBytes(normalized, "model", modelName)
		}
	}
	if !gjson.GetBytes(normalized, "instructions").Exists() {
		instructions := gjson.GetBytes(lastRequest, "instructions")
		if instructions.Exists() {
			normalized, _ = sjson.SetRawBytes(normalized, "instructions", []byte(instructions.Raw))
		}
	}
	normalized, _ = sjson.SetBytes(normalized, "stream", true)
	return normalized, bytes.Clone(normalized), 0, nil
}

func normalizeJSONArrayRaw(raw []byte) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "[]"
	}
	return trimmed
}

func mergeJSONArrayRaw(left string, right string) (string, error) {
	leftArray := gjson.Parse(left)
	rightArray := gjson.Parse(right)
	if !leftArray.IsArray() || !rightArray.IsArray() {
		return "", fmt.Errorf("expected arrays")
	}
	merged := make([]json.RawMessage, 0, len(leftArray.Array())+len(rightArray.Array()))
	for _, item := range leftArray.Array() {
		merged = append(merged, json.RawMessage(item.Raw))
	}
	for _, item := range rightArray.Array() {
		merged = append(merged, json.RawMessage(item.Raw))
	}
	data, err := json.Marshal(merged)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func ResponseCompletedOutputFromPayload(payload []byte) []byte {
	output := gjson.GetBytes(payload, "response.output")
	if output.Exists() && output.IsArray() {
		return bytes.Clone([]byte(output.Raw))
	}
	return []byte("[]")
}

func WebsocketJSONPayloadsFromChunk(chunk []byte) [][]byte {
	payloads := make([][]byte, 0, 2)
	lines := bytes.Split(chunk, []byte("\n"))
	for i := range lines {
		line := bytes.TrimSpace(lines[i])
		if len(line) == 0 || bytes.HasPrefix(line, []byte("event:")) {
			continue
		}
		if bytes.HasPrefix(line, []byte("data:")) {
			line = bytes.TrimSpace(line[len("data:"):])
		}
		if len(line) == 0 || bytes.Equal(line, []byte("[DONE]")) {
			continue
		}
		if json.Valid(line) {
			payloads = append(payloads, bytes.Clone(line))
		}
	}
	if len(payloads) > 0 {
		return payloads
	}
	trimmed := bytes.TrimSpace(chunk)
	if bytes.HasPrefix(trimmed, []byte("data:")) {
		trimmed = bytes.TrimSpace(trimmed[len("data:"):])
	}
	if len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("[DONE]")) && json.Valid(trimmed) {
		payloads = append(payloads, bytes.Clone(trimmed))
	}
	return payloads
}

func AppendWebsocketEvent(builder *strings.Builder, eventType string, payload []byte) {
	if builder == nil {
		return
	}
	trimmedPayload := bytes.TrimSpace(payload)
	if len(trimmedPayload) == 0 {
		return
	}
	if builder.Len() > 0 {
		builder.WriteString("\n")
	}
	builder.WriteString("websocket.")
	builder.WriteString(eventType)
	builder.WriteString("\n")
	builder.Write(trimmedPayload)
	builder.WriteString("\n")
}

func WebsocketPayloadEventType(payload []byte) string {
	eventType := strings.TrimSpace(gjson.GetBytes(payload, "type").String())
	if eventType == "" {
		return "-"
	}
	return eventType
}

func WebsocketPayloadPreview(payload []byte) string {
	trimmedPayload := bytes.TrimSpace(payload)
	if len(trimmedPayload) == 0 {
		return "<empty>"
	}
	preview := trimmedPayload
	if len(preview) > wsPayloadLogMaxSize {
		preview = preview[:wsPayloadLogMaxSize]
	}
	previewText := strings.ReplaceAll(string(preview), "\n", "\\n")
	previewText = strings.ReplaceAll(previewText, "\r", "\\r")
	if len(trimmedPayload) > wsPayloadLogMaxSize {
		return fmt.Sprintf("%s...(truncated,total=%d)", previewText, len(trimmedPayload))
	}
	return previewText
}

func SetWebsocketRequestBody(c *gin.Context, body string) {
	if c == nil {
		return
	}
	trimmedBody := strings.TrimSpace(body)
	if trimmedBody == "" {
		return
	}
	c.Set(WebsocketRequestBodyKey, []byte(trimmedBody))
}

func MarkAPIResponseTimestamp(c *gin.Context) {
	if c == nil {
		return
	}
	if _, exists := c.Get(APIResponseTimestampKey); exists {
		return
	}
	c.Set(APIResponseTimestampKey, time.Now())
}

func BuildResponsesWebsocketErrorPayload(status int, errText string, addon map[string][]string) ([]byte, error) {
	if status <= 0 {
		status = http.StatusInternalServerError
	}
	if strings.TrimSpace(errText) == "" {
		errText = http.StatusText(status)
	}
	body := handlers.BuildErrorResponseBody(status, errText)
	payload := []byte(`{}`)
	var err error
	payload, err = sjson.SetBytes(payload, "type", "error")
	if err != nil {
		return nil, err
	}
	payload, err = sjson.SetBytes(payload, "status", status)
	if err != nil {
		return nil, err
	}
	if addon != nil {
		headers := []byte(`{}`)
		hasHeaders := false
		for key, values := range addon {
			if len(values) == 0 {
				continue
			}
			headerPath := strings.ReplaceAll(strings.ReplaceAll(key, `\`, `\\`), ".", `\.`)
			headers, err = sjson.SetBytes(headers, headerPath, values[0])
			if err != nil {
				return nil, err
			}
			hasHeaders = true
		}
		if hasHeaders {
			payload, err = sjson.SetRawBytes(payload, "headers", headers)
			if err != nil {
				return nil, err
			}
		}
	}
	if len(body) > 0 && json.Valid(body) {
		errorNode := gjson.GetBytes(body, "error")
		if errorNode.Exists() {
			payload, err = sjson.SetRawBytes(payload, "error", []byte(errorNode.Raw))
		} else {
			payload, err = sjson.SetRawBytes(payload, "error", body)
		}
		if err != nil {
			return nil, err
		}
	}
	if !gjson.GetBytes(payload, "error").Exists() {
		payload, err = sjson.SetBytes(payload, "error.type", "server_error")
		if err != nil {
			return nil, err
		}
		payload, err = sjson.SetBytes(payload, "error.message", errText)
		if err != nil {
			return nil, err
		}
	}
	return payload, nil
}
