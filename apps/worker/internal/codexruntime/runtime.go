package codexruntime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	channels, err := dal.ListChannels(ctx, db)
	if err != nil {
		return fmt.Errorf("list channels for materialization: %w", err)
	}
	items := make([]types.CodexAuthFile, 0)
	for _, ch := range channels {
		if ch.Type != types.OutboundCodex {
			continue
		}
		channelItems, err := dal.ListCodexAuthFiles(ctx, db, ch.ID)
		if err != nil {
			return fmt.Errorf("list codex auth files for channel %d: %w", ch.ID, err)
		}
		items = append(items, channelItems...)
	}
	return replaceAllManagedAuthFiles(items)
}

func MaterializeChannelAuthFiles(ctx context.Context, db *bun.DB, channelID int) error {
	if db == nil {
		return nil
	}
	if err := EnsureManagedConfig(""); err != nil {
		return err
	}
	items, err := dal.ListCodexAuthFiles(ctx, db, channelID)
	if err != nil {
		return fmt.Errorf("list codex auth files for channel %d: %w", channelID, err)
	}
	return replaceChannelManagedAuthFiles(channelID, items)
}

func MaterializeOneAuthFile(item *types.CodexAuthFile) error {
	if item == nil {
		return nil
	}
	if err := EnsureManagedConfig(""); err != nil {
		return err
	}
	return writeManagedAuthFile(item)
}

func writeManagedAuthFile(item *types.CodexAuthFile) error {
	if item == nil {
		return nil
	}
	path := ManagedAuthFilePath(item)
	return writeManagedAuthFilePath(path, []byte(item.Content))
}

func writeManagedAuthFilePath(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create managed auth parent dir: %w", err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return fmt.Errorf("write managed auth file: %w", err)
	}
	return nil
}

func replaceChannelManagedAuthFiles(channelID int, items []types.CodexAuthFile) (err error) {
	baseDir := ManagedAuthDir()
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return fmt.Errorf("create managed auth dir: %w", err)
	}

	prefix := fmt.Sprintf("channel-%d--", channelID)
	stagingDir, err := os.MkdirTemp(filepath.Dir(baseDir), prefix+"staging-")
	if err != nil {
		return fmt.Errorf("create staging auth dir: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(stagingDir); err == nil && removeErr != nil {
			err = fmt.Errorf("remove staging auth dir: %w", removeErr)
		}
	}()

	for i := range items {
		stagedPath := filepath.Join(stagingDir, ManagedAuthFileName(items[i].ChannelID, items[i].Name))
		if writeErr := writeManagedAuthFilePath(stagedPath, []byte(items[i].Content)); writeErr != nil {
			return writeErr
		}
	}

	existingPaths, err := listChannelManagedAuthPaths(baseDir, prefix)
	if err != nil {
		return err
	}
	backupDir, err := os.MkdirTemp(filepath.Dir(baseDir), prefix+"backup-")
	if err != nil {
		return fmt.Errorf("create backup auth dir: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(backupDir); err == nil && removeErr != nil {
			err = fmt.Errorf("remove backup auth dir: %w", removeErr)
		}
	}()

	backedUp := make([]string, 0, len(existingPaths))
	for _, oldPath := range existingPaths {
		backupPath := filepath.Join(backupDir, filepath.Base(oldPath))
		if renameErr := os.Rename(oldPath, backupPath); renameErr != nil {
			restoreErr := restoreBackedUpManagedAuthFiles(baseDir, backupDir, backedUp)
			if restoreErr != nil {
				return fmt.Errorf("backup managed auth files: %w (restore failed: %v)", renameErr, restoreErr)
			}
			return fmt.Errorf("backup managed auth files: %w", renameErr)
		}
		backedUp = append(backedUp, filepath.Base(oldPath))
	}

	activated := make([]string, 0, len(items))
	for i := range items {
		name := ManagedAuthFileName(items[i].ChannelID, items[i].Name)
		stagedPath := filepath.Join(stagingDir, name)
		livePath := filepath.Join(baseDir, name)
		if renameErr := os.Rename(stagedPath, livePath); renameErr != nil {
			rollbackErr := rollbackActivatedManagedAuthFiles(baseDir, backupDir, activated, backedUp)
			if rollbackErr != nil {
				return fmt.Errorf("activate managed auth files: %w (rollback failed: %v)", renameErr, rollbackErr)
			}
			return fmt.Errorf("activate managed auth files: %w", renameErr)
		}
		activated = append(activated, name)
	}

	return nil
}

func replaceAllManagedAuthFiles(items []types.CodexAuthFile) (err error) {
	baseDir := ManagedAuthDir()
	parentDir := filepath.Dir(baseDir)
	if err := os.MkdirAll(parentDir, 0o700); err != nil {
		return fmt.Errorf("create managed auth parent dir: %w", err)
	}

	stagingDir, err := os.MkdirTemp(parentDir, filepath.Base(baseDir)+"-staging-")
	if err != nil {
		return fmt.Errorf("create staging auth dir: %w", err)
	}
	defer func() {
		if stagingDir == "" {
			return
		}
		if removeErr := os.RemoveAll(stagingDir); err == nil && removeErr != nil {
			err = fmt.Errorf("remove staging auth dir: %w", removeErr)
		}
	}()

	for i := range items {
		stagedPath := filepath.Join(stagingDir, ManagedAuthFileName(items[i].ChannelID, items[i].Name))
		if writeErr := writeManagedAuthFilePath(stagedPath, []byte(items[i].Content)); writeErr != nil {
			return writeErr
		}
	}

	backupDir, err := os.MkdirTemp(parentDir, filepath.Base(baseDir)+"-backup-")
	if err != nil {
		return fmt.Errorf("create backup auth dir: %w", err)
	}
	if removeErr := os.Remove(backupDir); removeErr != nil {
		return fmt.Errorf("prepare backup auth dir: %w", removeErr)
	}
	defer func() {
		if removeErr := os.RemoveAll(backupDir); err == nil && removeErr != nil {
			err = fmt.Errorf("remove backup auth dir: %w", removeErr)
		}
	}()

	if renameErr := os.Rename(baseDir, backupDir); renameErr != nil {
		if !os.IsNotExist(renameErr) {
			return fmt.Errorf("backup managed auth dir: %w", renameErr)
		}
		backupDir = ""
	}
	if renameErr := os.Rename(stagingDir, baseDir); renameErr != nil {
		if backupDir != "" {
			if restoreErr := os.Rename(backupDir, baseDir); restoreErr != nil {
				return fmt.Errorf("activate managed auth dir: %w (restore failed: %v)", renameErr, restoreErr)
			}
		}
		return fmt.Errorf("activate managed auth dir: %w", renameErr)
	}

	stagingDir = ""
	return nil
}

func listChannelManagedAuthPaths(baseDir string, prefix string) ([]string, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, fmt.Errorf("read managed auth dir: %w", err)
	}
	paths := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		paths = append(paths, filepath.Join(baseDir, entry.Name()))
	}
	return paths, nil
}

func restoreBackedUpManagedAuthFiles(baseDir string, backupDir string, names []string) error {
	for _, name := range names {
		if err := os.Rename(filepath.Join(backupDir, name), filepath.Join(baseDir, name)); err != nil {
			return err
		}
	}
	return nil
}

func rollbackActivatedManagedAuthFiles(baseDir string, backupDir string, activated []string, backedUp []string) error {
	for _, name := range activated {
		if err := os.Remove(filepath.Join(baseDir, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return restoreBackedUpManagedAuthFiles(baseDir, backupDir, backedUp)
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
