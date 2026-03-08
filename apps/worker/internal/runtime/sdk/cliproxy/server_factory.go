package cliproxy

import (
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/api"
	sdkaccess "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/access"
	coreauth "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy/auth"
	"github.com/kunish/wheel/apps/worker/internal/runtime/sdk/config"
)

func defaultServerFactory(cfg *config.Config, authManager *coreauth.Manager, accessManager *sdkaccess.Manager, configFilePath string, opts ...api.ServerOption) ServerRuntime {
	return api.NewServer(cfg, authManager, accessManager, configFilePath, opts...)
}

// DefaultServerFactory exposes the SDK default server factory for host-owned seams.
func DefaultServerFactory(cfg *config.Config, authManager *coreauth.Manager, accessManager *sdkaccess.Manager, configFilePath string, opts ...api.ServerOption) ServerRuntime {
	return defaultServerFactory(cfg, authManager, accessManager, configFilePath, opts...)
}
