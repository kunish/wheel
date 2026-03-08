package cliproxy

import (
	"github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/corelib/api"
	sdkaccess "github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/sdk/access"
	coreauth "github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/sdk/cliproxy/auth"
	"github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/sdk/config"
)

func defaultServerFactory(cfg *config.Config, authManager *coreauth.Manager, accessManager *sdkaccess.Manager, configFilePath string, opts ...api.ServerOption) ServerRuntime {
	return api.NewServer(cfg, authManager, accessManager, configFilePath, opts...)
}

// DefaultServerFactory exposes the SDK default server factory for host-owned seams.
func DefaultServerFactory(cfg *config.Config, authManager *coreauth.Manager, accessManager *sdkaccess.Manager, configFilePath string, opts ...api.ServerOption) ServerRuntime {
	return defaultServerFactory(cfg, authManager, accessManager, configFilePath, opts...)
}
