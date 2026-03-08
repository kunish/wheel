package geminiCLI

import (
	. "github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/corelib/constant"
	"github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/corelib/interfaces"
	"github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/corelib/translator/translator"
)

func init() {
	translator.Register(
		GeminiCLI,
		Codex,
		ConvertGeminiCLIRequestToCodex,
		interfaces.TranslateResponse{
			Stream:     ConvertCodexResponseToGeminiCLI,
			NonStream:  ConvertCodexResponseToGeminiCLINonStream,
			TokenCount: GeminiCLITokenCount,
		},
	)
}
