package translator

import (
	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/claude/gemini"
	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/claude/gemini-cli"
	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/claude/openai/chat-completions"
	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/claude/openai/responses"

	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/codex/claude"
	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/codex/gemini"
	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/codex/gemini-cli"
	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/codex/openai/chat-completions"
	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/codex/openai/responses"

	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/gemini-cli/claude"
	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/gemini-cli/gemini"
	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/gemini-cli/openai/chat-completions"
	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/gemini-cli/openai/responses"

	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/gemini/claude"
	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/gemini/gemini"
	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/gemini/gemini-cli"
	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/gemini/openai/chat-completions"
	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/gemini/openai/responses"

	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/openai/claude"
	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/openai/gemini"
	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/openai/gemini-cli"
	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/openai/openai/chat-completions"
	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/openai/openai/responses"

	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/antigravity/claude"
	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/antigravity/gemini"
	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/antigravity/openai/chat-completions"
	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/antigravity/openai/responses"

	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/kiro/claude"
	_ "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/translator/kiro/openai"
)
