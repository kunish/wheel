package protocol

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// GeminiRequestToAnthropic converts a Gemini request to an Anthropic Messages API request.
func GeminiRequestToAnthropic(req *GeminiRequest, modelName string, stream bool) *AnthropicRequest {
	out := &AnthropicRequest{
		Model:     modelName,
		MaxTokens: 32000,
		Stream:    Ptr(stream),
		Metadata:  &AnthropicMetadata{UserID: generateAnthropicUserID()},
	}

	// Generation config
	if gc := req.GenerationConfig; gc != nil {
		if gc.MaxOutputTokens != nil {
			out.MaxTokens = *gc.MaxOutputTokens
		}
		if gc.Temperature != nil {
			out.Temperature = gc.Temperature
		} else if gc.TopP != nil {
			out.TopP = gc.TopP
		}
		if len(gc.StopSequences) > 0 {
			out.StopSequences = gc.StopSequences
		}
		if gc.ThinkingConfig != nil {
			convertGeminiThinkingToAnthropic(gc.ThinkingConfig, out, modelName)
		}
	}

	// System instruction
	if si := req.SystemInstruction; si != nil {
		var textBuilder strings.Builder
		for _, part := range si.Parts {
			if part.Text != "" {
				if textBuilder.Len() > 0 {
					textBuilder.WriteString("\n")
				}
				textBuilder.WriteString(part.Text)
			}
		}
		if textBuilder.Len() > 0 {
			out.Messages = append(out.Messages, AnthropicMessage{
				Role: "user",
				Content: []AnthropicContentBlock{
					{Type: "text", Text: textBuilder.String()},
				},
			})
		}
	}

	// FIFO queue for tool call IDs
	var pendingToolIDs []string

	// Contents -> Messages
	for _, content := range req.Contents {
		role := content.Role
		if role == "model" {
			role = "assistant"
		}
		if role == "function" || role == "tool" {
			role = "user"
		}

		msg := AnthropicMessage{Role: role}
		var blocks []AnthropicContentBlock

		for _, part := range content.Parts {
			if part.Text != "" {
				blocks = append(blocks, AnthropicContentBlock{Type: "text", Text: part.Text})
			}

			if part.FunctionCall != nil && role == "assistant" {
				toolID := GenAnthropicToolUseID()
				pendingToolIDs = append(pendingToolIDs, toolID)
				blocks = append(blocks, AnthropicContentBlock{
					Type:  "tool_use",
					ID:    toolID,
					Name:  part.FunctionCall.Name,
					Input: part.FunctionCall.Args,
				})
			}

			if part.FunctionResponse != nil {
				var toolID string
				if len(pendingToolIDs) > 0 {
					toolID = pendingToolIDs[0]
					pendingToolIDs = pendingToolIDs[1:]
				} else {
					toolID = GenAnthropicToolUseID()
				}
				resultContent := ""
				if part.FunctionResponse.Response != nil {
					if result, ok := part.FunctionResponse.Response["result"]; ok {
						resultContent = fmt.Sprintf("%v", result)
					} else {
						data, _ := json.Marshal(part.FunctionResponse.Response)
						resultContent = string(data)
					}
				}
				blocks = append(blocks, AnthropicContentBlock{
					Type:      "tool_result",
					ToolUseID: toolID,
					Text:      resultContent,
				})
			}

			if part.InlineData != nil {
				blocks = append(blocks, AnthropicContentBlock{
					Type: "image",
					Source: &AnthropicImageSource{
						Type:      "base64",
						MediaType: part.InlineData.MimeType,
						Data:      part.InlineData.Data,
					},
				})
			}
		}

		if len(blocks) > 0 {
			msg.Content = blocks
			out.Messages = append(out.Messages, msg)
		}
	}

	// Tools
	if len(req.Tools) > 0 {
		for _, toolDecl := range req.Tools {
			for _, fd := range toolDecl.FunctionDeclarations {
				tool := AnthropicTool{
					Name:        fd.Name,
					Description: fd.Description,
				}
				params := fd.Parameters
				if params == nil {
					params = fd.ParametersJSONSchema
				}
				if params != nil {
					tool.InputSchema = params
				}
				out.Tools = append(out.Tools, tool)
			}
		}
	}

	// Tool config
	if tc := req.ToolConfig; tc != nil && tc.FunctionCallingConfig != nil {
		switch tc.FunctionCallingConfig.Mode {
		case "AUTO":
			out.ToolChoice = AnthropicToolChoice{Type: "auto"}
		case "NONE":
			out.ToolChoice = AnthropicToolChoice{Type: "none"}
		case "ANY":
			out.ToolChoice = AnthropicToolChoice{Type: "any"}
		}
	}

	return out
}

func convertGeminiThinkingToAnthropic(tc *GeminiThinkConfig, out *AnthropicRequest, modelName string) {
	if tc.ThinkingLevel != "" {
		level := strings.ToLower(strings.TrimSpace(tc.ThinkingLevel))
		switch level {
		case "", "none":
			out.Thinking = &AnthropicThinking{Type: "disabled"}
		default:
			budget := levelToBudget(level)
			if budget > 0 {
				out.Thinking = &AnthropicThinking{Type: "enabled", BudgetTokens: budget}
			}
		}
	} else if tc.ThinkingBudget != nil {
		budget := *tc.ThinkingBudget
		switch {
		case budget == 0:
			out.Thinking = &AnthropicThinking{Type: "disabled"}
		case budget < 0:
			out.Thinking = &AnthropicThinking{Type: "enabled"}
		default:
			out.Thinking = &AnthropicThinking{Type: "enabled", BudgetTokens: budget}
		}
	} else if tc.IncludeThoughts != nil && *tc.IncludeThoughts {
		out.Thinking = &AnthropicThinking{Type: "enabled"}
	}
}

func levelToBudget(level string) int {
	switch level {
	case "low", "minimal":
		return 1024
	case "medium", "auto":
		return 4096
	case "high", "xhigh":
		return 16384
	case "max":
		return 65536
	}
	return 4096
}

// GeminiResponseToAnthropic converts a non-streaming Gemini response to Anthropic format.
func GeminiResponseToAnthropic(resp *GeminiResponse) *AnthropicResponse {
	out := &AnthropicResponse{
		ID:   "msg_" + randomAlphanumeric(20),
		Type: "message",
		Role: "assistant",
	}

	if len(resp.Candidates) > 0 {
		candidate := resp.Candidates[0]

		if candidate.Content != nil {
			for _, part := range candidate.Content.Parts {
				if part.Thought != nil && *part.Thought && part.Text != "" {
					out.Content = append(out.Content, AnthropicContentBlock{
						Type:     "thinking",
						Thinking: part.Text,
					})
				} else if part.Text != "" {
					out.Content = append(out.Content, AnthropicContentBlock{
						Type: "text",
						Text: part.Text,
					})
				}
				if part.FunctionCall != nil {
					out.Content = append(out.Content, AnthropicContentBlock{
						Type:  "tool_use",
						ID:    GenAnthropicToolUseID(),
						Name:  part.FunctionCall.Name,
						Input: part.FunctionCall.Args,
					})
				}
			}
		}

		if candidate.FinishReason != "" {
			reason := MapGeminiFinishReasonToAnthropic(candidate.FinishReason)
			out.StopReason = &reason
		}
	}

	if resp.UsageMetadata != nil {
		out.Usage = AnthropicUsage{
			InputTokens:  resp.UsageMetadata.PromptTokenCount,
			OutputTokens: resp.UsageMetadata.CandidatesTokenCount,
		}
	}

	return out
}

// Gemini Streaming -> Anthropic Streaming

type GeminiToAnthropicAccum struct {
	MessageID          string
	Model              string
	MessageStarted     bool
	TextBlockStarted   bool
	TextBlockIndex     int
	ThinkBlockStarted  bool
	ThinkBlockIndex    int
	NextBlockIndex     int
	FinishReason       string
	ContentStopped     bool
	MessageDeltaSent   bool
	MessageStopSent    bool
	ToolCallIndexes    map[int]int
	SawToolCall        bool
}

func NewGeminiToAnthropicAccum() *GeminiToAnthropicAccum {
	return &GeminiToAnthropicAccum{
		MessageID:       "msg_" + randomAlphanumeric(20),
		TextBlockIndex:  -1,
		ThinkBlockIndex: -1,
		ToolCallIndexes: make(map[int]int),
	}
}

// ConvertGeminiChunkToAnthropic converts a Gemini streaming chunk to Anthropic SSE events.
func ConvertGeminiChunkToAnthropic(chunk *GeminiResponse, accum *GeminiToAnthropicAccum) []string {
	var results []string

	// Emit message_start
	if !accum.MessageStarted {
		msgStart := AnthropicSSEMessageStart{
			Type: "message_start",
			Message: AnthropicResponse{
				ID:      accum.MessageID,
				Type:    "message",
				Role:    "assistant",
				Model:   accum.Model,
				Content: []AnthropicContentBlock{},
				Usage:   AnthropicUsage{},
			},
		}
		data, _ := json.Marshal(msgStart)
		results = append(results, fmt.Sprintf("event: message_start\ndata: %s\n\n", data))
		accum.MessageStarted = true
	}

	if len(chunk.Candidates) == 0 {
		return results
	}

	candidate := chunk.Candidates[0]
	if candidate.Content != nil {
		for _, part := range candidate.Content.Parts {
			if part.Thought != nil && *part.Thought && part.Text != "" {
				// Thinking content
				stopGeminiAnthropicTextBlock(accum, &results)
				if !accum.ThinkBlockStarted {
					if accum.ThinkBlockIndex == -1 {
						accum.ThinkBlockIndex = accum.NextBlockIndex
						accum.NextBlockIndex++
					}
					blockStart := AnthropicSSEContentBlockStart{
						Type:         "content_block_start",
						Index:        accum.ThinkBlockIndex,
						ContentBlock: AnthropicContentBlock{Type: "thinking", Thinking: ""},
					}
					data, _ := json.Marshal(blockStart)
					results = append(results, fmt.Sprintf("event: content_block_start\ndata: %s\n\n", data))
					accum.ThinkBlockStarted = true
				}
				blockDelta := AnthropicSSEContentBlockDelta{
					Type:  "content_block_delta",
					Index: accum.ThinkBlockIndex,
					Delta: AnthropicStreamDelta{Type: "thinking_delta", Thinking: part.Text},
				}
				data, _ := json.Marshal(blockDelta)
				results = append(results, fmt.Sprintf("event: content_block_delta\ndata: %s\n\n", data))
			} else if part.Text != "" {
				stopGeminiAnthropicThinkBlock(accum, &results)
				if !accum.TextBlockStarted {
					if accum.TextBlockIndex == -1 {
						accum.TextBlockIndex = accum.NextBlockIndex
						accum.NextBlockIndex++
					}
					blockStart := AnthropicSSEContentBlockStart{
						Type:         "content_block_start",
						Index:        accum.TextBlockIndex,
						ContentBlock: AnthropicContentBlock{Type: "text", Text: ""},
					}
					data, _ := json.Marshal(blockStart)
					results = append(results, fmt.Sprintf("event: content_block_start\ndata: %s\n\n", data))
					accum.TextBlockStarted = true
				}
				blockDelta := AnthropicSSEContentBlockDelta{
					Type:  "content_block_delta",
					Index: accum.TextBlockIndex,
					Delta: AnthropicStreamDelta{Type: "text_delta", Text: part.Text},
				}
				data, _ := json.Marshal(blockDelta)
				results = append(results, fmt.Sprintf("event: content_block_delta\ndata: %s\n\n", data))
			}

			if part.FunctionCall != nil {
				accum.SawToolCall = true
				stopGeminiAnthropicThinkBlock(accum, &results)
				stopGeminiAnthropicTextBlock(accum, &results)

				blockIdx := accum.NextBlockIndex
				accum.NextBlockIndex++

				blockStart := AnthropicSSEContentBlockStart{
					Type:  "content_block_start",
					Index: blockIdx,
					ContentBlock: AnthropicContentBlock{
						Type:  "tool_use",
						ID:    GenAnthropicToolUseID(),
						Name:  part.FunctionCall.Name,
						Input: map[string]any{},
					},
				}
				data, _ := json.Marshal(blockStart)
				results = append(results, fmt.Sprintf("event: content_block_start\ndata: %s\n\n", data))

				// Send the entire args as input_json_delta
				if part.FunctionCall.Args != nil {
					argsData, _ := json.Marshal(part.FunctionCall.Args)
					argsDelta := AnthropicSSEContentBlockDelta{
						Type:  "content_block_delta",
						Index: blockIdx,
						Delta: AnthropicStreamDelta{Type: "input_json_delta", PartialJSON: string(argsData)},
					}
					argsBytes, _ := json.Marshal(argsDelta)
					results = append(results, fmt.Sprintf("event: content_block_delta\ndata: %s\n\n", argsBytes))
				}

				blockStop := AnthropicSSEContentBlockStop{Type: "content_block_stop", Index: blockIdx}
				stopData, _ := json.Marshal(blockStop)
				results = append(results, fmt.Sprintf("event: content_block_stop\ndata: %s\n\n", stopData))
			}
		}
	}

	// Finish reason
	if candidate.FinishReason != "" {
		accum.FinishReason = candidate.FinishReason
		stopGeminiAnthropicThinkBlock(accum, &results)
		stopGeminiAnthropicTextBlock(accum, &results)
	}

	// Usage
	if chunk.UsageMetadata != nil {
		delta := AnthropicSSEMessageDelta{
			Type: "message_delta",
			Delta: AnthropicMessageDeltaBody{
				StopReason: MapGeminiFinishReasonToAnthropic(accum.FinishReason),
			},
			Usage: &AnthropicUsage{
				InputTokens:  chunk.UsageMetadata.PromptTokenCount,
				OutputTokens: chunk.UsageMetadata.CandidatesTokenCount,
			},
		}
		data, _ := json.Marshal(delta)
		results = append(results, fmt.Sprintf("event: message_delta\ndata: %s\n\n", data))
		accum.MessageDeltaSent = true

		if !accum.MessageStopSent {
			stop := AnthropicSSEMessageStop{Type: "message_stop"}
			stopData, _ := json.Marshal(stop)
			results = append(results, fmt.Sprintf("event: message_stop\ndata: %s\n\n", stopData))
			accum.MessageStopSent = true
		}
	}

	return results
}

func stopGeminiAnthropicTextBlock(accum *GeminiToAnthropicAccum, results *[]string) {
	if !accum.TextBlockStarted {
		return
	}
	blockStop := AnthropicSSEContentBlockStop{Type: "content_block_stop", Index: accum.TextBlockIndex}
	data, _ := json.Marshal(blockStop)
	*results = append(*results, fmt.Sprintf("event: content_block_stop\ndata: %s\n\n", data))
	accum.TextBlockStarted = false
	accum.TextBlockIndex = -1
}

func stopGeminiAnthropicThinkBlock(accum *GeminiToAnthropicAccum, results *[]string) {
	if !accum.ThinkBlockStarted {
		return
	}
	blockStop := AnthropicSSEContentBlockStop{Type: "content_block_stop", Index: accum.ThinkBlockIndex}
	data, _ := json.Marshal(blockStop)
	*results = append(*results, fmt.Sprintf("event: content_block_stop\ndata: %s\n\n", data))
	accum.ThinkBlockStarted = false
	accum.ThinkBlockIndex = -1
}

func generateAnthropicUserID() string {
	u, _ := uuid.NewRandom()
	return "user_" + u.String()
}

// AnthropicTokenCountResponse generates an Anthropic-format token count response.
func AnthropicTokenCountResponse(count int64) string {
	return fmt.Sprintf(`{"input_tokens":%d}`, count)
}
