package responses

import (
	. "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/antigravity/gemini"
	. "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/gemini/openai/responses"
)

func ConvertOpenAIResponsesRequestToAntigravity(modelName string, inputRawJSON []byte, stream bool) []byte {
	rawJSON := inputRawJSON
	rawJSON = ConvertOpenAIResponsesRequestToGemini(modelName, rawJSON, stream)
	return ConvertGeminiRequestToAntigravity(modelName, rawJSON, stream)
}
