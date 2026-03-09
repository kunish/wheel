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
	Headers      http.Header
}

func (e *ProxyError) Error() string {
	return e.Message
}

// IsRetryableStatusCode reports whether a proxy status should be retried.
func IsRetryableStatusCode(statusCode int) bool {
	if statusCode <= 0 {
		return true
	}
	if statusCode == 429 || statusCode == 408 || statusCode == 409 {
		return true
	}
	if statusCode >= 500 {
		return true
	}
	return false
}

// IsRetryableProxyError reports whether a proxy error should be retried.
func IsRetryableProxyError(err error) bool {
	pe, ok := err.(*ProxyError)
	if !ok {
		return false
	}
	return IsRetryableStatusCode(pe.StatusCode)
}

// ProxyResult holds the result of a non-streaming proxy call.
type ProxyResult struct {
	Response            map[string]any
	InputTokens         int
	OutputTokens        int
	CacheReadTokens     int
	CacheCreationTokens int
	StatusCode          int
	UpstreamHeaders     http.Header
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
	UpstreamHeaders     http.Header
}

// ToResponseBody constructs a synthetic OpenAI chat completion response from accumulated stream data.
// This allows PostHook plugins (e.g. SemanticCache) to access the complete response body
// even for streaming requests.
func (s *StreamCompleteInfo) ToResponseBody(model string) map[string]any {
	message := map[string]any{
		"role":    "assistant",
		"content": s.ResponseContent,
	}
	if s.ThinkingContent != "" {
		message["reasoning_content"] = s.ThinkingContent
	}

	return map[string]any{
		"id":      fmt.Sprintf("chatcmpl-stream-%d", currentUnixSec()),
		"object":  "chat.completion",
		"created": float64(currentUnixSec()),
		"model":   model,
		"choices": []any{
			map[string]any{
				"index":         0,
				"message":       message,
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     s.InputTokens,
			"completion_tokens": s.OutputTokens,
			"total_tokens":      s.InputTokens + s.OutputTokens,
		},
	}
}

// extractCacheTokens extracts cache token counts from a response usage object.
func extractCacheTokens(data map[string]any, channelType types.OutboundType) (cacheRead, cacheCreation int) {
	switch channelType {
	case types.OutboundAnthropic, types.OutboundBedrock:
		usage, _ := data["usage"].(map[string]any)
		cacheRead = toInt(usage["cache_read_input_tokens"])
		cacheCreation = toInt(usage["cache_creation_input_tokens"])
	case types.OutboundGemini, types.OutboundVertex:
		usage, _ := data["usageMetadata"].(map[string]any)
		cacheRead = toInt(usage["cachedContentTokenCount"])
	case types.OutboundCohere:
		usage, _ := data["usage"].(map[string]any)
		if tokens, ok := usage["tokens"].(map[string]any); ok {
			cacheRead = toInt(tokens["cached_tokens"])
		}
	default:
		usage, _ := data["usage"].(map[string]any)
		if usage != nil {
			details, _ := usage["prompt_tokens_details"].(map[string]any)
			if details != nil {
				cacheRead = toInt(details["cached_tokens"])
			}
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
		if t, err := http.ParseTime(ra); err == nil {
			delayMs := int64(math.Ceil(float64(time.Until(t)) / float64(time.Millisecond)))
			if delayMs > 0 {
				return delayMs
			}
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
	client *http.Client,
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

	resp, err := client.Do(req)
	if err != nil {
		return nil, &ProxyError{Message: fmt.Sprintf("upstream request failed: %v", err), StatusCode: 502}
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024)) // 50MB max response
	if err != nil {
		return nil, &ProxyError{Message: fmt.Sprintf("failed to read response: %v", err), StatusCode: 502}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errorText := string(bodyBytes)
		return nil, &ProxyError{
			Message:      fmt.Sprintf("Upstream error %d: %s", resp.StatusCode, errorText),
			StatusCode:   resp.StatusCode,
			RetryAfterMs: parseRetryDelay(resp, errorText),
			Headers:      resp.Header.Clone(),
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
			UpstreamHeaders:     resp.Header.Clone(),
		}, nil
	}

	// Convert upstream response → OpenAI if needed
	finalResponse := data
	switch channelType {
	case types.OutboundAnthropic, types.OutboundBedrock:
		finalResponse = ConvertAnthropicResponse(data)
	case types.OutboundGemini, types.OutboundVertex:
		finalResponse = ConvertGeminiResponse(data)
	case types.OutboundCohere:
		finalResponse = ConvertCohereResponse(data)
	}

	usage, _ := finalResponse["usage"].(map[string]any)
	return &ProxyResult{
		Response:            finalResponse,
		InputTokens:         toInt(usage["prompt_tokens"]),
		OutputTokens:        toInt(usage["completion_tokens"]),
		CacheReadTokens:     cacheRead,
		CacheCreationTokens: cacheCreation,
		StatusCode:          resp.StatusCode,
		UpstreamHeaders:     resp.Header.Clone(),
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
	chunk := text
	if len(s.thinkingContent) == 0 {
		chunk = strings.TrimLeft(chunk, " \t\n\r")
	}
	if chunk == "" {
		return
	}
	s.thinkingContent += chunk
	s.maybeNotify()
}

func (s *streamingState) appendContent(text string) {
	chunk := text
	if len(s.responseContent) == 0 {
		chunk = strings.TrimLeft(chunk, " \t\n\r")
	}
	if chunk == "" {
		return
	}
	s.responseContent += chunk
	s.maybeNotify()
}

// ProxyStreaming performs an SSE streaming proxy and writes directly to http.ResponseWriter.
// It handles provider-specific protocol conversions (Anthropic/Bedrock, Gemini/Vertex,
// Cohere, and OpenAI<->Anthropic where applicable).
// clientCtx should be the request context (e.g. c.Request.Context()) so that
// client disconnection automatically cancels the upstream read loop.
func ProxyStreaming(
	w http.ResponseWriter,
	clientCtx context.Context,
	httpClient *http.Client,
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

	// Derive upstream context from the client request context so that
	// client disconnection (e.g. ESC in Claude Code) cancels the upstream read.
	ctx, cancel := context.WithCancel(clientCtx)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", upstreamUrl, strings.NewReader(upstreamBody))
	if err != nil {
		return nil, &ProxyError{Message: fmt.Sprintf("failed to create request: %v", err), StatusCode: 502}
	}
	for k, v := range upstreamHeaders {
		req.Header.Set(k, v)
	}

	startTime := time.Now()

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, &ProxyError{Message: fmt.Sprintf("upstream request failed: %v", err), StatusCode: 502}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024)) // 1MB max error body
		errorText := string(bodyBytes)
		return nil, &ProxyError{
			Message:      fmt.Sprintf("Upstream error %d: %s", resp.StatusCode, errorText),
			StatusCode:   resp.StatusCode,
			RetryAfterMs: parseRetryDelay(resp, errorText),
			Headers:      resp.Header.Clone(),
		}
	}

	// Forward upstream response headers before setting SSE headers
	ForwardResponseHeaders(w, resp)

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

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
	if !passthrough && (channelType == types.OutboundAnthropic || channelType == types.OutboundBedrock) {
		convertChunk = createAnthropicSSEConverter()
	}

	// Gemini/Vertex SSE → OpenAI SSE converter
	var convertGemini func(string) *anthropicSSEResult
	if channelType == types.OutboundGemini || channelType == types.OutboundVertex {
		convertGemini = createGeminiSSEConverter()
	}

	// Cohere SSE → OpenAI SSE converter
	var convertCohere func(string) *anthropicSSEResult
	if channelType == types.OutboundCohere {
		convertCohere = createCohereSSEConverter()
	}

	// OpenAI SSE → Anthropic SSE converter for anthropic inbound + openai outbound
	var convertToAnthropic func(string) []string
	if anthropicInbound && channelType != types.OutboundAnthropic && channelType != types.OutboundGemini &&
		channelType != types.OutboundBedrock && channelType != types.OutboundVertex && channelType != types.OutboundCohere {
		convertToAnthropic = createOpenAIToAnthropicSSEConverter()
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		// Check if the client has disconnected
		select {
		case <-ctx.Done():
			return nil, &ProxyError{Message: "Client disconnected", StatusCode: 499}
		default:
		}

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
			if _, err := fmt.Fprintf(w, "%s\n", line); err != nil {
				return nil, &ProxyError{Message: "Client disconnected", StatusCode: 499}
			}
			flusher.Flush()
		} else if (channelType == types.OutboundAnthropic || channelType == types.OutboundBedrock) && convertChunk != nil {
			processAnthropicConverted(line, convertChunk, state, markFirstToken, w, flusher)
		} else if (channelType == types.OutboundGemini || channelType == types.OutboundVertex) && convertGemini != nil {
			processConvertedSSE(line, convertGemini, state, markFirstToken, w, flusher)
		} else if channelType == types.OutboundCohere && convertCohere != nil {
			processConvertedSSE(line, convertCohere, state, markFirstToken, w, flusher)
		} else if convertToAnthropic != nil {
			processOpenAIToAnthropic(line, convertToAnthropic, state, markFirstToken, w, flusher)
		} else {
			processOpenAI(line, state, markFirstToken, w, flusher)
		}
	}

	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			return nil, &ProxyError{Message: "Client disconnected", StatusCode: 499}
		}
		return nil, &ProxyError{Message: fmt.Sprintf("failed to read stream: %v", err), StatusCode: 502}
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
		UpstreamHeaders:     resp.Header.Clone(),
	}, nil
}
