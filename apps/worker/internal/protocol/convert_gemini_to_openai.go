package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
)

// GeminiRequestToOpenAI converts a Gemini API request to an OpenAI Chat Completions request.
func GeminiRequestToOpenAI(req *GeminiRequest, modelName string, stream bool) *OpenAIChatRequest {
	out := &OpenAIChatRequest{
		Model:  modelName,
		Stream: Ptr(stream),
	}

	// Generation config
	if gc := req.GenerationConfig; gc != nil {
		out.Temperature = gc.Temperature
		out.TopP = gc.TopP
		out.TopK = gc.TopK
		if gc.MaxOutputTokens != nil {
			out.MaxTokens = gc.MaxOutputTokens
		}
		if len(gc.StopSequences) > 0 {
			out.Stop = gc.StopSequences
		}
		out.N = gc.CandidateCount

		if tc := gc.ThinkingConfig; tc != nil {
			if tc.ThinkingLevel != "" {
				out.ReasoningEffort = strings.ToLower(strings.TrimSpace(tc.ThinkingLevel))
			} else if tc.ThinkingBudget != nil {
				out.ReasoningEffort = budgetToLevel(*tc.ThinkingBudget)
			}
		}
	}

	// Track tool call IDs for matching function responses with function calls
	var toolCallIDs []string

	// System instruction
	if si := req.SystemInstruction; si != nil {
		var sysContentParts []OpenAIContentPart
		for _, part := range si.Parts {
			if part.Text != "" {
				sysContentParts = append(sysContentParts, OpenAIContentPart{Type: "text", Text: part.Text})
			}
			if part.InlineData != nil {
				sysContentParts = append(sysContentParts, OpenAIContentPart{
					Type:     "image_url",
					ImageURL: &OpenAIImageURL{URL: ImageDataURL(part.InlineData.MimeType, part.InlineData.Data)},
				})
			}
		}
		if len(sysContentParts) > 0 {
			out.Messages = append(out.Messages, OpenAIMessage{Role: "system", Content: sysContentParts})
		}
	}

	// Contents -> messages
	for _, content := range req.Contents {
		role := content.Role
		if role == "model" {
			role = "assistant"
		}

		var textBuilder strings.Builder
		var contentParts []OpenAIContentPart
		onlyText := true
		var toolCalls []OpenAIToolCall

		for _, part := range content.Parts {
			if part.Text != "" {
				textBuilder.WriteString(part.Text)
				contentParts = append(contentParts, OpenAIContentPart{Type: "text", Text: part.Text})
			}

			if part.InlineData != nil {
				onlyText = false
				contentParts = append(contentParts, OpenAIContentPart{
					Type:     "image_url",
					ImageURL: &OpenAIImageURL{URL: ImageDataURL(part.InlineData.MimeType, part.InlineData.Data)},
				})
			}

			if part.FunctionCall != nil {
				toolCallID := GenOpenAIToolCallID()
				toolCallIDs = append(toolCallIDs, toolCallID)
				argsStr := "{}"
				if part.FunctionCall.Args != nil {
					data, err := json.Marshal(part.FunctionCall.Args)
					if err == nil {
						argsStr = string(data)
					}
				}
				toolCalls = append(toolCalls, OpenAIToolCall{
					ID:   toolCallID,
					Type: "function",
					Function: OpenAIFunctionCall{
						Name:      part.FunctionCall.Name,
						Arguments: argsStr,
					},
				})
			}

			if part.FunctionResponse != nil {
				toolMsg := OpenAIMessage{
					Role: "tool",
				}
				if len(toolCallIDs) > 0 {
					toolMsg.ToolCallID = toolCallIDs[len(toolCallIDs)-1]
				} else {
					toolMsg.ToolCallID = GenOpenAIToolCallID()
				}
				if part.FunctionResponse.Response != nil {
					if contentField, ok := part.FunctionResponse.Response["content"]; ok {
						data, _ := json.Marshal(contentField)
						toolMsg.Content = string(data)
					} else {
						data, _ := json.Marshal(part.FunctionResponse.Response)
						toolMsg.Content = string(data)
					}
				}
				out.Messages = append(out.Messages, toolMsg)
			}
		}

		msg := OpenAIMessage{Role: role}
		if len(contentParts) > 0 {
			if onlyText {
				msg.Content = textBuilder.String()
			} else {
				msg.Content = contentPartsToAny(contentParts)
			}
		}
		if len(toolCalls) > 0 {
			msg.ToolCalls = toolCalls
		}
		out.Messages = append(out.Messages, msg)
	}

	// Tools
	for _, toolDecl := range req.Tools {
		for _, fd := range toolDecl.FunctionDeclarations {
			params := fd.Parameters
			if params == nil {
				params = fd.ParametersJSONSchema
			}
			out.Tools = append(out.Tools, OpenAITool{
				Type: "function",
				Function: OpenAIFunction{
					Name:        fd.Name,
					Description: fd.Description,
					Parameters:  params,
				},
			})
		}
	}

	// Tool config
	if tc := req.ToolConfig; tc != nil && tc.FunctionCallingConfig != nil {
		switch tc.FunctionCallingConfig.Mode {
		case "NONE":
			out.ToolChoice = "none"
		case "AUTO":
			out.ToolChoice = "auto"
		case "ANY":
			out.ToolChoice = "required"
		}
	}

	return out
}

func contentPartsToAny(parts []OpenAIContentPart) any {
	result := make([]any, len(parts))
	for i, p := range parts {
		data, _ := json.Marshal(p)
		var m any
		_ = json.Unmarshal(data, &m)
		result[i] = m
	}
	return result
}

// OpenAI Response -> Gemini Response conversion

// OpenAIResponseToGemini converts an OpenAI non-streaming response to Gemini format.
func OpenAIResponseToGemini(resp *OpenAIChatResponse) *GeminiResponse {
	out := &GeminiResponse{
		Model: resp.Model,
	}

	for _, choice := range resp.Choices {
		candidate := GeminiCandidate{
			Index: choice.Index,
			Content: &GeminiContent{
				Role: "model",
			},
		}

		msg := choice.Message
		if msg == nil {
			msg = choice.Delta
		}
		if msg != nil {
			// Reasoning content
			if msg.ReasoningContent != nil {
				for _, text := range collectReasoningTexts(msg.ReasoningContent) {
					if text == "" {
						continue
					}
					candidate.Content.Parts = append(candidate.Content.Parts, GeminiPart{
						Text:    text,
						Thought: Ptr(true),
					})
				}
			}

			// Text content
			if cs := msg.ContentString(); cs != "" {
				candidate.Content.Parts = append(candidate.Content.Parts, GeminiPart{Text: cs})
			}

			// Tool calls
			for _, tc := range msg.ToolCalls {
				candidate.Content.Parts = append(candidate.Content.Parts, GeminiPart{
					FunctionCall: &GeminiFunctionCall{
						Name: tc.Function.Name,
						Args: ParseJSONArgs(tc.Function.Arguments),
					},
				})
			}
		}

		if choice.FinishReason != nil {
			candidate.FinishReason = MapOpenAIFinishReasonToGemini(*choice.FinishReason)
		}

		out.Candidates = append(out.Candidates, candidate)
	}

	if resp.Usage != nil {
		out.UsageMetadata = &GeminiUsageMetadata{
			PromptTokenCount:     resp.Usage.PromptTokens,
			CandidatesTokenCount: resp.Usage.CompletionTokens,
			TotalTokenCount:      resp.Usage.TotalTokens,
		}
		if resp.Usage.CompletionTokensDetails != nil && resp.Usage.CompletionTokensDetails.ReasoningTokens > 0 {
			out.UsageMetadata.ThoughtsTokenCount = resp.Usage.CompletionTokensDetails.ReasoningTokens
		}
	}

	return out
}

// OpenAI Streaming -> Gemini Streaming conversion

type OpenAIToGeminiAccum struct {
	ToolCallsAccumulator map[int]*ToolCallAccum
	ContentAccumulator   strings.Builder
	IsFirstChunk         bool
}

func NewOpenAIToGeminiAccum() *OpenAIToGeminiAccum {
	return &OpenAIToGeminiAccum{
		ToolCallsAccumulator: make(map[int]*ToolCallAccum),
	}
}

// ConvertOpenAIChunkToGemini converts an OpenAI streaming chunk to Gemini format.
func ConvertOpenAIChunkToGemini(chunk *OpenAIChatResponse, accum *OpenAIToGeminiAccum) []string {
	if len(chunk.Choices) == 0 {
		// Usage-only chunk
		if chunk.Usage != nil {
			resp := GeminiResponse{
				Model: chunk.Model,
				UsageMetadata: &GeminiUsageMetadata{
					PromptTokenCount:     chunk.Usage.PromptTokens,
					CandidatesTokenCount: chunk.Usage.CompletionTokens,
					TotalTokenCount:      chunk.Usage.TotalTokens,
				},
			}
			if chunk.Usage.CompletionTokensDetails != nil {
				resp.UsageMetadata.ThoughtsTokenCount = chunk.Usage.CompletionTokensDetails.ReasoningTokens
			}
			data, _ := json.Marshal(resp)
			return []string{string(data)}
		}
		return nil
	}

	var results []string
	choice := chunk.Choices[0]
	delta := choice.Delta
	if delta == nil {
		return nil
	}

	baseTemplate := func() GeminiResponse {
		return GeminiResponse{
			Model: chunk.Model,
			Candidates: []GeminiCandidate{{
				Content: &GeminiContent{
					Parts: []GeminiPart{},
					Role:  "model",
				},
				Index: 0,
			}},
		}
	}

	// Reasoning content
	if delta.ReasoningContent != nil {
		for _, text := range collectReasoningTexts(delta.ReasoningContent) {
			if text == "" {
				continue
			}
			resp := baseTemplate()
			resp.Candidates[0].Content.Parts = []GeminiPart{{
				Text:    text,
				Thought: Ptr(true),
			}}
			data, _ := json.Marshal(resp)
			results = append(results, string(data))
		}
	}

	// Text content
	if cs := delta.ContentString(); cs != "" {
		accum.ContentAccumulator.WriteString(cs)
		resp := baseTemplate()
		resp.Candidates[0].Content.Parts = []GeminiPart{{Text: cs}}
		data, _ := json.Marshal(resp)
		results = append(results, string(data))
	}

	if len(results) > 0 {
		return results
	}

	// Tool calls (accumulate)
	for _, tc := range delta.ToolCalls {
		index := 0
		if tc.Index != nil {
			index = *tc.Index
		}
		if _, exists := accum.ToolCallsAccumulator[index]; !exists {
			accum.ToolCallsAccumulator[index] = &ToolCallAccum{
				ID:   tc.ID,
				Name: tc.Function.Name,
			}
		}
		acc := accum.ToolCallsAccumulator[index]
		if tc.ID != "" {
			acc.ID = tc.ID
		}
		if tc.Function.Name != "" {
			acc.Name = tc.Function.Name
		}
		if tc.Function.Arguments != "" {
			acc.Arguments.WriteString(tc.Function.Arguments)
		}
	}

	// Finish reason - emit accumulated tool calls
	if choice.FinishReason != nil {
		resp := baseTemplate()
		resp.Candidates[0].FinishReason = MapOpenAIFinishReasonToGemini(*choice.FinishReason)

		if len(accum.ToolCallsAccumulator) > 0 {
			var parts []GeminiPart
			for _, acc := range accum.ToolCallsAccumulator {
				parts = append(parts, GeminiPart{
					FunctionCall: &GeminiFunctionCall{
						Name: acc.Name,
						Args: ParseJSONArgs(acc.Arguments.String()),
					},
				})
			}
			resp.Candidates[0].Content.Parts = parts
			accum.ToolCallsAccumulator = make(map[int]*ToolCallAccum)
		}

		data, _ := json.Marshal(resp)
		results = append(results, string(data))
		return results
	}

	// Usage
	if chunk.Usage != nil {
		resp := baseTemplate()
		resp.UsageMetadata = &GeminiUsageMetadata{
			PromptTokenCount:     chunk.Usage.PromptTokens,
			CandidatesTokenCount: chunk.Usage.CompletionTokens,
			TotalTokenCount:      chunk.Usage.TotalTokens,
		}
		if chunk.Usage.CompletionTokensDetails != nil {
			resp.UsageMetadata.ThoughtsTokenCount = chunk.Usage.CompletionTokensDetails.ReasoningTokens
		}
		data, _ := json.Marshal(resp)
		results = append(results, string(data))
	}

	return results
}

// GeminiTokenCountResponse generates a Gemini-format token count response.
func GeminiTokenCountResponse(count int64) string {
	return fmt.Sprintf(`{"totalTokens":%d,"promptTokensDetails":[{"modality":"TEXT","tokenCount":%d}]}`, count, count)
}
