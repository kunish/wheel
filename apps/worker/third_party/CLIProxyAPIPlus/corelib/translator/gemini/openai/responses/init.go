package responses

import (
	. "github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/corelib/constant"
	"github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/corelib/interfaces"
	"github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/corelib/translator/translator"
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
