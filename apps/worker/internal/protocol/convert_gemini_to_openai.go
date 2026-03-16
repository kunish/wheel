package protocol

import (
	"encoding/json"
	"strings"

	"google.golang.org/genai"
)

// GeminiRequestToOpenAI converts a Gemini API request to an OpenAI Chat Completions request.
// Uses genai.Content/Part types for the input and produces raw JSON for OpenAI.
func GeminiRequestToOpenAI(req *GeminiRequest, modelName string, stream bool) ([]byte, error) {
	out := map[string]any{
		"model":  modelName,
		"stream": stream,
	}

	// Generation config
	if gc := req.GenerationConfig; gc != nil {
		if gc.Temperature != nil {
			out["temperature"] = float64(*gc.Temperature)
		}
		if gc.TopP != nil {
			out["top_p"] = float64(*gc.TopP)
		}
		if gc.TopK != nil {
			out["top_k"] = float64(*gc.TopK)
		}
		if gc.MaxOutputTokens > 0 {
			out["max_tokens"] = int64(gc.MaxOutputTokens)
		}
		if len(gc.StopSequences) > 0 {
			out["stop"] = gc.StopSequences
		}
		if gc.CandidateCount > 0 {
			out["n"] = int64(gc.CandidateCount)
		}
	}

	var messages []map[string]any
	var toolCallIDs []string

	// System instruction
	if si := req.SystemInstruction; si != nil {
		var parts []map[string]any
		for _, part := range si.Parts {
			if part.Text != "" {
				parts = append(parts, map[string]any{"type": "text", "text": part.Text})
			}
			if part.InlineData != nil {
				parts = append(parts, map[string]any{
					"type":      "image_url",
					"image_url": map[string]string{"url": ImageDataURL(part.InlineData.MIMEType, string(part.InlineData.Data))},
				})
			}
		}
		if len(parts) > 0 {
			messages = append(messages, map[string]any{"role": "system", "content": parts})
		}
	}

	// Contents
	for _, content := range req.Contents {
		role := content.Role
		if role == "model" {
			role = "assistant"
		}

		var textBuilder strings.Builder
		var contentParts []map[string]any
		onlyText := true
		var toolCalls []map[string]any

		for _, part := range content.Parts {
			if part.Text != "" {
				textBuilder.WriteString(part.Text)
				contentParts = append(contentParts, map[string]any{"type": "text", "text": part.Text})
			}
			if part.InlineData != nil {
				onlyText = false
				contentParts = append(contentParts, map[string]any{
					"type":      "image_url",
					"image_url": map[string]string{"url": ImageDataURL(part.InlineData.MIMEType, string(part.InlineData.Data))},
				})
			}
			if part.FunctionCall != nil {
				toolCallID := GenOpenAIToolCallID()
				toolCallIDs = append(toolCallIDs, toolCallID)
				argsJSON, _ := json.Marshal(part.FunctionCall.Args)
				toolCalls = append(toolCalls, map[string]any{
					"id":   toolCallID,
					"type": "function",
					"function": map[string]string{
						"name":      part.FunctionCall.Name,
						"arguments": string(argsJSON),
					},
				})
			}
			if part.FunctionResponse != nil {
				toolMsg := map[string]any{"role": "tool"}
				if len(toolCallIDs) > 0 {
					toolMsg["tool_call_id"] = toolCallIDs[len(toolCallIDs)-1]
				} else {
					toolMsg["tool_call_id"] = GenOpenAIToolCallID()
				}
				if part.FunctionResponse.Response != nil {
					if contentField, ok := part.FunctionResponse.Response["content"]; ok {
						data, _ := json.Marshal(contentField)
						toolMsg["content"] = string(data)
					} else {
						data, _ := json.Marshal(part.FunctionResponse.Response)
						toolMsg["content"] = string(data)
					}
				}
				messages = append(messages, toolMsg)
			}
		}

		msg := map[string]any{"role": role}
		if len(contentParts) > 0 {
			if onlyText {
				msg["content"] = textBuilder.String()
			} else {
				msg["content"] = contentParts
			}
		}
		if len(toolCalls) > 0 {
			msg["tool_calls"] = toolCalls
		}
		messages = append(messages, msg)
	}

	out["messages"] = messages

	// Tools
	for _, tool := range req.Tools {
		for _, fd := range tool.FunctionDeclarations {
			var params any
			if fd.Parameters != nil {
				params = fd.Parameters
			} else if fd.ParametersJsonSchema != nil {
				params = fd.ParametersJsonSchema
			}
			toolDef := map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        fd.Name,
					"description": fd.Description,
				},
			}
			if params != nil {
				toolDef["function"].(map[string]any)["parameters"] = params
			}
			if out["tools"] == nil {
				out["tools"] = []any{}
			}
			out["tools"] = append(out["tools"].([]any), toolDef)
		}
	}

	// Tool config
	if tc := req.ToolConfig; tc != nil && tc.FunctionCallingConfig != nil {
		switch tc.FunctionCallingConfig.Mode {
		case genai.FunctionCallingConfigModeNone:
			out["tool_choice"] = "none"
		case genai.FunctionCallingConfigModeAuto:
			out["tool_choice"] = "auto"
		case genai.FunctionCallingConfigModeAny:
			out["tool_choice"] = "required"
		}
	}

	return json.Marshal(out)
}

// OpenAI streaming → Gemini streaming accumulator.

type OpenAIToGeminiAccum struct {
	ToolCallsAccumulator map[int]*ToolCallAccum
	ContentAccumulator   strings.Builder
}

func NewOpenAIToGeminiAccum() *OpenAIToGeminiAccum {
	return &OpenAIToGeminiAccum{
		ToolCallsAccumulator: make(map[int]*ToolCallAccum),
	}
}

// ConvertOpenAIChunkToGemini converts an OpenAI streaming chunk to Gemini response format.
func ConvertOpenAIChunkToGemini(chunkJSON []byte, accum *OpenAIToGeminiAccum) []string {
	var chunk struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Created int64  `json:"created"`
		Choices []struct {
			Index        int    `json:"index"`
			FinishReason string `json:"finish_reason"`
			Delta        struct {
				Role             string `json:"role"`
				Content          string `json:"content"`
				ReasoningContent any    `json:"reasoning_content"`
				ToolCalls        []struct {
					Index    int    `json:"index"`
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"delta"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens            int64 `json:"prompt_tokens"`
			CompletionTokens        int64 `json:"completion_tokens"`
			TotalTokens             int64 `json:"total_tokens"`
			CompletionTokensDetails *struct {
				ReasoningTokens int64 `json:"reasoning_tokens"`
			} `json:"completion_tokens_details"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(chunkJSON, &chunk); err != nil {
		return nil
	}

	if len(chunk.Choices) == 0 {
		if chunk.Usage != nil {
			resp := map[string]any{
				"candidates": []any{},
				"model":      chunk.Model,
				"usageMetadata": map[string]any{
					"promptTokenCount":     chunk.Usage.PromptTokens,
					"candidatesTokenCount": chunk.Usage.CompletionTokens,
					"totalTokenCount":      chunk.Usage.TotalTokens,
				},
			}
			if chunk.Usage.CompletionTokensDetails != nil && chunk.Usage.CompletionTokensDetails.ReasoningTokens > 0 {
				resp["usageMetadata"].(map[string]any)["thoughtsTokenCount"] = chunk.Usage.CompletionTokensDetails.ReasoningTokens
			}
			data, _ := json.Marshal(resp)
			return []string{string(data)}
		}
		return nil
	}

	var results []string
	choice := chunk.Choices[0]

	makeResp := func() map[string]any {
		return map[string]any{
			"model": chunk.Model,
			"candidates": []any{map[string]any{
				"content": map[string]any{"parts": []any{}, "role": "model"},
				"index":   0,
			}},
		}
	}

	// Reasoning content
	if choice.Delta.ReasoningContent != nil {
		if text, ok := choice.Delta.ReasoningContent.(string); ok && text != "" {
			resp := makeResp()
			resp["candidates"].([]any)[0].(map[string]any)["content"].(map[string]any)["parts"] = []any{
				map[string]any{"text": text, "thought": true},
			}
			data, _ := json.Marshal(resp)
			results = append(results, string(data))
		}
	}

	// Text content
	if choice.Delta.Content != "" {
		accum.ContentAccumulator.WriteString(choice.Delta.Content)
		resp := makeResp()
		resp["candidates"].([]any)[0].(map[string]any)["content"].(map[string]any)["parts"] = []any{
			map[string]any{"text": choice.Delta.Content},
		}
		data, _ := json.Marshal(resp)
		results = append(results, string(data))
	}

	if len(results) > 0 {
		return results
	}

	// Tool calls accumulate
	for _, tc := range choice.Delta.ToolCalls {
		index := tc.Index
		if _, exists := accum.ToolCallsAccumulator[index]; !exists {
			accum.ToolCallsAccumulator[index] = &ToolCallAccum{ID: tc.ID, Name: tc.Function.Name}
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
	if choice.FinishReason != "" {
		resp := makeResp()
		resp["candidates"].([]any)[0].(map[string]any)["finishReason"] = MapOpenAIFinishReasonToGeminiString(choice.FinishReason)

		if len(accum.ToolCallsAccumulator) > 0 {
			var parts []any
			for _, acc := range accum.ToolCallsAccumulator {
				parts = append(parts, map[string]any{
					"functionCall": map[string]any{
						"name": acc.Name,
						"args": ParseJSONArgs(acc.Arguments.String()),
					},
				})
			}
			resp["candidates"].([]any)[0].(map[string]any)["content"].(map[string]any)["parts"] = parts
			accum.ToolCallsAccumulator = make(map[int]*ToolCallAccum)
		}

		data, _ := json.Marshal(resp)
		results = append(results, string(data))
		return results
	}

	// Usage
	if chunk.Usage != nil {
		resp := makeResp()
		resp["usageMetadata"] = map[string]any{
			"promptTokenCount":     chunk.Usage.PromptTokens,
			"candidatesTokenCount": chunk.Usage.CompletionTokens,
			"totalTokenCount":      chunk.Usage.TotalTokens,
		}
		data, _ := json.Marshal(resp)
		results = append(results, string(data))
	}

	return results
}

