package mgmthandler

import (
	"github.com/gin-gonic/gin"
	sdkconfig "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/config"
	sdkcliproxy "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy"
	sdkcliproxyauth "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy/auth"
)

type ManagementHandler struct {
	cfg          *sdkconfig.Config
	inner        sdkcliproxy.ManagementHandlerRoutes
	postAuthHook sdkcliproxyauth.PostAuthHook
}

func NewManagementHandler(cfg *sdkconfig.Config, configFilePath string, authManager *sdkcliproxyauth.Manager) *ManagementHandler {
	return &ManagementHandler{cfg: cfg, inner: sdkcliproxy.DefaultManagementHandlerFactory(cfg, configFilePath, authManager)}
}

type oauthRouteRegistrarWithoutCallback interface {
	RegisterRoutesWithoutOAuthCallback(*gin.RouterGroup)
}

type oauthRouteRegistrarWithoutSessionRoutes interface {
	RegisterRoutesWithoutOAuthSessionRoutes(*gin.RouterGroup)
}

type geminiCLIAuthURLRequester interface {
	RequestGeminiCLIToken(*gin.Context)
}

type kiroAuthURLRequester interface {
	RequestKiroToken(*gin.Context)
}

func (h *ManagementHandler) Middleware() gin.HandlerFunc {
	if h == nil || h.inner == nil {
		return func(c *gin.Context) {
			c.Abort()
		}
	}
	return h.inner.Middleware()
}

func (h *ManagementHandler) RegisterRoutes(group *gin.RouterGroup) {
	if h == nil || h.inner == nil {
		return
	}
	if registrar, ok := h.inner.(oauthRouteRegistrarWithoutSessionRoutes); ok {
		registrar.RegisterRoutesWithoutOAuthSessionRoutes(group)
		group.GET("/anthropic-auth-url", h.RequestAnthropicToken)
		group.GET("/codex-auth-url", h.RequestCodexToken)
		group.GET("/github-auth-url", h.RequestGitHubToken)
		group.GET("/gemini-cli-auth-url", h.RequestGeminiCLIToken)
		group.GET("/antigravity-auth-url", h.RequestAntigravityToken)
		group.GET("/qwen-auth-url", h.RequestQwenToken)
		group.GET("/kilo-auth-url", h.RequestKiloToken)
		group.GET("/kimi-auth-url", h.RequestKimiToken)
		group.GET("/iflow-auth-url", h.RequestIFlowToken)
		group.POST("/iflow-auth-url", h.RequestIFlowCookieToken)
		group.GET("/kiro-auth-url", h.RequestKiroToken)
		group.POST("/oauth-callback", h.PostOAuthCallback)
		group.GET("/get-auth-status", h.GetAuthStatus)
		return
	}
	if registrar, ok := h.inner.(oauthRouteRegistrarWithoutCallback); ok {
		registrar.RegisterRoutesWithoutOAuthCallback(group)
		group.GET("/anthropic-auth-url", h.RequestAnthropicToken)
		group.GET("/codex-auth-url", h.RequestCodexToken)
		group.GET("/github-auth-url", h.RequestGitHubToken)
		group.GET("/gemini-cli-auth-url", h.RequestGeminiCLIToken)
		group.GET("/antigravity-auth-url", h.RequestAntigravityToken)
		group.GET("/qwen-auth-url", h.RequestQwenToken)
		group.GET("/kilo-auth-url", h.RequestKiloToken)
		group.GET("/kimi-auth-url", h.RequestKimiToken)
		group.GET("/iflow-auth-url", h.RequestIFlowToken)
		group.POST("/iflow-auth-url", h.RequestIFlowCookieToken)
		group.GET("/kiro-auth-url", h.RequestKiroToken)
		group.POST("/oauth-callback", h.PostOAuthCallback)
		group.GET("/get-auth-status", h.GetAuthStatus)
		return
	}
	h.inner.RegisterRoutes(group)
}

func (h *ManagementHandler) SetConfig(cfg *sdkconfig.Config) {
	h.cfg = cfg
	if h == nil || h.inner == nil {
		return
	}
	h.inner.SetConfig(cfg)
}

func (h *ManagementHandler) SetAuthManager(manager *sdkcliproxyauth.Manager) {
	if h == nil || h.inner == nil {
		return
	}
	h.inner.SetAuthManager(manager)
}

func (h *ManagementHandler) SetLocalPassword(password string) {
	if h == nil || h.inner == nil {
		return
	}
	h.inner.SetLocalPassword(password)
}

func (h *ManagementHandler) SetLogDirectory(dir string) {
	if h == nil || h.inner == nil {
		return
	}
	h.inner.SetLogDirectory(dir)
}

func (h *ManagementHandler) SetPostAuthHook(hook sdkcliproxyauth.PostAuthHook) {
	h.postAuthHook = hook
	if h == nil || h.inner == nil {
		return
	}
	h.inner.SetPostAuthHook(hook)
}
