package relay

import "net/http"

// ForwardableHeaders lists upstream response headers worth forwarding to clients.
var ForwardableHeaders = []string{
	"X-Request-Id",
	"X-Ratelimit-Limit-Requests",
	"X-Ratelimit-Limit-Tokens",
	"X-Ratelimit-Remaining-Requests",
	"X-Ratelimit-Remaining-Tokens",
	"X-Ratelimit-Reset-Requests",
	"X-Ratelimit-Reset-Tokens",
	"Retry-After",
	"Openai-Organization",
	"Openai-Processing-Ms",
	"Openai-Version",
	"Anthropic-Ratelimit-Requests-Limit",
	"Anthropic-Ratelimit-Requests-Remaining",
	"Anthropic-Ratelimit-Requests-Reset",
	"Anthropic-Ratelimit-Tokens-Limit",
	"Anthropic-Ratelimit-Tokens-Remaining",
	"Anthropic-Ratelimit-Tokens-Reset",
	"Cf-Cache-Status",
}

// MeaningfulErrorHeaders lists the subset of upstream headers worth preserving
// on gateway-generated error envelopes.
var MeaningfulErrorHeaders = []string{
	"X-Request-Id",
	"X-Ratelimit-Limit-Requests",
	"X-Ratelimit-Limit-Tokens",
	"X-Ratelimit-Remaining-Requests",
	"X-Ratelimit-Remaining-Tokens",
	"X-Ratelimit-Reset-Requests",
	"X-Ratelimit-Reset-Tokens",
	"Retry-After",
	"Anthropic-Ratelimit-Requests-Limit",
	"Anthropic-Ratelimit-Requests-Remaining",
	"Anthropic-Ratelimit-Requests-Reset",
	"Anthropic-Ratelimit-Tokens-Limit",
	"Anthropic-Ratelimit-Tokens-Remaining",
	"Anthropic-Ratelimit-Tokens-Reset",
}

// forwardResponseHeaders copies forwardable headers from upstream response to client response writer.
func forwardResponseHeaders(w http.ResponseWriter, upstreamResp *http.Response) {
	CopyForwardableHeaders(w.Header(), upstreamResp.Header)
	// Also add a header indicating the response came through Wheel
	w.Header().Set("X-Wheel-Proxy", "true")
}

// CopyForwardableHeaders copies safe upstream headers into dst.
func CopyForwardableHeaders(dst, src http.Header) {
	copySelectedHeaders(dst, src, ForwardableHeaders)
}

// CopyMeaningfulErrorHeaders copies a minimal set of meaningful upstream error
// headers into dst for gateway-generated error responses.
func CopyMeaningfulErrorHeaders(dst, src http.Header) {
	copySelectedHeaders(dst, src, MeaningfulErrorHeaders)
}

func copySelectedHeaders(dst, src http.Header, allowed []string) {
	if len(src) == 0 {
		return
	}
	for _, h := range allowed {
		if v := src.Get(h); v != "" {
			dst.Set(h, v)
		}
	}
}
