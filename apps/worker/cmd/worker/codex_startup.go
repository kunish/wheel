package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"strings"

	"github.com/kunish/wheel/apps/worker/internal/codexruntime"
	"github.com/kunish/wheel/apps/worker/internal/config"
)

type codexRuntimeService interface {
	Start(ctx context.Context) <-chan error
}

func initEmbeddedCodexRuntime(
	ctx context.Context,
	cfg *config.Config,
	logger *log.Logger,
	factory func(*config.Config) (codexRuntimeService, error),
) (<-chan error, error) {
	if cfg == nil {
		return nil, nil
	}
	ensureEmbeddedCodexManagementKey(cfg)

	logger.Printf("[startup] embedded Codex runtime enabled (config=%s, strict=true)", codexruntime.ManagedConfigPath())

	svc, err := factory(cfg)
	if err != nil {
		return nil, err
	}
	if svc == nil {
		return nil, nil
	}

	return svc.Start(ctx), nil
}

func ensureEmbeddedCodexManagementKey(cfg *config.Config) {
	if cfg == nil || strings.TrimSpace(cfg.CodexRuntimeManagementKey) != "" {
		return
	}
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		cfg.CodexRuntimeManagementKey = "wheel-internal-management-key"
		return
	}
	cfg.CodexRuntimeManagementKey = hex.EncodeToString(buf)
}
