package handler

// antigravity_types.go defines strongly-typed Go structs for the Antigravity
// (Google Cloud Code) protocol conversion between Claude (Anthropic Messages API)
// and Gemini (V1Internal API). These replace the previous map[string]any approach
// for better type safety, maintainability, and alignment with sub2api patterns.

// ──────────────────────────────────────────────────────────────
// Claude (Anthropic Messages API) types
// ──────────────────────────────────────────────────────────────

// ClaudeRequest represents an incoming Anthropic Messages API request.
type ClaudeRequest struct {
	Model         string          `json:"model"`
	Messages      []ClaudeMessage `json:"messages"`
	System        any             `json:"system,omitempty"` // string or []ClaudeContentItem
	MaxTokens     int             `json:"max_tokens,omitempty"`
	Temperature   *float64        `json:"temperature,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
	TopK          *int            `json:"top_k,omitempty"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	Stream        bool            `json:"stream,omitempty"`
	Tools         []ClaudeTool    `json:"tools,omitempty"`
	ToolChoice    any             `json:"tool_choice,omitempty"` // string or map
	Thinking      *ClaudeThinking `json:"thinking,omitempty"`
	Metadata      map[string]any  `json:"metadata,omitempty"`
}

// ClaudeMessage represents a single message in the conversation.
type ClaudeMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []ClaudeContentItem
}

// ClaudeContentItem represents a typed content block in a message.
type ClaudeContentItem struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	Thinking  string         `json:"thinking,omitempty"`
	Signature string         `json:"signature,omitempty"`
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Input     map[string]any `json:"input,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
	Content   any            `json:"content,omitempty"` // tool_result content
	IsError   *bool          `json:"is_error,omitempty"`
	Source    *ImageSource   `json:"source,omitempty"` // image block
}

// ImageSource represents the source of an image content block.
type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// ClaudeTool represents a tool definition.
type ClaudeTool struct {
	Type           string          `json:"type,omitempty"` // "custom" for MCP tools
	Name           string          `json:"name"`
	Description    string          `json:"description,omitempty"`
	InputSchema    any             `json:"input_schema,omitempty"`
	CustomToolSpec *CustomToolSpec `json:"custom_tool_spec,omitempty"`
}

// CustomToolSpec represents a nested tool spec for MCP custom tools.
type CustomToolSpec struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"input_schema,omitempty"`
}

// ClaudeThinking represents the thinking configuration.
type ClaudeThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

// ClaudeResponse represents the Anthropic Messages API response.
type ClaudeResponse struct {
	ID           string              `json:"id"`
	Type         string              `json:"type"`
	Role         string              `json:"role"`
	Model        string              `json:"model"`
	Content      []ClaudeContentItem `json:"content"`
	StopReason   string              `json:"stop_reason"`
	StopSequence *string             `json:"stop_sequence"`
	Usage        ClaudeUsage         `json:"usage"`
}

// ClaudeUsage represents token usage in a Claude response.
type ClaudeUsage struct {
	InputTokens          int `json:"input_tokens"`
	OutputTokens         int `json:"output_tokens"`
	CacheReadInputTokens int `json:"cache_read_input_tokens,omitempty"`
}

// ──────────────────────────────────────────────────────────────
// Gemini (V1Internal API) types
// ──────────────────────────────────────────────────────────────

// V1InternalRequest is the Antigravity envelope that wraps the Gemini request body.
type V1InternalRequest struct {
	Project     string        `json:"project"`
	RequestID   string        `json:"requestId"`
	Model       string        `json:"model"`
	UserAgent   string        `json:"userAgent"`
	Request     GeminiRequest `json:"request"`
	SessionID   string        `json:"sessionId"`
	RequestType string        `json:"requestType,omitempty"` // "agent" or "web_search"
}

// GeminiRequest represents the core Gemini generateContent request body.
type GeminiRequest struct {
	Contents          []GeminiContent         `json:"contents"`
	SystemInstruction *GeminiContent          `json:"systemInstruction,omitempty"`
	GenerationConfig  *GeminiGenerationConfig `json:"generationConfig,omitempty"`
	Tools             []GeminiToolDeclaration `json:"tools,omitempty"`
	ToolConfig        *GeminiToolConfig       `json:"toolConfig,omitempty"`
	SafetySettings    []GeminiSafetySetting   `json:"safetySettings,omitempty"`
}

// GeminiContent represents a content block with role and parts.
type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

// GeminiPart represents a single part within a content block.
type GeminiPart struct {
	Text             string              `json:"text,omitempty"`
	Thought          *bool               `json:"thought,omitempty"`
	ThoughtSignature string              `json:"thoughtSignature,omitempty"`
	FunctionCall     *GeminiFunctionCall `json:"functionCall,omitempty"`
	FunctionResponse *GeminiFuncResponse `json:"functionResponse,omitempty"`
	InlineData       *GeminiInlineData   `json:"inlineData,omitempty"`
}

// GeminiFunctionCall represents a function call in the response.
type GeminiFunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

// GeminiFuncResponse represents the result of a function call.
type GeminiFuncResponse struct {
	ID       string         `json:"id,omitempty"`
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

// GeminiInlineData represents inline binary data (images).
type GeminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

// GeminiGenerationConfig represents generation parameters.
type GeminiGenerationConfig struct {
	Temperature     *float64              `json:"temperature,omitempty"`
	TopP            *float64              `json:"topP,omitempty"`
	TopK            *int                  `json:"topK,omitempty"`
	MaxOutputTokens int                   `json:"maxOutputTokens,omitempty"`
	StopSequences   []string              `json:"stopSequences,omitempty"`
	ThinkingConfig  *GeminiThinkingConfig `json:"thinkingConfig,omitempty"`
}

// GeminiThinkingConfig represents the thinking configuration for Gemini.
type GeminiThinkingConfig struct {
	ThinkingBudget  int  `json:"thinkingBudget,omitempty"`
	IncludeThoughts bool `json:"includeThoughts"`
}

// GeminiToolDeclaration represents a tool declaration.
type GeminiToolDeclaration struct {
	FunctionDeclarations []GeminiFunctionDecl `json:"functionDeclarations,omitempty"`
	GoogleSearch         *GeminiGoogleSearch  `json:"googleSearch,omitempty"`
}

// GeminiFunctionDecl represents a single function declaration.
type GeminiFunctionDecl struct {
	Name                 string `json:"name"`
	Description          string `json:"description,omitempty"`
	ParametersJSONSchema any    `json:"parametersJsonSchema,omitempty"`
}

// GeminiGoogleSearch represents the Google Search tool config.
type GeminiGoogleSearch struct {
	EnhancedContent *GeminiEnhancedContent `json:"enhancedContent,omitempty"`
}

// GeminiEnhancedContent represents enhanced content options for Google Search.
type GeminiEnhancedContent struct {
	ImageSearch bool `json:"imageSearch,omitempty"`
}

// GeminiToolConfig represents tool calling configuration.
type GeminiToolConfig struct {
	FunctionCallingConfig GeminiFunctionCallingConfig `json:"functionCallingConfig"`
}

// GeminiFunctionCallingConfig represents function calling mode configuration.
type GeminiFunctionCallingConfig struct {
	Mode                 string   `json:"mode"`
	AllowedFunctionNames []string `json:"allowedFunctionNames,omitempty"`
}

// GeminiSafetySetting represents a safety setting for harm categories.
type GeminiSafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

// ──────────────────────────────────────────────────────────────
// Gemini Response types
// ──────────────────────────────────────────────────────────────

// V1InternalResponse is the Antigravity envelope for Gemini responses.
type V1InternalResponse struct {
	Response *GeminiResponse `json:"response,omitempty"`
	// Fallback: sometimes the response is a flat GeminiResponse without wrapper.
}

// GeminiResponse represents a Gemini generateContent response.
type GeminiResponse struct {
	Candidates    []GeminiCandidate    `json:"candidates,omitempty"`
	UsageMetadata *GeminiUsageMetadata `json:"usageMetadata,omitempty"`
}

// GeminiCandidate represents a single candidate in the response.
type GeminiCandidate struct {
	Content           *GeminiContent           `json:"content,omitempty"`
	FinishReason      string                   `json:"finishReason,omitempty"`
	GroundingMetadata *GeminiGroundingMetadata `json:"groundingMetadata,omitempty"`
}

// GeminiGroundingMetadata represents grounding metadata from web search.
type GeminiGroundingMetadata struct {
	WebSearchQueries []string               `json:"webSearchQueries,omitempty"`
	GroundingChunks  []GeminiGroundingChunk `json:"groundingChunks,omitempty"`
}

// GeminiGroundingChunk represents a single grounding source.
type GeminiGroundingChunk struct {
	Web *GeminiGroundingWeb `json:"web,omitempty"`
}

// GeminiGroundingWeb represents a web source for grounding.
type GeminiGroundingWeb struct {
	URI   string `json:"uri,omitempty"`
	Title string `json:"title,omitempty"`
}

// GeminiUsageMetadata represents token usage metadata.
type GeminiUsageMetadata struct {
	PromptTokenCount        int `json:"promptTokenCount,omitempty"`
	CandidatesTokenCount    int `json:"candidatesTokenCount,omitempty"`
	TotalTokenCount         int `json:"totalTokenCount,omitempty"`
	CachedContentTokenCount int `json:"cachedContentTokenCount,omitempty"`
	ThoughtsTokenCount      int `json:"thoughtsTokenCount,omitempty"`
}

// ──────────────────────────────────────────────────────────────
// Constants and Defaults
// ──────────────────────────────────────────────────────────────

const (
	// DummyThoughtSignature is used for Gemini models which don't require real signatures.
	DummyThoughtSignature = "skip_thought_signature_validator"

	// Default generation limits.
	DefaultMaxOutputTokens       = 64000
	GeminiMaxOutputTokens        = 65000
	DefaultThinkingBudgetOpus46  = 24576
	GeminiFlashThinkingBudgetCap = 24576

	// WebSearchFallbackModel is the model used when web_search tool is detected.
	WebSearchFallbackModel = "gemini-2.5-flash"
)

// DefaultSafetySettings disables all safety filters for the Antigravity API.
var DefaultSafetySettings = []GeminiSafetySetting{
	{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: "OFF"},
	{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: "OFF"},
	{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: "OFF"},
	{Category: "HARM_CATEGORY_HARASSMENT", Threshold: "OFF"},
	{Category: "HARM_CATEGORY_CIVIC_INTEGRITY", Threshold: "OFF"},
}

// DefaultStopSequences are injected into generation config.
// Note: "[DONE]" is deliberately NOT included (see sub2api PR #949).
var DefaultStopSequences = []string{
	"\n```\nresult:",
}

// ──────────────────────────────────────────────────────────────
// Identity patch (system prompt injection)
// ──────────────────────────────────────────────────────────────

const (
	identityBoundaryStart = "<!-- IDENTITY_PATCH_START -->"
	identityBoundaryEnd   = "<!-- IDENTITY_PATCH_END -->"
	identityPatchText     = `You are Antigravity, an AI coding assistant made by Google. You are built on top of Gemini, Google's next-generation large language model.`
)

// mcpXMLProtocol is injected into system prompt when tools with "mcp__" prefix are detected.
const mcpXMLProtocol = `

When you need to call an MCP (Model Context Protocol) tool, format your call using XML tags like this:

<tool_call>
<tool_name>mcp__server_name__tool_name</tool_name>
<parameters>
{"param1": "value1", "param2": "value2"}
</parameters>
</tool_call>

The tool call MUST be wrapped in the exact XML format above. Do not deviate from this format.`

// openCodeDefaultPromptPrefixes are filtered from system prompts to remove OpenCode defaults.
var openCodeDefaultPromptPrefixes = []string{
	"You are an interactive CLI agent",
	"You are a powerful agentic AI coding assistant",
}
