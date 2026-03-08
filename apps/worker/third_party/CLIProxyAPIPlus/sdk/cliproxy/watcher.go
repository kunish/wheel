package cliproxy

import (
	"context"

	"github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/corelib/watcher"
	coreauth "github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/sdk/cliproxy/auth"
	"github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/sdk/config"
)

func defaultWatcherFactory(configPath, authDir string, reload func(*config.Config)) (*WatcherWrapper, error) {
	w, err := watcher.NewWatcher(configPath, authDir, reload)
	if err != nil {
		return nil, err
	}

	return &WatcherWrapper{
		start: func(ctx context.Context) error {
			return w.Start(ctx)
		},
		stop: func() error {
			return w.Stop()
		},
		setConfig: func(cfg *config.Config) {
			w.SetConfig(cfg)
		},
		snapshotAuths: func() []*coreauth.Auth { return w.SnapshotCoreAuths() },
		setUpdateQueue: func(queue chan<- watcher.AuthUpdate) {
			w.SetAuthUpdateQueue(queue)
		},
		dispatchRuntimeUpdate: func(update watcher.AuthUpdate) bool {
			return w.DispatchRuntimeAuthUpdate(update)
		},
		notifyTokenRefreshed: func(tokenID, accessToken, refreshToken, expiresAt string) {
			w.NotifyTokenRefreshed(tokenID, accessToken, refreshToken, expiresAt)
		},
	}, nil
}

// DefaultWatcherFactory exposes the SDK default watcher factory for host-owned seams.
func DefaultWatcherFactory(configPath, authDir string, reload func(*config.Config)) (*WatcherWrapper, error) {
	return defaultWatcherFactory(configPath, authDir, reload)
}
