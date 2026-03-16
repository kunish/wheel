package protocol

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"google.golang.org/genai"
)

// DefaultGeminiSafetySettings returns the default safety settings for Gemini requests.
func DefaultGeminiSafetySettings() []*genai.SafetySetting {
	categories := []genai.HarmCategory{
		genai.HarmCategoryDangerousContent,
		genai.HarmCategoryHarassment,
		genai.HarmCategoryHateSpeech,
		genai.HarmCategorySexuallyExplicit,
		genai.HarmCategoryCivicIntegrity,
	}
	settings := make([]*genai.SafetySetting, len(categories))
	for i, cat := range categories {
		settings[i] = &genai.SafetySetting{
			Category:  cat,
			Threshold: genai.HarmBlockThresholdBlockNone,
		}
	}
	return settings
}

// AnthropicRequestToGemini converts an Anthropic Messages API request (raw JSON)
// to a Gemini request body using genai SDK types.
func AnthropicRequestToGemini(rawJSON []byte, modelName string) (*GeminiRequest, error) {
	// Strip "url" format from JSON schemas
	rawJSON = bytes.Replace(rawJSON, []byte(`"url":{"type":"string","format":"uri",`), []byte(`"url":{"type":"string",`), -1)

	var req anthropic.MessageNewParams
	if err := json.Unmarshal(rawJSON, &req); err != nil {
		return nil, err
	}

	out := &GeminiRequest{
		Model:          modelName,
		SafetySettings: DefaultGeminiSafetySettings(),
	}

	// System instruction
	var sysParts []*genai.Part
	for _, sys := range req.System {
		if sys.Text != "" {
			sysParts = append(sysParts, genai.NewPartFromText(sys.Text))
		}
	}
	if len(sysParts) > 0 {
		out.SystemInstruction = &genai.Content{Role: "user", Parts: sysParts}
	}

	// Messages → Contents
	for _, msg := range req.Messages {
		content := convertAnthropicMsgToGeminiContent(msg)
		if content != nil {
			out.Contents = append(out.Contents, content)
		}
	}

	// Tools
	if len(req.Tools) > 0 {
		var funcDecls []*genai.FunctionDeclaration
		for _, toolUnion := range req.Tools {
			if toolUnion.OfTool == nil {
				continue
			}
			tool := toolUnion.OfTool
			fd := &genai.FunctionDeclaration{
				Name:        tool.Name,
				Description: tool.Description.Value,
			}
			if !param.IsOmitted(tool.InputSchema) {
				fd.ParametersJsonSchema = tool.InputSchema.Properties
			}
			funcDecls = append(funcDecls, fd)
		}
		if len(funcDecls) > 0 {
			out.Tools = []*genai.Tool{{FunctionDeclarations: funcDecls}}
		}
	}

	// Tool choice
	if !param.IsOmitted(req.ToolChoice) {
		tc := req.ToolChoice
		config := &genai.FunctionCallingConfig{}
		if tc.OfAuto != nil {
			config.Mode = genai.FunctionCallingConfigModeAuto
		} else if tc.OfNone != nil {
			config.Mode = genai.FunctionCallingConfigModeNone
		} else if tc.OfAny != nil {
			config.Mode = genai.FunctionCallingConfigModeAny
		} else if tc.OfTool != nil {
			config.Mode = genai.FunctionCallingConfigModeAny
			config.AllowedFunctionNames = []string{tc.OfTool.Name}
		}
		out.ToolConfig = &genai.ToolConfig{FunctionCallingConfig: config}
	}

	// Generation config params
	if req.Temperature.Valid() {
		if out.GenerationConfig == nil {
			out.GenerationConfig = &genai.GenerateContentConfig{}
		}
		out.GenerationConfig.Temperature = Ptr(float32(req.Temperature.Value))
	}
	if req.TopP.Valid() {
		if out.GenerationConfig == nil {
			out.GenerationConfig = &genai.GenerateContentConfig{}
		}
		out.GenerationConfig.TopP = Ptr(float32(req.TopP.Value))
	}

	return out, nil
}

func convertAnthropicMsgToGeminiContent(msg anthropic.MessageParam) *genai.Content {
	role := string(msg.Role)
	if role == "assistant" {
		role = "model"
	}

	content := &genai.Content{Role: role}

	for _, block := range msg.Content {
		if block.OfText != nil {
			content.Parts = append(content.Parts, genai.NewPartFromText(block.OfText.Text))
		}
		if block.OfToolUse != nil {
			var args map[string]any
			if block.OfToolUse.Input != nil {
				data, err := json.Marshal(block.OfToolUse.Input)
				if err == nil {
					_ = json.Unmarshal(data, &args)
				}
			}
			funcName := block.OfToolUse.Name
			if block.OfToolUse.ID != "" {
				if derived := toolNameFromClaudeToolUseID(block.OfToolUse.ID); derived != "" {
					funcName = derived
				}
			}
			content.Parts = append(content.Parts, &genai.Part{
				ThoughtSignature: []byte("skip_thought_signature_validator"),
				FunctionCall:     &genai.FunctionCall{Name: funcName, Args: args},
			})
		}
		if block.OfToolResult != nil {
			funcName := ""
			if block.OfToolResult.ToolUseID != "" {
				funcName = toolNameFromClaudeToolUseID(block.OfToolResult.ToolUseID)
				if funcName == "" {
					funcName = block.OfToolResult.ToolUseID
				}
			}
			responseData := ""
			for _, cb := range block.OfToolResult.Content {
				if cb.OfText != nil {
					responseData = cb.OfText.Text
				}
			}
			content.Parts = append(content.Parts, genai.NewPartFromFunctionResponse(funcName, map[string]any{"result": responseData}))
		}
		if block.OfImage != nil {
			src := block.OfImage.Source
			if src.OfBase64 != nil {
				content.Parts = append(content.Parts, &genai.Part{
					InlineData: &genai.Blob{
						MIMEType: string(src.OfBase64.MediaType),
						Data:     []byte(src.OfBase64.Data),
					},
				})
			}
		}
	}

	if len(content.Parts) == 0 {
		return nil
	}
	return content
}

func toolNameFromClaudeToolUseID(toolUseID string) string {
	parts := strings.Split(toolUseID, "-")
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts[0:len(parts)-1], "-")
}
