package chat_completions

import (
	. "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/constant"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/interfaces"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/translator"
)

func init() {
	translator.Register(
		OpenAI,
		OpenAI,
		ConvertOpenAIRequestToOpenAI,
		interfaces.TranslateResponse{
			Stream:    ConvertOpenAIResponseToOpenAI,
			NonStream: ConvertOpenAIResponseToOpenAINonStream,
		},
	)
}
