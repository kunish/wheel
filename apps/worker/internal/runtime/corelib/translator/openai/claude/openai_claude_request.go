// Package claude provides request translation functionality for Anthropic to OpenAI API.
package claude

import (
	"strings"

	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/thinking"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ConvertClaudeRequestToOpenAI parses and transforms an Anthropic API request into OpenAI Chat Completions API format.
func ConvertClaudeRequestToOpenAI(modelName string, inputRawJSON []byte, stream bool) []byte {
	root := gjson.ParseBytes(inputRawJSON)

	result := `{"model":"","stream":false}`
	result, _ = sjson.Set(result, "model", modelName)
	result, _ = sjson.Set(result, "stream", stream)

	if stream {
		result, _ = sjson.SetRaw(result, "stream_options", `{"include_usage":true}`)
	}

	// Max tokens
	if mt := root.Get("max_tokens"); mt.Exists() && mt.Int() > 0 {
		result, _ = sjson.Set(result, "max_tokens", mt.Int())
	}

	// Temperature / TopP
	if t := root.Get("temperature"); t.Exists() {
		result, _ = sjson.Set(result, "temperature", t.Float())
	} else if tp := root.Get("top_p"); tp.Exists() {
		result, _ = sjson.Set(result, "top_p", tp.Float())
	}

	// Stop sequences
	if stops := root.Get("stop_sequences"); stops.Exists() && stops.IsArray() {
		arr := stops.Array()
		if len(arr) == 1 {
			result, _ = sjson.Set(result, "stop", arr[0].String())
		} else if len(arr) > 1 {
			var seqs []string
			for _, s := range arr {
				seqs = append(seqs, s.String())
			}
			result, _ = sjson.Set(result, "stop", seqs)
		}
	}

	messagesJSON := "[]"

	// System messages
	sysField := root.Get("system")
	if sysField.Exists() {
		if sysField.Type == gjson.String && strings.TrimSpace(sysField.String()) != "" {
			msg := `{"role":"system","content":""}`
			msg, _ = sjson.Set(msg, "content", sysField.String())
			messagesJSON, _ = sjson.SetRaw(messagesJSON, "-1", msg)
		} else if sysField.IsArray() {
			partsJSON := "[]"
			hasParts := false
			for _, block := range sysField.Array() {
				if block.Get("type").String() == "text" && strings.TrimSpace(block.Get("text").String()) != "" {
					part := `{"type":"text","text":""}`
					part, _ = sjson.Set(part, "text", block.Get("text").String())
					partsJSON, _ = sjson.SetRaw(partsJSON, "-1", part)
					hasParts = true
				}
			}
			if hasParts {
				msg := `{"role":"system"}`
				msg, _ = sjson.SetRaw(msg, "content", partsJSON)
				messagesJSON, _ = sjson.SetRaw(messagesJSON, "-1", msg)
			}
		}
	}

	// Messages
	root.Get("messages").ForEach(func(_, msg gjson.Result) bool {
		role := msg.Get("role").String()
		content := msg.Get("content")

		if content.Type == gjson.String {
			m := `{"role":"","content":""}`
			m, _ = sjson.Set(m, "role", role)
			m, _ = sjson.Set(m, "content", content.String())
			messagesJSON, _ = sjson.SetRaw(messagesJSON, "-1", m)
			return true
		}

		if !content.IsArray() {
			return true
		}

		if role == "assistant" {
			messagesJSON = convertAssistantMessage(content, messagesJSON)
		} else {
			messagesJSON = convertUserMessage(role, content, messagesJSON)
		}
		return true
	})

	result, _ = sjson.SetRaw(result, "messages", messagesJSON)

	// Tools
	toolsJSON := "[]"
	hasTools := false
	root.Get("tools").ForEach(func(_, tool gjson.Result) bool {
		name := tool.Get("name").String()
		if name == "" {
			return true
		}
		toolDef := `{"type":"function","function":{"name":""}}`
		toolDef, _ = sjson.Set(toolDef, "function.name", name)
		if desc := tool.Get("description"); desc.Exists() {
			toolDef, _ = sjson.Set(toolDef, "function.description", desc.String())
		}
		if schema := tool.Get("input_schema"); schema.Exists() {
			toolDef, _ = sjson.SetRaw(toolDef, "function.parameters", schema.Raw)
		}
		toolsJSON, _ = sjson.SetRaw(toolsJSON, "-1", toolDef)
		hasTools = true
		return true
	})
	if hasTools {
		result, _ = sjson.SetRaw(result, "tools", toolsJSON)
	}

	// Tool choice
	tc := root.Get("tool_choice")
	if tc.Exists() {
		tcType := tc.Get("type").String()
		switch tcType {
		case "auto":
			result, _ = sjson.Set(result, "tool_choice", "auto")
		case "any":
			result, _ = sjson.Set(result, "tool_choice", "required")
		case "tool":
			if name := tc.Get("name"); name.Exists() {
				result, _ = sjson.SetRaw(result, "tool_choice", `{"type":"function","function":{"name":"`+name.String()+`"}}`)
			}
		}
	}

	// Reasoning effort from thinking config
	if effort := convertThinkingWithPackage(inputRawJSON); effort != "" {
		result, _ = sjson.Set(result, "reasoning_effort", effort)
	}

	return []byte(result)
}

func convertAssistantMessage(content gjson.Result, messagesJSON string) string {
	var textParts []string
	var reasoningParts []string
	toolCallsJSON := "[]"
	hasToolCalls := false
	contentPartsJSON := "[]"
	hasContentParts := false

	content.ForEach(func(_, block gjson.Result) bool {
		blockType := block.Get("type").String()
		switch blockType {
		case "text":
			text := block.Get("text").String()
			textParts = append(textParts, text)
			part := `{"type":"text","text":""}`
			part, _ = sjson.Set(part, "text", text)
			contentPartsJSON, _ = sjson.SetRaw(contentPartsJSON, "-1", part)
			hasContentParts = true
		case "thinking":
			text := block.Get("thinking").String()
			if strings.TrimSpace(text) != "" {
				reasoningParts = append(reasoningParts, text)
			}
		case "tool_use":
			id := block.Get("id").String()
			name := block.Get("name").String()
			input := block.Get("input")
			tc := `{"id":"","type":"function","function":{"name":"","arguments":""}}`
			tc, _ = sjson.Set(tc, "id", id)
			tc, _ = sjson.Set(tc, "function.name", name)
			if input.Exists() {
				tc, _ = sjson.Set(tc, "function.arguments", input.Raw)
			} else {
				tc, _ = sjson.Set(tc, "function.arguments", "{}")
			}
			toolCallsJSON, _ = sjson.SetRaw(toolCallsJSON, "-1", tc)
			hasToolCalls = true
		}
		return true
	})

	msg := `{"role":"assistant"}`

	if hasContentParts {
		msg, _ = sjson.SetRaw(msg, "content", contentPartsJSON)
	} else if len(textParts) > 0 {
		msg, _ = sjson.Set(msg, "content", strings.Join(textParts, ""))
	} else {
		msg, _ = sjson.Set(msg, "content", "")
	}

	if hasToolCalls {
		msg, _ = sjson.SetRaw(msg, "tool_calls", toolCallsJSON)
	}

	if len(reasoningParts) > 0 {
		msg, _ = sjson.Set(msg, "reasoning_content", strings.Join(reasoningParts, "\n\n"))
	}

	messagesJSON, _ = sjson.SetRaw(messagesJSON, "-1", msg)
	return messagesJSON
}

func convertUserMessage(role string, content gjson.Result, messagesJSON string) string {
	var toolResults []string
	contentPartsJSON := "[]"
	hasContentParts := false

	content.ForEach(func(_, block gjson.Result) bool {
		blockType := block.Get("type").String()
		switch blockType {
		case "text":
			text := block.Get("text").String()
			if strings.TrimSpace(text) != "" {
				part := `{"type":"text","text":""}`
				part, _ = sjson.Set(part, "text", text)
				contentPartsJSON, _ = sjson.SetRaw(contentPartsJSON, "-1", part)
				hasContentParts = true
			}
		case "image":
			if partJSON, ok := convertClaudeContentPart(block); ok {
				contentPartsJSON, _ = sjson.SetRaw(contentPartsJSON, "-1", partJSON)
				hasContentParts = true
			}
		case "tool_result":
			toolUseID := block.Get("tool_use_id").String()
			toolContent := block.Get("content")
			contentStr, isArray := convertClaudeToolResultContent(toolContent)

			toolMsg := `{"role":"tool","tool_call_id":"","content":""}`
			toolMsg, _ = sjson.Set(toolMsg, "tool_call_id", toolUseID)
			if isArray {
				toolMsg, _ = sjson.SetRaw(toolMsg, "content", contentStr)
			} else {
				toolMsg, _ = sjson.Set(toolMsg, "content", contentStr)
			}
			toolResults = append(toolResults, toolMsg)
		}
		return true
	})

	// Tool results must come before user content
	for _, tr := range toolResults {
		messagesJSON, _ = sjson.SetRaw(messagesJSON, "-1", tr)
	}

	if hasContentParts {
		msg := `{"role":""}`
		msg, _ = sjson.Set(msg, "role", role)
		msg, _ = sjson.SetRaw(msg, "content", contentPartsJSON)
		messagesJSON, _ = sjson.SetRaw(messagesJSON, "-1", msg)
	}

	return messagesJSON
}

func convertThinkingWithPackage(rawJSON []byte) string {
	root := gjson.ParseBytes(rawJSON)
	thinkingConfig := root.Get("thinking")
	if !thinkingConfig.Exists() || !thinkingConfig.IsObject() {
		return ""
	}

	thinkingType := thinkingConfig.Get("type").String()
	switch thinkingType {
	case "enabled":
		if budgetTokens := thinkingConfig.Get("budget_tokens"); budgetTokens.Exists() {
			budget := int(budgetTokens.Int())
			if effort, ok := thinking.ConvertBudgetToLevel(budget); ok && effort != "" {
				return effort
			}
		} else {
			if effort, ok := thinking.ConvertBudgetToLevel(-1); ok && effort != "" {
				return effort
			}
		}
	case "adaptive", "auto":
		effort := ""
		if v := root.Get("output_config.effort"); v.Exists() && v.Type == gjson.String {
			effort = strings.ToLower(strings.TrimSpace(v.String()))
		}
		if effort != "" {
			return effort
		}
		return string(thinking.LevelXHigh)
	case "disabled":
		if effort, ok := thinking.ConvertBudgetToLevel(0); ok && effort != "" {
			return effort
		}
	}
	return ""
}

func convertClaudeContentPart(part gjson.Result) (string, bool) {
	partType := part.Get("type").String()

	switch partType {
	case "text":
		text := part.Get("text").String()
		if strings.TrimSpace(text) == "" {
			return "", false
		}
		textContent := `{"type":"text","text":""}`
		textContent, _ = sjson.Set(textContent, "text", text)
		return textContent, true

	case "image":
		var imageURL string

		if source := part.Get("source"); source.Exists() {
			sourceType := source.Get("type").String()
			switch sourceType {
			case "base64":
				mediaType := source.Get("media_type").String()
				if mediaType == "" {
					mediaType = "application/octet-stream"
				}
				data := source.Get("data").String()
				if data != "" {
					imageURL = "data:" + mediaType + ";base64," + data
				}
			case "url":
				imageURL = source.Get("url").String()
			}
		}

		if imageURL == "" {
			imageURL = part.Get("url").String()
		}

		if imageURL == "" {
			return "", false
		}

		imageContent := `{"type":"image_url","image_url":{"url":""}}`
		imageContent, _ = sjson.Set(imageContent, "image_url.url", imageURL)

		return imageContent, true

	default:
		return "", false
	}
}

func convertClaudeToolResultContent(content gjson.Result) (string, bool) {
	if !content.Exists() {
		return "", false
	}

	if content.Type == gjson.String {
		return content.String(), false
	}

	if content.IsArray() {
		var parts []string
		contentJSON := "[]"
		hasImagePart := false
		content.ForEach(func(_, item gjson.Result) bool {
			switch {
			case item.Type == gjson.String:
				text := item.String()
				parts = append(parts, text)
				textContent := `{"type":"text","text":""}`
				textContent, _ = sjson.Set(textContent, "text", text)
				contentJSON, _ = sjson.SetRaw(contentJSON, "-1", textContent)
			case item.IsObject() && item.Get("type").String() == "text":
				text := item.Get("text").String()
				parts = append(parts, text)
				textContent := `{"type":"text","text":""}`
				textContent, _ = sjson.Set(textContent, "text", text)
				contentJSON, _ = sjson.SetRaw(contentJSON, "-1", textContent)
			case item.IsObject() && item.Get("type").String() == "image":
				contentItem, ok := convertClaudeContentPart(item)
				if ok {
					contentJSON, _ = sjson.SetRaw(contentJSON, "-1", contentItem)
					hasImagePart = true
				} else {
					parts = append(parts, item.Raw)
				}
			case item.IsObject() && item.Get("text").Exists() && item.Get("text").Type == gjson.String:
				parts = append(parts, item.Get("text").String())
			default:
				parts = append(parts, item.Raw)
			}
			return true
		})

		if hasImagePart {
			return contentJSON, true
		}

		joined := strings.Join(parts, "\n\n")
		if strings.TrimSpace(joined) != "" {
			return joined, false
		}
		return content.Raw, false
	}

	if content.IsObject() {
		if content.Get("type").String() == "image" {
			contentItem, ok := convertClaudeContentPart(content)
			if ok {
				contentJSON := "[]"
				contentJSON, _ = sjson.SetRaw(contentJSON, "-1", contentItem)
				return contentJSON, true
			}
		}
		if text := content.Get("text"); text.Exists() && text.Type == gjson.String {
			return text.String(), false
		}
		return content.Raw, false
	}

	return content.Raw, false
}

