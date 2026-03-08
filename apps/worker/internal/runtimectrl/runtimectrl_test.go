package runtimectrl

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	sdkcliproxy "github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/sdk/cliproxy"
	sdkcliproxyauth "github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/sdk/cliproxy/auth"
	sdkconfig "github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/sdk/config"
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

func TestWriteOAuthCallbackPersistsPendingSession(t *testing.T) {
	authDir := t.TempDir()
	state := "pending-state"
	sdkcliproxy.RegisterOAuthSession(state, "codex")

	path, err := WriteOAuthCallback(authDir, "codex", state, "test-code", "")
	if err != nil {
		t.Fatalf("WriteOAuthCallback() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var payload struct {
		Code  string `json:"code"`
		State string `json:"state"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if payload.Code != "test-code" || payload.State != state || payload.Error != "" {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestWriteOAuthCallbackRejectsNonPendingSession(t *testing.T) {
	if _, err := WriteOAuthCallback(t.TempDir(), "codex", "missing-state", "test-code", ""); err == nil {
		t.Fatal("expected pending session error")
	}
}

func TestManagementHandlerRegistersOwnedOAuthCallbackRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	cfg := &sdkconfig.Config{AuthDir: authDir}
	handler := NewManagementHandler(cfg, filepath.Join(authDir, "config.yaml"), sdkcliproxyauth.NewManager(nil, nil, nil))
	fake := &fakeManagementRoutes{}
	handler.inner = fake

	state := "pending-state"
	sdkcliproxy.RegisterOAuthSession(state, "codex")

	router := gin.New()
	mgmt := router.Group("/v0/management")
	mgmt.Use(handler.Middleware())
	handler.RegisterRoutes(mgmt)

	req := httptest.NewRequest(http.MethodPost, "/v0/management/oauth-callback", strings.NewReader(`{"provider":"codex","state":"pending-state","code":"test-code"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !fake.registerWithoutOAuthCalled {
		t.Fatal("expected wrapper to use RegisterRoutesWithoutOAuthCallback")
	}
	payloadFile := filepath.Join(authDir, ".oauth-codex-pending-state.oauth")
	data, err := os.ReadFile(payloadFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var payload struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if payload.Code != "test-code" {
		t.Fatalf("payload code = %q, want %q", payload.Code, "test-code")
	}
}

func TestManagementHandlerRegistersOwnedGetAuthStatusRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	handler := NewManagementHandler(&sdkconfig.Config{AuthDir: authDir}, filepath.Join(authDir, "config.yaml"), sdkcliproxyauth.NewManager(nil, nil, nil))
	fake := &fakeManagementRoutes{}
	handler.inner = fake

	state := "pending-status-state"
	sdkcliproxy.RegisterOAuthSession(state, "codex")

	router := gin.New()
	mgmt := router.Group("/v0/management")
	mgmt.Use(handler.Middleware())
	handler.RegisterRoutes(mgmt)

	req := httptest.NewRequest(http.MethodGet, "/v0/management/get-auth-status?state="+state, nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if payload["status"] != "wait" {
		t.Fatalf("status payload = %v, want wait", payload["status"])
	}
}

func TestManagementHandlerRegistersOwnedCodexAuthURLRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	handler := NewManagementHandler(&sdkconfig.Config{AuthDir: authDir}, filepath.Join(authDir, "config.yaml"), sdkcliproxyauth.NewManager(nil, nil, nil))
	fake := &fakeManagementRoutes{}
	handler.inner = fake

	router := gin.New()
	mgmt := router.Group("/v0/management")
	mgmt.Use(handler.Middleware())
	handler.RegisterRoutes(mgmt)

	req := httptest.NewRequest(http.MethodGet, "/v0/management/codex-auth-url", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	// Codex is now first-party: should return 200 with url+state, not delegate to inner
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("body status = %v, want ok", body["status"])
	}
	if body["url"] == nil || body["url"] == "" {
		t.Fatal("expected url in response")
	}
	if body["state"] == nil || body["state"] == "" {
		t.Fatal("expected state in response")
	}
	if fake.requestCodexTokenCalled {
		t.Fatal("expected first-party handler, not delegation to inner")
	}
}

func TestManagementHandlerRegistersOwnedGitHubAuthURLRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fake := &fakeManagementRoutes{}
	nilHandler := &ManagementHandler{cfg: nil, inner: fake}
	router := gin.New()
	mgmt := router.Group("/v0/management")
	nilHandler.RegisterRoutes(mgmt)

	req := httptest.NewRequest(http.MethodGet, "/v0/management/github-auth-url", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code == http.StatusNotFound {
		t.Fatal("expected first-party github handler to be registered (got 404)")
	}
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusInternalServerError, rr.Body.String())
	}
}

func TestManagementHandlerRegistersOwnedAnthropicAuthURLRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	handler := NewManagementHandler(&sdkconfig.Config{AuthDir: authDir}, filepath.Join(authDir, "config.yaml"), sdkcliproxyauth.NewManager(nil, nil, nil))
	fake := &fakeManagementRoutes{}
	handler.inner = fake

	router := gin.New()
	mgmt := router.Group("/v0/management")
	mgmt.Use(handler.Middleware())
	handler.RegisterRoutes(mgmt)

	req := httptest.NewRequest(http.MethodGet, "/v0/management/anthropic-auth-url", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("body status = %v, want ok", body["status"])
	}
	if body["url"] == nil || body["url"] == "" {
		t.Fatal("expected url in response")
	}
	if body["state"] == nil || body["state"] == "" {
		t.Fatal("expected state in response")
	}
	if fake.requestAnthropicTokenCalled {
		t.Fatal("expected first-party handler, not delegation to inner")
	}
}

func TestManagementHandlerRegistersOwnedGeminiCLIAuthURLRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	handler := NewManagementHandler(&sdkconfig.Config{AuthDir: authDir}, filepath.Join(authDir, "config.yaml"), sdkcliproxyauth.NewManager(nil, nil, nil))
	fake := &fakeManagementRoutes{}
	handler.inner = fake

	router := gin.New()
	mgmt := router.Group("/v0/management")
	mgmt.Use(handler.Middleware())
	handler.RegisterRoutes(mgmt)

	req := httptest.NewRequest(http.MethodGet, "/v0/management/gemini-cli-auth-url", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !fake.requestGeminiCLITokenCalled {
		t.Fatal("expected wrapper to route gemini-cli-auth-url through owned registration")
	}
}

func TestManagementHandlerRegistersOwnedAntigravityAuthURLRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	handler := NewManagementHandler(&sdkconfig.Config{AuthDir: authDir}, filepath.Join(authDir, "config.yaml"), sdkcliproxyauth.NewManager(nil, nil, nil))
	fake := &fakeManagementRoutes{}
	handler.inner = fake

	router := gin.New()
	mgmt := router.Group("/v0/management")
	mgmt.Use(handler.Middleware())
	handler.RegisterRoutes(mgmt)

	req := httptest.NewRequest(http.MethodGet, "/v0/management/antigravity-auth-url", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("body status = %v, want ok", body["status"])
	}
	if body["url"] == nil || body["url"] == "" {
		t.Fatal("expected url in response")
	}
	if fake.requestAntigravityTokenCalled {
		t.Fatal("expected first-party handler, not delegation to inner")
	}
}

func TestManagementHandlerRegistersOwnedQwenAuthURLRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	handler := NewManagementHandler(&sdkconfig.Config{AuthDir: authDir}, filepath.Join(authDir, "config.yaml"), sdkcliproxyauth.NewManager(nil, nil, nil))
	fake := &fakeManagementRoutes{}
	handler.inner = fake

	router := gin.New()
	mgmt := router.Group("/v0/management")
	mgmt.Use(handler.Middleware())
	handler.RegisterRoutes(mgmt)

	// Qwen is now a first-party handler that would make real network calls,
	// so we cannot invoke it in unit tests. Instead verify the route exists
	// by confirming a nil-cfg handler returns an error rather than 404.
	nilHandler := &ManagementHandler{cfg: nil, inner: fake}
	nilRouter := gin.New()
	nilMgmt := nilRouter.Group("/v0/management")
	nilHandler.RegisterRoutes(nilMgmt)

	req := httptest.NewRequest(http.MethodGet, "/v0/management/qwen-auth-url", nil)
	rr := httptest.NewRecorder()
	nilRouter.ServeHTTP(rr, req)

	// First-party handler with nil cfg should return 500, not 404.
	if rr.Code == http.StatusNotFound {
		t.Fatal("expected first-party qwen handler to be registered (got 404)")
	}
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusInternalServerError, rr.Body.String())
	}
}

func TestManagementHandlerRegistersOwnedKiloAuthURLRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fake := &fakeManagementRoutes{}
	nilHandler := &ManagementHandler{cfg: nil, inner: fake}
	router := gin.New()
	mgmt := router.Group("/v0/management")
	nilHandler.RegisterRoutes(mgmt)

	req := httptest.NewRequest(http.MethodGet, "/v0/management/kilo-auth-url", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code == http.StatusNotFound {
		t.Fatal("expected first-party kilo handler to be registered (got 404)")
	}
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusInternalServerError, rr.Body.String())
	}
}

func TestManagementHandlerRegistersOwnedKimiAuthURLRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fake := &fakeManagementRoutes{}
	nilHandler := &ManagementHandler{cfg: nil, inner: fake}
	router := gin.New()
	mgmt := router.Group("/v0/management")
	nilHandler.RegisterRoutes(mgmt)

	req := httptest.NewRequest(http.MethodGet, "/v0/management/kimi-auth-url", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code == http.StatusNotFound {
		t.Fatal("expected first-party kimi handler to be registered (got 404)")
	}
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusInternalServerError, rr.Body.String())
	}
}

func TestManagementHandlerRegistersOwnedIFlowAuthURLRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	handler := NewManagementHandler(&sdkconfig.Config{AuthDir: authDir}, filepath.Join(authDir, "config.yaml"), sdkcliproxyauth.NewManager(nil, nil, nil))
	fake := &fakeManagementRoutes{}
	handler.inner = fake

	router := gin.New()
	mgmt := router.Group("/v0/management")
	mgmt.Use(handler.Middleware())
	handler.RegisterRoutes(mgmt)

	// Test GET /iflow-auth-url — first-party OAuth handler
	req := httptest.NewRequest(http.MethodGet, "/v0/management/iflow-auth-url", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("GET body status = %v, want ok", body["status"])
	}
	if body["url"] == nil || body["url"] == "" {
		t.Fatal("expected url in GET response")
	}
	if fake.requestIFlowTokenCalled {
		t.Fatal("expected first-party handler for GET, not delegation")
	}

	// Test POST /iflow-auth-url — first-party cookie handler
	req = httptest.NewRequest(http.MethodPost, "/v0/management/iflow-auth-url", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// POST with empty body should return 400 (cookie required)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("POST status = %d, want %d body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
	if fake.requestIFlowCookieTokenCalled {
		t.Fatal("expected first-party handler for POST, not delegation")
	}
}

func TestManagementHandlerRegistersOwnedKiroAuthURLRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	handler := NewManagementHandler(&sdkconfig.Config{AuthDir: authDir}, filepath.Join(authDir, "config.yaml"), sdkcliproxyauth.NewManager(nil, nil, nil))
	fake := &fakeManagementRoutes{}
	handler.inner = fake

	router := gin.New()
	mgmt := router.Group("/v0/management")
	mgmt.Use(handler.Middleware())
	handler.RegisterRoutes(mgmt)

	req := httptest.NewRequest(http.MethodGet, "/v0/management/kiro-auth-url", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !fake.requestKiroTokenCalled {
		t.Fatal("expected wrapper to route kiro-auth-url through owned registration")
	}
}

type fakeManagementRoutes struct {
	registerWithoutOAuthCalled    bool
	requestAnthropicTokenCalled   bool
	requestCodexTokenCalled       bool
	requestGitHubTokenCalled      bool
	requestGeminiCLITokenCalled   bool
	requestAntigravityTokenCalled bool
	requestQwenTokenCalled        bool
	requestKiloTokenCalled        bool
	requestKimiTokenCalled        bool
	requestIFlowTokenCalled       bool
	requestIFlowCookieTokenCalled bool
	requestKiroTokenCalled        bool
}

func (f *fakeManagementRoutes) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

func (f *fakeManagementRoutes) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/config", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"handler": "fallback"})
	})
}

func (f *fakeManagementRoutes) RegisterRoutesWithoutOAuthCallback(group *gin.RouterGroup) {
	f.registerWithoutOAuthCalled = true
	f.RegisterRoutes(group)
}

func (f *fakeManagementRoutes) RegisterRoutesWithoutOAuthSessionRoutes(group *gin.RouterGroup) {
	f.registerWithoutOAuthCalled = true
	f.RegisterRoutes(group)
}

func (f *fakeManagementRoutes) RequestCodexToken(c *gin.Context) {
	f.requestCodexTokenCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok", "url": "https://example.test/codex"})
}

func (f *fakeManagementRoutes) RequestAnthropicToken(c *gin.Context) {
	f.requestAnthropicTokenCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok", "url": "https://example.test/anthropic"})
}

func (f *fakeManagementRoutes) RequestGitHubToken(c *gin.Context) {
	f.requestGitHubTokenCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok", "url": "https://example.test/github"})
}

func (f *fakeManagementRoutes) RequestGeminiCLIToken(c *gin.Context) {
	f.requestGeminiCLITokenCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok", "url": "https://example.test/gemini-cli"})
}

func (f *fakeManagementRoutes) RequestAntigravityToken(c *gin.Context) {
	f.requestAntigravityTokenCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok", "url": "https://example.test/antigravity"})
}

func (f *fakeManagementRoutes) RequestQwenToken(c *gin.Context) {
	f.requestQwenTokenCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok", "url": "https://example.test/qwen"})
}

func (f *fakeManagementRoutes) RequestKiloToken(c *gin.Context) {
	f.requestKiloTokenCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok", "url": "https://example.test/kilo"})
}

func (f *fakeManagementRoutes) RequestKimiToken(c *gin.Context) {
	f.requestKimiTokenCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok", "url": "https://example.test/kimi"})
}

func (f *fakeManagementRoutes) RequestIFlowToken(c *gin.Context) {
	f.requestIFlowTokenCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok", "url": "https://example.test/iflow"})
}

func (f *fakeManagementRoutes) RequestIFlowCookieToken(c *gin.Context) {
	f.requestIFlowCookieTokenCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (f *fakeManagementRoutes) RequestKiroToken(c *gin.Context) {
	f.requestKiroTokenCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok", "url": "https://example.test/kiro"})
}

func (f *fakeManagementRoutes) SetConfig(*sdkconfig.Config) {}

func (f *fakeManagementRoutes) SetAuthManager(*sdkcliproxyauth.Manager) {}

func (f *fakeManagementRoutes) SetLocalPassword(string) {}

func (f *fakeManagementRoutes) SetLogDirectory(string) {}

func (f *fakeManagementRoutes) SetPostAuthHook(sdkcliproxyauth.PostAuthHook) {}
