package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// relayAttemptParams holds per-attempt context shared by both strategies.
type relayAttemptParams struct {
	C                      *gin.Context
	Upstream               relay.UpstreamRequest
	Channel                *types.Channel
	SelectedKey            *types.ChannelKey
	TargetModel            string
	RequestModel           string
	Body                   map[string]any
	UpstreamBodyForLog     *string
	IsAnthropicPassthrough bool
	IsAnthropicInbound     bool
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
	streamId := fmt.Sprintf("%d-%d-%d", time.Now().UnixNano(), p.Channel.ID, p.ApiKeyID)

	h.Observer.StreamStarted(p.C.Request.Context())

	// Estimate input tokens from request body size
	bodyJSON, _ := json.Marshal(p.Body)
	estimatedInputTokens := len(bodyJSON) / 3

	// Lookup model pricing for real-time cost estimation
	var inputPrice, outputPrice float64
	if mp := relay.LookupModelPrice(p.TargetModel, context.Background(), h.DB); mp != nil {
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

	streamInfo, proxyErr := relay.ProxyStreaming(
		p.C.Writer,
		p.C.Request.Context(),
		h.StreamClient,
		p.Upstream.URL,
		p.Upstream.Headers,
		p.Upstream.Body,
		p.Channel.Type,
		p.FirstTokenTimeout,
		p.IsAnthropicPassthrough,
		p.IsAnthropicInbound,
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
	}, nil
}

func (s *streamStrategy) HandleSuccess(h *RelayHandler, p *relayAttemptParams, result *relayResult) {
	go h.asyncRecordLog(
		p.RequestModel, p.TargetModel, p.Channel, p.SelectedKey, p.ApiKeyID,
		p.Body, p.UpstreamBodyForLog, result, p.Attempts, p.StartTime,
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
	result, proxyErr := relay.ProxyNonStreaming(
		h.HTTPClient,
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
	}, nil
}

func (s *nonStreamStrategy) HandleSuccess(h *RelayHandler, p *relayAttemptParams, result *relayResult) {
	go h.asyncRecordLog(
		p.RequestModel, p.TargetModel, p.Channel, p.SelectedKey, p.ApiKeyID,
		p.Body, p.UpstreamBodyForLog, result, p.Attempts, p.StartTime,
	)

	// Write response
	if p.IsAnthropicPassthrough {
		p.C.JSON(200, result.Response)
		return
	}
	if p.IsAnthropicInbound {
		p.C.JSON(200, relay.ConvertToAnthropicResponse(result.Response))
		return
	}
	p.C.JSON(200, result.Response)
}

func (s *nonStreamStrategy) CleanupOnFailure(h *RelayHandler, p *relayAttemptParams, streamID string) {
	// No cleanup needed for non-streaming
}
