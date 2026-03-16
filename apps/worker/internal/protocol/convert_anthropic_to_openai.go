package protocol

import (
	"encoding/json"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared"
)

// AnthropicRequestToOpenAI converts an Anthropic Messages API request (raw JSON)
// to an OpenAI Chat Completions request (raw JSON), using SDK param types.
func AnthropicRequestToOpenAI(rawJSON []byte, modelName string, stream bool) ([]byte, error) {
	var req anthropic.MessageNewParams
	if err := json.Unmarshal(rawJSON, &req); err != nil {
		return rawJSON, err
	}

	params := openai.ChatCompletionNewParams{
		Model: modelName,
	}

	if stream {
		params.StreamOptions = openai.ChatCompletionStreamOptionsParam{
			IncludeUsage: openai.Bool(true),
		}
	}

	// Max tokens
	if req.MaxTokens > 0 {
		params.MaxTokens = openai.Int(req.MaxTokens)
	}

	// Temperature / TopP
	if req.Temperature.Valid() {
		params.Temperature = openai.Float(req.Temperature.Value)
	} else if req.TopP.Valid() {
		params.TopP = openai.Float(req.TopP.Value)
	}

	// Stop sequences
	if len(req.StopSequences) > 0 {
		seqs := req.StopSequences
		if len(seqs) == 1 {
			params.Stop = openai.ChatCompletionNewParamsStopUnion{OfString: openai.String(seqs[0])}
		} else {
			arr := make([]string, len(seqs))
			copy(arr, seqs)
			params.Stop = openai.ChatCompletionNewParamsStopUnion{OfStringArray: arr}
		}
	}

	// System messages
	for _, sys := range req.System {
		if sys.Text != "" {
			params.Messages = append(params.Messages, openai.SystemMessage(sys.Text))
		}
	}

	// Messages
	for _, msg := range req.Messages {
		converted := convertAnthropicMessageToOpenAI(msg)
		params.Messages = append(params.Messages, converted...)
	}

	// Tools
	for _, toolUnion := range req.Tools {
		if toolUnion.OfTool == nil {
			continue
		}
		tool := toolUnion.OfTool
		var schemaParam shared.FunctionParameters
		if !param.IsOmitted(tool.InputSchema) {
			data, _ := json.Marshal(tool.InputSchema)
			_ = json.Unmarshal(data, &schemaParam)
		}
		params.Tools = append(params.Tools, openai.ChatCompletionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        tool.Name,
				Description: openai.String(tool.Description.Value),
				Parameters:  schemaParam,
			},
		})
	}

	// Tool choice
	tc := req.ToolChoice
	if tc.OfAuto != nil {
		params.ToolChoice = openai.ChatCompletionToolChoiceOptionUnionParam{OfAuto: openai.String("auto")}
	} else if tc.OfAny != nil {
		params.ToolChoice = openai.ChatCompletionToolChoiceOptionUnionParam{OfAuto: openai.String("required")}
	} else if tc.OfTool != nil {
		params.ToolChoice = openai.ChatCompletionToolChoiceOptionUnionParam{
			OfChatCompletionNamedToolChoice: &openai.ChatCompletionNamedToolChoiceParam{
				Function: openai.ChatCompletionNamedToolChoiceFunctionParam{
					Name: tc.OfTool.Name,
				},
			},
		}
	}

	return json.Marshal(params)
}

func convertAnthropicMessageToOpenAI(msg anthropic.MessageParam) []openai.ChatCompletionMessageParamUnion {
	role := string(msg.Role)
	var result []openai.ChatCompletionMessageParamUnion

	var textParts []string
	var reasoningParts []string
	var toolCalls []openai.ChatCompletionMessageToolCallParam
	var toolResults []openai.ChatCompletionMessageParamUnion

	for _, block := range msg.Content {
		if block.OfText != nil {
			textParts = append(textParts, block.OfText.Text)
		}
		if block.OfThinking != nil {
			if role == "assistant" {
				text := block.OfThinking.Thinking
				if strings.TrimSpace(text) != "" {
					reasoningParts = append(reasoningParts, text)
				}
			}
		}
		if block.OfToolUse != nil && role == "assistant" {
			argsJSON, _ := json.Marshal(block.OfToolUse.Input)
			toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallParam{
				ID: block.OfToolUse.ID,
				Function: openai.ChatCompletionMessageToolCallFunctionParam{
					Name:      block.OfToolUse.Name,
					Arguments: string(argsJSON),
				},
			})
		}
		if block.OfToolResult != nil {
			contentStr := extractToolResultContentSDK(block.OfToolResult.Content)
			toolResults = append(toolResults, openai.ToolMessage(block.OfToolResult.ToolUseID, contentStr))
		}
	}

	// Tool results first (OpenAI requires tool results to immediately follow assistant tool_calls)
	result = append(result, toolResults...)

	if role == "assistant" {
		if len(textParts) > 0 || len(reasoningParts) > 0 || len(toolCalls) > 0 {
			assistantMsg := openai.ChatCompletionAssistantMessageParam{
				Content: openai.ChatCompletionAssistantMessageParamContentUnion{
					OfString: openai.String(strings.Join(textParts, "")),
				},
			}
			if len(toolCalls) > 0 {
				assistantMsg.ToolCalls = toolCalls
			}
			result = append(result, openai.ChatCompletionMessageParamUnion{OfAssistant: &assistantMsg})
		}
	} else if len(textParts) > 0 {
		result = append(result, openai.UserMessage(strings.Join(textParts, "")))
	}

	return result
}

func extractToolResultContentSDK(blocks []anthropic.ToolResultBlockParamContentUnion) string {
	var parts []string
	for _, block := range blocks {
		if block.OfText != nil {
			parts = append(parts, block.OfText.Text)
		}
	}
	return strings.Join(parts, "\n\n")
}

// OpenAIResponseToAnthropic converts an OpenAI Chat Completion response to Anthropic Message format.
func OpenAIResponseToAnthropic(resp *openai.ChatCompletion) *anthropic.Message {
	msg := &anthropic.Message{
		ID:   resp.ID,
		Role: "assistant",
	}

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]

		// Text content
		if choice.Message.Content != "" {
			msg.Content = append(msg.Content, anthropic.ContentBlockUnion{
				Type: "text",
				Text: choice.Message.Content,
			})
		}

		// Tool calls
		for _, tc := range choice.Message.ToolCalls {
			msg.Content = append(msg.Content, anthropic.ContentBlockUnion{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: json.RawMessage(tc.Function.Arguments),
			})
		}

		// Finish reason
		reason := MapOpenAIFinishReasonToAnthropic(string(choice.FinishReason))
		msg.StopReason = anthropic.StopReason(reason)
	}

	inputTokens := resp.Usage.PromptTokens
	outputTokens := resp.Usage.CompletionTokens
	cachedTokens := resp.Usage.PromptTokensDetails.CachedTokens

	if cachedTokens > 0 && inputTokens >= cachedTokens {
		inputTokens -= cachedTokens
	}

	msg.Usage = anthropic.Usage{
		InputTokens:          inputTokens,
		OutputTokens:         outputTokens,
		CacheReadInputTokens: cachedTokens,
	}

	return msg
}

// Streaming accumulator types (kept from our protocol package since SDKs
// don't provide streaming state management).

type OpenAIToAnthropicAccum struct {
	MessageID                 string
	Model                     string
	CreatedAt                 int64
	ToolNameMap               map[string]string
	SawToolCall               bool
	ContentAccumulator        strings.Builder
	ToolCallsAccumulator      map[int]*ToolCallAccum
	TextContentBlockStarted   bool
	ThinkingBlockStarted      bool
	FinishReason              string
	ContentBlocksStopped      bool
	MessageDeltaSent          bool
	MessageStarted            bool
	MessageStopSent           bool
	ToolCallBlockIndexes      map[int]int
	TextContentBlockIndex     int
	ThinkingContentBlockIndex int
	NextContentBlockIndex     int
}

type ToolCallAccum struct {
	ID        string
	Name      string
	Arguments strings.Builder
}

func NewOpenAIToAnthropicAccum() *OpenAIToAnthropicAccum {
	return &OpenAIToAnthropicAccum{
		ToolCallBlockIndexes:      make(map[int]int),
		TextContentBlockIndex:     -1,
		ThinkingContentBlockIndex: -1,
		NextContentBlockIndex:     0,
	}
}

func (a *OpenAIToAnthropicAccum) EffectiveFinishReason() string {
	if a.SawToolCall {
		return "tool_calls"
	}
	return a.FinishReason
}

func (a *OpenAIToAnthropicAccum) toolContentBlockIndex(openAIToolIndex int) int {
	if idx, ok := a.ToolCallBlockIndexes[openAIToolIndex]; ok {
		return idx
	}
	idx := a.NextContentBlockIndex
	a.NextContentBlockIndex++
	a.ToolCallBlockIndexes[openAIToolIndex] = idx
	return idx
}

// Streaming SSE conversion functions - these use raw JSON since SSE events
// are text-based and need precise formatting.
// They are kept from the previous implementation and work with the existing
// corelib translator response functions.
// (The streaming conversion logic remains in the translator layer)

// AnthropicTokenCountResponse generates an Anthropic-format token count response.
func AnthropicTokenCountResponse(count int64) string {
	return SafeJSONString(map[string]int64{"input_tokens": count})
}
