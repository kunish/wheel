// Package protocol provides typed API conversion between OpenAI, Anthropic, and Gemini
// using official SDK types from github.com/openai/openai-go, github.com/anthropics/anthropic-sdk-go,
// and google.golang.org/genai.
package protocol

import (
	"github.com/openai/openai-go"
)

// Re-export OpenAI SDK response types for use in conversion functions.
type (
	OpenAIChatCompletion      = openai.ChatCompletion
	OpenAIChatCompletionChunk = openai.ChatCompletionChunk
	OpenAICompletionUsage     = openai.CompletionUsage
)
