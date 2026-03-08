package responses

import (
	. "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/gemini-cli/gemini"
	. "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/gemini/openai/responses"
)

func ConvertOpenAIResponsesRequestToGeminiCLI(modelName string, inputRawJSON []byte, stream bool) []byte {
	rawJSON := inputRawJSON
	rawJSON = ConvertOpenAIResponsesRequestToGemini(modelName, rawJSON, stream)
	return ConvertGeminiRequestToGeminiCLI(modelName, rawJSON, stream)
}
