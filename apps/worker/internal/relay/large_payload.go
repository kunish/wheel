package relay

import (
	"io"
	"net/http"
)

const (
	// LargePayloadThreshold is the size in bytes above which we switch to passthrough mode.
	// 10 MB by default — avoids JSON parsing overhead for large audio/image payloads.
	LargePayloadThreshold = 10 * 1024 * 1024
)

// IsLargePayload checks if a request body exceeds the large payload threshold.
func IsLargePayload(contentLength int64) bool {
	return contentLength > LargePayloadThreshold
}

// ProxyLargePayload forwards a large request body directly to the upstream
// without JSON parsing, and streams the response back to the client.
func ProxyLargePayload(
	w http.ResponseWriter,
	r *http.Request,
	client *http.Client,
	upstreamURL string,
	upstreamHeaders map[string]string,
) error {
	req, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, r.Body)
	if err != nil {
		return &ProxyError{Message: "failed to create upstream request", StatusCode: 502}
	}
	defer r.Body.Close()

	for k, v := range upstreamHeaders {
		req.Header.Set(k, v)
	}
	if ct := r.Header.Get("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	if cl := r.Header.Get("Content-Length"); cl != "" {
		req.Header.Set("Content-Length", cl)
	}

	resp, err := client.Do(req)
	if err != nil {
		return &ProxyError{Message: "upstream request failed", StatusCode: 502}
	}
	defer resp.Body.Close()

	for k, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.Header().Set("X-Wheel-Proxy", "true")
	w.Header().Set("X-Wheel-Large-Payload", "true")
	w.WriteHeader(resp.StatusCode)

	_, _ = io.Copy(w, resp.Body)
	return nil
}
