package handler

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/config"
)

func TestRegisterRoutes_RegistersCursorRefreshModelsRoute(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &Handler{Config: &config.Config{JWTSecret: "test-secret"}}
	h.RegisterRoutes(r)

	for _, rt := range r.Routes() {
		if rt.Method == http.MethodPost && rt.Path == "/api/v1/channel/:id/cursor/refresh-models" {
			return
		}
	}
	t.Fatalf("missing route %s %s", http.MethodPost, "/api/v1/channel/:id/cursor/refresh-models")
}
