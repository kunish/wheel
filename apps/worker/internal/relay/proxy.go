package relay

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

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

	evType, _ := ev["type"].(string)

	if !state.firstTokenReceived {
		if evType == "message_start" || evType == "content_block_start" || evType == "content_block_delta" {
			markFirstToken()
		}
	}

	if evType == "message_start" {
		if msg, ok := ev["message"].(map[string]any); ok {
			if usage, ok := msg["usage"].(map[string]any); ok {
				if inTok := toInt(usage["input_tokens"]); inTok > 0 {
					state.inputTokens = inTok
				}
				state.cacheReadTokens = toInt(usage["cache_read_input_tokens"])
				state.cacheCreationTokens = toInt(usage["cache_creation_input_tokens"])
			}
		}
	}

	if evType == "message_delta" {
		if usage, ok := ev["usage"].(map[string]any); ok {
			inTok := toInt(usage["input_tokens"])
			outTok := toInt(usage["output_tokens"])
			if inTok > 0 {
				state.inputTokens = inTok
			}
			if outTok > 0 {
				state.outputTokens = outTok
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

	if chunk.inputTokens > 0 {
		state.inputTokens = chunk.inputTokens
	}
	if chunk.outputTokens > 0 {
		state.outputTokens = chunk.outputTokens
	}

	if chunk.done {
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
		blockType   string
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
			result.inputTokens = extra.inputTokens
			result.outputTokens = extra.outputTokens
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
			var cr, cc, inTok int
			if msg != nil {
				if usage, ok := msg["usage"].(map[string]any); ok {
					cr = toInt(usage["cache_read_input_tokens"])
					cc = toInt(usage["cache_creation_input_tokens"])
					inTok = toInt(usage["input_tokens"])
				}
			}
			return makeChunk(
				[]any{map[string]any{
					"index":         0,
					"delta":         map[string]any{"role": "assistant", "content": ""},
					"finish_reason": nil,
				}},
				&anthropicSSEResult{cacheReadTokens: cr, cacheCreationTokens: cc, inputTokens: inTok},
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
	messageStopped := false
	msgId := "msg_unknown"
	msgModel := ""
	blockKeyToIndex := make(map[string]int)
	nextBlockIndex := 0
	openBlock := make(map[int]bool)
	nextToolOrdinal := 0

	type toolBlockState struct {
		id          string
		name        string
		started     bool
		pendingArgs string
		blockIdx    int
	}
	toolBlocks := make(map[string]*toolBlockState)

	getBlockIndex := func(key string) int {
		if idx, ok := blockKeyToIndex[key]; ok {
			return idx
		}
		idx := nextBlockIndex
		nextBlockIndex++
		blockKeyToIndex[key] = idx
		return idx
	}

	closeOpenBlocks := func(lines []string) []string {
		if len(openBlock) == 0 {
			return lines
		}
		indexes := make([]int, 0, len(openBlock))
		for idx := range openBlock {
			indexes = append(indexes, idx)
		}
		sort.Ints(indexes)
		for _, idx := range indexes {
			evt := map[string]any{
				"type":  "content_block_stop",
				"index": idx,
			}
			b, _ := json.Marshal(evt)
			lines = append(lines,
				"event: content_block_stop",
				"data: "+string(b),
				"",
			)
			delete(openBlock, idx)
		}
		return lines
	}

	resolveToolKey := func(tc map[string]any) string {
		if tc == nil {
			key := fmt.Sprintf("ord:%d", nextToolOrdinal)
			nextToolOrdinal++
			return key
		}
		if rawIdx, ok := tc["index"]; ok {
			return fmt.Sprintf("idx:%d", toInt(rawIdx))
		}
		if id, ok := tc["id"].(string); ok && id != "" {
			return "id:" + id
		}
		key := fmt.Sprintf("ord:%d", nextToolOrdinal)
		nextToolOrdinal++
		return key
	}

	emitToolDelta := func(lines []string, blockIdx int, args string) []string {
		if args == "" {
			return lines
		}
		deltaEvent := map[string]any{
			"type":  "content_block_delta",
			"index": blockIdx,
			"delta": map[string]any{
				"type":         "input_json_delta",
				"partial_json": args,
			},
		}
		b, _ := json.Marshal(deltaEvent)
		lines = append(lines,
			"event: content_block_delta",
			"data: "+string(b),
			"",
		)
		return lines
	}

	return func(jsonStr string) []string {
		if jsonStr == "[DONE]" {
			if messageStopped {
				return nil
			}
			var lines []string
			lines = closeOpenBlocks(lines)
			delta := map[string]any{
				"type":  "message_delta",
				"delta": map[string]any{"stop_reason": "end_turn", "stop_sequence": nil},
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
			messageStopped = true
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
			reasoning, _ := delta["reasoning"].(string)

			if reasoning != "" {
				thinkIdx := getBlockIndex("thinking")
				if !openBlock[thinkIdx] {
					lines = append(lines,
						openaiToAnthropicBlockStart(thinkIdx, "thinking")...)
					openBlock[thinkIdx] = true
				}
				evt := map[string]any{
					"type":  "content_block_delta",
					"index": thinkIdx,
					"delta": map[string]any{
						"type":     "thinking_delta",
						"thinking": reasoning,
					},
				}
				b, _ := json.Marshal(evt)
				lines = append(lines,
					"event: content_block_delta",
					"data: "+string(b),
					"",
				)
			}

			content, _ := delta["content"].(string)

			if content != "" {
				textIdx := getBlockIndex("text")
				if !openBlock[textIdx] {
					lines = append(lines,
						openaiToAnthropicBlockStart(textIdx, "text")...)
					openBlock[textIdx] = true
				}
				evt := map[string]any{
					"type":  "content_block_delta",
					"index": textIdx,
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

			if rawToolCalls, ok := delta["tool_calls"].([]any); ok {
				for _, tcRaw := range rawToolCalls {
					tc, ok := tcRaw.(map[string]any)
					if !ok {
						continue
					}
					toolKey := resolveToolKey(tc)
					state, ok := toolBlocks[toolKey]
					if !ok {
						state = &toolBlockState{
							blockIdx: getBlockIndex("tool:" + toolKey),
						}
						toolBlocks[toolKey] = state
					}
					if id, ok := tc["id"].(string); ok && id != "" {
						state.id = id
					}
					if fn, ok := tc["function"].(map[string]any); ok {
						if name, ok := fn["name"].(string); ok && name != "" {
							state.name = name
						}
						if args, ok := fn["arguments"].(string); ok && args != "" {
							state.pendingArgs += args
						}
					}
					if state.id == "" {
						state.id = fmt.Sprintf("call_%d", state.blockIdx)
					}

					if !state.started {
						// Wait for a valid function name before emitting tool_use block.
						if state.name == "" {
							continue
						}
						contentBlock := map[string]any{
							"type":  "tool_use",
							"id":    state.id,
							"name":  state.name,
							"input": map[string]any{},
						}
						startEvent := map[string]any{
							"type":          "content_block_start",
							"index":         state.blockIdx,
							"content_block": contentBlock,
						}
						b, _ := json.Marshal(startEvent)
						lines = append(lines,
							"event: content_block_start",
							"data: "+string(b),
							"",
						)
						openBlock[state.blockIdx] = true
						state.started = true
					}
					if state.started && state.pendingArgs != "" {
						lines = emitToolDelta(lines, state.blockIdx, state.pendingArgs)
						state.pendingArgs = ""
					}
				}
			}
		}

		if finishReason != "" {
			lines = closeOpenBlocks(lines)

			usage, _ := obj["usage"].(map[string]any)
			inTok := toInt(usage["prompt_tokens"])
			outTok := toInt(usage["completion_tokens"])

			stopReason := mapOpenAIFinishReason(finishReason)
			if finishReason == "tool_calls" {
				toolUseStarted := false
				for _, tb := range toolBlocks {
					if tb.started {
						toolUseStarted = true
						break
					}
				}
				// Avoid hanging clients on tool_use when no valid tool block was emitted.
				if !toolUseStarted {
					stopReason = "end_turn"
				}
			}
			md := map[string]any{
				"type":  "message_delta",
				"delta": map[string]any{"stop_reason": stopReason, "stop_sequence": nil},
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
			messageStopped = true
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
	var contentBlock map[string]any
	if blockType == "thinking" {
		contentBlock = map[string]any{
			"type":     "thinking",
			"thinking": "",
		}
	} else {
		contentBlock = map[string]any{
			"type": blockType,
			"text": "",
		}
	}
	evt := map[string]any{
		"type":          "content_block_start",
		"index":         index,
		"content_block": contentBlock,
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
