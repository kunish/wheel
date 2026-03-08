package cliproxy

import (
	sdkcliproxy "github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/sdk/cliproxy"
	"github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/sdk/config"
)

func defaultWatcherFactory(configPath, authDir string, reload func(*config.Config)) (*sdkcliproxy.WatcherWrapper, error) {
	return sdkcliproxy.DefaultWatcherFactory(configPath, authDir, reload)
}
