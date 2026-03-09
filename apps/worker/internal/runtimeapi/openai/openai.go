package openai

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/kunish/wheel/apps/worker/internal/runtime/sdk/api/handlers"
	sdkopenai "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/api/handlers/openai"
	cliproxyexecutor "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy/executor"
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
