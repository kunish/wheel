package protocol

import "encoding/json"

// Anthropic Messages API types.

type AnthropicRequest struct {
	Model         string              `json:"model"`
	System        any                 `json:"system,omitempty"`
	Messages      []AnthropicMessage  `json:"messages"`
	Tools         []AnthropicTool     `json:"tools,omitempty"`
	ToolChoice    any                 `json:"tool_choice,omitempty"`
	MaxTokens     int64               `json:"max_tokens,omitempty"`
	Temperature   *float64            `json:"temperature,omitempty"`
	TopP          *float64            `json:"top_p,omitempty"`
	TopK          *float64            `json:"top_k,omitempty"`
	StopSequences []string            `json:"stop_sequences,omitempty"`
	Stream        *bool               `json:"stream,omitempty"`
	Thinking      *AnthropicThinking  `json:"thinking,omitempty"`
	OutputConfig  *AnthropicOutConfig `json:"output_config,omitempty"`
	User          string              `json:"user,omitempty"`
	Metadata      *AnthropicMetadata  `json:"metadata,omitempty"`
}

type AnthropicOutConfig struct {
	Effort string `json:"effort,omitempty"`
}

type AnthropicMetadata struct {
	UserID string `json:"user_id,omitempty"`
}

type AnthropicThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

type AnthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// AnthropicContentBlock is a polymorphic content block.
type AnthropicContentBlock struct {
	Type string `json:"type"`

	// text block
	Text string `json:"text,omitempty"`

	// thinking block
	Thinking string `json:"thinking,omitempty"`

	// redacted_thinking block
	Data string `json:"data,omitempty"`

	// tool_use block
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Input any    `json:"input,omitempty"`

	// tool_result block
	ToolUseID        string `json:"tool_use_id,omitempty"`
	ToolResultContent any   `json:"content,omitempty"`
	IsError          bool   `json:"is_error,omitempty"`

	// image block
	Source *AnthropicImageSource `json:"source,omitempty"`

	// cache_control
	CacheControl any `json:"cache_control,omitempty"`
}

type AnthropicImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

type AnthropicTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"input_schema,omitempty"`
}

type AnthropicToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// Response types.

type AnthropicResponse struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Model        string                  `json:"model"`
	Content      []AnthropicContentBlock `json:"content"`
	StopReason   *string                 `json:"stop_reason"`
	StopSequence *string                 `json:"stop_sequence"`
	Usage        AnthropicUsage          `json:"usage"`
}

type AnthropicUsage struct {
	InputTokens          int64 `json:"input_tokens"`
	OutputTokens         int64 `json:"output_tokens"`
	CacheReadInputTokens int64 `json:"cache_read_input_tokens,omitempty"`
}

// SSE event types.

type AnthropicSSEMessageStart struct {
	Type    string            `json:"type"`
	Message AnthropicResponse `json:"message"`
}

type AnthropicSSEContentBlockStart struct {
	Type         string                `json:"type"`
	Index        int                   `json:"index"`
	ContentBlock AnthropicContentBlock `json:"content_block"`
}

type AnthropicSSEContentBlockDelta struct {
	Type  string               `json:"type"`
	Index int                  `json:"index"`
	Delta AnthropicStreamDelta `json:"delta"`
}

type AnthropicStreamDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	Thinking    string `json:"thinking,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
}

type AnthropicSSEContentBlockStop struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
}

type AnthropicSSEMessageDelta struct {
	Type  string                    `json:"type"`
	Delta AnthropicMessageDeltaBody `json:"delta"`
	Usage *AnthropicUsage           `json:"usage,omitempty"`
}

type AnthropicMessageDeltaBody struct {
	StopReason   string  `json:"stop_reason,omitempty"`
	StopSequence *string `json:"stop_sequence"`
}

type AnthropicSSEMessageStop struct {
	Type string `json:"type"`
}

// Helpers for Anthropic message content.

func (m *AnthropicMessage) ContentString() string {
	switch v := m.Content.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return ""
	}
}

func (m *AnthropicMessage) ContentBlocks() []AnthropicContentBlock {
	arr, ok := m.Content.([]any)
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

func (m *AnthropicMessage) SetContentBlocks(blocks []AnthropicContentBlock) {
	m.Content = blocks
}
