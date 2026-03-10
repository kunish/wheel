package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"strings"

	"github.com/kunish/wheel/apps/worker/internal/codexruntime"
	"github.com/kunish/wheel/apps/worker/internal/config"
)

type codexRuntimeService interface {
	Start(ctx context.Context) <-chan error
	Handler() http.Handler
}

// codexRuntimeResult holds the start result of the embedded Codex runtime.
type codexRuntimeResult struct {
	errCh   <-chan error
	handler http.Handler
}

func initEmbeddedCodexRuntime(
	ctx context.Context,
	cfg *config.Config,
	logger *log.Logger,
	factory func(*config.Config) (codexRuntimeService, error),
) (*codexRuntimeResult, error) {
	if cfg == nil {
		return nil, nil
	}
	ensureEmbeddedCodexManagementKey(cfg)

	logger.Printf("[startup] embedded Codex runtime enabled (config=%s, handler-only=true)", codexruntime.ManagedConfigPath())

	svc, err := factory(cfg)
	if err != nil {
		return nil, err
	}
	if svc == nil {
		return nil, nil
	}

	errCh := svc.Start(ctx)

	// Handler() blocks until the runtime is fully initialised.
	handler := svc.Handler()

	return &codexRuntimeResult{
		errCh:   errCh,
		handler: handler,
	}, nil
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
