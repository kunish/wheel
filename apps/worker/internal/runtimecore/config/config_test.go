package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigParsesManagedRuntimeFields(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	configBody := "host: 127.0.0.1\nport: 8317\nauth-dir: /tmp/wheel-auth\nremote-management:\n  secret-key: local-secret\n"
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config")
	}
	if cfg.Host != "127.0.0.1" {
		t.Fatalf("Host = %q, want %q", cfg.Host, "127.0.0.1")
	}
	if cfg.Port != 8317 {
		t.Fatalf("Port = %d, want %d", cfg.Port, 8317)
	}
	if cfg.AuthDir != "/tmp/wheel-auth" {
		t.Fatalf("AuthDir = %q, want %q", cfg.AuthDir, "/tmp/wheel-auth")
	}
	if cfg.RemoteManagement.SecretKey == "" {
		t.Fatal("expected remote management secret key to be loaded")
	}
	if cfg.RemoteManagement.SecretKey == "local-secret" {
		t.Fatal("expected secret key to be normalized by owned config loader")
	}
}
