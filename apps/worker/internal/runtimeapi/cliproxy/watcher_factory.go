package cliproxy

import (
	sdkcliproxy "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

func defaultWatcherFactory(configPath, authDir string, reload func(*config.Config)) (*sdkcliproxy.WatcherWrapper, error) {
	return sdkcliproxy.DefaultWatcherFactory(configPath, authDir, reload)
}
