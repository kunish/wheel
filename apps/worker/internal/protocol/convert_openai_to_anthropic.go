package protocol

import (
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go"
)

// Streaming: OpenAI chunk → Anthropic SSE events.
// These functions build SSE text events from parsed OpenAI streaming chunks.

func ConvertOpenAIChunkToAnthropic(chunk *openai.ChatCompletionChunk, accum *OpenAIToAnthropicAccum, toolNameMap map[string]string) []string {
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
		if chunk.Usage.JSON.PromptTokens.Valid() && accum.FinishReason != "" {
			inputTokens, outputTokens, cachedTokens := extractChunkUsage(chunk)
			results = append(results, buildAnthropicMessageDelta(accum, inputTokens, outputTokens, cachedTokens))
			accum.MessageDeltaSent = true
			emitAnthropicMessageStop(accum, &results)
		}
		return results
	}

	choice := chunk.Choices[0]

	if !accum.MessageStarted {
		msgStart := fmt.Sprintf(`{"type":"message_start","message":{"id":"%s","type":"message","role":"assistant","model":"%s","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":0,"output_tokens":0}}}`,
			accum.MessageID, accum.Model)
		results = append(results, "event: message_start\ndata: "+msgStart+"\n\n")
		accum.MessageStarted = true
	}

	// Reasoning content - accessed via ExtraFields since the SDK may not have this field
	reasoningText := ""
	if rawJSON := chunk.JSON.ExtraFields["reasoning_content"]; rawJSON.Valid() {
		_ = json.Unmarshal([]byte(rawJSON.Raw()), &reasoningText)
	}
	if reasoningText != "" {
		stopAnthropicTextBlock(accum, &results)
		if !accum.ThinkingBlockStarted {
				if accum.ThinkingContentBlockIndex == -1 {
					accum.ThinkingContentBlockIndex = accum.NextContentBlockIndex
					accum.NextContentBlockIndex++
				}
				results = append(results, fmt.Sprintf(`event: content_block_start
data: {"type":"content_block_start","index":%d,"content_block":{"type":"thinking","thinking":""}}

`, accum.ThinkingContentBlockIndex))
				accum.ThinkingBlockStarted = true
			}
			data, _ := json.Marshal(map[string]any{
				"type":  "content_block_delta",
				"index": accum.ThinkingContentBlockIndex,
				"delta": map[string]string{"type": "thinking_delta", "thinking": reasoningText},
			})
			results = append(results, fmt.Sprintf("event: content_block_delta\ndata: %s\n\n", data))
	}

	// Text content
	if choice.Delta.Content != "" {
		if !accum.TextContentBlockStarted {
			stopAnthropicThinkingBlock(accum, &results)
			if accum.TextContentBlockIndex == -1 {
				accum.TextContentBlockIndex = accum.NextContentBlockIndex
				accum.NextContentBlockIndex++
			}
			results = append(results, fmt.Sprintf(`event: content_block_start
data: {"type":"content_block_start","index":%d,"content_block":{"type":"text","text":""}}

`, accum.TextContentBlockIndex))
			accum.TextContentBlockStarted = true
		}
		data, _ := json.Marshal(map[string]any{
			"type":  "content_block_delta",
			"index": accum.TextContentBlockIndex,
			"delta": map[string]string{"type": "text_delta", "text": choice.Delta.Content},
		})
		results = append(results, fmt.Sprintf("event: content_block_delta\ndata: %s\n\n", data))
		accum.ContentAccumulator.WriteString(choice.Delta.Content)
	}

	// Tool calls
	for _, tc := range choice.Delta.ToolCalls {
		if accum.ToolCallsAccumulator == nil {
			accum.ToolCallsAccumulator = make(map[int]*ToolCallAccum)
		}
		accum.SawToolCall = true
		index := int(tc.Index)
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

			data, _ := json.Marshal(map[string]any{
				"type":  "content_block_start",
				"index": blockIndex,
				"content_block": map[string]any{
					"type":  "tool_use",
					"id":    acc.ID,
					"name":  acc.Name,
					"input": map[string]any{},
				},
			})
			results = append(results, fmt.Sprintf("event: content_block_start\ndata: %s\n\n", data))
		}
		if tc.Function.Arguments != "" {
			acc.Arguments.WriteString(tc.Function.Arguments)
		}
	}

	// Finish reason
	if choice.FinishReason != "" {
		if accum.SawToolCall {
			accum.FinishReason = "tool_calls"
		} else {
			accum.FinishReason = string(choice.FinishReason)
		}

		stopAnthropicThinkingBlock(accum, &results)
		stopAnthropicTextBlock(accum, &results)

		if !accum.ContentBlocksStopped {
			for index, acc := range accum.ToolCallsAccumulator {
				blockIndex := accum.toolContentBlockIndex(index)
				if acc.Arguments.Len() > 0 {
					data, _ := json.Marshal(map[string]any{
						"type":  "content_block_delta",
						"index": blockIndex,
						"delta": map[string]string{"type": "input_json_delta", "partial_json": acc.Arguments.String()},
					})
					results = append(results, fmt.Sprintf("event: content_block_delta\ndata: %s\n\n", data))
				}
				results = append(results, fmt.Sprintf("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":%d}\n\n", blockIndex))
				delete(accum.ToolCallBlockIndexes, index)
			}
			accum.ContentBlocksStopped = true
		}
	}

	// Usage
	if accum.FinishReason != "" && chunk.Usage.JSON.PromptTokens.Valid() {
		inputTokens, outputTokens, cachedTokens := extractChunkUsage(chunk)
		results = append(results, buildAnthropicMessageDelta(accum, inputTokens, outputTokens, cachedTokens))
		accum.MessageDeltaSent = true
		emitAnthropicMessageStop(accum, &results)
	}

	return results
}

func ConvertOpenAIDoneToAnthropic(accum *OpenAIToAnthropicAccum) []string {
	var results []string

	stopAnthropicThinkingBlock(accum, &results)
	stopAnthropicTextBlock(accum, &results)

	if !accum.ContentBlocksStopped {
		for index, acc := range accum.ToolCallsAccumulator {
			blockIndex := accum.toolContentBlockIndex(index)
			if acc.Arguments.Len() > 0 {
				data, _ := json.Marshal(map[string]any{
					"type":  "content_block_delta",
					"index": blockIndex,
					"delta": map[string]string{"type": "input_json_delta", "partial_json": acc.Arguments.String()},
				})
				results = append(results, fmt.Sprintf("event: content_block_delta\ndata: %s\n\n", data))
			}
			results = append(results, fmt.Sprintf("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":%d}\n\n", blockIndex))
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

func extractChunkUsage(chunk *openai.ChatCompletionChunk) (inputTokens, outputTokens, cachedTokens int64) {
	inputTokens = chunk.Usage.PromptTokens
	outputTokens = chunk.Usage.CompletionTokens
	cachedTokens = chunk.Usage.PromptTokensDetails.CachedTokens
	if cachedTokens > 0 && inputTokens >= cachedTokens {
		inputTokens -= cachedTokens
	}
	return
}

func buildAnthropicMessageDelta(accum *OpenAIToAnthropicAccum, inputTokens, outputTokens, cachedTokens int64) string {
	usage := map[string]int64{
		"input_tokens":  inputTokens,
		"output_tokens": outputTokens,
	}
	if cachedTokens > 0 {
		usage["cache_read_input_tokens"] = cachedTokens
	}
	data, _ := json.Marshal(map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason":   MapOpenAIFinishReasonToAnthropic(accum.EffectiveFinishReason()),
			"stop_sequence": nil,
		},
		"usage": usage,
	})
	return fmt.Sprintf("event: message_delta\ndata: %s\n\n", data)
}

func emitAnthropicMessageStop(accum *OpenAIToAnthropicAccum, results *[]string) {
	if accum.MessageStopSent {
		return
	}
	*results = append(*results, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
	accum.MessageStopSent = true
}

func stopAnthropicThinkingBlock(accum *OpenAIToAnthropicAccum, results *[]string) {
	if !accum.ThinkingBlockStarted {
		return
	}
	*results = append(*results, fmt.Sprintf("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":%d}\n\n", accum.ThinkingContentBlockIndex))
	accum.ThinkingBlockStarted = false
	accum.ThinkingContentBlockIndex = -1
}

func stopAnthropicTextBlock(accum *OpenAIToAnthropicAccum, results *[]string) {
	if !accum.TextContentBlockStarted {
		return
	}
	*results = append(*results, fmt.Sprintf("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":%d}\n\n", accum.TextContentBlockIndex))
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
