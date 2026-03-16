package protocol

// Gemini API types.

type GeminiRequest struct {
	Model             string              `json:"model,omitempty"`
	Contents          []GeminiContent     `json:"contents"`
	SystemInstruction *GeminiContent      `json:"system_instruction,omitempty"`
	GenerationConfig  *GeminiGenConfig    `json:"generationConfig,omitempty"`
	Tools             []GeminiToolDecl    `json:"tools,omitempty"`
	ToolConfig        *GeminiToolConfig   `json:"toolConfig,omitempty"`
	SafetySettings    []GeminiSafety      `json:"safetySettings,omitempty"`
}

type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts,omitempty"`
}

type GeminiPart struct {
	Text             string              `json:"text,omitempty"`
	Thought          *bool               `json:"thought,omitempty"`
	InlineData       *GeminiInlineData   `json:"inlineData,omitempty"`
	FunctionCall     *GeminiFunctionCall `json:"functionCall,omitempty"`
	FunctionResponse *GeminiFuncResponse `json:"functionResponse,omitempty"`
	ThoughtSignature string              `json:"thoughtSignature,omitempty"`
}

type GeminiInlineData struct {
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"`
}

type GeminiFunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

type GeminiFuncResponse struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response,omitempty"`
}

type GeminiGenConfig struct {
	Temperature    *float64            `json:"temperature,omitempty"`
	TopP           *float64            `json:"topP,omitempty"`
	TopK           *int64              `json:"topK,omitempty"`
	MaxOutputTokens *int64             `json:"maxOutputTokens,omitempty"`
	StopSequences  []string            `json:"stopSequences,omitempty"`
	CandidateCount *int64              `json:"candidateCount,omitempty"`
	ThinkingConfig *GeminiThinkConfig  `json:"thinkingConfig,omitempty"`
}

type GeminiThinkConfig struct {
	ThinkingBudget  *int    `json:"thinkingBudget,omitempty"`
	ThinkingLevel   string  `json:"thinkingLevel,omitempty"`
	IncludeThoughts *bool   `json:"includeThoughts,omitempty"`
}

type GeminiToolDecl struct {
	FunctionDeclarations []GeminiFuncDecl `json:"functionDeclarations,omitempty"`
}

type GeminiFuncDecl struct {
	Name                 string `json:"name"`
	Description          string `json:"description,omitempty"`
	Parameters           any    `json:"parameters,omitempty"`
	ParametersJSONSchema any    `json:"parametersJsonSchema,omitempty"`
}

type GeminiToolConfig struct {
	FunctionCallingConfig *GeminiFuncCallingConfig `json:"functionCallingConfig,omitempty"`
}

type GeminiFuncCallingConfig struct {
	Mode                 string   `json:"mode,omitempty"`
	AllowedFunctionNames []string `json:"allowedFunctionNames,omitempty"`
}

type GeminiSafety struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

// Response types.

type GeminiResponse struct {
	Candidates    []GeminiCandidate    `json:"candidates,omitempty"`
	UsageMetadata *GeminiUsageMetadata `json:"usageMetadata,omitempty"`
	Model         string               `json:"model,omitempty"`
}

type GeminiCandidate struct {
	Content      *GeminiContent `json:"content,omitempty"`
	FinishReason string         `json:"finishReason,omitempty"`
	Index        int            `json:"index,omitempty"`
}

type GeminiUsageMetadata struct {
	PromptTokenCount     int64 `json:"promptTokenCount,omitempty"`
	CandidatesTokenCount int64 `json:"candidatesTokenCount,omitempty"`
	TotalTokenCount      int64 `json:"totalTokenCount,omitempty"`
	ThoughtsTokenCount   int64 `json:"thoughtsTokenCount,omitempty"`
}
