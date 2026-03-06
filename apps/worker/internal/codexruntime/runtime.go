package codexruntime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/types"
	"github.com/uptrace/bun"
)

const managedPort = 8317

func defaultManagedBaseDir() string {
	if cacheDir, err := os.UserCacheDir(); err == nil && cacheDir != "" {
		return filepath.Join(cacheDir, "wheel", "codex-runtime")
	}
	return filepath.Join(os.TempDir(), "wheel", "codex-runtime")
}

func ManagedConfigPath() string {
	return filepath.Join(defaultManagedBaseDir(), "config.yaml")
}

func ManagedAuthDir() string {
	return filepath.Join(filepath.Dir(ManagedConfigPath()), "wheel-auth")
}

func ManagedAuthFileName(channelID int, name string) string {
	return fmt.Sprintf("channel-%d--%s", channelID, filepath.Base(name))
}

func ManagedAuthFilePath(item *types.CodexAuthFile) string {
	return filepath.Join(ManagedAuthDir(), ManagedAuthFileName(item.ChannelID, item.Name))
}

func ManagementBaseURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", managedPort)
}

func EnsureManagedConfig(managementKey string) error {
	configPath := ManagedConfigPath()
	authDir := ManagedAuthDir()
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return fmt.Errorf("create codex runtime config dir: %w", err)
	}
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		return fmt.Errorf("create managed auth dir: %w", err)
	}
	content := fmt.Sprintf("host: 127.0.0.1\nport: %d\nauth-dir: %s\n", managedPort, authDir)
	if managementKey != "" {
		content += fmt.Sprintf("remote-management:\n  secret-key: %s\n", managementKey)
	}
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write managed codex runtime config: %w", err)
	}
	return nil
}

func MaterializeAuthFiles(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return nil
	}
	if err := EnsureManagedConfig(""); err != nil {
		return err
	}
	baseDir := ManagedAuthDir()
	if err := os.RemoveAll(baseDir); err != nil {
		return fmt.Errorf("reset managed auth dir: %w", err)
	}
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return fmt.Errorf("recreate managed auth dir: %w", err)
	}
	channels, err := dal.ListChannels(ctx, db)
	if err != nil {
		return fmt.Errorf("list channels for materialization: %w", err)
	}
	for _, ch := range channels {
		if ch.Type != types.OutboundCodex {
			continue
		}
		items, err := dal.ListCodexAuthFiles(ctx, db, ch.ID)
		if err != nil {
			return fmt.Errorf("list codex auth files for channel %d: %w", ch.ID, err)
		}
		for i := range items {
			if err := MaterializeOneAuthFile(&items[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func MaterializeOneAuthFile(item *types.CodexAuthFile) error {
	if item == nil {
		return nil
	}
	if err := EnsureManagedConfig(""); err != nil {
		return err
	}
	path := ManagedAuthFilePath(item)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create managed auth parent dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(item.Content), 0o600); err != nil {
		return fmt.Errorf("write managed auth file: %w", err)
	}
	return nil
}

func RemoveOneAuthFile(item *types.CodexAuthFile) error {
	if item == nil {
		return nil
	}
	path := ManagedAuthFilePath(item)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove managed auth file: %w", err)
	}
	return nil
}
