package handler

import (
	"testing"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

func TestNormalizeRuntimeTargetModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		channelType types.OutboundType
		model       string
		want        string
	}{
		{
			name:        "copilot opus hyphen alias becomes dot form",
			channelType: types.OutboundCopilot,
			model:       "claude-opus-4-6",
			want:        "claude-opus-4.6",
		},
		{
			name:        "copilot sonnet hyphen alias becomes dot form",
			channelType: types.OutboundCopilot,
			model:       "claude-sonnet-4-5",
			want:        "claude-sonnet-4.5",
		},
		{
			name:        "non copilot model stays unchanged",
			channelType: types.OutboundOpenAI,
			model:       "claude-opus-4-6",
			want:        "claude-opus-4-6",
		},
		{
			name:        "cursor maps gpt-4 alias to composer-2",
			channelType: types.OutboundCursor,
			model:       "gpt-4",
			want:        "composer-2",
		},
		{
			name:        "cursor maps group claude-opus-4-6 to Cursor opus id",
			channelType: types.OutboundCursor,
			model:       "claude-opus-4-6",
			want:        "claude-4.6-opus-high",
		},
		{
			name:        "cursor maps hyphen-normalized claude sonnet alias",
			channelType: types.OutboundCursor,
			model:       "claude-sonnet-4-5",
			want:        "claude-4.5-sonnet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeRuntimeTargetModel(tt.channelType, tt.model); got != tt.want {
				t.Fatalf("normalizeRuntimeTargetModel() = %q, want %q", got, tt.want)
			}
		})
	}
}
