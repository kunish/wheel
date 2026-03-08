package handler

import (
	"strings"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

func normalizeRuntimeTargetModel(channelType types.OutboundType, model string) string {
	if channelType != types.OutboundCopilot {
		return model
	}

	normalized := strings.TrimSpace(model)
	if strings.HasPrefix(normalized, "claude-") {
		normalized = strings.Replace(normalized, "-4-1", "-4.1", 1)
		normalized = strings.Replace(normalized, "-4-5", "-4.5", 1)
		normalized = strings.Replace(normalized, "-4-6", "-4.6", 1)
	}
	return normalized
}
