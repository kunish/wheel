package relay

import "testing"

func TestDetectRequestType(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"chat completions", "/v1/chat/completions", RequestTypeChat},
		{"anthropic messages", "/v1/messages", RequestTypeAnthropicMsg},
		{"embeddings", "/v1/embeddings", RequestTypeEmbeddings},
		{"responses", "/v1/responses", RequestTypeResponses},
		{"nested chat path", "/api/v1/chat/completions", RequestTypeChat},
		{"nested embeddings", "/api/v1/embeddings", RequestTypeEmbeddings},
		{"images generations", "/v1/images/generations", RequestTypeImageGeneration},
		{"audio speech", "/v1/audio/speech", RequestTypeAudioSpeech},
		{"audio transcriptions", "/v1/audio/transcriptions", RequestTypeAudioTranscribe},
		{"audio translations", "/v1/audio/translations", RequestTypeAudioTranslate},
		{"moderations", "/v1/moderations", RequestTypeModerations},
		{"empty path", "", ""},
		{"root", "/", ""},
		{"unknown path", "/v1/unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectRequestType(tt.path)
			if got != tt.want {
				t.Errorf("DetectRequestType(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestExtractModel(t *testing.T) {
	tests := []struct {
		name       string
		body       map[string]any
		wantModel  string
		wantStream bool
	}{
		{
			name:       "model and stream",
			body:       map[string]any{"model": "gpt-4o", "stream": true},
			wantModel:  "gpt-4o",
			wantStream: true,
		},
		{
			name:       "model only",
			body:       map[string]any{"model": "claude-3-opus-20240229"},
			wantModel:  "claude-3-opus-20240229",
			wantStream: false,
		},
		{
			name:       "stream false",
			body:       map[string]any{"model": "gpt-4o", "stream": false},
			wantModel:  "gpt-4o",
			wantStream: false,
		},
		{
			name:       "empty body",
			body:       map[string]any{},
			wantModel:  "",
			wantStream: false,
		},
		{
			name:       "model is not string",
			body:       map[string]any{"model": 123},
			wantModel:  "",
			wantStream: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, stream := ExtractModel(tt.body)
			if model != tt.wantModel {
				t.Errorf("model = %q, want %q", model, tt.wantModel)
			}
			if stream != tt.wantStream {
				t.Errorf("stream = %v, want %v", stream, tt.wantStream)
			}
		})
	}
}
