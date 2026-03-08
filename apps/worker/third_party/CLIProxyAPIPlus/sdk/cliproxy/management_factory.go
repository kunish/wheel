package cliproxy

import (
	"github.com/router-for-me/CLIProxyAPI/v6/internal/api"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

// DefaultManagementHandlerFactory exposes the SDK default management handler factory for host-owned seams.
func DefaultManagementHandlerFactory(cfg *config.Config, configFilePath string, authManager *coreauth.Manager) ManagementHandlerRoutes {
	return api.DefaultManagementHandlerFactory(cfg, configFilePath, authManager)
}

// DefaultOAuthCallbackWriter exposes the SDK default OAuth callback persistence seam.
func DefaultOAuthCallbackWriter(authDir, provider, state, code, errorMessage string) (string, error) {
	return api.DefaultOAuthCallbackWriter(authDir, provider, state, code, errorMessage)
}
