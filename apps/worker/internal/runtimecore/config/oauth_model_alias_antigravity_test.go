package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGitHubCopilotAliasesFromModels_GeneratesHyphenAliases(t *testing.T) {
	aliases := GitHubCopilotAliasesFromModels([]string{"claude-opus-4.6"})
	if len(aliases) != 1 {
		t.Fatalf("expected 1 alias, got %d", len(aliases))
	}
	if aliases[0].Name != "claude-opus-4.6" || aliases[0].Alias != "claude-opus-4-6" || !aliases[0].Fork {
		t.Fatalf("expected claude-opus-4.6 -> claude-opus-4-6 fork=true, got %+v", aliases[0])
	}
}

func TestLoadConfig_InjectsDefaultKiroAliases(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	configBody := "remote-management:\n  secret-key: local-secret\n"
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	kiroAliases := cfg.OAuthModelAlias["kiro"]
	if len(kiroAliases) == 0 {
		t.Fatal("expected default kiro aliases to be injected")
	}
}

func TestLoadConfig_InjectsDefaultGitHubCopilotAliases(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	configBody := "remote-management:\n  secret-key: local-secret\n"
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	githubCopilotAliases := cfg.OAuthModelAlias["github-copilot"]
	if len(githubCopilotAliases) == 0 {
		t.Fatal("expected default github-copilot aliases to be injected")
	}
}

func TestLoadConfig_InjectsDefaultAntigravityAliases(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	configBody := "remote-management:\n  secret-key: local-secret\n"
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	antigravityAliases := cfg.OAuthModelAlias["antigravity"]
	if len(antigravityAliases) == 0 {
		t.Fatal("expected default antigravity aliases to be injected")
	}
	aliasSet := make(map[string]string, len(antigravityAliases))
	for _, alias := range antigravityAliases {
		aliasSet[alias.Alias] = alias.Name
		if !alias.Fork {
			t.Fatalf("expected all default antigravity aliases to have fork=true, got fork=false for %q", alias.Alias)
		}
	}
	expectedAliases := map[string]string{
		"claude-opus-4-6":   "claude-opus-4-6-thinking",
		"claude-sonnet-4-6": "claude-sonnet-4-6-thinking",
	}
	if len(antigravityAliases) != len(expectedAliases) {
		t.Fatalf("expected %d default antigravity aliases, got %d", len(expectedAliases), len(antigravityAliases))
	}
	for alias, name := range expectedAliases {
		if got, ok := aliasSet[alias]; !ok {
			t.Fatalf("expected default antigravity alias %q to be present", alias)
		} else if got != name {
			t.Fatalf("expected antigravity alias %q to map to %q, got %q", alias, name, got)
		}
	}
}

func TestLoadConfig_PreservesUserDefinedAntigravityAliases(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	configBody := "remote-management:\n  secret-key: local-secret\noauth-model-alias:\n  antigravity:\n    - name: claude-opus-4-6-thinking\n      alias: my-opus\n      fork: true\n"
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	antigravityAliases := cfg.OAuthModelAlias["antigravity"]
	if len(antigravityAliases) != 1 {
		t.Fatalf("expected 1 user-configured antigravity alias, got %d", len(antigravityAliases))
	}
	if antigravityAliases[0].Alias != "my-opus" {
		t.Fatalf("expected user alias to be preserved, got %q", antigravityAliases[0].Alias)
	}
}

func TestLoadConfig_AntigravityExplicitNilOrEmptyDoesNotReinject(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.yaml")
		configBody := "remote-management:\n  secret-key: local-secret\noauth-model-alias:\n  antigravity: null\n"
		if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}

		cfg, err := LoadConfig(configPath)
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}

		if len(cfg.OAuthModelAlias["antigravity"]) != 0 {
			t.Fatalf("expected antigravity aliases to remain empty, got %d aliases", len(cfg.OAuthModelAlias["antigravity"]))
		}
		if _, exists := cfg.OAuthModelAlias["antigravity"]; !exists {
			t.Fatal("expected antigravity key to be preserved")
		}
	})

	t.Run("empty", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.yaml")
		configBody := "remote-management:\n  secret-key: local-secret\noauth-model-alias:\n  antigravity: []\n"
		if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}

		cfg, err := LoadConfig(configPath)
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}

		if len(cfg.OAuthModelAlias["antigravity"]) != 0 {
			t.Fatalf("expected antigravity aliases to remain empty, got %d aliases", len(cfg.OAuthModelAlias["antigravity"]))
		}
		if _, exists := cfg.OAuthModelAlias["antigravity"]; !exists {
			t.Fatal("expected antigravity key to be preserved")
		}
	})
}
