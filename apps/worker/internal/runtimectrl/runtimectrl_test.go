package runtimectrl

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	sdkcliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

func TestNewManagementHandlerProvidesManagementRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	cfg := &sdkconfig.Config{
		AuthDir: authDir,
		RemoteManagement: sdkconfig.RemoteManagement{
			SecretKey: "secret",
		},
	}
	handler := NewManagementHandler(cfg, filepath.Join(authDir, "config.yaml"), sdkcliproxyauth.NewManager(nil, nil, nil))
	if handler == nil {
		t.Fatal("expected non-nil management handler")
	}
	handler.SetLocalPassword("secret")
	if handler.Middleware() == nil {
		t.Fatal("expected management middleware")
	}

	router := gin.New()
	mgmt := router.Group("/v0/management")
	mgmt.Use(handler.Middleware())
	handler.RegisterRoutes(mgmt)

	req := httptest.NewRequest(http.MethodGet, "/v0/management/config", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Authorization", "Bearer secret")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestWriteOAuthCallbackDelegatesToSDKDefault(t *testing.T) {
	if _, err := WriteOAuthCallback(t.TempDir(), "codex", "", "code", ""); err == nil {
		t.Fatal("expected invalid state error")
	}
}
