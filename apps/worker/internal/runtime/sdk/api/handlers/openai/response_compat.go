package openai

import (
	"context"

	codexconverter "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/codex/openai/chat-completions"
	responsesconverter "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/openai/openai/responses"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const OpenAIResponsesHandlerType = "openai-response"

func ConvertResponsesRequestToChatCompletions(modelName string, rawJSON []byte, stream bool) []byte {
	return responsesconverter.ConvertOpenAIResponsesRequestToOpenAIChatCompletions(modelName, rawJSON, stream)
}

func ConvertChatCompletionsResponseToResponses(ctx context.Context, modelName string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) []string {
	return responsesconverter.ConvertOpenAIChatCompletionsResponseToOpenAIResponses(ctx, modelName, originalRequestRawJSON, requestRawJSON, rawJSON, param)
}

func ConvertChatCompletionsResponseToResponsesNonStream(ctx context.Context, modelName string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) string {
	return responsesconverter.ConvertOpenAIChatCompletionsResponseToOpenAIResponsesNonStream(ctx, modelName, originalRequestRawJSON, requestRawJSON, rawJSON, param)
}

func ConvertChatRequestToResponses(modelName string, rawJSON []byte, stream bool) []byte {
	return codexconverter.ConvertOpenAIRequestToCodex(modelName, rawJSON, stream)
}

func ConvertResponsesObjectToChatCompletion(ctx context.Context, modelName string, originalChatJSON, responsesRequestJSON, responsesPayload []byte) []byte {
	if len(responsesPayload) == 0 {
		return nil
	}
	wrapped := responsesPayload
	if !gjson.GetBytes(wrapped, "type").Exists() && gjson.GetBytes(wrapped, "object").String() == "response" {
		payload := []byte(`{"type":"response.completed","response":{}}`)
		payload, _ = sjson.SetRawBytes(payload, "response", wrapped)
		wrapped = payload
	}
	var param any
	converted := codexconverter.ConvertCodexResponseToOpenAINonStream(ctx, modelName, originalChatJSON, responsesRequestJSON, wrapped, &param)
	if converted == "" {
		return nil
	}
	return []byte(converted)
}

func ConvertResponsesChunkToChatCompletions(ctx context.Context, modelName string, originalChatJSON, responsesRequestJSON, chunk []byte, param *any) []string {
	return codexconverter.ConvertCodexResponseToOpenAI(ctx, modelName, originalChatJSON, responsesRequestJSON, chunk, param)
}
