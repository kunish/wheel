package relay

import (
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/genai"

	"github.com/kunish/wheel/apps/worker/internal/protocol"
)

func buildGeminiRequest(baseUrl, key string, body map[string]any, model string, channel ChannelConfig) UpstreamRequest {
	outBody := copyBody(body)
	delete(outBody, "model")

	messages, _ := outBody["messages"].([]any)
	delete(outBody, "messages")

	var systemParts []*genai.Part
	var contents []*genai.Content

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
			contents = append(contents, &genai.Content{Role: "model", Parts: parts})
		case "tool":
			contents = append(contents, toolResultToGeminiContentTyped(msg))
		default:
			contents = append(contents, &genai.Content{
				Role:  "user",
				Parts: contentToGeminiPartsList(msg["content"]),
			})
		}
	}

	geminiReq := protocol.GeminiRequest{Contents: contents}

	if len(systemParts) > 0 {
		geminiReq.SystemInstruction = &genai.Content{Parts: systemParts}
	}

	genConfig := &genai.GenerateContentConfig{}
	hasGenConfig := false
	if t, ok := outBody["temperature"].(float64); ok {
		f := float32(t)
		genConfig.Temperature = &f
		hasGenConfig = true
	}
	if tp, ok := outBody["top_p"].(float64); ok {
		f := float32(tp)
		genConfig.TopP = &f
		hasGenConfig = true
	}
	if mt, ok := outBody["max_tokens"].(float64); ok && mt > 0 {
		genConfig.MaxOutputTokens = int32(mt)
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

func contentToGeminiPartsList(content any) []*genai.Part {
	if s, ok := content.(string); ok {
		return []*genai.Part{{Text: s}}
	}
	parts, ok := content.([]any)
	if !ok {
		return []*genai.Part{{Text: fmt.Sprint(content)}}
	}
	var result []*genai.Part
	for _, p := range parts {
		part, ok := p.(map[string]any)
		if !ok {
			continue
		}
		partType, _ := part["type"].(string)
		switch partType {
		case "text":
			text, _ := part["text"].(string)
			result = append(result, &genai.Part{Text: text})
		case "image_url":
			if imgURL, ok := part["image_url"].(map[string]any); ok {
				url, _ := imgURL["url"].(string)
				if strings.HasPrefix(url, "data:") {
					mediaType, data := protocol.ParseDataURL(url)
					result = append(result, &genai.Part{
						InlineData: &genai.Blob{
							MIMEType: mediaType,
							Data:     []byte(data),
						},
					})
				}
			}
		}
	}
	if len(result) == 0 {
		result = append(result, &genai.Part{Text: ""})
	}
	return result
}

func assistantToGeminiPartsList(msg map[string]any) []*genai.Part {
	toolCalls, ok := msg["tool_calls"].([]any)
	if ok && len(toolCalls) > 0 {
		var parts []*genai.Part
		if content, ok := msg["content"].(string); ok && content != "" {
			parts = append(parts, &genai.Part{Text: content})
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
			parts = append(parts, &genai.Part{
				FunctionCall: &genai.FunctionCall{Name: name, Args: args},
			})
		}
		return parts
	}
	return contentToGeminiPartsList(msg["content"])
}

func toolResultToGeminiContentTyped(msg map[string]any) *genai.Content {
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
	return &genai.Content{
		Role: "user",
		Parts: []*genai.Part{{
			FunctionResponse: &genai.FunctionResponse{
				Name:     name,
				Response: response,
			},
		}},
	}
}

func convertOpenAIToolsToGeminiTyped(tools []any) []*genai.Tool {
	var decls []*genai.FunctionDeclaration
	for _, t := range tools {
		tool, ok := t.(map[string]any)
		if !ok || tool["type"] != "function" {
			continue
		}
		fn, ok := tool["function"].(map[string]any)
		if !ok {
			continue
		}
		decl := &genai.FunctionDeclaration{
			Name: fmt.Sprint(fn["name"]),
		}
		if desc, ok := fn["description"].(string); ok {
			decl.Description = desc
		}
		if params := fn["parameters"]; params != nil {
			decl.ParametersJsonSchema = params
		}
		decls = append(decls, decl)
	}
	if len(decls) == 0 {
		return nil
	}
	return []*genai.Tool{{FunctionDeclarations: decls}}
}

// Local types for OpenAI response serialization (the SDK types have special
// field types that are inconvenient for building responses from scratch).

type relayOpenAIResponse struct {
	ID      string              `json:"id"`
	Object  string              `json:"object"`
	Created int64               `json:"created"`
	Model   string              `json:"model"`
	Choices []relayOpenAIChoice `json:"choices"`
	Usage   *relayOpenAIUsage   `json:"usage,omitempty"`
}

type relayOpenAIChoice struct {
	Index        int32              `json:"index"`
	Message      *relayOpenAIMsg    `json:"message,omitempty"`
	FinishReason *string            `json:"finish_reason,omitempty"`
}

type relayOpenAIMsg struct {
	Role      string                `json:"role"`
	Content   any                   `json:"content"`
	ToolCalls []relayOpenAIToolCall `json:"tool_calls,omitempty"`
}

type relayOpenAIToolCall struct {
	Index    *int                    `json:"index,omitempty"`
	ID       string                  `json:"id"`
	Type     string                  `json:"type"`
	Function relayOpenAIFunctionCall `json:"function"`
}

type relayOpenAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type relayOpenAIUsage struct {
	PromptTokens     int32 `json:"prompt_tokens"`
	CompletionTokens int32 `json:"completion_tokens"`
	TotalTokens      int32 `json:"total_tokens"`
}

// convertGeminiResponse converts a Gemini response to OpenAI format.
func convertGeminiResponse(geminiResp map[string]any) map[string]any {
	data, err := json.Marshal(geminiResp)
	if err != nil {
		return geminiResp
	}
	var resp genai.GenerateContentResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return geminiResp
	}

	modelName, _ := geminiResp["model"].(string)
	if modelName == "" {
		modelName = resp.ModelVersion
	}

	openaiResp := geminiResponseToOpenAI(&resp, modelName)
	result, err := json.Marshal(openaiResp)
	if err != nil {
		return geminiResp
	}
	var out map[string]any
	_ = json.Unmarshal(result, &out)
	return out
}

func geminiResponseToOpenAI(resp *genai.GenerateContentResponse, modelName string) *relayOpenAIResponse {
	out := &relayOpenAIResponse{
		ID:      "chatcmpl-gemini",
		Object:  "chat.completion",
		Created: currentUnixSec(),
		Model:   modelName,
	}

	for _, candidate := range resp.Candidates {
		choice := relayOpenAIChoice{
			Index: candidate.Index,
			Message: &relayOpenAIMsg{
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
					choice.Message.ToolCalls = append(choice.Message.ToolCalls, relayOpenAIToolCall{
						Index: &idx,
						ID:    fmt.Sprintf("call_%d", idx),
						Type:  "function",
						Function: relayOpenAIFunctionCall{
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
			reason := protocol.MapGeminiFinishReasonToOpenAI(string(candidate.FinishReason))
			choice.FinishReason = &reason
		}

		out.Choices = append(out.Choices, choice)
	}

	if resp.UsageMetadata != nil {
		out.Usage = &relayOpenAIUsage{
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
