package protocol

import "encoding/json"

// OpenAI Chat Completions API types.

type OpenAIChatRequest struct {
	Model            string              `json:"model"`
	Messages         []OpenAIMessage     `json:"messages"`
	Tools            []OpenAITool        `json:"tools,omitempty"`
	ToolChoice       any                 `json:"tool_choice,omitempty"`
	Temperature      *float64            `json:"temperature,omitempty"`
	TopP             *float64            `json:"top_p,omitempty"`
	TopK             *int64              `json:"top_k,omitempty"`
	MaxTokens        *int64              `json:"max_tokens,omitempty"`
	Stop             any                 `json:"stop,omitempty"`
	Stream           *bool               `json:"stream,omitempty"`
	N                *int64              `json:"n,omitempty"`
	User             string              `json:"user,omitempty"`
	ReasoningEffort  string              `json:"reasoning_effort,omitempty"`
	StreamOptions    *OpenAIStreamOpts   `json:"stream_options,omitempty"`
	Extra            map[string]any      `json:"-"`
}

type OpenAIStreamOpts struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

type OpenAIMessage struct {
	Role             string             `json:"role"`
	Content          any                `json:"content"`
	Name             string             `json:"name,omitempty"`
	ToolCalls        []OpenAIToolCall   `json:"tool_calls,omitempty"`
	ToolCallID       string             `json:"tool_call_id,omitempty"`
	ReasoningContent any                `json:"reasoning_content,omitempty"`
}

type OpenAIContentPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *OpenAIImageURL `json:"image_url,omitempty"`
}

type OpenAIImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type OpenAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Index    *int               `json:"index,omitempty"`
	Function OpenAIFunctionCall `json:"function"`
}

type OpenAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type OpenAITool struct {
	Type     string          `json:"type"`
	Function OpenAIFunction  `json:"function"`
}

type OpenAIFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type OpenAIToolChoiceFunction struct {
	Type     string                       `json:"type"`
	Function OpenAIToolChoiceFunctionName `json:"function"`
}

type OpenAIToolChoiceFunctionName struct {
	Name string `json:"name"`
}

// Response types.

type OpenAIChatResponse struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Created int64             `json:"created"`
	Model   string            `json:"model"`
	Choices []OpenAIChoice    `json:"choices"`
	Usage   *OpenAIUsage      `json:"usage,omitempty"`
}

type OpenAIChoice struct {
	Index        int            `json:"index"`
	Message      *OpenAIMessage `json:"message,omitempty"`
	Delta        *OpenAIMessage `json:"delta,omitempty"`
	FinishReason *string        `json:"finish_reason"`
}

type OpenAIUsage struct {
	PromptTokens            int64                    `json:"prompt_tokens"`
	CompletionTokens        int64                    `json:"completion_tokens"`
	TotalTokens             int64                    `json:"total_tokens"`
	PromptTokensDetails     *OpenAIPromptTokenDetail `json:"prompt_tokens_details,omitempty"`
	CompletionTokensDetails *OpenAICompletionDetail  `json:"completion_tokens_details,omitempty"`
}

type OpenAIPromptTokenDetail struct {
	CachedTokens int64 `json:"cached_tokens,omitempty"`
}

type OpenAICompletionDetail struct {
	ReasoningTokens int64 `json:"reasoning_tokens,omitempty"`
}

// Helpers for OpenAI message content.

func (m *OpenAIMessage) ContentString() string {
	switch v := m.Content.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return ""
	}
}

func (m *OpenAIMessage) ContentParts() []OpenAIContentPart {
	arr, ok := m.Content.([]any)
	if !ok {
		return nil
	}
	var parts []OpenAIContentPart
	for _, item := range arr {
		data, err := json.Marshal(item)
		if err != nil {
			continue
		}
		var part OpenAIContentPart
		if err := json.Unmarshal(data, &part); err != nil {
			continue
		}
		parts = append(parts, part)
	}
	return parts
}

func (m *OpenAIMessage) ReasoningContentString() string {
	switch v := m.ReasoningContent.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return ""
	}
}
