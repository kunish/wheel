package protocol

import (
	"github.com/anthropics/anthropic-sdk-go"
)

// Re-export Anthropic SDK types for use in conversion functions.
type (
	AnthropicMessage          = anthropic.Message
	AnthropicContentBlock     = anthropic.ContentBlockUnion
	AnthropicUsage            = anthropic.Usage
	AnthropicMessageStopEvent = anthropic.MessageStopEvent
)
