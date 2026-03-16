package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
)

// OpenAIResponseToAnthropic converts an OpenAI Chat Completions response to an Anthropic response.
func OpenAIResponseToAnthropic(resp *OpenAIChatResponse) *AnthropicResponse {
	out := &AnthropicResponse{
		ID:    resp.ID,
		Type:  "message",
		Role:  "assistant",
		Model: resp.Model,
	}

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		msg := choice.Message
		if msg == nil {
			msg = choice.Delta
		}
		if msg != nil {
			out.Content = openAIMessageToAnthropicContent(msg)

			if choice.FinishReason != nil {
				reason := MapOpenAIFinishReasonToAnthropic(*choice.FinishReason)
				out.StopReason = &reason
			}
		}
	}

	if resp.Usage != nil {
		inputTokens, outputTokens, cachedTokens := extractOpenAIUsageValues(resp.Usage)
		out.Usage = AnthropicUsage{
			InputTokens:          inputTokens,
			OutputTokens:         outputTokens,
			CacheReadInputTokens: cachedTokens,
		}
	}

	return out
}

func openAIMessageToAnthropicContent(msg *OpenAIMessage) []AnthropicContentBlock {
	var blocks []AnthropicContentBlock

	// Reasoning/thinking content
	if msg.ReasoningContent != nil {
		for _, text := range collectReasoningTexts(msg.ReasoningContent) {
			if text == "" {
				continue
			}
			blocks = append(blocks, AnthropicContentBlock{
				Type:     "thinking",
				Thinking: text,
			})
		}
	}

	// Text content
	contentStr := msg.ContentString()
	if contentStr != "" {
		blocks = append(blocks, AnthropicContentBlock{
			Type: "text",
			Text: contentStr,
		})
	}

	// Tool calls
	for _, tc := range msg.ToolCalls {
		input := ParseJSONArgs(tc.Function.Arguments)
		blocks = append(blocks, AnthropicContentBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: input,
		})
	}

	return blocks
}

func extractOpenAIUsageValues(usage *OpenAIUsage) (inputTokens, outputTokens, cachedTokens int64) {
	if usage == nil {
		return 0, 0, 0
	}
	inputTokens = usage.PromptTokens
	outputTokens = usage.CompletionTokens
	if usage.PromptTokensDetails != nil {
		cachedTokens = usage.PromptTokensDetails.CachedTokens
	}
	if cachedTokens > 0 && inputTokens >= cachedTokens {
		inputTokens -= cachedTokens
	}
	return
}

func collectReasoningTexts(v any) []string {
	switch val := v.(type) {
	case string:
		if val != "" {
			return []string{val}
		}
	case []any:
		var texts []string
		for _, item := range val {
			texts = append(texts, collectReasoningTexts(item)...)
		}
		return texts
	case map[string]any:
		if text, ok := val["text"].(string); ok && text != "" {
			return []string{text}
		}
	}
	return nil
}

// OpenAIStreamChunkToAnthropicSSE converts an OpenAI streaming chunk to Anthropic SSE events.
// The accum parameter maintains state across chunks.
type OpenAIToAnthropicAccum struct {
	MessageID                 string
	Model                     string
	CreatedAt                 int64
	ToolNameMap               map[string]string
	SawToolCall               bool
	ContentAccumulator        strings.Builder
	ToolCallsAccumulator      map[int]*ToolCallAccum
	TextContentBlockStarted   bool
	ThinkingBlockStarted      bool
	FinishReason              string
	ContentBlocksStopped      bool
	MessageDeltaSent          bool
	MessageStarted            bool
	MessageStopSent           bool
	ToolCallBlockIndexes      map[int]int
	TextContentBlockIndex     int
	ThinkingContentBlockIndex int
	NextContentBlockIndex     int
}

type ToolCallAccum struct {
	ID        string
	Name      string
	Arguments strings.Builder
}

func NewOpenAIToAnthropicAccum() *OpenAIToAnthropicAccum {
	return &OpenAIToAnthropicAccum{
		ToolCallBlockIndexes:      make(map[int]int),
		TextContentBlockIndex:     -1,
		ThinkingContentBlockIndex: -1,
		NextContentBlockIndex:     0,
	}
}

func (a *OpenAIToAnthropicAccum) toolContentBlockIndex(openAIToolIndex int) int {
	if idx, ok := a.ToolCallBlockIndexes[openAIToolIndex]; ok {
		return idx
	}
	idx := a.NextContentBlockIndex
	a.NextContentBlockIndex++
	a.ToolCallBlockIndexes[openAIToolIndex] = idx
	return idx
}

func (a *OpenAIToAnthropicAccum) EffectiveFinishReason() string {
	if a.SawToolCall {
		return "tool_calls"
	}
	return a.FinishReason
}

// ConvertOpenAIChunkToAnthropic converts a single OpenAI streaming chunk to Anthropic SSE events.
func ConvertOpenAIChunkToAnthropic(chunk *OpenAIChatResponse, accum *OpenAIToAnthropicAccum, toolNameMap map[string]string) []string {
	accum.ToolNameMap = toolNameMap
	var results []string

	if accum.MessageID == "" && chunk.ID != "" {
		accum.MessageID = chunk.ID
	}
	if accum.Model == "" && chunk.Model != "" {
		accum.Model = chunk.Model
	}
	if accum.CreatedAt == 0 && chunk.Created != 0 {
		accum.CreatedAt = chunk.Created
	}

	if len(chunk.Choices) == 0 {
		if chunk.Usage != nil && accum.FinishReason != "" {
			inputTokens, outputTokens, cachedTokens := extractOpenAIUsageValues(chunk.Usage)
			results = append(results, buildAnthropicMessageDelta(accum, inputTokens, outputTokens, cachedTokens))
			accum.MessageDeltaSent = true
			emitAnthropicMessageStop(accum, &results)
		}
		return results
	}

	choice := chunk.Choices[0]
	delta := choice.Delta
	if delta == nil {
		return results
	}

	// Emit message_start on first chunk
	if !accum.MessageStarted {
		msgStart := AnthropicSSEMessageStart{
			Type: "message_start",
			Message: AnthropicResponse{
				ID:         accum.MessageID,
				Type:       "message",
				Role:       "assistant",
				Model:      accum.Model,
				Content:    []AnthropicContentBlock{},
				StopReason: nil,
				Usage:      AnthropicUsage{},
			},
		}
		data, _ := json.Marshal(msgStart)
		results = append(results, fmt.Sprintf("event: message_start\ndata: %s\n\n", data))
		accum.MessageStarted = true
	}

	// Reasoning content
	if delta.ReasoningContent != nil {
		for _, text := range collectReasoningTexts(delta.ReasoningContent) {
			if text == "" {
				continue
			}
			stopAnthropicTextBlock(accum, &results)
			if !accum.ThinkingBlockStarted {
				if accum.ThinkingContentBlockIndex == -1 {
					accum.ThinkingContentBlockIndex = accum.NextContentBlockIndex
					accum.NextContentBlockIndex++
				}
				blockStart := AnthropicSSEContentBlockStart{
					Type:         "content_block_start",
					Index:        accum.ThinkingContentBlockIndex,
					ContentBlock: AnthropicContentBlock{Type: "thinking", Thinking: ""},
				}
				data, _ := json.Marshal(blockStart)
				results = append(results, fmt.Sprintf("event: content_block_start\ndata: %s\n\n", data))
				accum.ThinkingBlockStarted = true
			}
			blockDelta := AnthropicSSEContentBlockDelta{
				Type:  "content_block_delta",
				Index: accum.ThinkingContentBlockIndex,
				Delta: AnthropicStreamDelta{Type: "thinking_delta", Thinking: text},
			}
			data, _ := json.Marshal(blockDelta)
			results = append(results, fmt.Sprintf("event: content_block_delta\ndata: %s\n\n", data))
		}
	}

	// Text content
	contentStr := delta.ContentString()
	if contentStr != "" {
		if !accum.TextContentBlockStarted {
			stopAnthropicThinkingBlock(accum, &results)
			if accum.TextContentBlockIndex == -1 {
				accum.TextContentBlockIndex = accum.NextContentBlockIndex
				accum.NextContentBlockIndex++
			}
			blockStart := AnthropicSSEContentBlockStart{
				Type:         "content_block_start",
				Index:        accum.TextContentBlockIndex,
				ContentBlock: AnthropicContentBlock{Type: "text", Text: ""},
			}
			data, _ := json.Marshal(blockStart)
			results = append(results, fmt.Sprintf("event: content_block_start\ndata: %s\n\n", data))
			accum.TextContentBlockStarted = true
		}
		blockDelta := AnthropicSSEContentBlockDelta{
			Type:  "content_block_delta",
			Index: accum.TextContentBlockIndex,
			Delta: AnthropicStreamDelta{Type: "text_delta", Text: contentStr},
		}
		data, _ := json.Marshal(blockDelta)
		results = append(results, fmt.Sprintf("event: content_block_delta\ndata: %s\n\n", data))
		accum.ContentAccumulator.WriteString(contentStr)
	}

	// Tool calls
	for _, tc := range delta.ToolCalls {
		if accum.ToolCallsAccumulator == nil {
			accum.ToolCallsAccumulator = make(map[int]*ToolCallAccum)
		}
		accum.SawToolCall = true
		index := 0
		if tc.Index != nil {
			index = *tc.Index
		}
		blockIndex := accum.toolContentBlockIndex(index)

		if _, exists := accum.ToolCallsAccumulator[index]; !exists {
			accum.ToolCallsAccumulator[index] = &ToolCallAccum{}
		}
		acc := accum.ToolCallsAccumulator[index]

		if tc.ID != "" {
			acc.ID = tc.ID
		}
		if tc.Function.Name != "" {
			acc.Name = mapToolName(toolNameMap, tc.Function.Name)
			stopAnthropicThinkingBlock(accum, &results)
			stopAnthropicTextBlock(accum, &results)

			blockStart := AnthropicSSEContentBlockStart{
				Type:  "content_block_start",
				Index: blockIndex,
				ContentBlock: AnthropicContentBlock{
					Type:  "tool_use",
					ID:    acc.ID,
					Name:  acc.Name,
					Input: map[string]any{},
				},
			}
			data, _ := json.Marshal(blockStart)
			results = append(results, fmt.Sprintf("event: content_block_start\ndata: %s\n\n", data))
		}
		if tc.Function.Arguments != "" {
			acc.Arguments.WriteString(tc.Function.Arguments)
		}
	}

	// Finish reason
	if choice.FinishReason != nil && *choice.FinishReason != "" {
		if accum.SawToolCall {
			accum.FinishReason = "tool_calls"
		} else {
			accum.FinishReason = *choice.FinishReason
		}

		stopAnthropicThinkingBlock(accum, &results)
		stopAnthropicTextBlock(accum, &results)

		if !accum.ContentBlocksStopped {
			for index, acc := range accum.ToolCallsAccumulator {
				blockIndex := accum.toolContentBlockIndex(index)
				if acc.Arguments.Len() > 0 {
					argsDelta := AnthropicSSEContentBlockDelta{
						Type:  "content_block_delta",
						Index: blockIndex,
						Delta: AnthropicStreamDelta{Type: "input_json_delta", PartialJSON: acc.Arguments.String()},
					}
					data, _ := json.Marshal(argsDelta)
					results = append(results, fmt.Sprintf("event: content_block_delta\ndata: %s\n\n", data))
				}
				blockStop := AnthropicSSEContentBlockStop{Type: "content_block_stop", Index: blockIndex}
				data, _ := json.Marshal(blockStop)
				results = append(results, fmt.Sprintf("event: content_block_stop\ndata: %s\n\n", data))
				delete(accum.ToolCallBlockIndexes, index)
			}
			accum.ContentBlocksStopped = true
		}
	}

	// Usage info
	if accum.FinishReason != "" && chunk.Usage != nil {
		inputTokens, outputTokens, cachedTokens := extractOpenAIUsageValues(chunk.Usage)
		results = append(results, buildAnthropicMessageDelta(accum, inputTokens, outputTokens, cachedTokens))
		accum.MessageDeltaSent = true
		emitAnthropicMessageStop(accum, &results)
	}

	return results
}

// ConvertOpenAIDoneToAnthropic handles the [DONE] marker in OpenAI streaming.
func ConvertOpenAIDoneToAnthropic(accum *OpenAIToAnthropicAccum) []string {
	var results []string

	stopAnthropicThinkingBlock(accum, &results)
	stopAnthropicTextBlock(accum, &results)

	if !accum.ContentBlocksStopped {
		for index, acc := range accum.ToolCallsAccumulator {
			blockIndex := accum.toolContentBlockIndex(index)
			if acc.Arguments.Len() > 0 {
				argsDelta := AnthropicSSEContentBlockDelta{
					Type:  "content_block_delta",
					Index: blockIndex,
					Delta: AnthropicStreamDelta{Type: "input_json_delta", PartialJSON: acc.Arguments.String()},
				}
				data, _ := json.Marshal(argsDelta)
				results = append(results, fmt.Sprintf("event: content_block_delta\ndata: %s\n\n", data))
			}
			blockStop := AnthropicSSEContentBlockStop{Type: "content_block_stop", Index: blockIndex}
			data, _ := json.Marshal(blockStop)
			results = append(results, fmt.Sprintf("event: content_block_stop\ndata: %s\n\n", data))
			delete(accum.ToolCallBlockIndexes, index)
		}
		accum.ContentBlocksStopped = true
	}

	if accum.FinishReason != "" && !accum.MessageDeltaSent {
		results = append(results, buildAnthropicMessageDelta(accum, 0, 0, 0))
		accum.MessageDeltaSent = true
	}

	emitAnthropicMessageStop(accum, &results)
	return results
}

func buildAnthropicMessageDelta(accum *OpenAIToAnthropicAccum, inputTokens, outputTokens, cachedTokens int64) string {
	delta := AnthropicSSEMessageDelta{
		Type: "message_delta",
		Delta: AnthropicMessageDeltaBody{
			StopReason: MapOpenAIFinishReasonToAnthropic(accum.EffectiveFinishReason()),
		},
		Usage: &AnthropicUsage{
			InputTokens:          inputTokens,
			OutputTokens:         outputTokens,
			CacheReadInputTokens: cachedTokens,
		},
	}
	data, _ := json.Marshal(delta)
	return fmt.Sprintf("event: message_delta\ndata: %s\n\n", data)
}

func emitAnthropicMessageStop(accum *OpenAIToAnthropicAccum, results *[]string) {
	if accum.MessageStopSent {
		return
	}
	stop := AnthropicSSEMessageStop{Type: "message_stop"}
	data, _ := json.Marshal(stop)
	*results = append(*results, fmt.Sprintf("event: message_stop\ndata: %s\n\n", data))
	accum.MessageStopSent = true
}

func stopAnthropicThinkingBlock(accum *OpenAIToAnthropicAccum, results *[]string) {
	if !accum.ThinkingBlockStarted {
		return
	}
	blockStop := AnthropicSSEContentBlockStop{Type: "content_block_stop", Index: accum.ThinkingContentBlockIndex}
	data, _ := json.Marshal(blockStop)
	*results = append(*results, fmt.Sprintf("event: content_block_stop\ndata: %s\n\n", data))
	accum.ThinkingBlockStarted = false
	accum.ThinkingContentBlockIndex = -1
}

func stopAnthropicTextBlock(accum *OpenAIToAnthropicAccum, results *[]string) {
	if !accum.TextContentBlockStarted {
		return
	}
	blockStop := AnthropicSSEContentBlockStop{Type: "content_block_stop", Index: accum.TextContentBlockIndex}
	data, _ := json.Marshal(blockStop)
	*results = append(*results, fmt.Sprintf("event: content_block_stop\ndata: %s\n\n", data))
	accum.TextContentBlockStarted = false
	accum.TextContentBlockIndex = -1
}

func mapToolName(nameMap map[string]string, name string) string {
	if nameMap == nil {
		return name
	}
	if mapped, ok := nameMap[name]; ok {
		return mapped
	}
	return name
}
