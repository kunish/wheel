// Package claude provides translation between Kiro and Claude formats.
package claude

import (
	. "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/constant"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/interfaces"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/translator"
)

func init() {
	translator.Register(
		Claude,
		Kiro,
		ConvertClaudeRequestToKiro,
		interfaces.TranslateResponse{
			Stream:    ConvertKiroStreamToClaude,
			NonStream: ConvertKiroNonStreamToClaude,
		},
	)
}
