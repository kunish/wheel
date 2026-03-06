package config

import "testing"

func TestLoadCodexRuntimeConfigDefaults(t *testing.T) {
	cfg := Load()

	if cfg.CodexRuntimeManagementURL != "http://127.0.0.1:8317" {
		t.Fatalf("expected internal CodexRuntimeManagementURL default to be http://127.0.0.1:8317, got %q", cfg.CodexRuntimeManagementURL)
	}
	if cfg.CodexRuntimeManagementKey != "" {
		t.Fatalf("expected CodexRuntimeManagementKey default to be empty")
	}
}
