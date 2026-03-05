package relay

import (
	"net/http"
	"testing"
	"time"
)

func TestParseRetryDelay_SupportsSeconds(t *testing.T) {
	resp := &http.Response{Header: http.Header{"Retry-After": []string{"2"}}}

	got := parseRetryDelay(resp, "")
	if got != 2000 {
		t.Fatalf("expected 2000ms from numeric Retry-After, got %d", got)
	}
}

func TestParseRetryDelay_SupportsHTTPDate(t *testing.T) {
	when := time.Now().UTC().Add(2 * time.Second).Format(http.TimeFormat)
	resp := &http.Response{Header: http.Header{"Retry-After": []string{when}}}

	got := parseRetryDelay(resp, "")
	if got < 1000 || got > 5000 {
		t.Fatalf("expected HTTP-date Retry-After to resolve to ~2s, got %dms", got)
	}
}
