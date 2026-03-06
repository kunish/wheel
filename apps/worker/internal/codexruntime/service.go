package codexruntime

import (
	"context"
	"fmt"

	"github.com/kunish/wheel/apps/worker/internal/config"
	codexsdk "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

// Service wraps the embedded Codex runtime.
type Service struct {
	run func(ctx context.Context) error
}

// NewFromConfig creates an embedded Codex runtime service from worker config.
// It uses Wheel-managed runtime files and config by default.
func NewFromConfig(cfg *config.Config) (*Service, error) {
	if cfg == nil {
		return nil, fmt.Errorf("nil config")
	}
	if err := EnsureManagedConfig(cfg.CodexRuntimeManagementKey); err != nil {
		return nil, err
	}

	configPath := ManagedConfigPath()
	cpaCfg, err := sdkconfig.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("load codex runtime config: %w", err)
	}

	inner, err := codexsdk.NewBuilder().
		WithConfig(cpaCfg).
		WithConfigPath(configPath).
		WithLocalManagementPassword(cfg.CodexRuntimeManagementKey).
		Build()
	if err != nil {
		return nil, fmt.Errorf("build codex runtime service: %w", err)
	}

	return &Service{run: inner.Run}, nil
}

// Start runs the embedded service in a goroutine and returns a channel
// carrying its terminal error.
func (s *Service) Start(ctx context.Context) <-chan error {
	errCh := make(chan error, 1)
	if s == nil || s.run == nil {
		errCh <- fmt.Errorf("codex runtime service is not initialized")
		close(errCh)
		return errCh
	}

	go func() {
		defer close(errCh)
		errCh <- s.run(ctx)
	}()

	return errCh
}
