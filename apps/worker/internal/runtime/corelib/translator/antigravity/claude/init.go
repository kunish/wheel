package claude

import (
	. "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/constant"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/interfaces"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/translator"
)

func init() {
	translator.Register(
		Claude,
		Antigravity,
		ConvertClaudeRequestToAntigravity,
		interfaces.TranslateResponse{
			Stream:     ConvertAntigravityResponseToClaude,
			NonStream:  ConvertAntigravityResponseToClaudeNonStream,
			TokenCount: ClaudeTokenCount,
		},
	)
}
