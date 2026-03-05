package handler

import (
	"net/http/httptest"
	"testing"
)

func TestBuildPublicMCPServerURL(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		headers map[string]string
		want    string
	}{
		{
			name: "uses forwarded proto and host",
			host: "worker:8787",
			headers: map[string]string{
				"X-Forwarded-Proto": "https",
				"X-Forwarded-Host":  "gateway.example.com",
			},
			want: "https://gateway.example.com/mcp/sse",
		},
		{
			name: "falls back to request host",
			host: "localhost:3000",
			want: "http://localhost:3000/mcp/sse",
		},
		{
			name: "forwarded host picks first value",
			host: "worker:8787",
			headers: map[string]string{
				"X-Forwarded-Host": "api.example.com, proxy.local",
			},
			want: "http://api.example.com/mcp/sse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://"+tt.host+"/api/v1/mcp/client/list", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			got := buildPublicMCPServerURL(req)
			if got != tt.want {
				t.Fatalf("buildPublicMCPServerURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
