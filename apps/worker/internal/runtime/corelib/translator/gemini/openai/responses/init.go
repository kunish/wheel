package responses

import (
	. "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/constant"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/interfaces"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/translator"
)

func init() {
	translator.Register(
		OpenaiResponse,
		Gemini,
		ConvertOpenAIResponsesRequestToGemini,
		interfaces.TranslateResponse{
			Stream:    ConvertGeminiResponseToOpenAIResponses,
			NonStream: ConvertGeminiResponseToOpenAIResponsesNonStream,
		},
	)
}
