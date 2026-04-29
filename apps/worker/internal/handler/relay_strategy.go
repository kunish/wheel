package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// guardOpenAICompatProxyToCursorAPI2WithTools blocks OpenAI-compatible channels that point at api2.cursor.sh
// for chat-like endpoints. That combination hits Cursor's OpenAI façade / Agent edge and routinely returns
// the confusing "client-side tools / plain text" error for Claude Code–style traffic. Use OutboundCursor (37)
// so Wheel can bridge via cursor.com/api/chat. Opt out with CURSOR_ALLOW_OPENAI_COMPAT_API2=1.
func guardOpenAICompatProxyToCursorAPI2WithTools(p *relayAttemptParams) *relay.ProxyError {
	if p == nil {
		return nil
	}
	if p.Channel.Type == types.OutboundCursor {
		return nil
	}
	if strings.TrimSpace(os.Getenv("CURSOR_ALLOW_OPENAI_COMPAT_API2")) == "1" {
		return nil
	}
	url := strings.ToLower(p.Upstream.URL)
	if !strings.Contains(url, "api2.cursor.sh") {
		return nil
	}
	if !strings.Contains(url, "/v1/chat/completions") &&
		!strings.Contains(url, "/v1/messages") &&
		!strings.Contains(url, "/v1/responses") {
		return nil
	}
	return &relay.ProxyError{
		Message: "Channel base URL targets api2.cursor.sh but the channel type is not Cursor (37). " +
			"Set the channel type to Cursor in the admin UI so Wheel uses the cursor.com/api/chat bridge. " +
			"OpenAI-compatible channels must not proxy chat to Cursor Agent hosts. " +
			"(Advanced: set env CURSOR_ALLOW_OPENAI_COMPAT_API2=1 on the worker to bypass this check.)",
		StatusCode: http.StatusUnprocessableEntity,
	}
}

// relayAttemptParams holds per-attempt context shared by both strategies.
type relayAttemptParams struct {
	C            *gin.Context
	RequestType  string
	Upstream     relay.UpstreamRequest
	Channel      *types.Channel
	SelectedKey  *types.ChannelKey
	TargetModel  string
	RequestModel string
	Body         map[string]any
	// BridgeOriginalBody is the inbound Anthropic /v1/messages body before convertAnthropicBodyToOpenAI.
	// Used for Cursor channels with tools (builtin cursor.com/api/chat path).
	BridgeOriginalBody map[string]any
	// InboundSnapshot is req.BodyBytes unmarshalled once per attempt (original wire JSON).
	// Plugins or routing may mutate req.Body; this preserves tools for Cursor com-chat routing.
	InboundSnapshot map[string]any
	// InboundRawJSON is the raw request body bytes (same as req.BodyBytes) for Cursor tooling heuristics
	// when map-based detection fails (unusual encodings, partial maps, etc.).
	InboundRawJSON         []byte
	UpstreamBodyForLog     *string
	IsAnthropicPassthrough bool
	IsAnthropicInbound     bool
	IsGeminiNative         bool
	ResponsesOutput        bool
	FirstTokenTimeout      int
	ApiKeyID               int
	SessionKeepTime        int
	Attempts               []attemptRecord
	StartTime              time.Time
}

// relayResult is the unified result from either streaming or non-streaming proxy.
type relayResult struct {
	InputTokens         int
	OutputTokens        int
	CacheReadTokens     int
	CacheCreationTokens int
	FirstTokenTime      int
	ResponseContent     string
	ThinkingContent     string
	Response            map[string]any // non-streaming JSON response
	StreamID            string         // streaming: the stream ID
	ResponseHeaders     http.Header
	BinaryResponse      bool
	// PassthroughJSON: non-streaming response is already in client wire format (e.g. from cursor2api bridge); skip ConvertToAnthropicResponse.
	PassthroughJSON bool
}

// RelayStrategy abstracts the streaming/non-streaming proxy execution.
type RelayStrategy interface {
	// Execute performs the proxy call. Returns a unified result on success.
	Execute(h *RelayHandler, p *relayAttemptParams) (*relayResult, error)
	// HandleSuccess writes the response and triggers async logging.
	HandleSuccess(h *RelayHandler, p *relayAttemptParams, result *relayResult)
	// CleanupOnFailure performs strategy-specific cleanup after a failed attempt.
	CleanupOnFailure(h *RelayHandler, p *relayAttemptParams, streamID string)
}

// ── Stream Strategy ─────────────────────────────────────────────

type streamStrategy struct{}

func (s *streamStrategy) Execute(h *RelayHandler, p *relayAttemptParams) (*relayResult, error) {
	// Copilot channels: call upstream API directly instead of HTTP proxy.
	if p.Channel.Type == types.OutboundCopilot && h.CopilotRelay != nil {
		return h.executeCopilotStreaming(p)
	}

	// Codex CLI channels: proxy to chatgpt.com Responses API.
	if p.Channel.Type == types.OutboundCodexCLI && h.CodexCLIRelay != nil {
		return h.executeCodexCLIStreaming(p)
	}

	// Antigravity channels: convert Anthropic → Gemini and proxy to Google internal API.
	if p.Channel.Type == types.OutboundAntigravity && h.AntigravityRelay != nil {
		return h.executeAntigravityStreaming(p)
	}

	// Cursor channels: cursor.com/api/chat (never fall through to generic api2 OpenAI-compat proxy).
	if p.Channel.Type == types.OutboundCursor {
		if h.CursorRelay == nil {
			return nil, &relay.ProxyError{Message: "Cursor channel requires CursorRelay (worker misconfiguration)", StatusCode: http.StatusInternalServerError}
		}
		return h.executeCursorStreaming(p)
	}

	streamId := fmt.Sprintf("%d-%d-%d", time.Now().UnixNano(), p.Channel.ID, p.ApiKeyID)

	h.Observer.StreamStarted(p.C.Request.Context())

	// Estimate input tokens from request body size
	bodyJSON, _ := json.Marshal(p.Body)
	estimatedInputTokens := len(bodyJSON) / 3

	// Lookup model pricing for real-time cost estimation
	var inputPrice, outputPrice float64
	if mp := relay.LookupModelPrice(p.TargetModel, p.C.Request.Context(), h.DB); mp != nil {
		inputPrice = mp.InputPrice
		outputPrice = mp.OutputPrice
	}

	streamStartPayload := map[string]any{
		"streamId":             streamId,
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
		h.StreamTracker.TrackStream(streamId, streamStartPayload)
	}

	var onContent relay.StreamContentCallback
	if h.Broadcast != nil {
		onContent = func(thinking, response string) {
			h.Broadcast("log-streaming", map[string]any{
				"streamId":        streamId,
				"thinkingContent": thinking,
				"responseContent": response,
				"thinkingLength":  len(thinking),
				"responseLength":  len(response),
			})
		}
	}

	// Use the in-process Codex client when available for Codex channels.
	streamClient := h.StreamClient
	if p.Channel.Type == types.OutboundCodex && h.CodexStreamClient != nil {
		streamClient = h.CodexStreamClient
	}

	if err := guardOpenAICompatProxyToCursorAPI2WithTools(p); err != nil {
		return nil, err
	}

	streamInfo, proxyErr := relay.ProxyStreaming(
		p.C.Writer,
		p.C.Request.Context(),
		streamClient,
		p.Upstream.URL,
		p.Upstream.Headers,
		p.Upstream.Body,
		p.Channel.Type,
		p.FirstTokenTimeout,
		p.IsAnthropicPassthrough,
		p.IsAnthropicInbound,
		p.ResponsesOutput,
		onContent,
	)

	if proxyErr != nil {
		// Attach streamId for cleanup
		return &relayResult{StreamID: streamId}, proxyErr
	}

	return &relayResult{
		InputTokens:         streamInfo.InputTokens,
		OutputTokens:        streamInfo.OutputTokens,
		CacheReadTokens:     streamInfo.CacheReadTokens,
		CacheCreationTokens: streamInfo.CacheCreationTokens,
		FirstTokenTime:      streamInfo.FirstTokenTime,
		ResponseContent:     streamInfo.ResponseContent,
		ThinkingContent:     streamInfo.ThinkingContent,
		StreamID:            streamId,
		ResponseHeaders:     streamInfo.UpstreamHeaders,
	}, nil
}

func (s *streamStrategy) HandleSuccess(h *RelayHandler, p *relayAttemptParams, result *relayResult) {
	go h.asyncRecordLog(
		p.RequestModel, p.TargetModel, p.Channel, p.SelectedKey, p.ApiKeyID,
		p.Body, p.Upstream.Headers, result.ResponseHeaders, p.UpstreamBodyForLog, result, p.Attempts, p.StartTime,
	)
	h.Observer.StreamEnded(p.C.Request.Context())
}

func (s *streamStrategy) CleanupOnFailure(h *RelayHandler, p *relayAttemptParams, streamID string) {
	if h.Broadcast != nil {
		h.Broadcast("log-stream-end", map[string]any{"streamId": streamID})
	}
	if h.StreamTracker != nil {
		h.StreamTracker.UntrackStream(streamID)
	}
	h.Observer.StreamEnded(p.C.Request.Context())
}

// ── Non-Stream Strategy ─────────────────────────────────────────

type nonStreamStrategy struct{}

func (s *nonStreamStrategy) Execute(h *RelayHandler, p *relayAttemptParams) (*relayResult, error) {
	// Copilot channels: call upstream API directly instead of HTTP proxy.
	if p.Channel.Type == types.OutboundCopilot && h.CopilotRelay != nil {
		return h.executeCopilotNonStreaming(p)
	}

	// Codex CLI channels: proxy to chatgpt.com Responses API.
	if p.Channel.Type == types.OutboundCodexCLI && h.CodexCLIRelay != nil {
		return h.executeCodexCLINonStreaming(p)
	}

	// Antigravity channels: convert Anthropic → Gemini and proxy to Google internal API.
	if p.Channel.Type == types.OutboundAntigravity && h.AntigravityRelay != nil {
		return h.executeAntigravityNonStreaming(p)
	}

	// Cursor channels: cursor.com/api/chat.
	if p.Channel.Type == types.OutboundCursor {
		if h.CursorRelay == nil {
			return nil, &relay.ProxyError{Message: "Cursor channel requires CursorRelay (worker misconfiguration)", StatusCode: http.StatusInternalServerError}
		}
		return h.executeCursorNonStreaming(p)
	}

	// Use the in-process Codex client when available for Codex channels.
	httpClient := h.HTTPClient
	if p.Channel.Type == types.OutboundCodex && h.CodexHTTPClient != nil {
		httpClient = h.CodexHTTPClient
	}

	if relay.IsAudioBinaryResponse(p.RequestType) {
		if proxyErr := relay.ProxyBinaryResponse(
			p.C.Writer,
			httpClient,
			p.Upstream.URL,
			p.Upstream.Headers,
			p.Upstream.Body,
		); proxyErr != nil {
			return nil, proxyErr
		}

		return &relayResult{
			ResponseContent: "[binary]",
			ResponseHeaders: p.C.Writer.Header().Clone(),
			BinaryResponse:  true,
		}, nil
	}

	if relay.ShouldUseMultimodalExecution(p.RequestType, p.Channel.Type) {
		result, proxyErr := relay.ProxyMultimodal(
			httpClient,
			p.Upstream.URL,
			p.Upstream.Headers,
			p.Upstream.Body,
			p.RequestType,
		)
		if proxyErr != nil {
			return nil, proxyErr
		}

		respJSON, _ := json.Marshal(result.Response)
		return &relayResult{
			InputTokens:         result.InputTokens,
			OutputTokens:        result.OutputTokens,
			CacheReadTokens:     result.CacheReadTokens,
			CacheCreationTokens: result.CacheCreationTokens,
			Response:            result.Response,
			ResponseContent:     string(respJSON),
			ResponseHeaders:     result.UpstreamHeaders,
		}, nil
	}

	if err := guardOpenAICompatProxyToCursorAPI2WithTools(p); err != nil {
		return nil, err
	}

	result, proxyErr := relay.ProxyNonStreaming(
		httpClient,
		p.Upstream.URL,
		p.Upstream.Headers,
		p.Upstream.Body,
		p.Channel.Type,
		p.IsAnthropicPassthrough,
	)
	if proxyErr != nil {
		return nil, proxyErr
	}

	respJSON, _ := json.Marshal(result.Response)
	return &relayResult{
		InputTokens:         result.InputTokens,
		OutputTokens:        result.OutputTokens,
		CacheReadTokens:     result.CacheReadTokens,
		CacheCreationTokens: result.CacheCreationTokens,
		Response:            result.Response,
		ResponseContent:     string(respJSON),
		ResponseHeaders:     result.UpstreamHeaders,
	}, nil
}

func (s *nonStreamStrategy) HandleSuccess(h *RelayHandler, p *relayAttemptParams, result *relayResult) {
	go h.asyncRecordLog(
		p.RequestModel, p.TargetModel, p.Channel, p.SelectedKey, p.ApiKeyID,
		p.Body, p.Upstream.Headers, result.ResponseHeaders, p.UpstreamBodyForLog, result, p.Attempts, p.StartTime,
	)
	if result.BinaryResponse {
		return
	}
	relay.CopyForwardableHeaders(p.C.Writer.Header(), result.ResponseHeaders)

	// Write response
	if result.PassthroughJSON {
		p.C.JSON(200, result.Response)
		return
	}
	if p.IsAnthropicPassthrough {
		p.C.JSON(200, result.Response)
		return
	}
	if p.IsAnthropicInbound {
		p.C.JSON(200, relay.ConvertToAnthropicResponse(result.Response))
		return
	}
	if p.IsGeminiNative {
		p.C.JSON(200, relay.ConvertOpenAIResponseToGemini(result.Response))
		return
	}
	p.C.JSON(200, result.Response)
}

func (s *nonStreamStrategy) CleanupOnFailure(h *RelayHandler, p *relayAttemptParams, streamID string) {
	// No cleanup needed for non-streaming
}
