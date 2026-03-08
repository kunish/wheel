package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	runtimeconfig "github.com/kunish/wheel/apps/worker/internal/runtimecore/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"gopkg.in/yaml.v3"
)

func TestReloadConfigOAuthAliasChangeForcesAuthRefresh(t *testing.T) {
	t.Parallel()

	authDir := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	oldCfg := &runtimeconfig.Config{
		AuthDir: authDir,
		OAuthModelAlias: map[string][]runtimeconfig.OAuthModelAlias{
			"github-copilot": {{Name: "claude-opus-4.6", Alias: "claude-opus-4-6", Fork: true}},
		},
	}
	newCfg := &runtimeconfig.Config{
		AuthDir: authDir,
		OAuthModelAlias: map[string][]runtimeconfig.OAuthModelAlias{
			"github-copilot": {{Name: "claude-sonnet-4.6", Alias: "claude-sonnet-4-6", Fork: true}},
		},
	}
	data, err := yaml.Marshal(newCfg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err = os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	queue := make(chan AuthUpdate, 1)
	w := &Watcher{
		configPath:     configPath,
		authDir:        authDir,
		lastAuthHashes: make(map[string]string),
	}
	w.SetConfig(oldCfg)
	w.SetAuthUpdateQueue(queue)
	defer w.stopDispatch()

	oldYAML, err := yaml.Marshal(oldCfg)
	if err != nil {
		t.Fatalf("Marshal old cfg error = %v", err)
	}
	w.oldConfigYaml = oldYAML

	prevSnapshot := snapshotCoreAuthsFunc
	t.Cleanup(func() { snapshotCoreAuthsFunc = prevSnapshot })
	snapshotCoreAuthsFunc = func(*runtimeconfig.Config, string) []*cliproxyauth.Auth {
		return []*cliproxyauth.Auth{{ID: "copilot-auth", Provider: "github-copilot"}}
	}

	w.clientsMutex.Lock()
	w.currentAuths = map[string]*cliproxyauth.Auth{
		"copilot-auth": {ID: "copilot-auth", Provider: "github-copilot"},
	}
	w.clientsMutex.Unlock()

	if ok := w.reloadConfig(); !ok {
		t.Fatal("expected reloadConfig to succeed")
	}

	select {
	case update := <-queue:
		if update.Action != AuthUpdateActionModify {
			t.Fatalf("update action = %q, want %q", update.Action, AuthUpdateActionModify)
		}
		if update.ID != "copilot-auth" {
			t.Fatalf("update id = %q, want copilot-auth", update.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for auth refresh")
	}
}
