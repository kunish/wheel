package relay

import "testing"

func TestDetectRequestType(t *testing.T) {
	type testCase struct {
		name string
		path string
		want string
	}

	audioCases := []testCase{
		{name: "audio speech", path: "/v1/audio/speech", want: RequestTypeAudioSpeech},
		{name: "audio transcriptions", path: "/v1/audio/transcriptions", want: RequestTypeAudioTranscribe},
		{name: "audio translations", path: "/v1/audio/translations", want: RequestTypeAudioTranslate},
		{name: "nested audio transcriptions", path: "/proxy/v1/audio/transcriptions", want: RequestTypeAudioTranscribe},
		{name: "nested audio translations", path: "/proxy/v1/audio/translations", want: RequestTypeAudioTranslate},
	}

	tests := append([]testCase{
		{name: "chat completions", path: "/v1/chat/completions", want: RequestTypeChat},
		{name: "completions", path: "/v1/completions", want: RequestTypeCompletions},
		{name: "anthropic messages", path: "/v1/messages", want: RequestTypeAnthropicMsg},
		{name: "embeddings", path: "/v1/embeddings", want: RequestTypeEmbeddings},
		{name: "responses", path: "/v1/responses", want: RequestTypeResponses},
		{name: "nested chat path", path: "/api/v1/chat/completions", want: RequestTypeChat},
		{name: "nested embeddings", path: "/api/v1/embeddings", want: RequestTypeEmbeddings},
		{name: "images generations", path: "/v1/images/generations", want: RequestTypeImageGeneration},
	}, append(audioCases,
		testCase{name: "moderations", path: "/v1/moderations", want: RequestTypeModerations},
		testCase{name: "nested moderations", path: "/proxy/v1/moderations", want: RequestTypeModerations},
		testCase{name: "empty path", path: "", want: ""},
		testCase{name: "root", path: "/", want: ""},
		testCase{name: "unknown path", path: "/v1/unknown", want: ""},
	)...)

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
		{
			name:       "stream is not bool",
			body:       map[string]any{"model": "gpt-4o", "stream": "true"},
			wantModel:  "gpt-4o",
			wantStream: false,
		},
		{
			name:       "empty model string",
			body:       map[string]any{"model": ""},
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
