package relay

import "testing"

func TestIsRetryableStatusCode(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   bool
	}{
		{name: "network_error", status: 0, want: true},
		{name: "negative", status: -1, want: true},
		{name: "bad_request", status: 400, want: true},
		{name: "unauthorized", status: 401, want: true},
		{name: "payment_required", status: 402, want: true},
		{name: "forbidden", status: 403, want: true},
		{name: "not_found", status: 404, want: true},
		{name: "timeout", status: 408, want: true},
		{name: "conflict", status: 409, want: true},
		{name: "unprocessable_entity", status: 422, want: false},
		{name: "rate_limited", status: 429, want: true},
		{name: "client_disconnect", status: 499, want: true},
		{name: "server_error", status: 500, want: true},
		{name: "bad_gateway", status: 502, want: true},
		{name: "service_unavailable", status: 503, want: true},
		{name: "success_200", status: 200, want: false},
		{name: "success_204", status: 204, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryableStatusCode(tt.status); got != tt.want {
				t.Fatalf("IsRetryableStatusCode(%d) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}
