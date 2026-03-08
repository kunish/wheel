package responses

import (
	. "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/constant"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/interfaces"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/translator"
)

func init() {
	translator.Register(
		OpenaiResponse,
		GeminiCLI,
		ConvertOpenAIResponsesRequestToGeminiCLI,
		interfaces.TranslateResponse{
			Stream:    ConvertGeminiCLIResponseToOpenAIResponses,
			NonStream: ConvertGeminiCLIResponseToOpenAIResponsesNonStream,
		},
	)
}
