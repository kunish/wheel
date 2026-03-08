package cliproxy

import (
	"github.com/router-for-me/CLIProxyAPI/v6/internal/api"
	sdkaccess "github.com/router-for-me/CLIProxyAPI/v6/sdk/access"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

func defaultServerFactory(cfg *config.Config, authManager *coreauth.Manager, accessManager *sdkaccess.Manager, configFilePath string, opts ...api.ServerOption) ServerRuntime {
	return api.NewServer(cfg, authManager, accessManager, configFilePath, opts...)
}

// DefaultServerFactory exposes the SDK default server factory for host-owned seams.
func DefaultServerFactory(cfg *config.Config, authManager *coreauth.Manager, accessManager *sdkaccess.Manager, configFilePath string, opts ...api.ServerOption) ServerRuntime {
	return defaultServerFactory(cfg, authManager, accessManager, configFilePath, opts...)
}
