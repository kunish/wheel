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

// ForwardResponseHeaders copies forwardable headers from upstream response to client response writer.
func ForwardResponseHeaders(w http.ResponseWriter, upstreamResp *http.Response) {
	for _, h := range ForwardableHeaders {
		if v := upstreamResp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	// Also add a header indicating the response came through Wheel
	w.Header().Set("X-Wheel-Proxy", "true")
}
