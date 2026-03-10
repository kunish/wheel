package config

import "testing"

func TestLoadCodexRuntimeConfigDefaults(t *testing.T) {
	cfg := Load()

	if cfg.CodexRuntimeManagementURL != "http://codex-internal" {
		t.Fatalf("expected internal CodexRuntimeManagementURL default to be http://codex-internal, got %q", cfg.CodexRuntimeManagementURL)
	}
	if cfg.CodexRuntimeManagementKey != "" {
		t.Fatalf("expected CodexRuntimeManagementKey default to be empty")
	}
}
