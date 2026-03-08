// Package openai provides translation between OpenAI Chat Completions and Kiro formats.
package openai

import (
	. "github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/corelib/constant"
	"github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/corelib/interfaces"
	"github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/corelib/translator/translator"
)

func init() {
	translator.Register(
		OpenAI, // source format
		Kiro,   // target format
		ConvertOpenAIRequestToKiro,
		interfaces.TranslateResponse{
			Stream:    ConvertKiroStreamToOpenAI,
			NonStream: ConvertKiroNonStreamToOpenAI,
		},
	)
}