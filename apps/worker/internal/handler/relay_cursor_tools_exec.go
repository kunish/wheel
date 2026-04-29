package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kunish/wheel/apps/worker/internal/relay"
)

type cursorParsedTool struct {
	Name string
	Args map[string]any
}

// cursorParseToolCalls extracts ```json action``` blocks (string-aware closing fence).
func cursorParseToolCalls(responseText string) (cleanText string, tools []cursorParsedTool) {
	type span struct{ start, end int }
	var blocks []span
	searchStart := 0
	for {
		rel := strings.Index(responseText[searchStart:], "```json")
		if rel < 0 {
			break
		}
		blockStart := searchStart + rel
		contentStart := blockStart + len("```json")
		if nl := strings.Index(responseText[contentStart:], "\n"); nl >= 0 {
			contentStart += nl + 1
		} else {
			contentStart = len(responseText)
			break
		}

		pos := contentStart
		inStr := false
		closing := -1
		for pos <= len(responseText)-3 {
			ch := responseText[pos]
			if ch == '"' {
				bs := 0
				for j := pos - 1; j >= contentStart && responseText[j] == '\\'; j-- {
					bs++
				}
				if bs%2 == 0 {
					inStr = !inStr
				}
				pos++
				continue
			}
			if !inStr && responseText[pos] == '`' && pos+2 < len(responseText) &&
				responseText[pos+1] == '`' && responseText[pos+2] == '`' {
				closing = pos
				break
			}
			pos++
		}

		jsonSlice := ""
		if closing >= 0 {
			jsonSlice = strings.TrimSpace(responseText[contentStart:closing])
		} else {
			jsonSlice = strings.TrimSpace(responseText[contentStart:])
		}
		if pt := cursorTryParseToolJSON(jsonSlice); pt != nil {
			tools = append(tools, *pt)
			if closing >= 0 {
				blocks = append(blocks, span{blockStart, closing + 3})
				searchStart = closing + 3
				continue
			}
			blocks = append(blocks, span{blockStart, len(responseText)})
			break
		}
		searchStart = blockStart + 7
	}

	clean := responseText
	for i := len(blocks) - 1; i >= 0; i-- {
		b := blocks[i]
		clean = clean[:b.start] + clean[b.end:]
	}
	return strings.TrimSpace(clean), tools
}

func cursorTryParseToolJSON(s string) *cursorParsedTool {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil
	}
	name, _ := m["tool"].(string)
	if name == "" {
		name, _ = m["name"].(string)
	}
	if name == "" {
		return nil
	}
	var args map[string]any
	if p, ok := m["parameters"].(map[string]any); ok {
		args = p
	} else if p, ok := m["arguments"].(map[string]any); ok {
		args = p
	} else if p, ok := m["input"].(map[string]any); ok {
		args = p
	}
	if args == nil {
		args = map[string]any{}
	}
	return &cursorParsedTool{Name: name, Args: args}
}

func cursorEstimateInputTokensFromAnthropic(anth map[string]any) int {
	b, _ := json.Marshal(anth)
	n := len(b) / 3
	if n < 1 {
		return 1
	}
	return n
}

func cursorAnthropicMsgID() string {
	return "msg_" + cursorShortID() + cursorShortID()
}

func cursorToolUseID() string {
	return "toolu_" + cursorShortID() + cursorShortID()
}

func buildAnthropicToolMessageResponse(requestModel, cleanText string, tools []cursorParsedTool, inputTokEst int) map[string]any {
	id := cursorAnthropicMsgID()
	var content []any
	if cleanText != "" {
		content = append(content, map[string]any{"type": "text", "text": cleanText})
	}
	for _, t := range tools {
		content = append(content, map[string]any{
			"type":  "tool_use",
			"id":    cursorToolUseID(),
			"name":  t.Name,
			"input": t.Args,
		})
	}
	stop := "end_turn"
	if len(tools) > 0 {
		stop = "tool_use"
	}
	outTok := len(cleanText)/4 + len(tools)*16
	if outTok < 1 {
		outTok = 1
	}
	return map[string]any{
		"id":          id,
		"type":        "message",
		"role":        "assistant",
		"model":       requestModel,
		"content":     content,
		"stop_reason": stop,
		"usage": map[string]any{
			"input_tokens":  inputTokEst,
			"output_tokens": outTok,
		},
	}
}

func buildOpenAIToolChatResponse(requestModel, cleanText string, tools []cursorParsedTool) map[string]any {
	msg := map[string]any{"role": "assistant"}
	if cleanText != "" {
		msg["content"] = cleanText
	} else {
		msg["content"] = nil
	}
	finish := "stop"
	if len(tools) > 0 {
		var tcalls []any
		for i, t := range tools {
			args, _ := json.Marshal(t.Args)
			tcalls = append(tcalls, map[string]any{
				"id":   "call_" + cursorShortID(),
				"type": "function",
				"function": map[string]any{
					"name":      t.Name,
					"arguments": string(args),
				},
				"index": i,
			})
		}
		msg["tool_calls"] = tcalls
		finish = "tool_calls"
	}
	return map[string]any{
		"id":      "chatcmpl-" + cursorShortID(),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   requestModel,
		"choices": []any{map[string]any{
			"index":         0,
			"message":       msg,
			"finish_reason": finish,
		}},
	}
}

func (h *RelayHandler) executeCursorComChatToolsNonStreaming(p *relayAttemptParams) (*relayResult, error) {
	client := h.cursorComChatHTTPClient()
	if client == nil {
		return nil, &relay.ProxyError{Message: "http client not configured", StatusCode: http.StatusInternalServerError}
	}
	anth, err := cursorAnthropicSourceForTools(p)
	if err != nil {
		return nil, &relay.ProxyError{Message: err.Error(), StatusCode: http.StatusBadRequest}
	}
	model := cursorComChatAPIModel(mapCursorUpstreamModel(p.TargetModel))
	chatBody, err := anthropicBodyToCursorComChat(anth, model)
	if err != nil {
		return nil, &relay.ProxyError{Message: err.Error(), StatusCode: http.StatusBadRequest}
	}
	tok := ""
	if p.SelectedKey != nil {
		tok = cursorAccessTokenFromChannelKey(p.SelectedKey.ChannelKey)
	}
	text, err := collectCursorComChatText(p.C.Request.Context(), client, chatBody, tok)
	if err != nil {
		return nil, &relay.ProxyError{Message: fmt.Sprintf("cursor web chat: %v", err), StatusCode: http.StatusBadGateway}
	}
	clean, tools := cursorParseToolCalls(text)
	inTok := cursorEstimateInputTokensFromAnthropic(anth)
	var resp map[string]any
	if p.IsAnthropicInbound {
		resp = buildAnthropicToolMessageResponse(p.RequestModel, clean, tools, inTok)
	} else {
		resp = buildOpenAIToolChatResponse(p.RequestModel, clean, tools)
	}
	rb, _ := json.Marshal(resp)
	return &relayResult{
		Response:        resp,
		ResponseContent: string(rb),
		ResponseHeaders: http.Header{"Content-Type": []string{"application/json"}},
		PassthroughJSON: true,
		InputTokens:     inTok,
		OutputTokens:    len(clean)/4 + len(tools)*16,
	}, nil
}

func chunkedUTF8Prefix(s string, maxBytes int) (piece string, rest string) {
	if maxBytes <= 0 || s == "" {
		return "", s
	}
	if len(s) <= maxBytes {
		return s, ""
	}
	end := maxBytes
	for end > 0 && !utf8.RuneStart(s[end]) {
		end--
	}
	if end == 0 {
		r, sz := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError {
			return s[:1], s[1:]
		}
		return s[:sz], s[sz:]
	}
	return s[:end], s[end:]
}

// writeAnthropicToolSSE streams a completed assistant message in Anthropic SSE form.
func writeAnthropicToolSSE(w http.ResponseWriter, model, msgID, clean string, tools []cursorParsedTool, inTok, outTok int) error {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	write := func(event, data string) {
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
		if flusher != nil {
			flusher.Flush()
		}
	}

	msgStart, _ := json.Marshal(map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id": msgID, "type": "message", "role": "assistant", "content": []any{},
			"model": model, "stop_reason": nil, "stop_sequence": nil,
			"usage": map[string]any{"input_tokens": inTok, "output_tokens": 0},
		},
	})
	write("message_start", string(msgStart))

	idx := 0
	if clean != "" {
		startTxt, _ := json.Marshal(map[string]any{
			"type":          "content_block_start",
			"index":         idx,
			"content_block": map[string]any{"type": "text", "text": ""},
		})
		write("content_block_start", string(startTxt))
		rem := clean
		for rem != "" {
			var part string
			part, rem = chunkedUTF8Prefix(rem, 256)
			delta, _ := json.Marshal(map[string]any{
				"type":  "content_block_delta",
				"index": idx,
				"delta": map[string]any{"type": "text_delta", "text": part},
			})
			write("content_block_delta", string(delta))
		}
		stopBlk, _ := json.Marshal(map[string]any{"type": "content_block_stop", "index": idx})
		write("content_block_stop", string(stopBlk))
		idx++
	}

	for _, t := range tools {
		tid := cursorToolUseID()
		startTU, _ := json.Marshal(map[string]any{
			"type":  "content_block_start",
			"index": idx,
			"content_block": map[string]any{
				"type":  "tool_use",
				"id":    tid,
				"name":  t.Name,
				"input": map[string]any{},
			},
		})
		write("content_block_start", string(startTU))
		args, _ := json.Marshal(t.Args)
		delta, _ := json.Marshal(map[string]any{
			"type":  "content_block_delta",
			"index": idx,
			"delta": map[string]any{"type": "input_json_delta", "partial_json": string(args)},
		})
		write("content_block_delta", string(delta))
		stopTU, _ := json.Marshal(map[string]any{"type": "content_block_stop", "index": idx})
		write("content_block_stop", string(stopTU))
		idx++
	}

	stopReason := "end_turn"
	if len(tools) > 0 {
		stopReason = "tool_use"
	}
	msgDelta, _ := json.Marshal(map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason":   stopReason,
			"stop_sequence": nil,
		},
		"usage": map[string]any{"output_tokens": outTok},
	})
	write("message_delta", string(msgDelta))
	write("message_stop", `{"type":"message_stop"}`)
	return nil
}

func writeOpenAIToolChatSSE(w http.ResponseWriter, model, completionID string, clean string, tools []cursorParsedTool) error {
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	emit := func(obj map[string]any) {
		b, _ := json.Marshal(obj)
		fmt.Fprintf(w, "data: %s\n\n", b)
		if flusher != nil {
			flusher.Flush()
		}
	}
	created := time.Now().Unix()
	// role
	emit(map[string]any{
		"id":      completionID,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   model,
		"choices": []any{map[string]any{
			"index": 0,
			"delta": map[string]any{"role": "assistant", "content": ""},
		}},
	})
	rem := clean
	for rem != "" {
		var part string
		part, rem = chunkedUTF8Prefix(rem, 256)
		emit(map[string]any{
			"id": completionID, "object": "chat.completion.chunk", "created": created, "model": model,
			"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": part}}},
		})
	}
	if len(tools) > 0 {
		var tcalls []any
		for i, t := range tools {
			args, _ := json.Marshal(t.Args)
			tcalls = append(tcalls, map[string]any{
				"index": i,
				"id":    "call_" + cursorShortID(),
				"type":  "function",
				"function": map[string]any{
					"name": t.Name, "arguments": string(args),
				},
			})
		}
		emit(map[string]any{
			"id": completionID, "object": "chat.completion.chunk", "created": created, "model": model,
			"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"tool_calls": tcalls}}},
		})
	}
	finish := "stop"
	if len(tools) > 0 {
		finish = "tool_calls"
	}
	emit(map[string]any{
		"id": completionID, "object": "chat.completion.chunk", "created": created, "model": model,
		"choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": finish}},
	})
	fmt.Fprintf(w, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
	}
	return nil
}

func (h *RelayHandler) executeCursorComChatToolsStreaming(p *relayAttemptParams, streamID string) (*relayResult, error) {
	client := h.cursorComChatHTTPClient()
	if client == nil {
		return &relayResult{StreamID: streamID}, &relay.ProxyError{Message: "http client not configured", StatusCode: http.StatusInternalServerError}
	}
	anth, err := cursorAnthropicSourceForTools(p)
	if err != nil {
		return &relayResult{StreamID: streamID}, &relay.ProxyError{Message: err.Error(), StatusCode: http.StatusBadRequest}
	}
	modelAPI := cursorComChatAPIModel(mapCursorUpstreamModel(p.TargetModel))
	chatBody, err := anthropicBodyToCursorComChat(anth, modelAPI)
	if err != nil {
		return &relayResult{StreamID: streamID}, &relay.ProxyError{Message: err.Error(), StatusCode: http.StatusBadRequest}
	}

	tok := ""
	if p.SelectedKey != nil {
		tok = cursorAccessTokenFromChannelKey(p.SelectedKey.ChannelKey)
	}
	start := time.Now()
	firstMs := 0
	text, err := collectCursorComChatText(p.C.Request.Context(), client, chatBody, tok)
	if err != nil {
		return &relayResult{StreamID: streamID}, &relay.ProxyError{Message: fmt.Sprintf("cursor web chat: %v", err), StatusCode: http.StatusBadGateway}
	}
	if text != "" {
		firstMs = int(time.Since(start).Milliseconds())
		if firstMs < 1 {
			firstMs = 1
		}
		h.Observer.RecordTTFB(p.C.Request.Context(), p.Channel.Name, p.TargetModel, firstMs)
	}

	clean, tools := cursorParseToolCalls(text)
	inTok := cursorEstimateInputTokensFromAnthropic(anth)
	outTok := len(clean)/4 + len(tools)*16
	if outTok < 1 {
		outTok = 1
	}

	w := p.C.Writer
	if p.IsAnthropicInbound {
		msgID := cursorAnthropicMsgID()
		_ = writeAnthropicToolSSE(w, p.RequestModel, msgID, clean, tools, inTok, outTok)
	} else {
		_ = writeOpenAIToolChatSSE(w, p.RequestModel, "chatcmpl-"+cursorShortID(), clean, tools)
	}

	fullForLog := text
	return &relayResult{
		StreamID:        streamID,
		ResponseContent: fullForLog,
		ResponseHeaders: w.Header().Clone(),
		FirstTokenTime:  firstMs,
		InputTokens:     inTok,
		OutputTokens:    outTok,
	}, nil
}

// executeCursorComChatToolsBackground serves deferred/background jobs that use Cursor + tools (no gin.Context).
// cursorComChatProxyResult runs cursor.com/api/chat for OpenAI-shaped chat bodies (used by CursorRelay fallback).
func cursorComChatProxyResult(
	ctx context.Context,
	httpClient *http.Client,
	accessToken string,
	body map[string]any,
	requestModel string,
	targetModel string,
	anthropicInbound bool,
) (*relay.ProxyResult, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("http client not configured")
	}
	anth := openAIChatBodyToAnthropicShape(body)
	modelAPI := cursorComChatAPIModel(mapCursorUpstreamModel(targetModel))
	chatBody, err := anthropicBodyToCursorComChat(anth, modelAPI)
	if err != nil {
		return nil, err
	}
	text, err := collectCursorComChatText(ctx, httpClient, chatBody, accessToken)
	if err != nil {
		return nil, err
	}
	clean, tools := cursorParseToolCalls(text)
	inTok := cursorEstimateInputTokensFromAnthropic(anth)
	outTok := len(clean)/4 + len(tools)*16
	if outTok < 1 {
		outTok = 1
	}
	var resp map[string]any
	if anthropicInbound {
		resp = buildAnthropicToolMessageResponse(requestModel, clean, tools, inTok)
	} else {
		resp = buildOpenAIToolChatResponse(requestModel, clean, tools)
	}
	return &relay.ProxyResult{
		Response:        resp,
		InputTokens:     inTok,
		OutputTokens:    outTok,
		StatusCode:      http.StatusOK,
		UpstreamHeaders: http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

// cursorComChatStreamProxy streams tool results after collecting cursor.com/api/chat (CursorRelay fallback).
func cursorComChatStreamProxy(
	w http.ResponseWriter,
	ctx context.Context,
	httpClient *http.Client,
	accessToken string,
	body map[string]any,
	requestModel string,
	targetModel string,
	anthropicInbound bool,
) (*relay.StreamCompleteInfo, error) {
	anth := openAIChatBodyToAnthropicShape(body)
	modelAPI := cursorComChatAPIModel(mapCursorUpstreamModel(targetModel))
	chatBody, err := anthropicBodyToCursorComChat(anth, modelAPI)
	if err != nil {
		return nil, &relay.ProxyError{Message: err.Error(), StatusCode: http.StatusBadRequest}
	}
	start := time.Now()
	text, err := collectCursorComChatText(ctx, httpClient, chatBody, accessToken)
	if err != nil {
		return nil, &relay.ProxyError{Message: fmt.Sprintf("cursor web chat: %v", err), StatusCode: http.StatusBadGateway}
	}
	firstMs := 0
	if text != "" {
		firstMs = int(time.Since(start).Milliseconds())
		if firstMs < 1 {
			firstMs = 1
		}
	}
	clean, tools := cursorParseToolCalls(text)
	inTok := cursorEstimateInputTokensFromAnthropic(anth)
	outTok := len(clean)/4 + len(tools)*16
	if outTok < 1 {
		outTok = 1
	}
	if anthropicInbound {
		msgID := cursorAnthropicMsgID()
		_ = writeAnthropicToolSSE(w, requestModel, msgID, clean, tools, inTok, outTok)
	} else {
		_ = writeOpenAIToolChatSSE(w, requestModel, "chatcmpl-"+cursorShortID(), clean, tools)
	}
	return &relay.StreamCompleteInfo{
		InputTokens:     inTok,
		OutputTokens:    outTok,
		FirstTokenTime:  firstMs,
		ResponseContent: text,
		UpstreamHeaders: w.Header().Clone(),
	}, nil
}

func (h *RelayHandler) executeCursorComChatToolsBackground(
	ctxToUse context.Context,
	requestType string,
	rawBody map[string]any,
	bodyOpenAI map[string]any,
	requestModel string,
	targetModel string,
	channelKey string,
) (*relay.ProxyResult, error) {
	client := h.cursorComChatHTTPClient()
	if client == nil {
		return nil, fmt.Errorf("http client not configured")
	}
	var anth map[string]any
	if requestType == relay.RequestTypeAnthropicMsg {
		anth = rawBody
	} else {
		anth = openAIChatBodyToAnthropicShape(bodyOpenAI)
	}
	modelAPI := cursorComChatAPIModel(mapCursorUpstreamModel(targetModel))
	chatBody, err := anthropicBodyToCursorComChat(anth, modelAPI)
	if err != nil {
		return nil, err
	}
	tok := cursorAccessTokenFromChannelKey(channelKey)
	text, err := collectCursorComChatText(ctxToUse, client, chatBody, tok)
	if err != nil {
		return nil, err
	}
	clean, tools := cursorParseToolCalls(text)
	inTok := cursorEstimateInputTokensFromAnthropic(anth)
	outTok := len(clean)/4 + len(tools)*16
	if outTok < 1 {
		outTok = 1
	}
	var resp map[string]any
	if requestType == relay.RequestTypeAnthropicMsg {
		resp = buildAnthropicToolMessageResponse(requestModel, clean, tools, inTok)
	} else {
		resp = buildOpenAIToolChatResponse(requestModel, clean, tools)
	}
	return &relay.ProxyResult{
		Response:        resp,
		InputTokens:     inTok,
		OutputTokens:    outTok,
		StatusCode:      http.StatusOK,
		UpstreamHeaders: http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}
