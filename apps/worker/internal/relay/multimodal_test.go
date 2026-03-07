package relay

import (
	"encoding/json"
	"testing"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

func TestExtractMultimodalModel(t *testing.T) {
	tests := []struct {
		name        string
		body        map[string]any
		requestType string
		want        string
	}{
		{
			name:        "explicit audio speech model wins",
			body:        map[string]any{"model": "gpt-4o-mini-tts"},
			requestType: RequestTypeAudioSpeech,
			want:        "gpt-4o-mini-tts",
		},
		{
			name:        "audio speech default",
			body:        map[string]any{"voice": "alloy"},
			requestType: RequestTypeAudioSpeech,
			want:        "tts-1",
		},
		{
			name:        "audio transcription default",
			body:        map[string]any{"language": "en"},
			requestType: RequestTypeAudioTranscribe,
			want:        "whisper-1",
		},
		{
			name:        "audio translation default",
			body:        map[string]any{"temperature": 0},
			requestType: RequestTypeAudioTranslate,
			want:        "whisper-1",
		},
		{
			name:        "moderations default",
			body:        map[string]any{"input": "hello"},
			requestType: RequestTypeModerations,
			want:        "omni-moderation-latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractMultimodalModel(tt.body, tt.requestType); got != tt.want {
				t.Fatalf("ExtractMultimodalModel(%v, %q) = %q, want %q", tt.body, tt.requestType, got, tt.want)
			}
		})
	}
}

func TestIsMultimodalRequest(t *testing.T) {
	tests := []struct {
		name        string
		requestType string
		want        bool
	}{
		{name: "image generation is multimodal", requestType: RequestTypeImageGeneration, want: true},
		{name: "audio speech is multimodal", requestType: RequestTypeAudioSpeech, want: true},
		{name: "audio transcription is multimodal", requestType: RequestTypeAudioTranscribe, want: true},
		{name: "audio translation is multimodal", requestType: RequestTypeAudioTranslate, want: true},
		{name: "moderations is not multimodal", requestType: RequestTypeModerations, want: false},
		{name: "chat is not multimodal", requestType: RequestTypeChat, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsMultimodalRequest(tt.requestType); got != tt.want {
				t.Fatalf("IsMultimodalRequest(%q) = %v, want %v", tt.requestType, got, tt.want)
			}
		})
	}
}

func TestBuildMultimodalUpstreamRequest_AzureUsesDeploymentURLSemantics(t *testing.T) {
	overrides := `{"temperature":0}`
	upstream := BuildMultimodalUpstreamRequest(
		ChannelConfig{
			Type:          types.OutboundAzureOpenAI,
			BaseUrls:      []types.BaseUrl{{URL: "https://azure.example.net/", Delay: 0}},
			CustomHeader:  []types.CustomHeader{{Key: "api-version", Value: "2025-02-01-preview"}},
			ParamOverride: &overrides,
		},
		"azure-key",
		map[string]any{"input": "hello"},
		"omni-moderation-latest",
		RequestTypeModerations,
	)

	if got, want := upstream.URL, "https://azure.example.net/openai/deployments/omni-moderation-latest/moderations?api-version=2025-02-01-preview"; got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}
	if got := upstream.Headers["api-key"]; got != "azure-key" {
		t.Fatalf("api-key = %q, want %q", got, "azure-key")
	}
	if got := upstream.Headers["Authorization"]; got != "" {
		t.Fatalf("Authorization = %q, want empty", got)
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(upstream.Body), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if _, ok := body["model"]; ok {
		t.Fatalf("expected azure multimodal body to omit model, got %#v", body)
	}
	if got := body["temperature"]; got != float64(0) {
		t.Fatalf("temperature = %#v, want 0", got)
	}
}
