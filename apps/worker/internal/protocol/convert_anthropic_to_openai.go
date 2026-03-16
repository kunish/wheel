package protocol

import (
	"encoding/json"
	"strings"
)

// AnthropicRequestToOpenAI converts an Anthropic Messages API request to an OpenAI Chat Completions request.
func AnthropicRequestToOpenAI(req *AnthropicRequest, modelName string, stream bool) *OpenAIChatRequest {
	out := &OpenAIChatRequest{
		Model:  modelName,
		Stream: Ptr(stream),
	}

	if req.MaxTokens > 0 {
		out.MaxTokens = &req.MaxTokens
	}
	if req.Temperature != nil {
		out.Temperature = req.Temperature
	} else if req.TopP != nil {
		out.TopP = req.TopP
	}

	out.Stop = convertAnthropicStopSequences(req.StopSequences)
	out.ReasoningEffort = convertAnthropicThinkingToReasoningEffort(req)
	if req.User != "" {
		out.User = req.User
	}

	// System message
	if req.System != nil {
		if sysMsg := convertAnthropicSystemToOpenAI(req.System); sysMsg != nil {
			out.Messages = append(out.Messages, *sysMsg)
		}
	}

	// Messages
	for _, msg := range req.Messages {
		converted := convertAnthropicMessageToOpenAI(msg)
		out.Messages = append(out.Messages, converted...)
	}

	// Tools
	for _, tool := range req.Tools {
		out.Tools = append(out.Tools, OpenAITool{
			Type: "function",
			Function: OpenAIFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}

	// Tool choice
	out.ToolChoice = convertAnthropicToolChoiceToOpenAI(req.ToolChoice)

	return out
}

func convertAnthropicStopSequences(seqs []string) any {
	if len(seqs) == 0 {
		return nil
	}
	if len(seqs) == 1 {
		return seqs[0]
	}
	return seqs
}

func convertAnthropicThinkingToReasoningEffort(req *AnthropicRequest) string {
	if req.Thinking == nil {
		return ""
	}
	switch req.Thinking.Type {
	case "enabled":
		if req.Thinking.BudgetTokens > 0 {
			return budgetToLevel(req.Thinking.BudgetTokens)
		}
		return budgetToLevel(-1) // auto
	case "adaptive", "auto":
		if req.OutputConfig != nil && req.OutputConfig.Effort != "" {
			return strings.ToLower(strings.TrimSpace(req.OutputConfig.Effort))
		}
		return "xhigh"
	case "disabled":
		return budgetToLevel(0)
	}
	return ""
}

func budgetToLevel(budget int) string {
	switch {
	case budget == 0:
		return ""
	case budget < 0:
		return "medium"
	case budget <= 1024:
		return "low"
	case budget <= 8192:
		return "medium"
	default:
		return "high"
	}
}

func convertAnthropicSystemToOpenAI(system any) *OpenAIMessage {
	switch v := system.(type) {
	case string:
		if v == "" {
			return nil
		}
		return &OpenAIMessage{
			Role: "system",
			Content: []OpenAIContentPart{
				{Type: "text", Text: v},
			},
		}
	case []any:
		var parts []OpenAIContentPart
		for _, item := range v {
			data, err := json.Marshal(item)
			if err != nil {
				continue
			}
			var block AnthropicContentBlock
			if err := json.Unmarshal(data, &block); err != nil {
				continue
			}
			if part, ok := anthropicContentPartToOpenAI(block); ok {
				parts = append(parts, part)
			}
		}
		if len(parts) == 0 {
			return nil
		}
		return &OpenAIMessage{
			Role:    "system",
			Content: parts,
		}
	}
	return nil
}

func convertAnthropicMessageToOpenAI(msg AnthropicMessage) []OpenAIMessage {
	// Handle simple string content
	if s, ok := msg.Content.(string); ok {
		return []OpenAIMessage{{
			Role:    msg.Role,
			Content: s,
		}}
	}

	blocks := parseAnthropicContentBlocks(msg.Content)
	if len(blocks) == 0 {
		return nil
	}

	var contentParts []OpenAIContentPart
	var reasoningParts []string
	var toolCalls []OpenAIToolCall
	var toolResults []OpenAIMessage

	for _, block := range blocks {
		switch block.Type {
		case "thinking":
			if msg.Role == "assistant" {
				text := block.Thinking
				if strings.TrimSpace(text) != "" {
					reasoningParts = append(reasoningParts, text)
				}
			}
		case "redacted_thinking":
			// Explicitly ignored
		case "text", "image":
			if part, ok := anthropicContentPartToOpenAI(block); ok {
				contentParts = append(contentParts, part)
			}
		case "tool_use":
			if msg.Role == "assistant" {
				argsStr := "{}"
				if block.Input != nil {
					data, err := json.Marshal(block.Input)
					if err == nil {
						argsStr = string(data)
					}
				}
				toolCalls = append(toolCalls, OpenAIToolCall{
					ID:   block.ID,
					Type: "function",
					Function: OpenAIFunctionCall{
						Name:      block.Name,
						Arguments: argsStr,
					},
				})
			}
		case "tool_result":
			toolResults = append(toolResults, OpenAIMessage{
				Role:       "tool",
				ToolCallID: block.ToolUseID,
				Content:    convertAnthropicToolResultToOpenAI(block),
			})
		}
	}

	var result []OpenAIMessage

	// Tool results go first (to maintain OpenAI ordering: tool results follow the assistant's tool_calls)
	result = append(result, toolResults...)

	hasContent := len(contentParts) > 0
	hasReasoning := len(reasoningParts) > 0
	hasToolCalls := len(toolCalls) > 0

	if msg.Role == "assistant" {
		if hasContent || hasReasoning || hasToolCalls {
			outMsg := OpenAIMessage{Role: "assistant"}
			if hasContent {
				outMsg.Content = contentParts
			} else {
				outMsg.Content = ""
			}
			if hasReasoning {
				outMsg.ReasoningContent = strings.Join(reasoningParts, "\n\n")
			}
			if hasToolCalls {
				outMsg.ToolCalls = toolCalls
			}
			result = append(result, outMsg)
		}
	} else if hasContent {
		result = append(result, OpenAIMessage{
			Role:    msg.Role,
			Content: contentParts,
		})
	}

	return result
}

func anthropicContentPartToOpenAI(block AnthropicContentBlock) (OpenAIContentPart, bool) {
	switch block.Type {
	case "text":
		if strings.TrimSpace(block.Text) == "" {
			return OpenAIContentPart{}, false
		}
		return OpenAIContentPart{Type: "text", Text: block.Text}, true
	case "image":
		var imageURL string
		if block.Source != nil {
			switch block.Source.Type {
			case "base64":
				imageURL = ImageDataURL(block.Source.MediaType, block.Source.Data)
			case "url":
				imageURL = block.Source.URL
			}
		}
		if imageURL == "" {
			return OpenAIContentPart{}, false
		}
		return OpenAIContentPart{
			Type:     "image_url",
			ImageURL: &OpenAIImageURL{URL: imageURL},
		}, true
	}
	return OpenAIContentPart{}, false
}

func convertAnthropicToolResultToOpenAI(block AnthropicContentBlock) any {
	content := block.ToolResultContent
	if content == nil {
		return ""
	}

	switch v := content.(type) {
	case string:
		return v
	case []any:
		var textParts []string
		var hasImage bool
		var parts []OpenAIContentPart

		for _, item := range v {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			itemType, _ := itemMap["type"].(string)

			switch itemType {
			case "text":
				text, _ := itemMap["text"].(string)
				textParts = append(textParts, text)
				parts = append(parts, OpenAIContentPart{Type: "text", Text: text})
			case "image":
				hasImage = true
				source, _ := itemMap["source"].(map[string]any)
				if source != nil {
					srcType, _ := source["type"].(string)
					switch srcType {
					case "base64":
						mediaType, _ := source["media_type"].(string)
						data, _ := source["data"].(string)
						parts = append(parts, OpenAIContentPart{
							Type:     "image_url",
							ImageURL: &OpenAIImageURL{URL: ImageDataURL(mediaType, data)},
						})
					case "url":
						url, _ := source["url"].(string)
						parts = append(parts, OpenAIContentPart{
							Type:     "image_url",
							ImageURL: &OpenAIImageURL{URL: url},
						})
					}
				}
			default:
				if text, ok := itemMap["text"].(string); ok {
					textParts = append(textParts, text)
				}
			}
		}

		if hasImage {
			return contentPartsToAny(parts)
		}
		return strings.Join(textParts, "\n\n")

	case map[string]any:
		itemType, _ := v["type"].(string)
		if itemType == "image" {
			source, _ := v["source"].(map[string]any)
			if source != nil {
				srcType, _ := source["type"].(string)
				switch srcType {
				case "base64":
					mediaType, _ := source["media_type"].(string)
					data, _ := source["data"].(string)
					return contentPartsToAny([]OpenAIContentPart{{
						Type:     "image_url",
						ImageURL: &OpenAIImageURL{URL: ImageDataURL(mediaType, data)},
					}})
				case "url":
					url, _ := source["url"].(string)
					return contentPartsToAny([]OpenAIContentPart{{
						Type:     "image_url",
						ImageURL: &OpenAIImageURL{URL: url},
					}})
				}
			}
		}
		if text, ok := v["text"].(string); ok {
			return text
		}
		data, _ := json.Marshal(v)
		return string(data)
	}

	data, _ := json.Marshal(content)
	return string(data)
}

func convertAnthropicToolChoiceToOpenAI(choice any) any {
	if choice == nil {
		return nil
	}

	switch v := choice.(type) {
	case string:
		return v
	case map[string]any:
		tcType, _ := v["type"].(string)
		switch tcType {
		case "auto":
			return "auto"
		case "any":
			return "required"
		case "tool":
			name, _ := v["name"].(string)
			return OpenAIToolChoiceFunction{
				Type:     "function",
				Function: OpenAIToolChoiceFunctionName{Name: name},
			}
		default:
			return "auto"
		}
	}
	return nil
}

func parseAnthropicContentBlocks(content any) []AnthropicContentBlock {
	arr, ok := content.([]any)
	if !ok {
		return nil
	}
	var blocks []AnthropicContentBlock
	for _, item := range arr {
		data, err := json.Marshal(item)
		if err != nil {
			continue
		}
		var block AnthropicContentBlock
		if err := json.Unmarshal(data, &block); err != nil {
			continue
		}
		blocks = append(blocks, block)
	}
	return blocks
}
