package relay

import "testing"

func TestIsRetryableStatusCode(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   bool
	}{
		{name: "timeout", status: 408, want: true},
		{name: "conflict", status: 409, want: true},
		{name: "rate_limited", status: 429, want: true},
		{name: "server_error", status: 500, want: true},
		{name: "bad_request", status: 400, want: false},
		{name: "unauthorized", status: 401, want: false},
		{name: "client_disconnect", status: 499, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryableStatusCode(tt.status); got != tt.want {
				t.Fatalf("IsRetryableStatusCode(%d) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}
