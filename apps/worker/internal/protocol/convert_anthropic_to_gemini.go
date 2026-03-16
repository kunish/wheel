package protocol

import (
	"bytes"
	"encoding/json"
	"strings"
)

// DefaultGeminiSafetySettings returns the default safety settings for Gemini requests.
func DefaultGeminiSafetySettings() []GeminiSafety {
	categories := []string{
		"HARM_CATEGORY_DANGEROUS_CONTENT",
		"HARM_CATEGORY_HARASSMENT",
		"HARM_CATEGORY_HATE_SPEECH",
		"HARM_CATEGORY_SEXUALLY_EXPLICIT",
		"HARM_CATEGORY_CIVIC_INTEGRITY",
	}
	settings := make([]GeminiSafety, len(categories))
	for i, cat := range categories {
		settings[i] = GeminiSafety{
			Category:  cat,
			Threshold: "BLOCK_NONE",
		}
	}
	return settings
}

// AnthropicRequestToGemini converts an Anthropic Messages API request to a Gemini request.
func AnthropicRequestToGemini(req *AnthropicRequest, modelName string) *GeminiRequest {
	// Strip "url" format from JSON schemas (Gemini doesn't support it)
	rawReq, _ := json.Marshal(req)
	rawReq = bytes.Replace(rawReq, []byte(`"url":{"type":"string","format":"uri",`), []byte(`"url":{"type":"string",`), -1)
	var cleanReq AnthropicRequest
	_ = json.Unmarshal(rawReq, &cleanReq)

	out := &GeminiRequest{
		Model:          modelName,
		SafetySettings: DefaultGeminiSafetySettings(),
	}

	// System instruction
	if cleanReq.System != nil {
		sysParts := convertAnthropicSystemToGeminiParts(cleanReq.System)
		if len(sysParts) > 0 {
			out.SystemInstruction = &GeminiContent{
				Role:  "user",
				Parts: sysParts,
			}
		}
	}

	// Messages -> Contents
	for _, msg := range cleanReq.Messages {
		content := convertAnthropicMessageToGeminiContent(msg)
		if content != nil {
			out.Contents = append(out.Contents, *content)
		}
	}

	// Tools
	if len(cleanReq.Tools) > 0 {
		var funcDecls []GeminiFuncDecl
		for _, tool := range cleanReq.Tools {
			fd := GeminiFuncDecl{
				Name:        tool.Name,
				Description: tool.Description,
			}
			if tool.InputSchema != nil {
				fd.ParametersJSONSchema = tool.InputSchema
			}
			funcDecls = append(funcDecls, fd)
		}
		if len(funcDecls) > 0 {
			out.Tools = []GeminiToolDecl{{FunctionDeclarations: funcDecls}}
		}
	}

	// Tool choice
	if cleanReq.ToolChoice != nil {
		out.ToolConfig = convertAnthropicToolChoiceToGemini(cleanReq.ToolChoice)
	}

	// Thinking config
	convertAnthropicThinkingToGemini(&cleanReq, out, modelName)

	// Generation config params
	if cleanReq.Temperature != nil {
		if out.GenerationConfig == nil {
			out.GenerationConfig = &GeminiGenConfig{}
		}
		out.GenerationConfig.Temperature = cleanReq.Temperature
	}
	if cleanReq.TopP != nil {
		if out.GenerationConfig == nil {
			out.GenerationConfig = &GeminiGenConfig{}
		}
		out.GenerationConfig.TopP = cleanReq.TopP
	}
	if cleanReq.TopK != nil {
		if out.GenerationConfig == nil {
			out.GenerationConfig = &GeminiGenConfig{}
		}
		k := int64(*cleanReq.TopK)
		out.GenerationConfig.TopK = &k
	}

	return out
}

func convertAnthropicSystemToGeminiParts(system any) []GeminiPart {
	switch v := system.(type) {
	case string:
		if v != "" {
			return []GeminiPart{{Text: v}}
		}
	case []any:
		var parts []GeminiPart
		for _, item := range v {
			data, err := json.Marshal(item)
			if err != nil {
				continue
			}
			var block AnthropicContentBlock
			if err := json.Unmarshal(data, &block); err != nil {
				continue
			}
			if block.Type == "text" && block.Text != "" {
				parts = append(parts, GeminiPart{Text: block.Text})
			}
		}
		return parts
	}
	return nil
}

func convertAnthropicMessageToGeminiContent(msg AnthropicMessage) *GeminiContent {
	role := msg.Role
	if role == "assistant" {
		role = "model"
	}

	content := &GeminiContent{Role: role}

	if s, ok := msg.Content.(string); ok {
		content.Parts = []GeminiPart{{Text: s}}
		return content
	}

	blocks := parseAnthropicContentBlocks(msg.Content)
	for _, block := range blocks {
		switch block.Type {
		case "text":
			content.Parts = append(content.Parts, GeminiPart{Text: block.Text})

		case "tool_use":
			var args map[string]any
			if block.Input != nil {
				data, err := json.Marshal(block.Input)
				if err == nil {
					_ = json.Unmarshal(data, &args)
				}
			}
			funcName := block.Name
			if block.ID != "" {
				if derived := toolNameFromClaudeToolUseID(block.ID); derived != "" {
					funcName = derived
				}
			}
			content.Parts = append(content.Parts, GeminiPart{
				ThoughtSignature: "skip_thought_signature_validator",
				FunctionCall: &GeminiFunctionCall{
					Name: funcName,
					Args: args,
				},
			})

		case "tool_result":
			funcName := ""
			if block.ToolUseID != "" {
				funcName = toolNameFromClaudeToolUseID(block.ToolUseID)
				if funcName == "" {
					funcName = block.ToolUseID
				}
			}
			responseData := ""
			if block.Text != "" {
				responseData = block.Text
			}
			content.Parts = append(content.Parts, GeminiPart{
				FunctionResponse: &GeminiFuncResponse{
					Name: funcName,
					Response: map[string]any{
						"result": responseData,
					},
				},
			})
		}
	}

	if len(content.Parts) == 0 {
		return nil
	}

	return content
}

func convertAnthropicToolChoiceToGemini(choice any) *GeminiToolConfig {
	tc := &GeminiToolConfig{
		FunctionCallingConfig: &GeminiFuncCallingConfig{},
	}

	switch v := choice.(type) {
	case string:
		switch v {
		case "auto":
			tc.FunctionCallingConfig.Mode = "AUTO"
		case "none":
			tc.FunctionCallingConfig.Mode = "NONE"
		case "any":
			tc.FunctionCallingConfig.Mode = "ANY"
		}
	case map[string]any:
		tcType, _ := v["type"].(string)
		switch tcType {
		case "auto":
			tc.FunctionCallingConfig.Mode = "AUTO"
		case "none":
			tc.FunctionCallingConfig.Mode = "NONE"
		case "any":
			tc.FunctionCallingConfig.Mode = "ANY"
		case "tool":
			tc.FunctionCallingConfig.Mode = "ANY"
			if name, ok := v["name"].(string); ok && name != "" {
				tc.FunctionCallingConfig.AllowedFunctionNames = []string{name}
			}
		}
	}

	return tc
}

func convertAnthropicThinkingToGemini(req *AnthropicRequest, out *GeminiRequest, modelName string) {
	if req.Thinking == nil {
		return
	}
	switch req.Thinking.Type {
	case "enabled":
		if req.Thinking.BudgetTokens > 0 {
			if out.GenerationConfig == nil {
				out.GenerationConfig = &GeminiGenConfig{}
			}
			out.GenerationConfig.ThinkingConfig = &GeminiThinkConfig{
				ThinkingBudget:  Ptr(req.Thinking.BudgetTokens),
				IncludeThoughts: Ptr(true),
			}
		}
	case "adaptive", "auto":
		if out.GenerationConfig == nil {
			out.GenerationConfig = &GeminiGenConfig{}
		}
		effort := ""
		if req.OutputConfig != nil && req.OutputConfig.Effort != "" {
			effort = strings.ToLower(strings.TrimSpace(req.OutputConfig.Effort))
		}
		if effort != "" {
			out.GenerationConfig.ThinkingConfig = &GeminiThinkConfig{
				ThinkingLevel:   effort,
				IncludeThoughts: Ptr(true),
			}
		} else {
			out.GenerationConfig.ThinkingConfig = &GeminiThinkConfig{
				ThinkingLevel:   "high",
				IncludeThoughts: Ptr(true),
			}
		}
	}
}

func toolNameFromClaudeToolUseID(toolUseID string) string {
	parts := strings.Split(toolUseID, "-")
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts[0:len(parts)-1], "-")
}
