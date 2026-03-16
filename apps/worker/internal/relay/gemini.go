package relay

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kunish/wheel/apps/worker/internal/protocol"
)

func buildGeminiRequest(baseUrl, key string, body map[string]any, model string, channel ChannelConfig) UpstreamRequest {
	outBody := copyBody(body)
	delete(outBody, "model")

	messages, _ := outBody["messages"].([]any)
	delete(outBody, "messages")

	var systemParts []protocol.GeminiPart
	var contents []protocol.GeminiContent

	for _, m := range messages {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		switch role {
		case "system":
			systemParts = append(systemParts, contentToGeminiPartsList(msg["content"])...)
		case "assistant":
			parts := assistantToGeminiPartsList(msg)
			contents = append(contents, protocol.GeminiContent{Role: "model", Parts: parts})
		case "tool":
			contents = append(contents, toolResultToGeminiContentTyped(msg))
		default:
			contents = append(contents, protocol.GeminiContent{
				Role:  "user",
				Parts: contentToGeminiPartsList(msg["content"]),
			})
		}
	}

	geminiReq := protocol.GeminiRequest{Contents: contents}

	if len(systemParts) > 0 {
		geminiReq.SystemInstruction = &protocol.GeminiContent{Parts: systemParts}
	}

	genConfig := &protocol.GeminiGenConfig{}
	hasGenConfig := false
	if t, ok := outBody["temperature"].(float64); ok {
		genConfig.Temperature = &t
		hasGenConfig = true
	}
	if tp, ok := outBody["top_p"].(float64); ok {
		genConfig.TopP = &tp
		hasGenConfig = true
	}
	if mt, ok := outBody["max_tokens"].(float64); ok && mt > 0 {
		v := int64(mt)
		genConfig.MaxOutputTokens = &v
		hasGenConfig = true
	}
	if stop, ok := outBody["stop"].([]any); ok {
		var seqs []string
		for _, s := range stop {
			if str, ok := s.(string); ok {
				seqs = append(seqs, str)
			}
		}
		if len(seqs) > 0 {
			genConfig.StopSequences = seqs
			hasGenConfig = true
		}
	}
	if hasGenConfig {
		geminiReq.GenerationConfig = genConfig
	}

	if tools, ok := outBody["tools"].([]any); ok {
		geminiReq.Tools = convertOpenAIToolsToGeminiTyped(tools)
	}

	geminiBody, _ := json.Marshal(geminiReq)
	var geminiMap map[string]any
	_ = json.Unmarshal(geminiBody, &geminiMap)
	applyParamOverrides(geminiMap, channel.ParamOverride)

	stream, _ := body["stream"].(bool)
	var endpoint string
	if stream {
		endpoint = fmt.Sprintf("/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s", model, key)
	} else {
		endpoint = fmt.Sprintf("/v1beta/models/%s:generateContent?key=%s", model, key)
	}

	headers := map[string]string{"Content-Type": "application/json"}
	for _, h := range channel.CustomHeader {
		headers[h.Key] = h.Value
	}

	bodyJSON, _ := json.Marshal(geminiMap)
	return UpstreamRequest{
		URL:     baseUrl + endpoint,
		Headers: headers,
		Body:    string(bodyJSON),
	}
}

func contentToGeminiPartsList(content any) []protocol.GeminiPart {
	if s, ok := content.(string); ok {
		return []protocol.GeminiPart{{Text: s}}
	}
	parts, ok := content.([]any)
	if !ok {
		return []protocol.GeminiPart{{Text: fmt.Sprint(content)}}
	}
	var result []protocol.GeminiPart
	for _, p := range parts {
		part, ok := p.(map[string]any)
		if !ok {
			continue
		}
		partType, _ := part["type"].(string)
		switch partType {
		case "text":
			text, _ := part["text"].(string)
			result = append(result, protocol.GeminiPart{Text: text})
		case "image_url":
			if imgURL, ok := part["image_url"].(map[string]any); ok {
				url, _ := imgURL["url"].(string)
				if strings.HasPrefix(url, "data:") {
					mediaType, data := protocol.ParseDataURL(url)
					result = append(result, protocol.GeminiPart{
						InlineData: &protocol.GeminiInlineData{
							MimeType: mediaType,
							Data:     data,
						},
					})
				}
			}
		}
	}
	if len(result) == 0 {
		result = append(result, protocol.GeminiPart{Text: ""})
	}
	return result
}

func assistantToGeminiPartsList(msg map[string]any) []protocol.GeminiPart {
	toolCalls, ok := msg["tool_calls"].([]any)
	if ok && len(toolCalls) > 0 {
		var parts []protocol.GeminiPart
		if content, ok := msg["content"].(string); ok && content != "" {
			parts = append(parts, protocol.GeminiPart{Text: content})
		}
		for _, tc := range toolCalls {
			tcMap, ok := tc.(map[string]any)
			if !ok {
				continue
			}
			fn, _ := tcMap["function"].(map[string]any)
			if fn == nil {
				continue
			}
			name, _ := fn["name"].(string)
			args := protocol.ParseJSONArgs(fmt.Sprint(fn["arguments"]))
			parts = append(parts, protocol.GeminiPart{
				FunctionCall: &protocol.GeminiFunctionCall{Name: name, Args: args},
			})
		}
		return parts
	}
	return contentToGeminiPartsList(msg["content"])
}

func toolResultToGeminiContentTyped(msg map[string]any) protocol.GeminiContent {
	name, _ := msg["name"].(string)
	content := msg["content"]
	var response map[string]any
	if s, ok := content.(string); ok {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(s), &parsed); err == nil {
			response = parsed
		} else {
			response = map[string]any{"result": s}
		}
	} else if m, ok := content.(map[string]any); ok {
		response = m
	} else {
		response = map[string]any{"result": fmt.Sprint(content)}
	}
	return protocol.GeminiContent{
		Role: "user",
		Parts: []protocol.GeminiPart{{
			FunctionResponse: &protocol.GeminiFuncResponse{
				Name:     name,
				Response: response,
			},
		}},
	}
}

func convertOpenAIToolsToGeminiTyped(tools []any) []protocol.GeminiToolDecl {
	var decls []protocol.GeminiFuncDecl
	for _, t := range tools {
		tool, ok := t.(map[string]any)
		if !ok || tool["type"] != "function" {
			continue
		}
		fn, ok := tool["function"].(map[string]any)
		if !ok {
			continue
		}
		decl := protocol.GeminiFuncDecl{
			Name: fmt.Sprint(fn["name"]),
		}
		if desc, ok := fn["description"].(string); ok {
			decl.Description = desc
		}
		if params := fn["parameters"]; params != nil {
			decl.Parameters = params
		}
		decls = append(decls, decl)
	}
	if len(decls) == 0 {
		return nil
	}
	return []protocol.GeminiToolDecl{{FunctionDeclarations: decls}}
}

// convertGeminiResponse converts a Gemini response to OpenAI format.
func convertGeminiResponse(geminiResp map[string]any) map[string]any {
	data, err := json.Marshal(geminiResp)
	if err != nil {
		return geminiResp
	}
	var resp protocol.GeminiResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return geminiResp
	}

	openaiResp := geminiResponseToOpenAI(&resp)
	result, err := json.Marshal(openaiResp)
	if err != nil {
		return geminiResp
	}
	var out map[string]any
	_ = json.Unmarshal(result, &out)
	return out
}

func geminiResponseToOpenAI(resp *protocol.GeminiResponse) *protocol.OpenAIChatResponse {
	out := &protocol.OpenAIChatResponse{
		ID:      "chatcmpl-gemini",
		Object:  "chat.completion",
		Created: int64(currentUnixSec()),
		Model:   resp.Model,
	}

	for _, candidate := range resp.Candidates {
		choice := protocol.OpenAIChoice{
			Index: candidate.Index,
			Message: &protocol.OpenAIMessage{
				Role: "assistant",
			},
		}

		if candidate.Content != nil {
			var text strings.Builder
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					text.WriteString(part.Text)
				}
				if part.FunctionCall != nil {
					argsJSON, _ := json.Marshal(part.FunctionCall.Args)
					idx := len(choice.Message.ToolCalls)
					choice.Message.ToolCalls = append(choice.Message.ToolCalls, protocol.OpenAIToolCall{
						Index: &idx,
						ID:    fmt.Sprintf("call_%d", idx),
						Type:  "function",
						Function: protocol.OpenAIFunctionCall{
							Name:      part.FunctionCall.Name,
							Arguments: string(argsJSON),
						},
					})
				}
			}
			if text.Len() > 0 {
				choice.Message.Content = text.String()
			} else if len(choice.Message.ToolCalls) > 0 {
				choice.Message.Content = nil
			} else {
				choice.Message.Content = ""
			}
		}

		if candidate.FinishReason != "" {
			reason := protocol.MapGeminiFinishReasonToOpenAI(candidate.FinishReason)
			choice.FinishReason = &reason
		}

		out.Choices = append(out.Choices, choice)
	}

	if resp.UsageMetadata != nil {
		out.Usage = &protocol.OpenAIUsage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      resp.UsageMetadata.PromptTokenCount + resp.UsageMetadata.CandidatesTokenCount,
		}
	}

	return out
}

func mapGeminiFinishReason(reason string) string {
	return protocol.MapGeminiFinishReasonToOpenAI(reason)
}

// Legacy wrappers for vertex.go and other callers that use map[string]any.

func contentToGeminiParts(content any) []any {
	parts := contentToGeminiPartsList(content)
	result := make([]any, len(parts))
	for i, p := range parts {
		data, _ := json.Marshal(p)
		var m any
		_ = json.Unmarshal(data, &m)
		result[i] = m
	}
	return result
}

func assistantToGeminiParts(msg map[string]any) []any {
	parts := assistantToGeminiPartsList(msg)
	result := make([]any, len(parts))
	for i, p := range parts {
		data, _ := json.Marshal(p)
		var m any
		_ = json.Unmarshal(data, &m)
		result[i] = m
	}
	return result
}

func toolResultToGeminiContent(msg map[string]any) map[string]any {
	content := toolResultToGeminiContentTyped(msg)
	data, _ := json.Marshal(content)
	var result map[string]any
	_ = json.Unmarshal(data, &result)
	return result
}

func convertOpenAIToolsToGemini(tools []any) []any {
	typed := convertOpenAIToolsToGeminiTyped(tools)
	if typed == nil {
		return nil
	}
	data, _ := json.Marshal(typed)
	var result []any
	_ = json.Unmarshal(data, &result)
	return result
}
