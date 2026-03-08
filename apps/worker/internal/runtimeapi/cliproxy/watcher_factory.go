package cliproxy

import (
	"github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/corelib/config"
	sdkcliproxy "github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/sdk/cliproxy"
)

func defaultWatcherFactory(configPath, authDir string, reload func(*config.Config)) (*sdkcliproxy.WatcherWrapper, error) {
	return sdkcliproxy.DefaultWatcherFactory(configPath, authDir, reload)
}
