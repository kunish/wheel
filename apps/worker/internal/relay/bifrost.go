package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kunish/wheel/apps/worker/internal/bifrostx"
	"github.com/kunish/wheel/apps/worker/internal/types"
	schemas "github.com/maximhq/bifrost/core/schemas"
)

type BifrostProxyRequest struct {
	RequestType       string
	Path              string
	Body              map[string]any
	TargetModel       string
	Channel           *types.Channel
	SelectedKey       *types.ChannelKey
	FirstTokenTimeout int
	DebugRaw          bool
}

func ProxyBifrostNonStreaming(
	clientCtx context.Context,
	client *bifrostx.Client,
	req BifrostProxyRequest,
) (*ProxyResult, *string, error) {
	if client == nil || client.Core() == nil {
		return nil, nil, &ProxyError{Message: "bifrost executor unavailable", StatusCode: 502}
	}
	if req.Channel == nil || req.SelectedKey == nil {
		return nil, nil, &ProxyError{Message: "invalid bifrost request context", StatusCode: 500}
	}

	if err := client.EnsureProvider(req.Channel.ID); err != nil {
		return nil, nil, &ProxyError{Message: fmt.Sprintf("failed to update bifrost provider: %v", err), StatusCode: 502}
	}

	openAIBody, err := normalizeInboundToOpenAIRequest(req.RequestType, req.Body)
	if err != nil {
		return nil, nil, &ProxyError{Message: err.Error(), StatusCode: 400}
	}

	bifrostReq, err := openAIToBifrostChatRequest(openAIBody, bifrostx.ProviderKeyForChannelID(req.Channel.ID), req.TargetModel)
	if err != nil {
		return nil, nil, &ProxyError{Message: err.Error(), StatusCode: 400}
	}

	bifrostCtx, cancel := newBifrostContext(clientCtx, req.FirstTokenTimeout)
	defer cancel()

	bifrostx.SetRequestSelection(bifrostCtx, req.Channel.ID, req.SelectedKey.ID, req.SelectedKey.ChannelKey, req.TargetModel)
	bifrostCtx.SetValue(schemas.BifrostContextKeySendBackRawRequest, req.DebugRaw)
	bifrostCtx.SetValue(schemas.BifrostContextKeySendBackRawResponse, req.DebugRaw)

	if shouldUseResponsesAPI(req.Channel) {
		responsesReq := normalizeResponsesRequestForProvider(bifrostReq.ToResponsesRequest())
		resp, bifrostErr := client.Core().ResponsesRequest(bifrostCtx, responsesReq)
		if bifrostErr != nil {
			return nil, nil, toProxyErrorFromBifrost(bifrostErr)
		}
		if resp == nil {
			return nil, nil, &ProxyError{Message: "empty response from bifrost", StatusCode: 502}
		}

		chatResp := resp.ToBifrostChatResponse()
		if chatResp == nil {
			return nil, nil, &ProxyError{Message: "invalid responses payload from bifrost", StatusCode: 502}
		}

		openAIResp := bifrostChatResponseToOpenAI(chatResp)
		usage, _ := openAIResp["usage"].(map[string]any)
		cacheReadTokens, cacheCreationTokens := cacheTokensFromUsage(chatResp.Usage)
		result := &ProxyResult{
			Response:            openAIResp,
			InputTokens:         toInt(usage["prompt_tokens"]),
			OutputTokens:        toInt(usage["completion_tokens"]),
			CacheReadTokens:     cacheReadTokens,
			CacheCreationTokens: cacheCreationTokens,
			StatusCode:          200,
		}

		return result, extractRawPayloadForLog(resp.ExtraFields.RawRequest, resp.ExtraFields.RawResponse), nil
	}

	resp, bifrostErr := client.Core().ChatCompletionRequest(bifrostCtx, bifrostReq)
	if bifrostErr != nil {
		return nil, nil, toProxyErrorFromBifrost(bifrostErr)
	}
	if resp == nil {
		return nil, nil, &ProxyError{Message: "empty response from bifrost", StatusCode: 502}
	}

	openAIResp := bifrostChatResponseToOpenAI(resp)
	usage, _ := openAIResp["usage"].(map[string]any)
	cacheReadTokens, cacheCreationTokens := cacheTokensFromUsage(resp.Usage)
	result := &ProxyResult{
		Response:            openAIResp,
		InputTokens:         toInt(usage["prompt_tokens"]),
		OutputTokens:        toInt(usage["completion_tokens"]),
		CacheReadTokens:     cacheReadTokens,
		CacheCreationTokens: cacheCreationTokens,
		StatusCode:          200,
	}

	return result, extractRawPayloadForLog(resp.ExtraFields.RawRequest, resp.ExtraFields.RawResponse), nil
}

func ProxyBifrostStreaming(
	w http.ResponseWriter,
	clientCtx context.Context,
	client *bifrostx.Client,
	req BifrostProxyRequest,
	onContent StreamContentCallback,
) (*StreamCompleteInfo, *string, error) {
	if client == nil || client.Core() == nil {
		return nil, nil, &ProxyError{Message: "bifrost executor unavailable", StatusCode: 502}
	}
	if req.Channel == nil || req.SelectedKey == nil {
		return nil, nil, &ProxyError{Message: "invalid bifrost request context", StatusCode: 500}
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, nil, &ProxyError{Message: "streaming not supported", StatusCode: 500}
	}

	if err := client.EnsureProvider(req.Channel.ID); err != nil {
		return nil, nil, &ProxyError{Message: fmt.Sprintf("failed to update bifrost provider: %v", err), StatusCode: 502}
	}

	openAIBody, err := normalizeInboundToOpenAIRequest(req.RequestType, req.Body)
	if err != nil {
		return nil, nil, &ProxyError{Message: err.Error(), StatusCode: 400}
	}
	openAIBody["stream"] = true

	bifrostReq, err := openAIToBifrostChatRequest(openAIBody, bifrostx.ProviderKeyForChannelID(req.Channel.ID), req.TargetModel)
	if err != nil {
		return nil, nil, &ProxyError{Message: err.Error(), StatusCode: 400}
	}

	bifrostCtx, cancel := newBifrostContext(clientCtx, req.FirstTokenTimeout)
	defer cancel()

	bifrostx.SetRequestSelection(bifrostCtx, req.Channel.ID, req.SelectedKey.ID, req.SelectedKey.ChannelKey, req.TargetModel)
	bifrostCtx.SetValue(schemas.BifrostContextKeySendBackRawResponse, true) // DEBUG

	if shouldUseResponsesAPI(req.Channel) {
		return proxyBifrostResponsesStreaming(
			w,
			flusher,
			clientCtx,
			client,
			bifrostCtx,
			req,
			bifrostReq,
			onContent,
		)
	}

	streamCh, bifrostErr := client.Core().ChatCompletionStreamRequest(bifrostCtx, bifrostReq)
	if bifrostErr != nil {
		return nil, nil, toProxyErrorFromBifrost(bifrostErr)
	}
	if streamCh == nil {
		return nil, nil, &ProxyError{Message: "empty stream from bifrost", StatusCode: 502}
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	startTime := time.Now()
	state := &streamingState{
		onContent: onContent,
	}

	var timeoutCh <-chan time.Time
	var timeoutTimer *time.Timer
	if req.FirstTokenTimeout > 0 {
		timeoutTimer = time.NewTimer(time.Duration(req.FirstTokenTimeout) * time.Second)
		timeoutCh = timeoutTimer.C
		defer timeoutTimer.Stop()
	}

	markFirstToken := func() {
		if state.firstTokenReceived {
			return
		}
		state.firstTokenReceived = true
		state.firstTokenTime = int(time.Since(startTime).Milliseconds())
		if timeoutTimer != nil {
			timeoutTimer.Stop()
		}
	}

	convertToAnthropic := createOpenAIToAnthropicSSEConverter()
	var rawPayload *string
	chunkCount := 0

	for {
		select {
		case <-clientCtx.Done():
			return nil, rawPayload, &ProxyError{Message: "Client disconnected", StatusCode: 499}
		case <-timeoutCh:
			if !state.firstTokenReceived {
				return nil, rawPayload, &ProxyError{Message: "First token timeout exceeded", StatusCode: 504}
			}
		case chunk, ok := <-streamCh:
			if !ok {
				if chunkCount == 0 {
					return nil, rawPayload, &ProxyError{Message: "invalid upstream SSE response: no chunks received", StatusCode: 502}
				}
				if err := writeStreamDone(w, flusher, req.RequestType, convertToAnthropic); err != nil {
					return nil, rawPayload, &ProxyError{Message: "Client disconnected", StatusCode: 499}
				}
				return &StreamCompleteInfo{
					InputTokens:         state.inputTokens,
					OutputTokens:        state.outputTokens,
					CacheReadTokens:     state.cacheReadTokens,
					CacheCreationTokens: state.cacheCreationTokens,
					FirstTokenTime:      state.firstTokenTime,
					StatusCode:          200,
					ResponseContent:     state.responseContent,
					ThinkingContent:     state.thinkingContent,
				}, rawPayload, nil
			}
			if chunk == nil {
				continue
			}
			chunkCount++

			if chunk.BifrostError != nil {
				return nil, rawPayload, toProxyErrorFromBifrost(chunk.BifrostError)
			}
			if chunk.BifrostChatResponse == nil {
				continue
			}

			resp := chunk.BifrostChatResponse
			rawPayload = extractRawPayloadForLog(resp.ExtraFields.RawRequest, resp.ExtraFields.RawResponse)
			openAIChunk := bifrostChatResponseToOpenAI(resp)

			usage, _ := openAIChunk["usage"].(map[string]any)
			if usage != nil {
				if inTok := toInt(usage["prompt_tokens"]); inTok > 0 {
					state.inputTokens = inTok
				}
				if outTok := toInt(usage["completion_tokens"]); outTok > 0 {
					state.outputTokens = outTok
				}
			}

			cacheRead, cacheCreation := cacheTokensFromUsage(resp.Usage)
			if cacheRead > 0 {
				state.cacheReadTokens = cacheRead
			}
			if cacheCreation > 0 {
				state.cacheCreationTokens = cacheCreation
			}

			if consumeOpenAIChunk(openAIChunk, state) {
				markFirstToken()
			}

			if err := writeStreamChunk(w, flusher, req.RequestType, openAIChunk, convertToAnthropic); err != nil {
				return nil, rawPayload, &ProxyError{Message: "Client disconnected", StatusCode: 499}
			}
		}
	}
}

func shouldUseResponsesAPI(channel *types.Channel) bool {
	return channel != nil && channel.Type == types.OutboundOpenAIResponses
}

type responsesStreamToolState struct {
	Index       int
	Name        string
	HeaderSent  bool
	ArgsSent    bool
	PendingArgs string
}

type responsesStreamBridgeState struct {
	ResponseID        string
	Model             string
	Created           int
	InputTokens       int
	OutputTokens      int
	CacheReadTokens   int
	CacheCreateTokens int
	AssistantRoleSent bool
	SeenText          bool
	SeenReasoning     bool
	SeenToolCalls     bool
	ToolCalls         map[string]*responsesStreamToolState
	NextToolIndex     int
}

func newResponsesStreamBridgeState(targetModel string) *responsesStreamBridgeState {
	return &responsesStreamBridgeState{
		Model:     targetModel,
		Created:   int(time.Now().Unix()),
		ToolCalls: make(map[string]*responsesStreamToolState),
	}
}

func proxyBifrostResponsesStreaming(
	w http.ResponseWriter,
	flusher http.Flusher,
	clientCtx context.Context,
	client *bifrostx.Client,
	bifrostCtx *schemas.BifrostContext,
	req BifrostProxyRequest,
	bifrostReq *schemas.BifrostChatRequest,
	onContent StreamContentCallback,
) (*StreamCompleteInfo, *string, error) {
	if client == nil || client.Core() == nil {
		return nil, nil, &ProxyError{Message: "bifrost executor unavailable", StatusCode: 502}
	}
	if bifrostReq == nil {
		return nil, nil, &ProxyError{Message: "invalid bifrost request", StatusCode: 500}
	}

	responsesReq := normalizeResponsesRequestForProvider(bifrostReq.ToResponsesRequest())
	// DEBUG: dump request on error
	if debugBody, debugErr := json.Marshal(responsesReq); debugErr == nil {
		_ = os.WriteFile("/tmp/bifrost_debug_req.json", debugBody, 0644)
	}
	streamCh, bifrostErr := client.Core().ResponsesStreamRequest(bifrostCtx, responsesReq)
	if bifrostErr != nil {
		return nil, nil, toProxyErrorFromBifrost(bifrostErr)
	}
	if streamCh == nil {
		return nil, nil, &ProxyError{Message: "empty stream from bifrost responses", StatusCode: 502}
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	startTime := time.Now()
	state := &streamingState{onContent: onContent}
	bridgeState := newResponsesStreamBridgeState(req.TargetModel)

	var timeoutCh <-chan time.Time
	var timeoutTimer *time.Timer
	if req.FirstTokenTimeout > 0 {
		timeoutTimer = time.NewTimer(time.Duration(req.FirstTokenTimeout) * time.Second)
		timeoutCh = timeoutTimer.C
		defer timeoutTimer.Stop()
	}

	markFirstToken := func() {
		if state.firstTokenReceived {
			return
		}
		state.firstTokenReceived = true
		state.firstTokenTime = int(time.Since(startTime).Milliseconds())
		if timeoutTimer != nil {
			timeoutTimer.Stop()
		}
	}

	convertToAnthropic := createOpenAIToAnthropicSSEConverter()
	var rawPayload *string
	emittedChunkCount := 0

	for {
		select {
		case <-clientCtx.Done():
			return nil, rawPayload, &ProxyError{Message: "Client disconnected", StatusCode: 499}
		case <-timeoutCh:
			if !state.firstTokenReceived {
				return nil, rawPayload, &ProxyError{Message: "First token timeout exceeded", StatusCode: 504}
			}
		case chunk, ok := <-streamCh:
			if !ok {
				if emittedChunkCount == 0 {
					return nil, rawPayload, &ProxyError{Message: "invalid upstream SSE response: no chunks received", StatusCode: 502}
				}
				if err := writeStreamDone(w, flusher, req.RequestType, convertToAnthropic); err != nil {
					return nil, rawPayload, &ProxyError{Message: "Client disconnected", StatusCode: 499}
				}
				if bridgeState.InputTokens > 0 {
					state.inputTokens = bridgeState.InputTokens
				}
				if bridgeState.OutputTokens > 0 {
					state.outputTokens = bridgeState.OutputTokens
				}
				if bridgeState.CacheReadTokens > 0 {
					state.cacheReadTokens = bridgeState.CacheReadTokens
				}
				if bridgeState.CacheCreateTokens > 0 {
					state.cacheCreationTokens = bridgeState.CacheCreateTokens
				}
				return &StreamCompleteInfo{
					InputTokens:         state.inputTokens,
					OutputTokens:        state.outputTokens,
					CacheReadTokens:     state.cacheReadTokens,
					CacheCreationTokens: state.cacheCreationTokens,
					FirstTokenTime:      state.firstTokenTime,
					StatusCode:          200,
					ResponseContent:     state.responseContent,
					ThinkingContent:     state.thinkingContent,
				}, rawPayload, nil
			}
			if chunk == nil {
				continue
			}
			if chunk.BifrostError != nil {
				return nil, rawPayload, toProxyErrorFromBifrost(chunk.BifrostError)
			}

			// Compatibility: process chat stream chunks if provider falls back to chat stream internally.
			if chunk.BifrostChatResponse != nil {
				resp := chunk.BifrostChatResponse
				rawPayload = extractRawPayloadForLog(resp.ExtraFields.RawRequest, resp.ExtraFields.RawResponse)
				openAIChunk := bifrostChatResponseToOpenAI(resp)
				emittedChunkCount++

				usage, _ := openAIChunk["usage"].(map[string]any)
				if usage != nil {
					if inTok := toInt(usage["prompt_tokens"]); inTok > 0 {
						state.inputTokens = inTok
					}
					if outTok := toInt(usage["completion_tokens"]); outTok > 0 {
						state.outputTokens = outTok
					}
				}

				cacheRead, cacheCreation := cacheTokensFromUsage(resp.Usage)
				if cacheRead > 0 {
					state.cacheReadTokens = cacheRead
				}
				if cacheCreation > 0 {
					state.cacheCreationTokens = cacheCreation
				}

				if consumeOpenAIChunk(openAIChunk, state) {
					markFirstToken()
				}
				if err := writeStreamChunk(w, flusher, req.RequestType, openAIChunk, convertToAnthropic); err != nil {
					return nil, rawPayload, &ProxyError{Message: "Client disconnected", StatusCode: 499}
				}
				continue
			}

			streamResp := chunk.BifrostResponsesStreamResponse
			if streamResp == nil {
				continue
			}

			candidateRaw := extractRawPayloadForLog(streamResp.ExtraFields.RawRequest, streamResp.ExtraFields.RawResponse)
			if candidateRaw != nil {
				rawPayload = candidateRaw
			}

			openAIChunk, proxyErr := responsesStreamEventToOpenAIChunk(streamResp, bridgeState)
			if proxyErr != nil {
				return nil, rawPayload, proxyErr
			}
			if openAIChunk == nil {
				continue
			}
			emittedChunkCount++

			usage, _ := openAIChunk["usage"].(map[string]any)
			if usage != nil {
				if inTok := toInt(usage["prompt_tokens"]); inTok > 0 {
					state.inputTokens = inTok
				}
				if outTok := toInt(usage["completion_tokens"]); outTok > 0 {
					state.outputTokens = outTok
				}
			}

			if bridgeState.CacheReadTokens > 0 {
				state.cacheReadTokens = bridgeState.CacheReadTokens
			}
			if bridgeState.CacheCreateTokens > 0 {
				state.cacheCreationTokens = bridgeState.CacheCreateTokens
			}

			if consumeOpenAIChunk(openAIChunk, state) {
				markFirstToken()
			}
			if err := writeStreamChunk(w, flusher, req.RequestType, openAIChunk, convertToAnthropic); err != nil {
				return nil, rawPayload, &ProxyError{Message: "Client disconnected", StatusCode: 499}
			}
		}
	}
}

func responsesStreamEventToOpenAIChunk(
	event *schemas.BifrostResponsesStreamResponse,
	state *responsesStreamBridgeState,
) (map[string]any, *ProxyError) {
	if event == nil {
		return nil, nil
	}
	state.applyResponseMeta(event.Response)
	state.recordToolFromItem(event.Item, event.ItemID, event.OutputIndex)

	switch event.Type {
	case schemas.ResponsesStreamResponseTypeCreated,
		schemas.ResponsesStreamResponseTypeInProgress,
		schemas.ResponsesStreamResponseTypeQueued,
		schemas.ResponsesStreamResponseTypePing,
		schemas.ResponsesStreamResponseTypeOutputItemAdded,
		schemas.ResponsesStreamResponseTypeContentPartAdded,
		schemas.ResponsesStreamResponseTypeContentPartDone,
		schemas.ResponsesStreamResponseTypeReasoningSummaryPartAdded,
		schemas.ResponsesStreamResponseTypeReasoningSummaryPartDone,
		schemas.ResponsesStreamResponseTypeReasoningSummaryTextDone,
		schemas.ResponsesStreamResponseTypeRefusalDone:
		return state.toolChunkFromOutputItemDone(event), nil
	case schemas.ResponsesStreamResponseTypeOutputTextDone:
		if chunk := state.textChunkFromOutputTextDone(event); chunk != nil {
			return chunk, nil
		}
		return state.toolChunkFromOutputItemDone(event), nil
	case schemas.ResponsesStreamResponseTypeOutputItemDone:
		if chunk := state.textChunkFromOutputItemDone(event); chunk != nil {
			return chunk, nil
		}
		return state.toolChunkFromOutputItemDone(event), nil
	case schemas.ResponsesStreamResponseTypeOutputTextDelta:
		if event.Delta == nil || *event.Delta == "" {
			return nil, nil
		}
		state.SeenText = true
		delta := map[string]any{"content": *event.Delta}
		return state.chunkFromDelta(delta), nil
	case schemas.ResponsesStreamResponseTypeReasoningSummaryTextDelta:
		if event.Delta == nil || *event.Delta == "" {
			return nil, nil
		}
		state.SeenReasoning = true
		delta := map[string]any{"reasoning": *event.Delta}
		return state.chunkFromDelta(delta), nil
	case schemas.ResponsesStreamResponseTypeRefusalDelta:
		if event.Refusal == nil || *event.Refusal == "" {
			return nil, nil
		}
		state.SeenText = true
		delta := map[string]any{"content": *event.Refusal}
		return state.chunkFromDelta(delta), nil
	case schemas.ResponsesStreamResponseTypeFunctionCallArgumentsDelta:
		if event.Delta == nil || *event.Delta == "" {
			return nil, nil
		}
		return state.toolChunkFromArguments(event, *event.Delta), nil
	case schemas.ResponsesStreamResponseTypeFunctionCallArgumentsDone:
		if event.Arguments == nil || *event.Arguments == "" {
			return nil, nil
		}
		// If deltas already delivered the arguments, skip the done event to avoid
		// emitting duplicate input_json_delta chunks that produce invalid JSON.
		id := state.resolveItemID(event.ItemID, event.Item, event.OutputIndex)
		if id != "" {
			if tool, ok := state.ToolCalls[id]; ok && tool.ArgsSent {
				return nil, nil
			}
		}
		return state.toolChunkFromArguments(event, *event.Arguments), nil
	case schemas.ResponsesStreamResponseTypeCompleted:
		finishReason := mapResponsesStopReasonToFinishReason(nil)
		if event.Response != nil && event.Response.StopReason != nil {
			finishReason = mapResponsesStopReasonToFinishReason(event.Response.StopReason)
		}
		if finishReason == "stop" && state.SeenToolCalls && !state.SeenText {
			finishReason = "tool_calls"
		}
		return state.finishChunk(finishReason), nil
	case schemas.ResponsesStreamResponseTypeFailed,
		schemas.ResponsesStreamResponseTypeIncomplete,
		schemas.ResponsesStreamResponseTypeError:
		return nil, responsesStreamProxyError(event)
	default:
		return nil, nil
	}
}

func (s *responsesStreamBridgeState) textChunkFromOutputTextDone(event *schemas.BifrostResponsesStreamResponse) map[string]any {
	if event == nil || event.Text == nil || strings.TrimSpace(*event.Text) == "" {
		return nil
	}
	if s.SeenText {
		return nil
	}
	s.SeenText = true
	return s.chunkFromDelta(map[string]any{"content": *event.Text})
}

func (s *responsesStreamBridgeState) textChunkFromOutputItemDone(event *schemas.BifrostResponsesStreamResponse) map[string]any {
	if event == nil || event.Item == nil || event.Item.Type == nil {
		return nil
	}
	// For some providers, textual output is only populated in response.output_item.done.
	if *event.Item.Type != schemas.ResponsesMessageTypeMessage {
		return nil
	}
	if s.SeenText {
		return nil
	}
	text := extractTextFromResponsesMessage(event.Item)
	if strings.TrimSpace(text) == "" {
		return nil
	}
	s.SeenText = true
	return s.chunkFromDelta(map[string]any{"content": text})
}

func extractTextFromResponsesMessage(item *schemas.ResponsesMessage) string {
	if item == nil || item.Content == nil {
		return ""
	}
	if item.Content.ContentStr != nil {
		return strings.TrimSpace(*item.Content.ContentStr)
	}
	if len(item.Content.ContentBlocks) == 0 {
		return ""
	}
	var b strings.Builder
	for _, block := range item.Content.ContentBlocks {
		switch block.Type {
		case schemas.ResponsesOutputMessageContentTypeText, schemas.ResponsesOutputMessageContentTypeRefusal, schemas.ResponsesOutputMessageContentTypeReasoning:
			if block.Text == nil || strings.TrimSpace(*block.Text) == "" {
				continue
			}
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(strings.TrimSpace(*block.Text))
		}
	}
	return b.String()
}

func (s *responsesStreamBridgeState) applyResponseMeta(resp *schemas.BifrostResponsesResponse) {
	if resp == nil {
		return
	}
	if resp.ID != nil && *resp.ID != "" {
		s.ResponseID = *resp.ID
	}
	if resp.Model != "" {
		s.Model = resp.Model
	}
	if resp.CreatedAt > 0 {
		s.Created = resp.CreatedAt
	}
	if resp.Usage != nil {
		s.InputTokens = resp.Usage.InputTokens
		s.OutputTokens = resp.Usage.OutputTokens
		if resp.Usage.InputTokensDetails != nil {
			s.CacheReadTokens = resp.Usage.InputTokensDetails.CachedTokens
		}
		if resp.Usage.OutputTokensDetails != nil {
			s.CacheCreateTokens = resp.Usage.OutputTokensDetails.CachedTokens
		}
	}
}

func (s *responsesStreamBridgeState) resolveItemID(itemID *string, item *schemas.ResponsesMessage, outputIndex *int) string {
	if itemID != nil && *itemID != "" {
		return *itemID
	}
	if item != nil && item.ID != nil && *item.ID != "" {
		return *item.ID
	}
	if outputIndex != nil && *outputIndex >= 0 {
		return fmt.Sprintf("call_%d", *outputIndex)
	}
	return ""
}

func (s *responsesStreamBridgeState) ensureToolState(itemID string, outputIndex *int) *responsesStreamToolState {
	if itemID == "" {
		return nil
	}
	if tool, ok := s.ToolCalls[itemID]; ok {
		if outputIndex != nil && *outputIndex >= 0 {
			tool.Index = *outputIndex
			if *outputIndex >= s.NextToolIndex {
				s.NextToolIndex = *outputIndex + 1
			}
		}
		return tool
	}

	index := s.NextToolIndex
	if outputIndex != nil && *outputIndex >= 0 {
		index = *outputIndex
	}
	if index >= s.NextToolIndex {
		s.NextToolIndex = index + 1
	}

	tool := &responsesStreamToolState{Index: index}
	s.ToolCalls[itemID] = tool
	return tool
}

func (s *responsesStreamBridgeState) recordToolFromItem(item *schemas.ResponsesMessage, itemID *string, outputIndex *int) {
	if item == nil || item.Type == nil || *item.Type != schemas.ResponsesMessageTypeFunctionCall {
		return
	}
	id := s.resolveItemID(itemID, item, outputIndex)
	if id == "" {
		return
	}
	tool := s.ensureToolState(id, outputIndex)
	if tool == nil {
		return
	}
	if item.ResponsesToolMessage != nil && item.ResponsesToolMessage.Name != nil && *item.ResponsesToolMessage.Name != "" {
		tool.Name = *item.ResponsesToolMessage.Name
	}
}

func (s *responsesStreamBridgeState) toolChunkFromArguments(event *schemas.BifrostResponsesStreamResponse, args string) map[string]any {
	id := s.resolveItemID(event.ItemID, event.Item, event.OutputIndex)
	if id == "" {
		return nil
	}
	tool := s.ensureToolState(id, event.OutputIndex)
	if tool == nil {
		return nil
	}
	if event.Item != nil && event.Item.ResponsesToolMessage != nil && event.Item.ResponsesToolMessage.Name != nil && *event.Item.ResponsesToolMessage.Name != "" {
		tool.Name = *event.Item.ResponsesToolMessage.Name
	}

	if !tool.HeaderSent {
		if args != "" {
			tool.PendingArgs += args
		}
		// Do not emit malformed tool calls; wait until function name is known.
		if tool.Name == "" {
			return nil
		}

		fullArgs := tool.PendingArgs
		tool.PendingArgs = ""

		function := map[string]any{
			"name":      tool.Name,
			"arguments": fullArgs,
		}
		entry := map[string]any{
			"index":    tool.Index,
			"id":       id,
			"type":     "function",
			"function": function,
		}
		tool.HeaderSent = true
		tool.ArgsSent = fullArgs != ""
		s.SeenToolCalls = true
		return s.chunkFromDelta(map[string]any{
			"tool_calls": []any{entry},
		})
	}

	if args == "" {
		return nil
	}
	tool.ArgsSent = true
	s.SeenToolCalls = true
	return s.chunkFromDelta(map[string]any{
		"tool_calls": []any{
			map[string]any{
				"index": tool.Index,
				"function": map[string]any{
					"arguments": args,
				},
			},
		},
	})
}

func (s *responsesStreamBridgeState) toolChunkFromOutputItemDone(event *schemas.BifrostResponsesStreamResponse) map[string]any {
	if event == nil || event.Item == nil || event.Item.Type == nil || *event.Item.Type != schemas.ResponsesMessageTypeFunctionCall {
		return nil
	}
	if event.Item.ResponsesToolMessage == nil || event.Item.ResponsesToolMessage.Arguments == nil || *event.Item.ResponsesToolMessage.Arguments == "" {
		return nil
	}
	id := s.resolveItemID(event.ItemID, event.Item, event.OutputIndex)
	if id == "" {
		return nil
	}
	tool := s.ensureToolState(id, event.OutputIndex)
	if tool == nil || tool.ArgsSent {
		return nil
	}
	if event.Item.ResponsesToolMessage.Name != nil && *event.Item.ResponsesToolMessage.Name != "" {
		tool.Name = *event.Item.ResponsesToolMessage.Name
	}
	tool.PendingArgs += *event.Item.ResponsesToolMessage.Arguments
	return s.toolChunkFromArguments(event, "")
}

func (s *responsesStreamBridgeState) chunkFromDelta(delta map[string]any) map[string]any {
	if delta == nil {
		delta = map[string]any{}
	}
	if !s.AssistantRoleSent {
		delta["role"] = "assistant"
		s.AssistantRoleSent = true
	}
	return map[string]any{
		"id":      s.streamID(),
		"object":  "chat.completion.chunk",
		"created": s.streamCreated(),
		"model":   s.streamModel(),
		"choices": []any{
			map[string]any{
				"index":         0,
				"delta":         delta,
				"finish_reason": nil,
			},
		},
	}
}

func (s *responsesStreamBridgeState) finishChunk(finishReason string) map[string]any {
	return map[string]any{
		"id":      s.streamID(),
		"object":  "chat.completion.chunk",
		"created": s.streamCreated(),
		"model":   s.streamModel(),
		"choices": []any{
			map[string]any{
				"index":         0,
				"delta":         map[string]any{},
				"finish_reason": finishReason,
			},
		},
		"usage": s.usageMap(),
	}
}

func (s *responsesStreamBridgeState) streamID() string {
	if s.ResponseID != "" {
		return s.ResponseID
	}
	return "chatcmpl-responses-stream"
}

func (s *responsesStreamBridgeState) streamModel() string {
	if s.Model != "" {
		return s.Model
	}
	return "unknown"
}

func (s *responsesStreamBridgeState) streamCreated() int {
	if s.Created > 0 {
		return s.Created
	}
	return int(time.Now().Unix())
}

func (s *responsesStreamBridgeState) usageMap() map[string]any {
	total := s.InputTokens + s.OutputTokens
	return map[string]any{
		"prompt_tokens":     s.InputTokens,
		"completion_tokens": s.OutputTokens,
		"total_tokens":      total,
	}
}

func mapResponsesStopReasonToFinishReason(stopReason *string) string {
	if stopReason == nil || *stopReason == "" {
		return "stop"
	}
	switch strings.ToLower(strings.TrimSpace(*stopReason)) {
	case "max_output_tokens", "max_tokens", "length":
		return "length"
	case "tool_calls", "tool_use", "function_call":
		return "tool_calls"
	case "content_filter", "safety":
		return "content_filter"
	default:
		return "stop"
	}
}

func responsesStreamProxyError(event *schemas.BifrostResponsesStreamResponse) *ProxyError {
	if event == nil {
		return &ProxyError{Message: "responses stream failed", StatusCode: 502}
	}

	message := "responses stream failed"
	switch event.Type {
	case schemas.ResponsesStreamResponseTypeIncomplete:
		message = "upstream response incomplete"
	case schemas.ResponsesStreamResponseTypeFailed:
		message = "upstream response failed"
	case schemas.ResponsesStreamResponseTypeError:
		message = "upstream stream error"
	}

	if event.Message != nil && strings.TrimSpace(*event.Message) != "" {
		message = strings.TrimSpace(*event.Message)
	}
	if event.Response != nil {
		if event.Response.Error != nil && strings.TrimSpace(event.Response.Error.Message) != "" {
			message = strings.TrimSpace(event.Response.Error.Message)
		} else if event.Response.IncompleteDetails != nil && strings.TrimSpace(event.Response.IncompleteDetails.Reason) != "" {
			message = "upstream response incomplete: " + strings.TrimSpace(event.Response.IncompleteDetails.Reason)
		}
	}

	return &ProxyError{
		Message:    message,
		StatusCode: 502,
	}
}

func proxyBifrostResponsesSyntheticStreaming(
	w http.ResponseWriter,
	flusher http.Flusher,
	client *bifrostx.Client,
	bifrostCtx *schemas.BifrostContext,
	req BifrostProxyRequest,
	bifrostReq *schemas.BifrostChatRequest,
	onContent StreamContentCallback,
) (*StreamCompleteInfo, *string, error) {
	if client == nil || client.Core() == nil {
		return nil, nil, &ProxyError{Message: "bifrost executor unavailable", StatusCode: 502}
	}
	if bifrostReq == nil {
		return nil, nil, &ProxyError{Message: "invalid bifrost request", StatusCode: 500}
	}

	responsesReq := normalizeResponsesRequestForProvider(bifrostReq.ToResponsesRequest())
	resp, bifrostErr := client.Core().ResponsesRequest(bifrostCtx, responsesReq)
	if bifrostErr != nil {
		return nil, nil, toProxyErrorFromBifrost(bifrostErr)
	}
	if resp == nil {
		return nil, nil, &ProxyError{Message: "empty response from bifrost", StatusCode: 502}
	}

	chatResp := resp.ToBifrostChatResponse()
	if chatResp == nil {
		return nil, nil, &ProxyError{Message: "invalid responses payload from bifrost", StatusCode: 502}
	}

	openAIResp := bifrostChatResponseToOpenAI(chatResp)
	chunks := buildSyntheticOpenAIStreamChunks(openAIResp)
	if len(chunks) == 0 {
		return nil, nil, &ProxyError{Message: "empty synthetic stream from responses payload", StatusCode: 502}
	}

	state := &streamingState{
		onContent: onContent,
	}
	convertToAnthropic := createOpenAIToAnthropicSSEConverter()
	startTime := time.Now()
	firstTokenRecorded := false

	for _, chunk := range chunks {
		if err := writeStreamChunk(w, flusher, req.RequestType, chunk, convertToAnthropic); err != nil {
			return nil, nil, &ProxyError{Message: "Client disconnected", StatusCode: 499}
		}
		if consumeOpenAIChunk(chunk, state) && !firstTokenRecorded {
			firstTokenRecorded = true
			state.firstTokenReceived = true
			state.firstTokenTime = int(time.Since(startTime).Milliseconds())
		}
	}

	if err := writeStreamDone(w, flusher, req.RequestType, convertToAnthropic); err != nil {
		return nil, nil, &ProxyError{Message: "Client disconnected", StatusCode: 499}
	}

	usage, _ := openAIResp["usage"].(map[string]any)
	cacheReadTokens, cacheCreationTokens := cacheTokensFromUsage(chatResp.Usage)
	return &StreamCompleteInfo{
		InputTokens:         toInt(usage["prompt_tokens"]),
		OutputTokens:        toInt(usage["completion_tokens"]),
		CacheReadTokens:     cacheReadTokens,
		CacheCreationTokens: cacheCreationTokens,
		FirstTokenTime:      state.firstTokenTime,
		StatusCode:          200,
		ResponseContent:     state.responseContent,
		ThinkingContent:     state.thinkingContent,
	}, extractRawPayloadForLog(resp.ExtraFields.RawRequest, resp.ExtraFields.RawResponse), nil
}

func buildSyntheticOpenAIStreamChunks(openAIResp map[string]any) []map[string]any {
	choices, _ := openAIResp["choices"].([]any)
	if len(choices) == 0 {
		return nil
	}
	choice, _ := choices[0].(map[string]any)
	if choice == nil {
		return nil
	}

	message, _ := choice["message"].(map[string]any)
	if message == nil {
		return nil
	}

	id, _ := openAIResp["id"].(string)
	if id == "" {
		id = "chatcmpl-responses"
	}
	model, _ := openAIResp["model"].(string)
	created := toInt(openAIResp["created"])

	chunks := make([]map[string]any, 0, 2)

	delta := map[string]any{
		"role": "assistant",
	}
	hasDeltaPayload := false
	if content, ok := message["content"].(string); ok && content != "" {
		delta["content"] = content
		hasDeltaPayload = true
	}
	if toolCalls, ok := message["tool_calls"].([]any); ok && len(toolCalls) > 0 {
		delta["tool_calls"] = toolCalls
		hasDeltaPayload = true
	}

	if hasDeltaPayload {
		chunks = append(chunks, map[string]any{
			"id":      id,
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   model,
			"choices": []any{
				map[string]any{
					"index":         0,
					"delta":         delta,
					"finish_reason": nil,
				},
			},
		})
	}

	finishReason, _ := choice["finish_reason"].(string)
	if finishReason == "" {
		finishReason = "stop"
	}

	finalChunk := map[string]any{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   model,
		"choices": []any{
			map[string]any{
				"index":         0,
				"delta":         map[string]any{},
				"finish_reason": finishReason,
			},
		},
	}
	if usage, ok := openAIResp["usage"].(map[string]any); ok && usage != nil {
		finalChunk["usage"] = usage
	}
	chunks = append(chunks, finalChunk)

	return chunks
}

func normalizeResponsesRequestForProvider(req *schemas.BifrostResponsesRequest) *schemas.BifrostResponsesRequest {
	if req == nil {
		return nil
	}

	if req.Params == nil {
		req.Params = &schemas.ResponsesParameters{}
	}

	filtered := make([]schemas.ResponsesMessage, 0, len(req.Input))
	instructionParts := make([]string, 0)

	for _, msg := range req.Input {
		role := ""
		if msg.Role != nil {
			role = string(*msg.Role)
		}

		// Some OpenAI-compatible gateways reject "system"/"developer" in input[] for Responses API.
		// Promote them to top-level instructions and keep input[] as user/assistant/tool messages.
		if role == string(schemas.ResponsesInputMessageRoleSystem) || role == string(schemas.ResponsesInputMessageRoleDeveloper) {
			if text := extractResponsesMessageText(msg); strings.TrimSpace(text) != "" {
				instructionParts = append(instructionParts, text)
			}
			continue
		}

		filtered = append(filtered, msg)
	}

	if len(instructionParts) > 0 {
		instructions := strings.Join(instructionParts, "\n\n")
		if req.Params.Instructions != nil && strings.TrimSpace(*req.Params.Instructions) != "" {
			instructions = strings.TrimSpace(*req.Params.Instructions) + "\n\n" + instructions
		}
		req.Params.Instructions = &instructions
	}

	req.Input = filtered

	// Bifrost SDK sets role="assistant" on function_call messages, but many upstreams
	// reject role on non-message types. Strip role from function_call/function_call_output.
	for i := range req.Input {
		if req.Input[i].Type != nil && *req.Input[i].Type != schemas.ResponsesMessageTypeMessage {
			req.Input[i].Role = nil
		}
	}

	// Bifrost SDK serializes *bool Strict as "strict":null when nil (no omitempty).
	// Many OpenAI-compatible upstreams reject null strict, so default it to false.
	if req.Params != nil {
		// Reasoning models (codex etc.) reject temperature/top_p; strip for Responses API.
		req.Params.Temperature = nil
		req.Params.TopP = nil

		for i := range req.Params.Tools {
			if req.Params.Tools[i].ResponsesToolFunction != nil && req.Params.Tools[i].ResponsesToolFunction.Strict == nil {
				f := false
				req.Params.Tools[i].ResponsesToolFunction.Strict = &f
			}
		}
	}

	return req
}

func extractResponsesMessageText(msg schemas.ResponsesMessage) string {
	if msg.Content == nil {
		return ""
	}
	if msg.Content.ContentStr != nil {
		return *msg.Content.ContentStr
	}
	if len(msg.Content.ContentBlocks) == 0 {
		return ""
	}

	var b strings.Builder
	for _, part := range msg.Content.ContentBlocks {
		if part.Text == nil || *part.Text == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(*part.Text)
	}
	return b.String()
}

func normalizeInboundToOpenAIRequest(requestType string, body map[string]any) (map[string]any, error) {
	switch requestType {
	case "openai-chat", "openai-responses", "openai-embeddings":
		return copyBody(body), nil
	case "anthropic-messages":
		return anthropicRequestToOpenAI(body), nil
	case "gemini-generate-content", "gemini-stream-generate-content":
		return geminiRequestToOpenAI(body, requestType == "gemini-stream-generate-content"), nil
	default:
		return nil, fmt.Errorf("unsupported request type for bifrost: %s", requestType)
	}
}

func anthropicRequestToOpenAI(body map[string]any) map[string]any {
	result := map[string]any{
		"messages": []any{},
	}

	if model, ok := body["model"].(string); ok {
		result["model"] = model
	}
	if stream, ok := body["stream"].(bool); ok {
		result["stream"] = stream
	}
	if maxTokens, ok := readInt(body["max_tokens"]); ok {
		result["max_tokens"] = maxTokens
	}
	if temperature, ok := readFloat(body["temperature"]); ok {
		result["temperature"] = temperature
	}
	if topP, ok := readFloat(body["top_p"]); ok {
		result["top_p"] = topP
	}
	if stop, ok := body["stop_sequences"]; ok {
		result["stop"] = stop
	}

	var openAIMessages []any

	if systemText := anthropicSystemText(body["system"]); systemText != "" {
		openAIMessages = append(openAIMessages, map[string]any{
			"role":    "system",
			"content": systemText,
		})
	}

	if messages, ok := body["messages"].([]any); ok {
		for _, msgRaw := range messages {
			msg, ok := msgRaw.(map[string]any)
			if !ok {
				continue
			}
			role, _ := msg["role"].(string)
			content := msg["content"]

			switch role {
			case "assistant":
				text, toolCalls := anthropicAssistantContent(content)
				assistant := map[string]any{
					"role": "assistant",
				}
				if text != "" {
					assistant["content"] = text
				} else {
					assistant["content"] = ""
				}
				if len(toolCalls) > 0 {
					assistant["tool_calls"] = toolCalls
				}
				openAIMessages = append(openAIMessages, assistant)
			case "user":
				text, toolMsgs := anthropicUserContent(content)
				if text != "" {
					openAIMessages = append(openAIMessages, map[string]any{
						"role":    "user",
						"content": text,
					})
				}
				openAIMessages = append(openAIMessages, toolMsgs...)
			default:
				if text := extractPlainText(content); text != "" {
					openAIMessages = append(openAIMessages, map[string]any{
						"role":    role,
						"content": text,
					})
				}
			}
		}
	}

	result["messages"] = openAIMessages

	if tools, ok := body["tools"].([]any); ok {
		openAITools := make([]any, 0, len(tools))
		for _, raw := range tools {
			tool, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			openAITools = append(openAITools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        tool["name"],
					"description": tool["description"],
					"parameters":  tool["input_schema"],
				},
			})
		}
		if len(openAITools) > 0 {
			result["tools"] = openAITools
		}
	}

	return result
}

func geminiRequestToOpenAI(body map[string]any, stream bool) map[string]any {
	result := map[string]any{
		"messages": []any{},
		"stream":   stream,
	}

	var openAIMessages []any
	toolCallSeq := 0

	if systemInstruction, ok := body["system_instruction"].(map[string]any); ok {
		if text := geminiPartsToText(systemInstruction["parts"]); text != "" {
			openAIMessages = append(openAIMessages, map[string]any{
				"role":    "system",
				"content": text,
			})
		}
	}

	if contents, ok := body["contents"].([]any); ok {
		for _, cRaw := range contents {
			content, ok := cRaw.(map[string]any)
			if !ok {
				continue
			}
			role, _ := content["role"].(string)
			parts, _ := content["parts"].([]any)
			textParts := make([]string, 0)
			toolCalls := make([]any, 0)

			for _, pRaw := range parts {
				part, ok := pRaw.(map[string]any)
				if !ok {
					continue
				}
				if text, ok := part["text"].(string); ok {
					textParts = append(textParts, text)
					continue
				}
				if fc, ok := part["functionCall"].(map[string]any); ok {
					toolCallSeq++
					callID := fmt.Sprintf("call_%d", toolCallSeq)
					if id, ok := fc["id"].(string); ok && id != "" {
						callID = id
					}
					args := "{}"
					if a, ok := fc["args"]; ok {
						if b, err := json.Marshal(a); err == nil {
							args = string(b)
						}
					}
					toolCalls = append(toolCalls, map[string]any{
						"id":   callID,
						"type": "function",
						"function": map[string]any{
							"name":      fc["name"],
							"arguments": args,
						},
					})
					continue
				}
				if fr, ok := part["functionResponse"].(map[string]any); ok {
					responseContent := ""
					if responseObj, ok := fr["response"]; ok {
						if b, err := json.Marshal(responseObj); err == nil {
							responseContent = string(b)
						}
					}
					name, _ := fr["name"].(string)
					if name == "" {
						name = "tool"
					}
					openAIMessages = append(openAIMessages, map[string]any{
						"role":         "tool",
						"tool_call_id": name,
						"content":      responseContent,
					})
				}
			}

			msgRole := "user"
			if role == "model" {
				msgRole = "assistant"
			}

			text := strings.Join(textParts, "")
			if msgRole == "assistant" && len(toolCalls) > 0 {
				msg := map[string]any{
					"role":       "assistant",
					"content":    text,
					"tool_calls": toolCalls,
				}
				openAIMessages = append(openAIMessages, msg)
				continue
			}
			if text != "" {
				openAIMessages = append(openAIMessages, map[string]any{
					"role":    msgRole,
					"content": text,
				})
			}
		}
	}

	result["messages"] = openAIMessages

	if generationConfig, ok := body["generationConfig"].(map[string]any); ok {
		if maxOut, ok := readInt(generationConfig["maxOutputTokens"]); ok {
			result["max_tokens"] = maxOut
		}
		if temperature, ok := readFloat(generationConfig["temperature"]); ok {
			result["temperature"] = temperature
		}
		if topP, ok := readFloat(generationConfig["topP"]); ok {
			result["top_p"] = topP
		}
		if stop, ok := generationConfig["stopSequences"]; ok {
			result["stop"] = stop
		}
	}

	if tools, ok := body["tools"].([]any); ok {
		var openAITools []any
		for _, tRaw := range tools {
			tool, ok := tRaw.(map[string]any)
			if !ok {
				continue
			}
			functionDeclarations, _ := tool["functionDeclarations"].([]any)
			for _, fdRaw := range functionDeclarations {
				fd, ok := fdRaw.(map[string]any)
				if !ok {
					continue
				}
				openAITools = append(openAITools, map[string]any{
					"type": "function",
					"function": map[string]any{
						"name":        fd["name"],
						"description": fd["description"],
						"parameters":  fd["parameters"],
					},
				})
			}
		}
		if len(openAITools) > 0 {
			result["tools"] = openAITools
		}
	}

	return result
}

func openAIToBifrostChatRequest(
	openAIBody map[string]any,
	provider schemas.ModelProvider,
	targetModel string,
) (*schemas.BifrostChatRequest, error) {
	messagesRaw, ok := openAIBody["messages"]
	if !ok {
		return nil, fmt.Errorf("messages is required")
	}

	messageBytes, err := json.Marshal(messagesRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize messages: %w", err)
	}
	var input []schemas.ChatMessage
	if err := json.Unmarshal(messageBytes, &input); err != nil {
		return nil, fmt.Errorf("failed to parse messages: %w", err)
	}
	if len(input) == 0 {
		return nil, fmt.Errorf("messages is required")
	}

	params := &schemas.ChatParameters{
		ExtraParams: map[string]any{},
	}

	if maxTokens, ok := readInt(openAIBody["max_tokens"]); ok {
		params.MaxCompletionTokens = &maxTokens
	}
	if temperature, ok := readFloat(openAIBody["temperature"]); ok {
		params.Temperature = &temperature
	}
	if topP, ok := readFloat(openAIBody["top_p"]); ok {
		params.TopP = &topP
	}
	if stop, ok := stopSequences(openAIBody["stop"]); ok {
		params.Stop = stop
	}

	if toolsRaw, ok := openAIBody["tools"]; ok {
		toolsBytes, err := json.Marshal(toolsRaw)
		if err == nil {
			var tools []schemas.ChatTool
			if err := json.Unmarshal(toolsBytes, &tools); err == nil {
				params.Tools = tools
			}
		}
	}

	if toolChoiceRaw, ok := openAIBody["tool_choice"]; ok {
		toolChoiceBytes, err := json.Marshal(toolChoiceRaw)
		if err == nil {
			var toolChoice schemas.ChatToolChoice
			if err := json.Unmarshal(toolChoiceBytes, &toolChoice); err == nil {
				params.ToolChoice = &toolChoice
			}
		}
	}

	if user, ok := openAIBody["user"].(string); ok && user != "" {
		params.User = &user
	}

	if len(params.ExtraParams) == 0 {
		params.ExtraParams = nil
	}

	return &schemas.BifrostChatRequest{
		Provider: provider,
		Model:    targetModel,
		Input:    input,
		Params:   params,
	}, nil
}

func bifrostChatResponseToOpenAI(resp *schemas.BifrostChatResponse) map[string]any {
	if resp == nil {
		return map[string]any{}
	}
	object := resp.Object
	if object == "" {
		object = "chat.completion"
	}
	result := map[string]any{
		"id":      resp.ID,
		"object":  object,
		"created": resp.Created,
		"model":   resp.Model,
	}

	choices := make([]any, 0, len(resp.Choices))
	for _, choice := range resp.Choices {
		item := map[string]any{
			"index": choice.Index,
		}
		if choice.FinishReason != nil {
			item["finish_reason"] = *choice.FinishReason
		} else {
			item["finish_reason"] = nil
		}
		if choice.ChatNonStreamResponseChoice != nil {
			item["message"] = chatMessageToMap(choice.ChatNonStreamResponseChoice.Message)
		}
		if choice.ChatStreamResponseChoice != nil {
			item["delta"] = chatDeltaToMap(choice.ChatStreamResponseChoice.Delta)
		}
		choices = append(choices, item)
	}
	result["choices"] = choices

	if resp.Usage != nil {
		result["usage"] = map[string]any{
			"prompt_tokens":     resp.Usage.PromptTokens,
			"completion_tokens": resp.Usage.CompletionTokens,
			"total_tokens":      resp.Usage.TotalTokens,
		}
	}
	return result
}

func ConvertToAnthropicResponseFromOpenAI(openAIResp map[string]any) map[string]any {
	return ConvertToAnthropicResponse(openAIResp)
}

func ConvertToGeminiResponseFromOpenAI(openAIResp map[string]any) map[string]any {
	candidates := make([]any, 0, 1)
	choices, _ := openAIResp["choices"].([]any)
	if len(choices) > 0 {
		choice, _ := choices[0].(map[string]any)
		if choice != nil {
			contentParts := make([]any, 0, 2)
			if message, ok := choice["message"].(map[string]any); ok {
				if text, ok := message["content"].(string); ok && text != "" {
					contentParts = append(contentParts, map[string]any{"text": text})
				}
				if toolCalls, ok := message["tool_calls"].([]any); ok {
					for _, tcRaw := range toolCalls {
						tc, ok := tcRaw.(map[string]any)
						if !ok {
							continue
						}
						function, _ := tc["function"].(map[string]any)
						args := map[string]any{}
						if argStr, ok := function["arguments"].(string); ok && argStr != "" {
							_ = json.Unmarshal([]byte(argStr), &args)
						}
						contentParts = append(contentParts, map[string]any{
							"functionCall": map[string]any{
								"name": function["name"],
								"args": args,
							},
						})
					}
				}
			}

			finishReason := "STOP"
			if reason, ok := choice["finish_reason"].(string); ok && reason != "" {
				switch reason {
				case "length":
					finishReason = "MAX_TOKENS"
				default:
					finishReason = "STOP"
				}
			}

			candidates = append(candidates, map[string]any{
				"content": map[string]any{
					"role":  "model",
					"parts": contentParts,
				},
				"finishReason": finishReason,
			})
		}
	}

	resp := map[string]any{
		"candidates": candidates,
	}
	if usage, ok := openAIResp["usage"].(map[string]any); ok {
		resp["usageMetadata"] = map[string]any{
			"promptTokenCount":     toInt(usage["prompt_tokens"]),
			"candidatesTokenCount": toInt(usage["completion_tokens"]),
			"totalTokenCount":      toInt(usage["total_tokens"]),
		}
	}
	return resp
}

func writeStreamChunk(
	w http.ResponseWriter,
	flusher http.Flusher,
	requestType string,
	openAIChunk map[string]any,
	convertToAnthropic func(string) []string,
) error {
	data, err := json.Marshal(openAIChunk)
	if err != nil {
		return err
	}
	chunkStr := string(data)

	switch requestType {
	case "anthropic-messages":
		lines := convertToAnthropic(chunkStr)
		for _, line := range lines {
			if _, err := fmt.Fprintf(w, "%s\n", line); err != nil {
				return err
			}
		}
		flusher.Flush()
		return nil
	case "gemini-stream-generate-content":
		geminiChunk := toGeminiStreamEventFromOpenAI(openAIChunk)
		if geminiChunk == nil {
			return nil
		}
		geminiData, err := json.Marshal(geminiChunk)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", string(geminiData)); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	default:
		if _, err := fmt.Fprintf(w, "data: %s\n\n", chunkStr); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}
}

func writeStreamDone(
	w http.ResponseWriter,
	flusher http.Flusher,
	requestType string,
	convertToAnthropic func(string) []string,
) error {
	switch requestType {
	case "anthropic-messages":
		lines := convertToAnthropic("[DONE]")
		for _, line := range lines {
			if _, err := fmt.Fprintf(w, "%s\n", line); err != nil {
				return err
			}
		}
		flusher.Flush()
		return nil
	case "gemini-stream-generate-content":
		if _, err := fmt.Fprintf(w, "data: {\"done\":true}\n\n"); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	default:
		if _, err := fmt.Fprintf(w, "data: [DONE]\n\n"); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}
}

func consumeOpenAIChunk(openAIChunk map[string]any, state *streamingState) bool {
	choices, _ := openAIChunk["choices"].([]any)
	if len(choices) == 0 {
		return false
	}
	choice, _ := choices[0].(map[string]any)
	if choice == nil {
		return false
	}

	seenToken := false

	if delta, ok := choice["delta"].(map[string]any); ok {
		if content, ok := delta["content"].(string); ok && content != "" {
			seenToken = true
			state.appendContent(content)
		}
		if thinking, ok := delta["reasoning"].(string); ok && thinking != "" {
			seenToken = true
			state.appendThinking(thinking)
		}
		if toolCalls, ok := delta["tool_calls"].([]any); ok && len(toolCalls) > 0 {
			seenToken = true
		}
	}
	if finishReason, ok := choice["finish_reason"].(string); ok && finishReason != "" {
		seenToken = true
	}

	return seenToken
}

func toGeminiStreamEventFromOpenAI(openAIChunk map[string]any) map[string]any {
	choices, _ := openAIChunk["choices"].([]any)
	if len(choices) == 0 {
		return nil
	}
	choice, _ := choices[0].(map[string]any)
	if choice == nil {
		return nil
	}

	delta, _ := choice["delta"].(map[string]any)
	parts := make([]any, 0, 2)
	if delta != nil {
		if content, ok := delta["content"].(string); ok && content != "" {
			parts = append(parts, map[string]any{"text": content})
		}
		if toolCalls, ok := delta["tool_calls"].([]any); ok {
			for _, tcRaw := range toolCalls {
				tc, ok := tcRaw.(map[string]any)
				if !ok {
					continue
				}
				fn, _ := tc["function"].(map[string]any)
				args := map[string]any{}
				if argStr, ok := fn["arguments"].(string); ok && argStr != "" {
					_ = json.Unmarshal([]byte(argStr), &args)
				}
				parts = append(parts, map[string]any{
					"functionCall": map[string]any{
						"name": fn["name"],
						"args": args,
					},
				})
			}
		}
	}
	if len(parts) == 0 {
		return nil
	}

	finishReason := ""
	if reason, ok := choice["finish_reason"].(string); ok && reason != "" {
		switch reason {
		case "length":
			finishReason = "MAX_TOKENS"
		default:
			finishReason = "STOP"
		}
	}

	candidate := map[string]any{
		"content": map[string]any{
			"role":  "model",
			"parts": parts,
		},
	}
	if finishReason != "" {
		candidate["finishReason"] = finishReason
	}

	event := map[string]any{
		"candidates": []any{candidate},
	}
	if usage, ok := openAIChunk["usage"].(map[string]any); ok {
		event["usageMetadata"] = map[string]any{
			"promptTokenCount":     toInt(usage["prompt_tokens"]),
			"candidatesTokenCount": toInt(usage["completion_tokens"]),
			"totalTokenCount":      toInt(usage["total_tokens"]),
		}
	}
	return event
}

func toProxyErrorFromBifrost(err *schemas.BifrostError) *ProxyError {
	if err == nil {
		return &ProxyError{Message: "unknown bifrost error", StatusCode: 502}
	}
	status := 502
	if err.StatusCode != nil && *err.StatusCode > 0 {
		status = *err.StatusCode
	}
	message := "bifrost request failed"
	if err.Error != nil && err.Error.Message != "" {
		message = err.Error.Message
	}
	log.Printf("[DEBUG bifrost] error: status=%d message=%s", status, message)
	if err.ExtraFields.RawResponse != nil {
		if b, e := json.Marshal(err.ExtraFields.RawResponse); e == nil {
			_ = os.WriteFile("/tmp/bifrost_error_resp.json", b, 0644)
			log.Printf("[DEBUG bifrost] raw_response written to /tmp/bifrost_error_resp.json")
		}
	}
	return &ProxyError{
		Message:    message,
		StatusCode: status,
	}
}

func extractRawPayloadForLog(rawRequest, rawResponse any) *string {
	if rawRequest == nil && rawResponse == nil {
		return nil
	}
	payload := map[string]any{
		"raw_request":  rawRequest,
		"raw_response": rawResponse,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	s := string(b)
	return &s
}

func newBifrostContext(parent context.Context, firstTokenTimeout int) (*schemas.BifrostContext, context.CancelFunc) {
	timeout := 120 * time.Second
	if firstTokenTimeout > 0 {
		timeout = time.Duration(firstTokenTimeout+60) * time.Second
	}
	return schemas.NewBifrostContextWithTimeout(parent, timeout)
}

func chatMessageToMap(message *schemas.ChatMessage) map[string]any {
	if message == nil {
		return map[string]any{"role": "assistant", "content": ""}
	}
	data, err := json.Marshal(message)
	if err != nil {
		return map[string]any{"role": "assistant", "content": ""}
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return map[string]any{"role": "assistant", "content": ""}
	}
	return result
}

func chatDeltaToMap(delta *schemas.ChatStreamResponseChoiceDelta) map[string]any {
	if delta == nil {
		return map[string]any{}
	}
	data, err := json.Marshal(delta)
	if err != nil {
		return map[string]any{}
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return map[string]any{}
	}
	return result
}

func cacheTokensFromUsage(usage *schemas.BifrostLLMUsage) (int, int) {
	if usage == nil {
		return 0, 0
	}
	cacheRead := 0
	cacheCreation := 0
	if usage.PromptTokensDetails != nil {
		cacheRead = usage.PromptTokensDetails.CachedTokens
	}
	if usage.CompletionTokensDetails != nil {
		cacheCreation = usage.CompletionTokensDetails.CachedTokens
	}
	return cacheRead, cacheCreation
}

func anthropicSystemText(raw any) string {
	switch v := raw.(type) {
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := block["text"].(string); ok {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "")
	default:
		return ""
	}
}

func anthropicAssistantContent(raw any) (string, []any) {
	if content, ok := raw.(string); ok {
		return content, nil
	}
	blocks, ok := raw.([]any)
	if !ok {
		return "", nil
	}

	textParts := make([]string, 0)
	toolCalls := make([]any, 0)
	callIndex := 0
	for _, blockRaw := range blocks {
		block, ok := blockRaw.(map[string]any)
		if !ok {
			continue
		}
		blockType, _ := block["type"].(string)
		switch blockType {
		case "text":
			if text, ok := block["text"].(string); ok {
				textParts = append(textParts, text)
			}
		case "tool_use":
			callID, _ := block["id"].(string)
			if callID == "" {
				callID = fmt.Sprintf("call_%d", callIndex)
			}
			callIndex++
			args := "{}"
			if input, ok := block["input"]; ok {
				if b, err := json.Marshal(input); err == nil {
					args = string(b)
				}
			}
			toolCalls = append(toolCalls, map[string]any{
				"id":   callID,
				"type": "function",
				"function": map[string]any{
					"name":      block["name"],
					"arguments": args,
				},
			})
		}
	}
	return strings.Join(textParts, ""), toolCalls
}

func anthropicUserContent(raw any) (string, []any) {
	if content, ok := raw.(string); ok {
		return content, nil
	}
	blocks, ok := raw.([]any)
	if !ok {
		return "", nil
	}
	textParts := make([]string, 0)
	toolMessages := make([]any, 0)
	for _, blockRaw := range blocks {
		block, ok := blockRaw.(map[string]any)
		if !ok {
			continue
		}
		blockType, _ := block["type"].(string)
		switch blockType {
		case "text":
			if text, ok := block["text"].(string); ok {
				textParts = append(textParts, text)
			}
		case "tool_result":
			content := ""
			if text, ok := block["content"].(string); ok {
				content = text
			} else if obj, ok := block["content"]; ok {
				if b, err := json.Marshal(obj); err == nil {
					content = string(b)
				}
			}
			toolMessages = append(toolMessages, map[string]any{
				"role":         "tool",
				"tool_call_id": block["tool_use_id"],
				"content":      content,
			})
		}
	}
	return strings.Join(textParts, ""), toolMessages
}

func geminiPartsToText(partsRaw any) string {
	parts, ok := partsRaw.([]any)
	if !ok {
		return ""
	}
	out := make([]string, 0, len(parts))
	for _, pRaw := range parts {
		part, ok := pRaw.(map[string]any)
		if !ok {
			continue
		}
		if text, ok := part["text"].(string); ok {
			out = append(out, text)
		}
	}
	return strings.Join(out, "")
}

func extractPlainText(raw any) string {
	switch v := raw.(type) {
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := block["text"].(string); ok {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "")
	default:
		return ""
	}
}

func readFloat(raw any) (float64, bool) {
	switch v := raw.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

func readInt(raw any) (int, bool) {
	switch v := raw.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	default:
		return 0, false
	}
}

func stopSequences(raw any) ([]string, bool) {
	switch v := raw.(type) {
	case string:
		if v == "" {
			return nil, false
		}
		return []string{v}, true
	case []string:
		return v, len(v) > 0
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out, len(out) > 0
	default:
		return nil, false
	}
}
