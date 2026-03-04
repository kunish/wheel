package handler

import (
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
)

func hasRoute(routes []gin.RouteInfo, method, path string) bool {
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}

func TestRegisterRelayRoutes_NoWildcardConflicts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &RelayHandler{}

	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("register relay routes panicked: %v", rec)
		}
	}()

	h.RegisterRelayRoutes(r)

	routes := r.Routes()
	required := []struct {
		method string
		path   string
	}{
		{method: "POST", path: "/v1/chat/completions"},
		{method: "POST", path: "/v1/messages"},
		{method: "POST", path: "/v1/embeddings"},
		{method: "POST", path: "/v1/responses"},
		{method: "POST", path: "/v1/images/generations"},
		{method: "POST", path: "/v1/audio/speech"},
		{method: "POST", path: "/v1/audio/transcriptions"},
		{method: "POST", path: "/v1/audio/translations"},
		{method: "POST", path: "/v1/moderations"},
		{method: "POST", path: "/v1/mcp/tool/execute"},
		{method: "POST", path: "/v1/batch"},
		{method: "POST", path: "/v1/async/chat/completions"},
	}

	for _, item := range required {
		if !hasRoute(routes, item.method, item.path) {
			t.Fatalf("missing route %s %s", item.method, item.path)
		}
	}

	if hasRoute(routes, "POST", "/v1/*path") {
		t.Fatal("unexpected wildcard relay route POST /v1/*path")
	}

	if hasRoute(routes, "ANY", "/v1/*path") {
		t.Fatal("unexpected wildcard relay route ANY /v1/*path")
	}

	for _, route := range routes {
		if route.Method == "POST" && route.Path == "/v1/*path" {
			t.Fatal(fmt.Sprintf("unexpected wildcard route detected: %s %s", route.Method, route.Path))
		}
	}
}
