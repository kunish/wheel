package relay

import "testing"

func TestDetectRequestType(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"chat completions", "/v1/chat/completions", "openai-chat"},
		{"anthropic messages", "/v1/messages", "anthropic-messages"},
		{"embeddings", "/v1/embeddings", "openai-embeddings"},
		{"responses", "/v1/responses", "openai-responses"},
		{"gemini generate content", "/v1beta/models/gemini-2.5-pro:generateContent", "gemini-generate-content"},
		{"gemini stream generate content", "/v1beta/models/gemini-2.5-pro:streamGenerateContent", "gemini-stream-generate-content"},
		{"nested chat path", "/api/v1/chat/completions", "openai-chat"},
		{"nested embeddings", "/api/v1/embeddings", "openai-embeddings"},
		{"unknown path", "/v1/images/generations", ""},
		{"empty path", "", ""},
		{"root", "/", ""},
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

func TestExtractModelForRequest(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		requestType string
		body        map[string]any
		wantModel   string
		wantStream  bool
	}{
		{
			name:        "gemini non stream",
			path:        "/v1beta/models/gemini-2.5-pro:generateContent",
			requestType: "gemini-generate-content",
			body:        map[string]any{},
			wantModel:   "gemini-2.5-pro",
			wantStream:  false,
		},
		{
			name:        "gemini stream",
			path:        "/v1beta/models/gemini-2.5-pro:streamGenerateContent",
			requestType: "gemini-stream-generate-content",
			body:        map[string]any{},
			wantModel:   "gemini-2.5-pro",
			wantStream:  true,
		},
		{
			name:        "openai fallback",
			path:        "/v1/chat/completions",
			requestType: "openai-chat",
			body:        map[string]any{"model": "gpt-5", "stream": true},
			wantModel:   "gpt-5",
			wantStream:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, stream := ExtractModelForRequest(tt.path, tt.requestType, tt.body)
			if model != tt.wantModel {
				t.Fatalf("model = %q, want %q", model, tt.wantModel)
			}
			if stream != tt.wantStream {
				t.Fatalf("stream = %v, want %v", stream, tt.wantStream)
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
