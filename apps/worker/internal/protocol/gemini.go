package protocol

import (
	"google.golang.org/genai"
)

// GeminiRequest is the raw Gemini API request body. The genai SDK doesn't export
// this type since it uses its own client to build requests. We define it here using
// the SDK types for the body fields.
type GeminiRequest struct {
	Model             string                       `json:"model,omitempty"`
	Contents          []*genai.Content             `json:"contents"`
	SystemInstruction *genai.Content               `json:"system_instruction,omitempty"`
	GenerationConfig  *genai.GenerateContentConfig `json:"generationConfig,omitempty"`
	Tools             []*genai.Tool                `json:"tools,omitempty"`
	ToolConfig        *genai.ToolConfig            `json:"toolConfig,omitempty"`
	SafetySettings    []*genai.SafetySetting       `json:"safetySettings,omitempty"`
}
