package handler

import (
	"net/http"
	"strings"
)

func firstHeaderValue(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	parts := strings.Split(v, ",")
	return strings.TrimSpace(parts[0])
}

func requestScheme(r *http.Request) string {
	if r == nil {
		return "http"
	}
	if proto := firstHeaderValue(r.Header.Get("X-Forwarded-Proto")); proto != "" {
		return proto
	}
	if r.TLS != nil {
		return "https"
	}
	if r.URL != nil && r.URL.Scheme != "" {
		return r.URL.Scheme
	}
	return "http"
}

func requestHost(r *http.Request) string {
	if r == nil {
		return ""
	}
	if host := firstHeaderValue(r.Header.Get("X-Forwarded-Host")); host != "" {
		return host
	}
	return strings.TrimSpace(r.Host)
}

func buildPublicMCPServerURL(r *http.Request) string {
	host := requestHost(r)
	if host == "" {
		return "/mcp/sse"
	}
	return requestScheme(r) + "://" + host + "/mcp/sse"
}
