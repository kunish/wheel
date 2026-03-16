package protocol

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/genai"
)

// GeminiRequestToAnthropic converts a Gemini request (using genai types) to
// an Anthropic Messages API request as raw JSON.
func GeminiRequestToAnthropic(req *GeminiRequest, modelName string, stream bool) ([]byte, error) {
	out := map[string]any{
		"model":      modelName,
		"max_tokens": 32000,
		"messages":   []any{},
		"stream":     stream,
		"metadata":   map[string]string{"user_id": generateAnthropicUserID()},
	}

	// Generation config
	if gc := req.GenerationConfig; gc != nil {
		if gc.MaxOutputTokens > 0 {
			out["max_tokens"] = int64(gc.MaxOutputTokens)
		}
		if gc.Temperature != nil {
			out["temperature"] = float64(*gc.Temperature)
		} else if gc.TopP != nil {
			out["top_p"] = float64(*gc.TopP)
		}
		if len(gc.StopSequences) > 0 {
			out["stop_sequences"] = gc.StopSequences
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
			messages := out["messages"].([]any)
			messages = append(messages, map[string]any{
				"role":    "user",
				"content": []any{map[string]string{"type": "text", "text": textBuilder.String()}},
			})
			out["messages"] = messages
		}
	}

	// FIFO queue for tool call IDs
	var pendingToolIDs []string

	// Contents → Messages
	for _, content := range req.Contents {
		role := content.Role
		if role == "model" {
			role = "assistant"
		}
		if role == "function" || role == "tool" {
			role = "user"
		}

		var blocks []any
		for _, part := range content.Parts {
			if part.Text != "" {
				blocks = append(blocks, map[string]string{"type": "text", "text": part.Text})
			}
			if part.FunctionCall != nil && role == "assistant" {
				toolID := GenAnthropicToolUseID()
				pendingToolIDs = append(pendingToolIDs, toolID)
				blocks = append(blocks, map[string]any{
					"type":  "tool_use",
					"id":    toolID,
					"name":  part.FunctionCall.Name,
					"input": part.FunctionCall.Args,
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
				blocks = append(blocks, map[string]any{
					"type":        "tool_result",
					"tool_use_id": toolID,
					"content":     resultContent,
				})
			}
			if part.InlineData != nil {
				blocks = append(blocks, map[string]any{
					"type": "image",
					"source": map[string]any{
						"type":       "base64",
						"media_type": part.InlineData.MIMEType,
						"data":       string(part.InlineData.Data),
					},
				})
			}
		}

		if len(blocks) > 0 {
			messages := out["messages"].([]any)
			messages = append(messages, map[string]any{"role": role, "content": blocks})
			out["messages"] = messages
		}
	}

	// Tools
	if len(req.Tools) > 0 {
		var tools []any
		for _, tool := range req.Tools {
			for _, fd := range tool.FunctionDeclarations {
				toolDef := map[string]any{
					"name":        fd.Name,
					"description": fd.Description,
				}
				if fd.Parameters != nil {
					toolDef["input_schema"] = fd.Parameters
				} else if fd.ParametersJsonSchema != nil {
					toolDef["input_schema"] = fd.ParametersJsonSchema
				}
				tools = append(tools, toolDef)
			}
		}
		if len(tools) > 0 {
			out["tools"] = tools
		}
	}

	// Tool config
	if tc := req.ToolConfig; tc != nil && tc.FunctionCallingConfig != nil {
		switch tc.FunctionCallingConfig.Mode {
		case genai.FunctionCallingConfigModeAuto:
			out["tool_choice"] = map[string]string{"type": "auto"}
		case genai.FunctionCallingConfigModeNone:
			out["tool_choice"] = map[string]string{"type": "none"}
		case genai.FunctionCallingConfigModeAny:
			out["tool_choice"] = map[string]string{"type": "any"}
		}
	}

	return json.Marshal(out)
}

func generateAnthropicUserID() string {
	u, _ := uuid.NewRandom()
	return "user_" + u.String()
}
