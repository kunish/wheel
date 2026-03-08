package gemini

import (
	. "github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/corelib/constant"
	"github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/corelib/interfaces"
	"github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/corelib/translator/translator"
)

func init() {
	translator.Register(
		Gemini,
		Codex,
		ConvertGeminiRequestToCodex,
		interfaces.TranslateResponse{
			Stream:     ConvertCodexResponseToGemini,
			NonStream:  ConvertCodexResponseToGeminiNonStream,
			TokenCount: GeminiTokenCount,
		},
	)
}
