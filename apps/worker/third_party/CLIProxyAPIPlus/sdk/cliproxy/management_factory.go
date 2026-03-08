package cliproxy

import (
	"github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/corelib/api"
	coreauth "github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/sdk/cliproxy/auth"
	"github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/sdk/config"
)

// DefaultManagementHandlerFactory exposes the SDK default management handler factory for host-owned seams.
func DefaultManagementHandlerFactory(cfg *config.Config, configFilePath string, authManager *coreauth.Manager) ManagementHandlerRoutes {
	return api.DefaultManagementHandlerFactory(cfg, configFilePath, authManager)
}

// DefaultOAuthCallbackWriter exposes the SDK default OAuth callback persistence seam.
func DefaultOAuthCallbackWriter(authDir, provider, state, code, errorMessage string) (string, error) {
	return api.DefaultOAuthCallbackWriter(authDir, provider, state, code, errorMessage)
}
