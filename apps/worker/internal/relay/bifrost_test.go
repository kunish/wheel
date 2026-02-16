package relay

import (
	"testing"

	schemas "github.com/maximhq/bifrost/core/schemas"
)

func TestNormalizeInboundToOpenAIRequest_Anthropic(t *testing.T) {
	in := map[string]any{
		"model": "claude-3-5-sonnet",
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "hello"},
				},
			},
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{
						"type":  "tool_use",
						"id":    "call_1",
						"name":  "weather",
						"input": map[string]any{"city": "SF"},
					},
				},
			},
		},
	}

	out, err := normalizeInboundToOpenAIRequest("anthropic-messages", in)
	if err != nil {
		t.Fatalf("normalizeInboundToOpenAIRequest failed: %v", err)
	}

	if out["model"] != "claude-3-5-sonnet" {
		t.Fatalf("model mismatch, got %v", out["model"])
	}
	messages, _ := out["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	assistant, _ := messages[1].(map[string]any)
	toolCalls, _ := assistant["tool_calls"].([]any)
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}
}

func TestNormalizeInboundToOpenAIRequest_Gemini(t *testing.T) {
	in := map[string]any{
		"contents": []any{
			map[string]any{
				"role": "user",
				"parts": []any{
					map[string]any{"text": "hello"},
				},
			},
		},
		"generationConfig": map[string]any{
			"temperature":     0.2,
			"maxOutputTokens": 1024,
		},
	}

	out, err := normalizeInboundToOpenAIRequest("gemini-generate-content", in)
	if err != nil {
		t.Fatalf("normalizeInboundToOpenAIRequest failed: %v", err)
	}

	messages, _ := out["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	msg, _ := messages[0].(map[string]any)
	if msg["role"] != "user" {
		t.Fatalf("expected role user, got %v", msg["role"])
	}
	if msg["content"] != "hello" {
		t.Fatalf("expected content hello, got %v", msg["content"])
	}
	if out["max_tokens"] != 1024 {
		t.Fatalf("expected max_tokens 1024, got %v", out["max_tokens"])
	}
}

func TestConvertToGeminiResponseFromOpenAI(t *testing.T) {
	openAIResp := map[string]any{
		"choices": []any{
			map[string]any{
				"message": map[string]any{
					"role":    "assistant",
					"content": "hello world",
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     10,
			"completion_tokens": 5,
			"total_tokens":      15,
		},
	}

	out := ConvertToGeminiResponseFromOpenAI(openAIResp)
	candidates, _ := out["candidates"].([]any)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	usage, _ := out["usageMetadata"].(map[string]any)
	if usage["totalTokenCount"] != 15 {
		t.Fatalf("expected totalTokenCount 15, got %v", usage["totalTokenCount"])
	}
}

func TestNormalizeResponsesRequestForProvider_PromoteSystemToInstructions(t *testing.T) {
	systemRole := schemas.ResponsesInputMessageRoleSystem
	userRole := schemas.ResponsesInputMessageRoleUser
	msgType := schemas.ResponsesMessageTypeMessage

	systemText := "system policy"
	userText := "hello"
	req := &schemas.BifrostResponsesRequest{
		Input: []schemas.ResponsesMessage{
			{
				Type: &msgType,
				Role: &systemRole,
				Content: &schemas.ResponsesMessageContent{
					ContentStr: &systemText,
				},
			},
			{
				Type: &msgType,
				Role: &userRole,
				Content: &schemas.ResponsesMessageContent{
					ContentStr: &userText,
				},
			},
		},
	}

	out := normalizeResponsesRequestForProvider(req)
	if out == nil || out.Params == nil || out.Params.Instructions == nil {
		t.Fatalf("expected instructions to be set")
	}
	if *out.Params.Instructions != systemText {
		t.Fatalf("unexpected instructions: %q", *out.Params.Instructions)
	}
	if len(out.Input) != 1 {
		t.Fatalf("expected only one input message, got %d", len(out.Input))
	}
	if out.Input[0].Role == nil || *out.Input[0].Role != schemas.ResponsesInputMessageRoleUser {
		t.Fatalf("expected remaining input role=user, got %+v", out.Input[0].Role)
	}
}

func TestNormalizeResponsesRequestForProvider_KeepExistingInstructions(t *testing.T) {
	systemRole := schemas.ResponsesInputMessageRoleSystem
	msgType := schemas.ResponsesMessageTypeMessage
	base := "existing instruction"
	systemText := "system message"

	req := &schemas.BifrostResponsesRequest{
		Params: &schemas.ResponsesParameters{
			Instructions: &base,
		},
		Input: []schemas.ResponsesMessage{
			{
				Type: &msgType,
				Role: &systemRole,
				Content: &schemas.ResponsesMessageContent{
					ContentStr: &systemText,
				},
			},
		},
	}

	out := normalizeResponsesRequestForProvider(req)
	want := "existing instruction\n\nsystem message"
	if out.Params == nil || out.Params.Instructions == nil || *out.Params.Instructions != want {
		t.Fatalf("expected merged instructions %q, got %+v", want, out.Params)
	}
}

func TestResponsesStreamEventToOpenAIChunk_TextAndCompleted(t *testing.T) {
	state := newResponsesStreamBridgeState("gpt-5.3-codex")

	respID := "resp_123"
	model := "gpt-5.3-codex"
	created := 123
	createdEvent := &schemas.BifrostResponsesStreamResponse{
		Type: schemas.ResponsesStreamResponseTypeCreated,
		Response: &schemas.BifrostResponsesResponse{
			ID:        &respID,
			Model:     model,
			CreatedAt: created,
		},
	}
	chunk, proxyErr := responsesStreamEventToOpenAIChunk(createdEvent, state)
	if proxyErr != nil {
		t.Fatalf("unexpected error on created event: %v", proxyErr)
	}
	if chunk != nil {
		t.Fatalf("created event should not emit chunk")
	}

	text := "hello"
	textEvent := &schemas.BifrostResponsesStreamResponse{
		Type:  schemas.ResponsesStreamResponseTypeOutputTextDelta,
		Delta: &text,
	}
	chunk, proxyErr = responsesStreamEventToOpenAIChunk(textEvent, state)
	if proxyErr != nil {
		t.Fatalf("unexpected error on text delta: %v", proxyErr)
	}
	if chunk == nil {
		t.Fatalf("expected chunk for text delta")
	}
	choices, _ := chunk["choices"].([]any)
	if len(choices) != 1 {
		t.Fatalf("expected one choice, got %d", len(choices))
	}
	choice, _ := choices[0].(map[string]any)
	delta, _ := choice["delta"].(map[string]any)
	if delta["content"] != text {
		t.Fatalf("expected content %q, got %v", text, delta["content"])
	}
	if delta["role"] != "assistant" {
		t.Fatalf("expected role assistant, got %v", delta["role"])
	}

	usage := &schemas.ResponsesResponseUsage{
		InputTokens:         12,
		OutputTokens:        4,
		TotalTokens:         16,
		InputTokensDetails:  &schemas.ResponsesResponseInputTokens{CachedTokens: 3},
		OutputTokensDetails: &schemas.ResponsesResponseOutputTokens{CachedTokens: 2},
	}
	completedEvent := &schemas.BifrostResponsesStreamResponse{
		Type: schemas.ResponsesStreamResponseTypeCompleted,
		Response: &schemas.BifrostResponsesResponse{
			ID:        &respID,
			Model:     model,
			CreatedAt: created,
			Usage:     usage,
		},
	}
	chunk, proxyErr = responsesStreamEventToOpenAIChunk(completedEvent, state)
	if proxyErr != nil {
		t.Fatalf("unexpected error on completed event: %v", proxyErr)
	}
	if chunk == nil {
		t.Fatalf("expected finish chunk on completed event")
	}
	choices, _ = chunk["choices"].([]any)
	choice, _ = choices[0].(map[string]any)
	if choice["finish_reason"] != "stop" {
		t.Fatalf("expected finish_reason=stop, got %v", choice["finish_reason"])
	}
	usageMap, _ := chunk["usage"].(map[string]any)
	if toInt(usageMap["prompt_tokens"]) != 12 || toInt(usageMap["completion_tokens"]) != 4 || toInt(usageMap["total_tokens"]) != 16 {
		t.Fatalf("unexpected usage: %v", usageMap)
	}
	if state.CacheReadTokens != 3 || state.CacheCreateTokens != 2 {
		t.Fatalf("unexpected cache token state: read=%d create=%d", state.CacheReadTokens, state.CacheCreateTokens)
	}
}

func TestResponsesStreamEventToOpenAIChunk_ToolCalls(t *testing.T) {
	state := newResponsesStreamBridgeState("gpt-5.3-codex")

	itemType := schemas.ResponsesMessageTypeFunctionCall
	itemID := "call_1"
	outputIndex := 0
	name := "lookup"
	addedEvent := &schemas.BifrostResponsesStreamResponse{
		Type:        schemas.ResponsesStreamResponseTypeOutputItemAdded,
		ItemID:      &itemID,
		OutputIndex: &outputIndex,
		Item: &schemas.ResponsesMessage{
			ID:   &itemID,
			Type: &itemType,
			ResponsesToolMessage: &schemas.ResponsesToolMessage{
				Name: &name,
			},
		},
	}
	chunk, proxyErr := responsesStreamEventToOpenAIChunk(addedEvent, state)
	if proxyErr != nil {
		t.Fatalf("unexpected error on output_item.added: %v", proxyErr)
	}
	if chunk != nil {
		t.Fatalf("output_item.added should not emit chunk")
	}

	args := `{"q":"x"}`
	doneEvent := &schemas.BifrostResponsesStreamResponse{
		Type:        schemas.ResponsesStreamResponseTypeFunctionCallArgumentsDone,
		ItemID:      &itemID,
		OutputIndex: &outputIndex,
		Arguments:   &args,
	}
	chunk, proxyErr = responsesStreamEventToOpenAIChunk(doneEvent, state)
	if proxyErr != nil {
		t.Fatalf("unexpected error on tool args done: %v", proxyErr)
	}
	if chunk == nil {
		t.Fatalf("expected tool call chunk")
	}
	choices, _ := chunk["choices"].([]any)
	choice, _ := choices[0].(map[string]any)
	delta, _ := choice["delta"].(map[string]any)
	toolCalls, _ := delta["tool_calls"].([]any)
	if len(toolCalls) != 1 {
		t.Fatalf("expected one tool_call delta, got %d", len(toolCalls))
	}
	tc, _ := toolCalls[0].(map[string]any)
	if tc["id"] != itemID {
		t.Fatalf("expected tool_call id=%s, got %v", itemID, tc["id"])
	}
	if tc["type"] != "function" {
		t.Fatalf("expected tool_call type=function, got %v", tc["type"])
	}
	function, _ := tc["function"].(map[string]any)
	if function["name"] != name {
		t.Fatalf("expected function.name=%s, got %v", name, function["name"])
	}
	if function["arguments"] != args {
		t.Fatalf("expected function.arguments=%s, got %v", args, function["arguments"])
	}

	completedEvent := &schemas.BifrostResponsesStreamResponse{
		Type: schemas.ResponsesStreamResponseTypeCompleted,
		Response: &schemas.BifrostResponsesResponse{
			Usage: &schemas.ResponsesResponseUsage{
				InputTokens:  1,
				OutputTokens: 1,
			},
		},
	}
	chunk, proxyErr = responsesStreamEventToOpenAIChunk(completedEvent, state)
	if proxyErr != nil {
		t.Fatalf("unexpected error on completed event: %v", proxyErr)
	}
	if chunk == nil {
		t.Fatalf("expected finish chunk")
	}
	choices, _ = chunk["choices"].([]any)
	choice, _ = choices[0].(map[string]any)
	if choice["finish_reason"] != "tool_calls" {
		t.Fatalf("expected finish_reason=tool_calls, got %v", choice["finish_reason"])
	}
}

func TestResponsesStreamEventToOpenAIChunk_OutputItemDoneTextFallback(t *testing.T) {
	state := newResponsesStreamBridgeState("gpt-5.3-codex")

	itemType := schemas.ResponsesMessageTypeMessage
	role := schemas.ResponsesInputMessageRoleAssistant
	text := "fallback text from done"
	event := &schemas.BifrostResponsesStreamResponse{
		Type: schemas.ResponsesStreamResponseTypeOutputItemDone,
		Item: &schemas.ResponsesMessage{
			Type: &itemType,
			Role: &role,
			Content: &schemas.ResponsesMessageContent{
				ContentBlocks: []schemas.ResponsesMessageContentBlock{
					{
						Type: schemas.ResponsesOutputMessageContentTypeText,
						Text: &text,
					},
				},
			},
		},
	}

	chunk, proxyErr := responsesStreamEventToOpenAIChunk(event, state)
	if proxyErr != nil {
		t.Fatalf("unexpected error: %v", proxyErr)
	}
	if chunk == nil {
		t.Fatalf("expected fallback chunk from output_item.done")
	}
	choices, _ := chunk["choices"].([]any)
	choice, _ := choices[0].(map[string]any)
	delta, _ := choice["delta"].(map[string]any)
	if delta["content"] != text {
		t.Fatalf("expected fallback content %q, got %v", text, delta["content"])
	}
}

func TestResponsesStreamEventToOpenAIChunk_OutputTextDoneFallback(t *testing.T) {
	state := newResponsesStreamBridgeState("gpt-5.3-codex")
	text := "final text"
	event := &schemas.BifrostResponsesStreamResponse{
		Type: schemas.ResponsesStreamResponseTypeOutputTextDone,
		Text: &text,
	}
	chunk, proxyErr := responsesStreamEventToOpenAIChunk(event, state)
	if proxyErr != nil {
		t.Fatalf("unexpected error: %v", proxyErr)
	}
	if chunk == nil {
		t.Fatalf("expected fallback chunk from output_text.done")
	}
	choices, _ := chunk["choices"].([]any)
	choice, _ := choices[0].(map[string]any)
	delta, _ := choice["delta"].(map[string]any)
	if delta["content"] != text {
		t.Fatalf("expected fallback content %q, got %v", text, delta["content"])
	}
}

func TestResponsesStreamEventToOpenAIChunk_ToolArgsBufferedUntilName(t *testing.T) {
	state := newResponsesStreamBridgeState("gpt-5.3-codex")
	itemID := "call_buffered"
	outputIndex := 0
	argsPart := `{"x":`

	first := &schemas.BifrostResponsesStreamResponse{
		Type:        schemas.ResponsesStreamResponseTypeFunctionCallArgumentsDelta,
		ItemID:      &itemID,
		OutputIndex: &outputIndex,
		Delta:       &argsPart,
	}
	chunk, proxyErr := responsesStreamEventToOpenAIChunk(first, state)
	if proxyErr != nil {
		t.Fatalf("unexpected error on first tool args: %v", proxyErr)
	}
	if chunk != nil {
		t.Fatalf("expected nil chunk before tool name is known")
	}

	itemType := schemas.ResponsesMessageTypeFunctionCall
	name := "lookup"
	argsDone := `1}`
	second := &schemas.BifrostResponsesStreamResponse{
		Type:        schemas.ResponsesStreamResponseTypeOutputItemDone,
		ItemID:      &itemID,
		OutputIndex: &outputIndex,
		Item: &schemas.ResponsesMessage{
			ID:   &itemID,
			Type: &itemType,
			ResponsesToolMessage: &schemas.ResponsesToolMessage{
				Name:      &name,
				Arguments: &argsDone,
			},
		},
	}
	chunk, proxyErr = responsesStreamEventToOpenAIChunk(second, state)
	if proxyErr != nil {
		t.Fatalf("unexpected error on output_item.done: %v", proxyErr)
	}
	if chunk == nil {
		t.Fatalf("expected chunk once name and final args are available")
	}
	choices, _ := chunk["choices"].([]any)
	choice, _ := choices[0].(map[string]any)
	delta, _ := choice["delta"].(map[string]any)
	toolCalls, _ := delta["tool_calls"].([]any)
	if len(toolCalls) != 1 {
		t.Fatalf("expected one tool call chunk, got %d", len(toolCalls))
	}
	tc, _ := toolCalls[0].(map[string]any)
	fn, _ := tc["function"].(map[string]any)
	if fn["name"] != name {
		t.Fatalf("expected function name %q, got %v", name, fn["name"])
	}
	if fn["arguments"] != `{"x":1}` {
		t.Fatalf("expected buffered arguments to merge, got %v", fn["arguments"])
	}
}
