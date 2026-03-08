package gemini

import (
	. "github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/corelib/constant"
	"github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/corelib/interfaces"
	"github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/corelib/translator/translator"
)

// Register a no-op response translator and a request normalizer for Gemini→Gemini.
// The request converter ensures missing or invalid roles are normalized to valid values.
func init() {
	translator.Register(
		Gemini,
		Gemini,
		ConvertGeminiRequestToGemini,
		interfaces.TranslateResponse{
			Stream:     PassthroughGeminiResponseStream,
			NonStream:  PassthroughGeminiResponseNonStream,
			TokenCount: GeminiTokenCount,
		},
	)
}
