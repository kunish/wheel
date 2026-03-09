package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/kunish/wheel/apps/worker/internal/runtime/sdk/api/handlers"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

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
