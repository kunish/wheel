package relay

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

// ProxyError represents an upstream proxy error with optional retry info.
type ProxyError struct {
	Message      string
	StatusCode   int
	RetryAfterMs int64
}

func (e *ProxyError) Error() string {
	return e.Message
}

// ProxyResult holds the result of a non-streaming proxy call.
type ProxyResult struct {
	Response            map[string]any
	InputTokens         int
	OutputTokens        int
	CacheReadTokens     int
	CacheCreationTokens int
	StatusCode          int
}

// StreamCompleteInfo holds usage info collected after a stream finishes.
type StreamCompleteInfo struct {
	InputTokens         int
	OutputTokens        int
	CacheReadTokens     int
	CacheCreationTokens int
	FirstTokenTime      int
	StatusCode          int
	ResponseContent     string
	ThinkingContent     string
}

const maxResponseContent = 10000

// extractCacheTokens extracts cache token counts from a response usage object.
func extractCacheTokens(data map[string]any, channelType types.OutboundType) (cacheRead, cacheCreation int) {
	if channelType == types.OutboundAnthropic {
		usage, _ := data["usage"].(map[string]any)
		cacheRead = toInt(usage["cache_read_input_tokens"])
		cacheCreation = toInt(usage["cache_creation_input_tokens"])
		return
	}
	// OpenAI: prompt_tokens_details.cached_tokens
	usage, _ := data["usage"].(map[string]any)
	if usage != nil {
		details, _ := usage["prompt_tokens_details"].(map[string]any)
		if details != nil {
			cacheRead = toInt(details["cached_tokens"])
		}
	}
	return
}

var quotaResetPattern = regexp.MustCompile(`quotaResetDelay["'\s:]+["']?([\d.]+)(ms|s)`)

// parseRetryDelay extracts retry delay from response headers or body.
func parseRetryDelay(resp *http.Response, body string) int64 {
	// 1. Check Retry-After header (seconds)
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if secs, err := strconv.ParseFloat(ra, 64); err == nil && secs > 0 {
			return int64(math.Ceil(secs * 1000))
		}
	}

	// 2. Parse quotaResetDelay from Google Cloud error body
	if matches := quotaResetPattern.FindStringSubmatch(body); len(matches) == 3 {
		val, err := strconv.ParseFloat(matches[1], 64)
		if err == nil {
			if matches[2] == "s" {
				return int64(math.Ceil(val * 1000))
			}
			return int64(math.Ceil(val))
		}
	}

	return 0
}

// ProxyNonStreaming performs a single non-streaming HTTP POST to the upstream.
func ProxyNonStreaming(
	upstreamUrl string,
	upstreamHeaders map[string]string,
	upstreamBody string,
	channelType types.OutboundType,
	passthrough bool,
) (*ProxyResult, error) {
	req, err := http.NewRequest("POST", upstreamUrl, strings.NewReader(upstreamBody))
	if err != nil {
		return nil, &ProxyError{Message: fmt.Sprintf("failed to create request: %v", err), StatusCode: 502}
	}
	for k, v := range upstreamHeaders {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, &ProxyError{Message: fmt.Sprintf("upstream request failed: %v", err), StatusCode: 502}
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ProxyError{Message: fmt.Sprintf("failed to read response: %v", err), StatusCode: 502}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errorText := string(bodyBytes)
		return nil, &ProxyError{
			Message:      fmt.Sprintf("Upstream error %d: %s", resp.StatusCode, errorText),
			StatusCode:   resp.StatusCode,
			RetryAfterMs: parseRetryDelay(resp, errorText),
		}
	}

	var data map[string]any
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return nil, &ProxyError{Message: fmt.Sprintf("failed to parse response: %v", err), StatusCode: 502}
	}

	cacheRead, cacheCreation := extractCacheTokens(data, channelType)

	// Passthrough mode: return raw Anthropic response
	if passthrough && channelType == types.OutboundAnthropic {
		usage, _ := data["usage"].(map[string]any)
		return &ProxyResult{
			Response:            data,
			InputTokens:         toInt(usage["input_tokens"]),
			OutputTokens:        toInt(usage["output_tokens"]),
			CacheReadTokens:     cacheRead,
			CacheCreationTokens: cacheCreation,
			StatusCode:          resp.StatusCode,
		}, nil
	}

	// Convert Anthropic → OpenAI if needed
	finalResponse := data
	if channelType == types.OutboundAnthropic {
		finalResponse = ConvertAnthropicResponse(data)
	}

	usage, _ := finalResponse["usage"].(map[string]any)
	return &ProxyResult{
		Response:            finalResponse,
		InputTokens:         toInt(usage["prompt_tokens"]),
		OutputTokens:        toInt(usage["completion_tokens"]),
		CacheReadTokens:     cacheRead,
		CacheCreationTokens: cacheCreation,
		StatusCode:          resp.StatusCode,
	}, nil
}

// StreamContentCallback is called periodically during streaming with accumulated content.
type StreamContentCallback func(thinking, response string)

// streamingState tracks state during SSE streaming.
type streamingState struct {
	firstTokenReceived  bool
	firstTokenTime      int
	inputTokens         int
	outputTokens        int
	cacheReadTokens     int
	cacheCreationTokens int
	responseContent     string
	thinkingContent     string
	onContent           StreamContentCallback
	lastNotifyLen       int
}

const streamNotifyThreshold = 100 // notify every 100 chars of new content

func (s *streamingState) maybeNotify() {
	if s.onContent == nil {
		return
	}
	totalLen := len(s.thinkingContent) + len(s.responseContent)
	if totalLen-s.lastNotifyLen >= streamNotifyThreshold {
		s.lastNotifyLen = totalLen
		s.onContent(s.thinkingContent, s.responseContent)
	}
}

func (s *streamingState) appendThinking(text string) {
	if len(s.thinkingContent) >= maxResponseContent {
		return
	}
	chunk := text
	if len(s.thinkingContent) == 0 {
		chunk = strings.TrimLeft(chunk, " \t\n\r")
	}
	if chunk == "" {
		return
	}
	s.thinkingContent += chunk
	if len(s.thinkingContent) > maxResponseContent {
		s.thinkingContent = s.thinkingContent[:maxResponseContent]
	}
	s.maybeNotify()
}

func (s *streamingState) appendContent(text string) {
	if len(s.responseContent) >= maxResponseContent {
		return
	}
	chunk := text
	if len(s.responseContent) == 0 {
		chunk = strings.TrimLeft(chunk, " \t\n\r")
	}
	if chunk == "" {
		return
	}
	s.responseContent += chunk
	if len(s.responseContent) > maxResponseContent {
		s.responseContent = s.responseContent[:maxResponseContent]
	}
	s.maybeNotify()
}

// ProxyStreaming performs an SSE streaming proxy, writing directly to the http.ResponseWriter.
// It handles protocol conversion between Anthropic SSE and OpenAI SSE formats.
func ProxyStreaming(
	w http.ResponseWriter,
	upstreamUrl string,
	upstreamHeaders map[string]string,
	upstreamBody string,
	channelType types.OutboundType,
	firstTokenTimeout int,
	passthrough bool,
	anthropicInbound bool,
	onContent StreamContentCallback,
) (*StreamCompleteInfo, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, &ProxyError{Message: "streaming not supported", StatusCode: 500}
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Create upstream request with optional timeout for first token
	ctx := context.Background()
	var cancel context.CancelFunc
	if firstTokenTimeout > 0 {
		ctx, cancel = context.WithCancel(ctx)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, "POST", upstreamUrl, strings.NewReader(upstreamBody))
	if err != nil {
		return nil, &ProxyError{Message: fmt.Sprintf("failed to create request: %v", err), StatusCode: 502}
	}
	for k, v := range upstreamHeaders {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, &ProxyError{Message: fmt.Sprintf("upstream request failed: %v", err), StatusCode: 502}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errorText := string(bodyBytes)
		return nil, &ProxyError{
			Message:      fmt.Sprintf("Upstream error %d: %s", resp.StatusCode, errorText),
			StatusCode:   resp.StatusCode,
			RetryAfterMs: parseRetryDelay(resp, errorText),
		}
	}

	startTime := time.Now()
	state := &streamingState{onContent: onContent}

	// First token timeout timer
	var timeoutTimer *time.Timer
	timeoutCh := make(chan struct{})
	if firstTokenTimeout > 0 {
		timeoutTimer = time.AfterFunc(time.Duration(firstTokenTimeout)*time.Second, func() {
			close(timeoutCh)
		})
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

	// Determine converter
	var convertChunk func(string) *anthropicSSEResult
	if !passthrough && channelType == types.OutboundAnthropic {
		convertChunk = createAnthropicSSEConverter()
	}

	// OpenAI SSE → Anthropic SSE converter for anthropic inbound + openai outbound
	var convertToAnthropic func(string) []string
	if anthropicInbound && channelType != types.OutboundAnthropic {
		convertToAnthropic = createOpenAIToAnthropicSSEConverter()
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		// Check first token timeout
		if !state.firstTokenReceived {
			select {
			case <-timeoutCh:
				return nil, &ProxyError{Message: "First token timeout exceeded", StatusCode: 504}
			default:
			}
		}

		line := scanner.Text()

		if passthrough && channelType == types.OutboundAnthropic {
			processAnthropicPassthrough(line, state, markFirstToken)
			fmt.Fprintf(w, "%s\n", line)
			flusher.Flush()
		} else if channelType == types.OutboundAnthropic && convertChunk != nil {
			processAnthropicConverted(line, convertChunk, state, markFirstToken, w, flusher)
		} else if convertToAnthropic != nil {
			processOpenAIToAnthropic(line, convertToAnthropic, state, markFirstToken, w, flusher)
		} else {
			processOpenAI(line, state, markFirstToken, w, flusher)
		}
	}

	return &StreamCompleteInfo{
		InputTokens:         state.inputTokens,
		OutputTokens:        state.outputTokens,
		CacheReadTokens:     state.cacheReadTokens,
		CacheCreationTokens: state.cacheCreationTokens,
		FirstTokenTime:      state.firstTokenTime,
		StatusCode:          resp.StatusCode,
		ResponseContent:     state.responseContent,
		ThinkingContent:     state.thinkingContent,
	}, nil
}

// extractThinking parses an Anthropic SSE JSON payload and accumulates thinking content.
func extractThinking(jsonStr string, state *streamingState) {
	var ev map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &ev); err != nil {
		return
	}
	evType, _ := ev["type"].(string)
	if evType != "content_block_delta" {
		return
	}
	delta, ok := ev["delta"].(map[string]any)
	if !ok {
		return
	}
	if delta["type"] == "thinking_delta" {
		if text, ok := delta["thinking"].(string); ok {
			state.appendThinking(text)
		}
	}
}

// processAnthropicPassthrough handles SSE lines in Anthropic passthrough mode.
func processAnthropicPassthrough(line string, state *streamingState, markFirstToken func()) {
	if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
		return
	}

	var ev map[string]any
	if err := json.Unmarshal([]byte(line[6:]), &ev); err != nil {
		return
	}

	if !state.firstTokenReceived {
		evType, _ := ev["type"].(string)
		if evType == "message_start" || evType == "content_block_start" || evType == "content_block_delta" {
			markFirstToken()
		}
	}

	evType, _ := ev["type"].(string)

	if evType == "message_start" {
		if msg, ok := ev["message"].(map[string]any); ok {
			if usage, ok := msg["usage"].(map[string]any); ok {
				state.cacheReadTokens = toInt(usage["cache_read_input_tokens"])
				state.cacheCreationTokens = toInt(usage["cache_creation_input_tokens"])
			}
		}
	}

	if evType == "message_delta" {
		if usage, ok := ev["usage"].(map[string]any); ok {
			inTok := toInt(usage["input_tokens"])
			outTok := toInt(usage["output_tokens"])
			if inTok > 0 || outTok > 0 {
				if inTok > 0 {
					state.inputTokens = inTok
				}
				if outTok > 0 {
					state.outputTokens = outTok
				}
			}
		}
	}

	if evType == "content_block_delta" {
		if delta, ok := ev["delta"].(map[string]any); ok {
			if delta["type"] == "text_delta" {
				if text, ok := delta["text"].(string); ok {
					state.appendContent(text)
				}
			}
			if delta["type"] == "thinking_delta" {
				if text, ok := delta["thinking"].(string); ok {
					state.appendThinking(text)
				}
			}
		}
	}
}

// processAnthropicConverted handles SSE lines by converting Anthropic → OpenAI format.
func processAnthropicConverted(
	line string,
	convertChunk func(string) *anthropicSSEResult,
	state *streamingState,
	markFirstToken func(),
	w http.ResponseWriter,
	flusher http.Flusher,
) {
	if !strings.HasPrefix(line, "data: ") {
		return
	}

	payload := line[6:]

	// Extract thinking content before conversion (converter drops thinking_delta)
	extractThinking(payload, state)

	chunk := convertChunk(payload)
	if chunk == nil {
		return
	}

	markFirstToken()

	if chunk.cacheReadTokens > 0 {
		state.cacheReadTokens = chunk.cacheReadTokens
	}
	if chunk.cacheCreationTokens > 0 {
		state.cacheCreationTokens = chunk.cacheCreationTokens
	}

	if chunk.done {
		if chunk.inputTokens > 0 || chunk.outputTokens > 0 {
			state.inputTokens = chunk.inputTokens
			state.outputTokens = chunk.outputTokens
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	} else if chunk.data != nil {
		// Accumulate text content
		if choices, ok := chunk.data["choices"].([]any); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]any); ok {
				if delta, ok := choice["delta"].(map[string]any); ok {
					if content, ok := delta["content"].(string); ok {
						state.appendContent(content)
					}
				}
			}
		}
		dataJSON, _ := json.Marshal(chunk.data)
		fmt.Fprintf(w, "data: %s\n\n", dataJSON)
		flusher.Flush()
	}
}

// processOpenAI handles SSE lines in OpenAI passthrough mode.
func processOpenAI(
	line string,
	state *streamingState,
	markFirstToken func(),
	w http.ResponseWriter,
	flusher http.Flusher,
) {
	if strings.HasPrefix(line, "data: ") && line != "data: [DONE]" {
		markFirstToken()

		var obj map[string]any
		if err := json.Unmarshal([]byte(line[6:]), &obj); err == nil {
			if usage, ok := obj["usage"].(map[string]any); ok {
				state.inputTokens = toIntOr(usage["prompt_tokens"], state.inputTokens)
				state.outputTokens = toIntOr(usage["completion_tokens"], state.outputTokens)
				if details, ok := usage["prompt_tokens_details"].(map[string]any); ok {
					if ct := toInt(details["cached_tokens"]); ct > 0 {
						state.cacheReadTokens = ct
					}
				}
			}
			// Accumulate text content
			if choices, ok := obj["choices"].([]any); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]any); ok {
					if delta, ok := choice["delta"].(map[string]any); ok {
						if content, ok := delta["content"].(string); ok {
							state.appendContent(content)
						}
						if reasoning, ok := delta["reasoning_content"].(string); ok {
							state.appendThinking(reasoning)
						}
					}
				}
			}
		}
	}

	fmt.Fprintf(w, "%s\n", line)
	flusher.Flush()
}

func toIntOr(v any, fallback int) int {
	n := toInt(v)
	if n > 0 {
		return n
	}
	return fallback
}

// ── Anthropic SSE Converter ──────────────────────────────────────

type anthropicSSEResult struct {
	done                bool
	data                map[string]any
	inputTokens         int
	outputTokens        int
	cacheReadTokens     int
	cacheCreationTokens int
}

func mapStopReason(reason string) *string {
	var r string
	switch reason {
	case "end_turn", "stop_sequence":
		r = "stop"
	case "max_tokens":
		r = "length"
	default:
		return nil
	}
	return &r
}

// createAnthropicSSEConverter returns a stateful converter from Anthropic SSE to OpenAI SSE format.
func createAnthropicSSEConverter() func(string) *anthropicSSEResult {
	msgId := "chatcmpl-unknown"
	msgModel := ""
	toolCallIndex := 0
	blockMap := make(map[int]struct {
		blockType    string
		toolCallIdx int
	})

	makeChunk := func(choices []any, extra *anthropicSSEResult) *anthropicSSEResult {
		result := &anthropicSSEResult{
			data: map[string]any{
				"id":      msgId,
				"object":  "chat.completion.chunk",
				"created": float64(currentUnixSec()),
				"model":   msgModel,
				"choices": choices,
			},
		}
		if extra != nil {
			result.cacheReadTokens = extra.cacheReadTokens
			result.cacheCreationTokens = extra.cacheCreationTokens
		}
		return result
	}

	return func(jsonStr string) *anthropicSSEResult {
		var event map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &event); err != nil {
			return nil
		}

		evType, _ := event["type"].(string)

		switch evType {
		case "message_start":
			msg, _ := event["message"].(map[string]any)
			if msg != nil {
				if id, ok := msg["id"].(string); ok {
					msgId = id
				}
				if model, ok := msg["model"].(string); ok {
					msgModel = model
				}
			}
			var cr, cc int
			if msg != nil {
				if usage, ok := msg["usage"].(map[string]any); ok {
					cr = toInt(usage["cache_read_input_tokens"])
					cc = toInt(usage["cache_creation_input_tokens"])
				}
			}
			return makeChunk(
				[]any{map[string]any{
					"index":         0,
					"delta":         map[string]any{"role": "assistant", "content": ""},
					"finish_reason": nil,
				}},
				&anthropicSSEResult{cacheReadTokens: cr, cacheCreationTokens: cc},
			)

		case "content_block_start":
			idx := toInt(event["index"])
			block, _ := event["content_block"].(map[string]any)
			if block == nil {
				return nil
			}
			blockType, _ := block["type"].(string)

			if blockType == "tool_use" {
				tcIdx := toolCallIndex
				toolCallIndex++
				blockMap[idx] = struct {
					blockType   string
					toolCallIdx int
				}{"tool_use", tcIdx}

				return makeChunk([]any{map[string]any{
					"index": 0,
					"delta": map[string]any{
						"tool_calls": []any{map[string]any{
							"index":    tcIdx,
							"id":       block["id"],
							"type":     "function",
							"function": map[string]any{"name": block["name"], "arguments": ""},
						}},
					},
					"finish_reason": nil,
				}}, nil)
			}

			blockMap[idx] = struct {
				blockType   string
				toolCallIdx int
			}{blockType, 0}
			return nil

		case "content_block_delta":
			idx := toInt(event["index"])
			delta, _ := event["delta"].(map[string]any)
			if delta == nil {
				return nil
			}
			deltaType, _ := delta["type"].(string)

			if deltaType == "text_delta" {
				text, _ := delta["text"].(string)
				return makeChunk([]any{map[string]any{
					"index":         0,
					"delta":         map[string]any{"content": text},
					"finish_reason": nil,
				}}, nil)
			}

			if deltaType == "input_json_delta" {
				info, ok := blockMap[idx]
				if ok && info.blockType == "tool_use" {
					partialJSON, _ := delta["partial_json"].(string)
					return makeChunk([]any{map[string]any{
						"index": 0,
						"delta": map[string]any{
							"tool_calls": []any{map[string]any{
								"index":    info.toolCallIdx,
								"function": map[string]any{"arguments": partialJSON},
							}},
						},
						"finish_reason": nil,
					}}, nil)
				}
			}

			// thinking_delta — silently drop
			return nil

		case "message_delta":
			delta, _ := event["delta"].(map[string]any)
			usage, _ := event["usage"].(map[string]any)

			stopReason, _ := delta["stop_reason"].(string)
			var finishReason any
			if stopReason == "tool_use" {
				finishReason = "tool_calls"
			} else if r := mapStopReason(stopReason); r != nil {
				finishReason = *r
			}

			result := &anthropicSSEResult{
				data: map[string]any{
					"id":      msgId,
					"object":  "chat.completion.chunk",
					"created": float64(currentUnixSec()),
					"model":   msgModel,
					"choices": []any{map[string]any{
						"index":         0,
						"delta":         map[string]any{},
						"finish_reason": finishReason,
					}},
				},
			}

			if usage != nil {
				inTok := toInt(usage["input_tokens"])
				outTok := toInt(usage["output_tokens"])
				if inTok > 0 || outTok > 0 {
					result.inputTokens = inTok
					result.outputTokens = outTok
				}
			}
			return result

		case "message_stop":
			return &anthropicSSEResult{done: true}

		default:
			return nil
		}
	}
}

// ── OpenAI SSE → Anthropic SSE Converter ──────────────────────────

// createOpenAIToAnthropicSSEConverter returns a stateful converter
// from OpenAI SSE chunks to Anthropic SSE event lines.
func createOpenAIToAnthropicSSEConverter() func(string) []string {
	started := false
	contentBlockOpen := false
	msgId := "msg_unknown"
	msgModel := ""

	return func(jsonStr string) []string {
		if jsonStr == "[DONE]" {
			var lines []string
			if contentBlockOpen {
				evt := map[string]any{
					"type":  "content_block_stop",
					"index": 0,
				}
				b, _ := json.Marshal(evt)
				lines = append(lines,
					"event: content_block_stop",
					"data: "+string(b),
					"",
				)
				contentBlockOpen = false
			}
			delta := map[string]any{
				"type":  "message_delta",
				"delta": map[string]any{"stop_reason": "end_turn"},
				"usage": map[string]any{"output_tokens": 0},
			}
			b, _ := json.Marshal(delta)
			lines = append(lines,
				"event: message_delta",
				"data: "+string(b),
				"",
			)
			stop := map[string]any{"type": "message_stop"}
			b, _ = json.Marshal(stop)
			lines = append(lines,
				"event: message_stop",
				"data: "+string(b),
				"",
			)
			return lines
		}

		var obj map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &obj); err != nil {
			return nil
		}

		if id, ok := obj["id"].(string); ok {
			msgId = id
		}
		if model, ok := obj["model"].(string); ok {
			msgModel = model
		}

		var lines []string

		if !started {
			started = true
			inputTokens := 0
			if usage, ok := obj["usage"].(map[string]any); ok {
				inputTokens = toInt(usage["prompt_tokens"])
			}
			lines = append(lines, openaiToAnthropicStart(msgId, msgModel, inputTokens)...)
		}

		choices, _ := obj["choices"].([]any)
		if len(choices) == 0 {
			if len(lines) > 0 {
				return lines
			}
			return nil
		}

		choice, _ := choices[0].(map[string]any)
		if choice == nil {
			return lines
		}

		delta, _ := choice["delta"].(map[string]any)
		finishReason, _ := choice["finish_reason"].(string)

		if delta != nil {
			content, _ := delta["content"].(string)

			if content != "" {
				if !contentBlockOpen {
					lines = append(lines,
						openaiToAnthropicBlockStart(0, "text")...)
					contentBlockOpen = true
				}
				evt := map[string]any{
					"type":  "content_block_delta",
					"index": 0,
					"delta": map[string]any{
						"type": "text_delta",
						"text": content,
					},
				}
				b, _ := json.Marshal(evt)
				lines = append(lines,
					"event: content_block_delta",
					"data: "+string(b),
					"",
				)
			}
		}

		if finishReason != "" {
			if contentBlockOpen {
				evt := map[string]any{
					"type":  "content_block_stop",
					"index": 0,
				}
				b, _ := json.Marshal(evt)
				lines = append(lines,
					"event: content_block_stop",
					"data: "+string(b),
					"",
				)
				contentBlockOpen = false
			}

			usage, _ := obj["usage"].(map[string]any)
			inTok := toInt(usage["prompt_tokens"])
			outTok := toInt(usage["completion_tokens"])

			stopReason := mapOpenAIFinishReason(finishReason)
			md := map[string]any{
				"type":  "message_delta",
				"delta": map[string]any{"stop_reason": stopReason},
				"usage": map[string]any{
					"input_tokens":  inTok,
					"output_tokens": outTok,
				},
			}
			b, _ := json.Marshal(md)
			lines = append(lines,
				"event: message_delta",
				"data: "+string(b),
				"",
			)

			stop := map[string]any{"type": "message_stop"}
			b, _ = json.Marshal(stop)
			lines = append(lines,
				"event: message_stop",
				"data: "+string(b),
				"",
			)
		}

		if len(lines) > 0 {
			return lines
		}
		return nil
	}
}

func openaiToAnthropicStart(id, model string, inputTokens int) []string {
	msg := map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            id,
			"type":          "message",
			"role":          "assistant",
			"model":         model,
			"content":       []any{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage":         map[string]any{"input_tokens": inputTokens, "output_tokens": 0},
		},
	}
	b, _ := json.Marshal(msg)
	return []string{
		"event: message_start",
		"data: " + string(b),
		"",
	}
}

func openaiToAnthropicBlockStart(index int, blockType string) []string {
	evt := map[string]any{
		"type":  "content_block_start",
		"index": index,
		"content_block": map[string]any{
			"type": blockType,
			"text": "",
		},
	}
	b, _ := json.Marshal(evt)
	return []string{
		"event: content_block_start",
		"data: " + string(b),
		"",
	}
}

func processOpenAIToAnthropic(
	line string,
	convert func(string) []string,
	state *streamingState,
	markFirstToken func(),
	w http.ResponseWriter,
	flusher http.Flusher,
) {
	if !strings.HasPrefix(line, "data: ") {
		return
	}
	data := strings.TrimPrefix(line, "data: ")

	var obj map[string]any
	if data != "[DONE]" {
		if err := json.Unmarshal([]byte(data), &obj); err == nil {
			if usage, ok := obj["usage"].(map[string]any); ok {
				state.inputTokens = toIntOr(
					usage["prompt_tokens"], state.inputTokens)
				state.outputTokens = toIntOr(
					usage["completion_tokens"], state.outputTokens)
			}
			if choices, ok := obj["choices"].([]any); ok && len(choices) > 0 {
				if ch, ok := choices[0].(map[string]any); ok {
					if d, ok := ch["delta"].(map[string]any); ok {
						if c, ok := d["content"].(string); ok {
							state.appendContent(c)
						}
					}
				}
			}
		}
	}

	markFirstToken()

	converted := convert(data)
	if converted == nil {
		return
	}
	for _, l := range converted {
		fmt.Fprintf(w, "%s\n", l)
	}
	flusher.Flush()
}
