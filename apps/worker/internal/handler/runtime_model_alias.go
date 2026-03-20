package handler

import (
	"strings"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

// normalizeClaudeHyphenVersions maps common group-style Claude names
// (e.g. claude-opus-4-6) to provider-style minor versions (claude-opus-4.6).
func normalizeClaudeHyphenVersions(model string) string {
	normalized := strings.TrimSpace(model)
	if !strings.HasPrefix(normalized, "claude-") {
		return normalized
	}
	normalized = strings.Replace(normalized, "-4-1", "-4.1", 1)
	normalized = strings.Replace(normalized, "-4-5", "-4.5", 1)
	normalized = strings.Replace(normalized, "-4-6", "-4.6", 1)
	return normalized
}

func normalizeRuntimeTargetModel(channelType types.OutboundType, model string) string {
	if channelType == types.OutboundCursor {
		return mapCursorUpstreamModel(model)
	}
	if channelType != types.OutboundCopilot {
		return model
	}

	return normalizeClaudeHyphenVersions(model)
}
