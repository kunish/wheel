package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/kunish/wheel/apps/worker/internal/relay"
)

// requestTypeSupportedByCursor reports whether the inbound request can be executed on a Cursor channel.
// Anthropic POST /v1/messages and OpenAI POST /v1/responses are converted to Chat Completions shape
// in relay_retry (attemptBody) before these handlers run.
func requestTypeSupportedByCursor(rt string, isAnthropicInbound bool) bool {
	switch rt {
	case relay.RequestTypeChat:
		return true
	case relay.RequestTypeAnthropicMsg:
		return isAnthropicInbound
	case relay.RequestTypeResponses:
		return true
	default:
		return false
	}
}

func (h *RelayHandler) executeCursorNonStreaming(p *relayAttemptParams) (*relayResult, error) {
	if !requestTypeSupportedByCursor(p.RequestType, p.IsAnthropicInbound) {
		return nil, &relay.ProxyError{
			Message:    "Cursor channel supports /v1/chat/completions, /v1/messages, and /v1/responses only",
			StatusCode: http.StatusNotImplemented,
		}
	}
	hasCom := h.cursorComChatHTTPClient() != nil
	toolsComChat := cursorRelayShouldUseComChat(p)
	if hasCom {
		if toolsComChat && p.IsGeminiNative {
			return nil, &relay.ProxyError{
				Message:    "Cursor channel: Gemini native streaming with tools is not supported",
				StatusCode: http.StatusNotImplemented,
			}
		}
		return h.executeCursorComChatToolsNonStreaming(p)
	}
	if h.CursorRelay == nil {
		return nil, &relay.ProxyError{Message: "cursor relay not configured", StatusCode: http.StatusInternalServerError}
	}
	result, proxyErr := h.CursorRelay.ProxyNonStreaming(
		p.C.Request.Context(),
		p.Channel,
		p.SelectedKey.ChannelKey,
		p.RequestModel,
		p.TargetModel,
		p.Body,
		p.IsAnthropicInbound,
		p.IsGeminiNative,
		h.cursorComChatHTTPClient(),
	)
	if proxyErr != nil {
		return nil, proxyErr
	}
	respJSON, _ := json.Marshal(result.Response)
	return &relayResult{
		InputTokens:     result.InputTokens,
		OutputTokens:    result.OutputTokens,
		Response:        result.Response,
		ResponseContent: string(respJSON),
		ResponseHeaders: result.UpstreamHeaders,
	}, nil
}

func (h *RelayHandler) executeCursorStreaming(p *relayAttemptParams) (*relayResult, error) {
	if !requestTypeSupportedByCursor(p.RequestType, p.IsAnthropicInbound) {
		return nil, &relay.ProxyError{
			Message:    "Cursor channel supports /v1/chat/completions, /v1/messages, and /v1/responses only",
			StatusCode: http.StatusNotImplemented,
		}
	}

	streamID := fmt.Sprintf("%d-%d-%d", time.Now().UnixNano(), p.Channel.ID, p.ApiKeyID)

	hasCom := h.cursorComChatHTTPClient() != nil
	toolsComChat := cursorRelayShouldUseComChat(p)
	if hasCom {
		if toolsComChat && p.IsGeminiNative {
			return &relayResult{StreamID: streamID}, &relay.ProxyError{
				Message:    "Cursor channel: Gemini native streaming with tools is not supported",
				StatusCode: http.StatusNotImplemented,
			}
		}
		h.Observer.StreamStarted(p.C.Request.Context())
		bodyJSON, _ := json.Marshal(p.Body)
		estimatedInputTokens := len(bodyJSON) / 3
		var inputPrice, outputPrice float64
		if mp := relay.LookupModelPrice(p.TargetModel, p.C.Request.Context(), h.DB); mp != nil {
			inputPrice = mp.InputPrice
			outputPrice = mp.OutputPrice
		}
		streamStartPayload := map[string]any{
			"streamId":             streamID,
			"requestModelName":     p.RequestModel,
			"actualModelName":      p.TargetModel,
			"channelId":            p.Channel.ID,
			"channelName":          p.Channel.Name,
			"time":                 time.Now().Unix(),
			"estimatedInputTokens": estimatedInputTokens,
			"inputPrice":           inputPrice,
			"outputPrice":          outputPrice,
			"requestContent":       string(bodyJSON),
			"cursorWebChat":        true,
			"cursorWebChatTools":   toolsComChat,
		}
		if h.Broadcast != nil {
			h.Broadcast("log-stream-start", streamStartPayload)
		}
		if h.StreamTracker != nil {
			h.StreamTracker.TrackStream(streamID, streamStartPayload)
		}
		var onContent relay.StreamContentCallback
		if h.Broadcast != nil {
			onContent = func(thinking, response string) {
				h.Broadcast("log-streaming", map[string]any{
					"streamId":        streamID,
					"thinkingContent": thinking,
					"responseContent": response,
					"thinkingLength":  len(thinking),
					"responseLength":  len(response),
				})
			}
		}
		res, err := h.executeCursorComChatToolsStreaming(p, streamID)
		if err != nil {
			return res, err
		}
		if onContent != nil {
			onContent("", res.ResponseContent)
		}
		return res, nil
	}

	if h.CursorRelay == nil {
		return nil, &relay.ProxyError{Message: "cursor relay not configured", StatusCode: http.StatusInternalServerError}
	}

	h.Observer.StreamStarted(p.C.Request.Context())

	bodyJSON, _ := json.Marshal(p.Body)
	estimatedInputTokens := len(bodyJSON) / 3
	var inputPrice, outputPrice float64
	if mp := relay.LookupModelPrice(p.TargetModel, p.C.Request.Context(), h.DB); mp != nil {
		inputPrice = mp.InputPrice
		outputPrice = mp.OutputPrice
	}
	streamStartPayload := map[string]any{
		"streamId":             streamID,
		"requestModelName":     p.RequestModel,
		"actualModelName":      p.TargetModel,
		"channelId":            p.Channel.ID,
		"channelName":          p.Channel.Name,
		"time":                 time.Now().Unix(),
		"estimatedInputTokens": estimatedInputTokens,
		"inputPrice":           inputPrice,
		"outputPrice":          outputPrice,
		"requestContent":       string(bodyJSON),
	}
	if h.Broadcast != nil {
		h.Broadcast("log-stream-start", streamStartPayload)
	}
	if h.StreamTracker != nil {
		h.StreamTracker.TrackStream(streamID, streamStartPayload)
	}

	var onContent relay.StreamContentCallback
	if h.Broadcast != nil {
		onContent = func(thinking, response string) {
			h.Broadcast("log-streaming", map[string]any{
				"streamId":        streamID,
				"thinkingContent": thinking,
				"responseContent": response,
				"thinkingLength":  len(thinking),
				"responseLength":  len(response),
			})
		}
	}

	streamInfo, proxyErr := h.CursorRelay.ProxyStreaming(
		p.C.Writer,
		p.C.Request.Context(),
		p.Channel,
		p.SelectedKey.ChannelKey,
		p.RequestModel,
		p.TargetModel,
		p.Body,
		p.IsAnthropicInbound,
		p.IsGeminiNative,
		h.cursorComChatHTTPClient(),
	)
	if proxyErr != nil {
		return &relayResult{StreamID: streamID}, proxyErr
	}
	if onContent != nil {
		onContent("", streamInfo.ResponseContent)
	}

	return &relayResult{
		InputTokens:     streamInfo.InputTokens,
		OutputTokens:    streamInfo.OutputTokens,
		FirstTokenTime:  streamInfo.FirstTokenTime,
		ResponseContent: streamInfo.ResponseContent,
		StreamID:        streamID,
		ResponseHeaders: streamInfo.UpstreamHeaders,
	}, nil
}
