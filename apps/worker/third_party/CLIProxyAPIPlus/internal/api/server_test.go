package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	gin "github.com/gin-gonic/gin"
	proxyconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	internallogging "github.com/router-for-me/CLIProxyAPI/v6/internal/logging"
	sdkaccess "github.com/router-for-me/CLIProxyAPI/v6/sdk/access"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/api/handlers"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

type stubOpenAIHandler struct{}

func (stubOpenAIHandler) OpenAIModels(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"handler": "chat-models"})
}

func (stubOpenAIHandler) ChatCompletions(c *gin.Context) {
	c.JSON(http.StatusTeapot, gin.H{"handler": "chat"})
}

func (stubOpenAIHandler) Completions(c *gin.Context) {
	c.JSON(http.StatusCreated, gin.H{"handler": "completions"})
}

type stubOpenAIResponsesHandler struct{}

func (stubOpenAIResponsesHandler) OpenAIResponsesModels(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"handler": "responses-models"})
}

func (stubOpenAIResponsesHandler) Responses(c *gin.Context) {
	c.JSON(http.StatusAccepted, gin.H{"handler": "responses"})
}

func (stubOpenAIResponsesHandler) Compact(c *gin.Context) {
	c.JSON(http.StatusNonAuthoritativeInfo, gin.H{"handler": "compact"})
}

func (stubOpenAIResponsesHandler) ResponsesWebsocket(c *gin.Context) {
	c.JSON(http.StatusSwitchingProtocols, gin.H{"handler": "responses-websocket"})
}

type stubManagementHandler struct{}

func (stubManagementHandler) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

func (stubManagementHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/config", func(c *gin.Context) {
		c.JSON(http.StatusAccepted, gin.H{"handler": "management"})
	})
}

func (stubManagementHandler) SetConfig(*proxyconfig.Config) {}

func (stubManagementHandler) SetAuthManager(*auth.Manager) {}

func (stubManagementHandler) SetLocalPassword(string) {}

func (stubManagementHandler) SetLogDirectory(string) {}

func (stubManagementHandler) SetPostAuthHook(auth.PostAuthHook) {}

func readManagerOAuthAliasReverse(t *testing.T, mgr *auth.Manager) map[string]map[string]string {
	t.Helper()

	field := reflect.ValueOf(mgr).Elem().FieldByName("oauthModelAlias")
	if !field.IsValid() {
		t.Fatal("oauthModelAlias field not found")
	}
	value := (*atomic.Value)(unsafe.Pointer(field.UnsafeAddr())).Load()
	if value == nil {
		return nil
	}

	tableValue := reflect.ValueOf(value)
	if tableValue.Kind() == reflect.Ptr {
		tableValue = tableValue.Elem()
	}
	reverseField := tableValue.FieldByName("reverse")
	if !reverseField.IsValid() || reverseField.IsNil() {
		return nil
	}

	return reflect.NewAt(reverseField.Type(), unsafe.Pointer(reverseField.UnsafeAddr())).Elem().Interface().(map[string]map[string]string)
}

func TestNewServer_PropagatesOAuthModelAliasesToAuthManager(t *testing.T) {
	t.Helper()

	cfg := &proxyconfig.Config{
		SDKConfig: sdkconfig.SDKConfig{APIKeys: []string{"test-key"}},
		OAuthModelAlias: map[string][]proxyconfig.OAuthModelAlias{
			"github-copilot": {{Name: "claude-opus-4.6", Alias: "claude-opus-4-6", Fork: true}},
		},
	}
	authManager := auth.NewManager(nil, nil, nil)
	accessManager := sdkaccess.NewManager()

	_ = NewServer(cfg, authManager, accessManager, filepath.Join(t.TempDir(), "config.yaml"))

	reverse := readManagerOAuthAliasReverse(t, authManager)
	if got := reverse["github-copilot"]["claude-opus-4-6"]; got != "claude-opus-4.6" {
		t.Fatalf("github-copilot alias = %q, want %q", got, "claude-opus-4.6")
	}
}

func TestServerUpdateClients_RefreshesOAuthModelAliasesInAuthManager(t *testing.T) {
	server := newTestServer(t)

	updated := *server.cfg
	updated.OAuthModelAlias = map[string][]proxyconfig.OAuthModelAlias{
		"github-copilot": {{Name: "claude-opus-4.6", Alias: "claude-opus-4-6", Fork: true}},
	}

	server.UpdateClients(&updated)

	reverse := readManagerOAuthAliasReverse(t, server.handlers.AuthManager)
	if got := reverse["github-copilot"]["claude-opus-4-6"]; got != "claude-opus-4.6" {
		t.Fatalf("github-copilot alias after update = %q, want %q", got, "claude-opus-4.6")
	}
}

func TestNewServer_UsesInjectedOpenAIHandlerFactories(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	authDir := filepath.Join(tmpDir, "auth")
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		t.Fatalf("failed to create auth dir: %v", err)
	}

	cfg := &proxyconfig.Config{
		SDKConfig:              sdkconfig.SDKConfig{APIKeys: []string{"test-key"}},
		Port:                   0,
		AuthDir:                authDir,
		Debug:                  true,
		LoggingToFile:          false,
		UsageStatisticsEnabled: false,
	}

	authManager := auth.NewManager(nil, nil, nil)
	accessManager := sdkaccess.NewManager()
	server := NewServer(
		cfg,
		authManager,
		accessManager,
		filepath.Join(tmpDir, "config.yaml"),
		WithOpenAIHandlerFactory(func(*handlers.BaseAPIHandler) OpenAIHandlerRoutes {
			return stubOpenAIHandler{}
		}),
		WithOpenAIResponsesHandlerFactory(func(*handlers.BaseAPIHandler) OpenAIResponsesHandlerRoutes {
			return stubOpenAIResponsesHandler{}
		}),
	)

	tests := []struct {
		name           string
		method         string
		path           string
		wantStatus     int
		wantBodySubstr string
	}{
		{name: "models", method: http.MethodGet, path: "/v1/models", wantStatus: http.StatusOK, wantBodySubstr: `"handler":"chat-models"`},
		{name: "chat", method: http.MethodPost, path: "/v1/chat/completions", wantStatus: http.StatusTeapot, wantBodySubstr: `"handler":"chat"`},
		{name: "completions", method: http.MethodPost, path: "/v1/completions", wantStatus: http.StatusCreated, wantBodySubstr: `"handler":"completions"`},
		{name: "responses", method: http.MethodPost, path: "/v1/responses", wantStatus: http.StatusAccepted, wantBodySubstr: `"handler":"responses"`},
		{name: "compact", method: http.MethodPost, path: "/v1/responses/compact", wantStatus: http.StatusNonAuthoritativeInfo, wantBodySubstr: `"handler":"compact"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"model":"test"}`))
			req.Header.Set("Authorization", "Bearer test-key")
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			server.engine.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d body=%s", rr.Code, tt.wantStatus, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), tt.wantBodySubstr) {
				t.Fatalf("body = %s, want substring %s", rr.Body.String(), tt.wantBodySubstr)
			}
		})
	}
}

func TestNewServer_UsesInjectedManagementHandlerFactory(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	authDir := filepath.Join(tmpDir, "auth")
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		t.Fatalf("failed to create auth dir: %v", err)
	}

	cfg := &proxyconfig.Config{
		SDKConfig:              sdkconfig.SDKConfig{APIKeys: []string{"test-key"}},
		Port:                   0,
		AuthDir:                authDir,
		Debug:                  true,
		LoggingToFile:          false,
		UsageStatisticsEnabled: false,
		RemoteManagement: proxyconfig.RemoteManagement{
			SecretKey: "$2a$10$abcdefghijklmnopqrstuuN1H8A0b7A0J8sM7l6L8b7lW5x4a3b2",
		},
	}

	authManager := auth.NewManager(nil, nil, nil)
	accessManager := sdkaccess.NewManager()
	server := NewServer(
		cfg,
		authManager,
		accessManager,
		filepath.Join(tmpDir, "config.yaml"),
		WithManagementHandlerFactory(func(*proxyconfig.Config, string, *auth.Manager) ManagementHandlerRoutes {
			return stubManagementHandler{}
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/v0/management/config", nil)
	rr := httptest.NewRecorder()

	server.engine.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusAccepted, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"handler":"management"`) {
		t.Fatalf("body = %s, want injected management handler response", rr.Body.String())
	}
}

func TestNewServer_UsesInjectedOAuthCallbackWriter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	authDir := filepath.Join(tmpDir, "auth")
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		t.Fatalf("failed to create auth dir: %v", err)
	}

	cfg := &proxyconfig.Config{
		SDKConfig:              sdkconfig.SDKConfig{APIKeys: []string{"test-key"}},
		Port:                   0,
		AuthDir:                authDir,
		Debug:                  true,
		LoggingToFile:          false,
		UsageStatisticsEnabled: false,
	}

	authManager := auth.NewManager(nil, nil, nil)
	accessManager := sdkaccess.NewManager()
	called := false
	var gotAuthDir string
	var gotProvider string
	var gotState string
	var gotCode string
	var gotError string

	server := NewServer(
		cfg,
		authManager,
		accessManager,
		filepath.Join(tmpDir, "config.yaml"),
		WithOAuthCallbackWriter(func(authDir, provider, state, code, errorMessage string) (string, error) {
			called = true
			gotAuthDir = authDir
			gotProvider = provider
			gotState = state
			gotCode = code
			gotError = errorMessage
			return filepath.Join(authDir, "callback.oauth"), nil
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/codex/callback?state=test-state&code=test-code&error_description=test-error", nil)
	rr := httptest.NewRecorder()

	server.engine.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !called {
		t.Fatal("expected injected oauth callback writer to be called")
	}
	if gotAuthDir != authDir || gotProvider != "codex" || gotState != "test-state" || gotCode != "test-code" || gotError != "test-error" {
		t.Fatalf("writer args = (%q, %q, %q, %q, %q)", gotAuthDir, gotProvider, gotState, gotCode, gotError)
	}
	if !strings.Contains(rr.Body.String(), "Authentication successful") {
		t.Fatalf("body = %s, want success HTML", rr.Body.String())
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	authDir := filepath.Join(tmpDir, "auth")
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		t.Fatalf("failed to create auth dir: %v", err)
	}

	cfg := &proxyconfig.Config{
		SDKConfig: sdkconfig.SDKConfig{
			APIKeys: []string{"test-key"},
		},
		Port:                   0,
		AuthDir:                authDir,
		Debug:                  true,
		LoggingToFile:          false,
		UsageStatisticsEnabled: false,
	}

	authManager := auth.NewManager(nil, nil, nil)
	accessManager := sdkaccess.NewManager()

	configPath := filepath.Join(tmpDir, "config.yaml")
	return NewServer(cfg, authManager, accessManager, configPath)
}

func TestAmpProviderModelRoutes(t *testing.T) {
	testCases := []struct {
		name         string
		path         string
		wantStatus   int
		wantContains string
	}{
		{
			name:         "openai root models",
			path:         "/api/provider/openai/models",
			wantStatus:   http.StatusOK,
			wantContains: `"object":"list"`,
		},
		{
			name:         "groq root models",
			path:         "/api/provider/groq/models",
			wantStatus:   http.StatusOK,
			wantContains: `"object":"list"`,
		},
		{
			name:         "openai models",
			path:         "/api/provider/openai/v1/models",
			wantStatus:   http.StatusOK,
			wantContains: `"object":"list"`,
		},
		{
			name:         "anthropic models",
			path:         "/api/provider/anthropic/v1/models",
			wantStatus:   http.StatusOK,
			wantContains: `"data"`,
		},
		{
			name:         "google models v1",
			path:         "/api/provider/google/v1/models",
			wantStatus:   http.StatusOK,
			wantContains: `"models"`,
		},
		{
			name:         "google models v1beta",
			path:         "/api/provider/google/v1beta/models",
			wantStatus:   http.StatusOK,
			wantContains: `"models"`,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			server := newTestServer(t)

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Header.Set("Authorization", "Bearer test-key")

			rr := httptest.NewRecorder()
			server.engine.ServeHTTP(rr, req)

			if rr.Code != tc.wantStatus {
				t.Fatalf("unexpected status code for %s: got %d want %d; body=%s", tc.path, rr.Code, tc.wantStatus, rr.Body.String())
			}
			if body := rr.Body.String(); !strings.Contains(body, tc.wantContains) {
				t.Fatalf("response body for %s missing %q: %s", tc.path, tc.wantContains, body)
			}
		})
	}
}

func TestDefaultRequestLoggerFactory_UsesResolvedLogDirectory(t *testing.T) {
	t.Setenv("WRITABLE_PATH", "")
	t.Setenv("writable_path", "")

	originalWD, errGetwd := os.Getwd()
	if errGetwd != nil {
		t.Fatalf("failed to get current working directory: %v", errGetwd)
	}

	tmpDir := t.TempDir()
	if errChdir := os.Chdir(tmpDir); errChdir != nil {
		t.Fatalf("failed to switch working directory: %v", errChdir)
	}
	defer func() {
		if errChdirBack := os.Chdir(originalWD); errChdirBack != nil {
			t.Fatalf("failed to restore working directory: %v", errChdirBack)
		}
	}()

	// Force ResolveLogDirectory to fallback to auth-dir/logs by making ./logs not a writable directory.
	if errWriteFile := os.WriteFile(filepath.Join(tmpDir, "logs"), []byte("not-a-directory"), 0o644); errWriteFile != nil {
		t.Fatalf("failed to create blocking logs file: %v", errWriteFile)
	}

	configDir := filepath.Join(tmpDir, "config")
	if errMkdirConfig := os.MkdirAll(configDir, 0o755); errMkdirConfig != nil {
		t.Fatalf("failed to create config dir: %v", errMkdirConfig)
	}
	configPath := filepath.Join(configDir, "config.yaml")

	authDir := filepath.Join(tmpDir, "auth")
	if errMkdirAuth := os.MkdirAll(authDir, 0o700); errMkdirAuth != nil {
		t.Fatalf("failed to create auth dir: %v", errMkdirAuth)
	}

	cfg := &proxyconfig.Config{
		SDKConfig: proxyconfig.SDKConfig{
			RequestLog: false,
		},
		AuthDir:           authDir,
		ErrorLogsMaxFiles: 10,
	}

	logger := defaultRequestLoggerFactory(cfg, configPath)
	fileLogger, ok := logger.(*internallogging.FileRequestLogger)
	if !ok {
		t.Fatalf("expected *FileRequestLogger, got %T", logger)
	}

	errLog := fileLogger.LogRequestWithOptions(
		"/v1/chat/completions",
		http.MethodPost,
		map[string][]string{"Content-Type": []string{"application/json"}},
		[]byte(`{"input":"hello"}`),
		http.StatusBadGateway,
		map[string][]string{"Content-Type": []string{"application/json"}},
		[]byte(`{"error":"upstream failure"}`),
		nil,
		nil,
		nil,
		true,
		"issue-1711",
		time.Now(),
		time.Now(),
	)
	if errLog != nil {
		t.Fatalf("failed to write forced error request log: %v", errLog)
	}

	authLogsDir := filepath.Join(authDir, "logs")
	authEntries, errReadAuthDir := os.ReadDir(authLogsDir)
	if errReadAuthDir != nil {
		t.Fatalf("failed to read auth logs dir %s: %v", authLogsDir, errReadAuthDir)
	}
	foundErrorLogInAuthDir := false
	for _, entry := range authEntries {
		if strings.HasPrefix(entry.Name(), "error-") && strings.HasSuffix(entry.Name(), ".log") {
			foundErrorLogInAuthDir = true
			break
		}
	}
	if !foundErrorLogInAuthDir {
		t.Fatalf("expected forced error log in auth fallback dir %s, got entries: %+v", authLogsDir, authEntries)
	}

	configLogsDir := filepath.Join(configDir, "logs")
	configEntries, errReadConfigDir := os.ReadDir(configLogsDir)
	if errReadConfigDir != nil && !os.IsNotExist(errReadConfigDir) {
		t.Fatalf("failed to inspect config logs dir %s: %v", configLogsDir, errReadConfigDir)
	}
	for _, entry := range configEntries {
		if strings.HasPrefix(entry.Name(), "error-") && strings.HasSuffix(entry.Name(), ".log") {
			t.Fatalf("unexpected forced error log in config dir %s", configLogsDir)
		}
	}
}
