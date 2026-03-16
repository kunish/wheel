// Package claude provides request translation functionality for Anthropic to OpenAI API.
package claude

import (
	"encoding/json"
	"strings"

	"github.com/kunish/wheel/apps/worker/internal/protocol"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/thinking"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ConvertClaudeRequestToOpenAI parses and transforms an Anthropic API request into OpenAI Chat Completions API format.
func ConvertClaudeRequestToOpenAI(modelName string, inputRawJSON []byte, stream bool) []byte {
	var req protocol.AnthropicRequest
	if err := json.Unmarshal(inputRawJSON, &req); err != nil {
		return inputRawJSON
	}

	out := protocol.AnthropicRequestToOpenAI(&req, modelName, stream)

	// Use the thinking package's ConvertBudgetToLevel for accurate conversion
	// instead of the simplified one in protocol package
	out.ReasoningEffort = convertThinkingWithPackage(inputRawJSON)

	result, err := json.Marshal(out)
	if err != nil {
		return inputRawJSON
	}

	return result
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
