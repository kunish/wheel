package runtimectrl

import (
	"github.com/gin-gonic/gin"
	sdkcliproxy "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy"
	sdkcliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

type ManagementHandler struct {
	inner sdkcliproxy.ManagementHandlerRoutes
}

func NewManagementHandler(cfg *sdkconfig.Config, configFilePath string, authManager *sdkcliproxyauth.Manager) *ManagementHandler {
	return &ManagementHandler{inner: sdkcliproxy.DefaultManagementHandlerFactory(cfg, configFilePath, authManager)}
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
	h.inner.RegisterRoutes(group)
}

func (h *ManagementHandler) SetConfig(cfg *sdkconfig.Config) {
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
	if h == nil || h.inner == nil {
		return
	}
	h.inner.SetPostAuthHook(hook)
}

func WriteOAuthCallback(authDir, provider, state, code, errorMessage string) (string, error) {
	return sdkcliproxy.DefaultOAuthCallbackWriter(authDir, provider, state, code, errorMessage)
}
