package codexruntime

import (
	"context"
	"fmt"

	"github.com/kunish/wheel/apps/worker/internal/config"
	runtimecliproxy "github.com/kunish/wheel/apps/worker/internal/runtimeapi/cliproxy"
	runtimeconfig "github.com/kunish/wheel/apps/worker/internal/runtimecore/config"
)

// Service wraps the embedded Codex runtime.
type Service struct {
	run func(ctx context.Context) error
}

var ensureManagedConfig = EnsureManagedConfig

var loadRuntimeConfig = runtimeconfig.LoadConfig

type runtimeService interface {
	Run(context.Context) error
}

type runtimeBuilder interface {
	WithConfig(*runtimeconfig.Config) runtimeBuilder
	WithConfigPath(string) runtimeBuilder
	WithLocalManagementPassword(string) runtimeBuilder
	Build() (runtimeService, error)
}

type runtimeBuilderAdapter struct {
	inner *runtimecliproxy.Builder
}

func (a *runtimeBuilderAdapter) WithConfig(cfg *runtimeconfig.Config) runtimeBuilder {
	a.inner = a.inner.WithConfig(cfg)
	return a
}

func (a *runtimeBuilderAdapter) WithConfigPath(path string) runtimeBuilder {
	a.inner = a.inner.WithConfigPath(path)
	return a
}

func (a *runtimeBuilderAdapter) WithLocalManagementPassword(password string) runtimeBuilder {
	a.inner = a.inner.WithLocalManagementPassword(password)
	return a
}

func (a *runtimeBuilderAdapter) Build() (runtimeService, error) {
	return a.inner.Build()
}

var newRuntimeBuilder = func() runtimeBuilder {
	return &runtimeBuilderAdapter{inner: runtimecliproxy.NewBuilder()}
}

var buildRuntimeService = func(cfg *runtimeconfig.Config, configPath string, managementKey string) (func(context.Context) error, error) {
	inner, err := newRuntimeBuilder().
		WithConfig(cfg).
		WithConfigPath(configPath).
		WithLocalManagementPassword(managementKey).
		Build()
	if err != nil {
		return nil, err
	}

	return inner.Run, nil
}

// NewFromConfig creates an embedded Codex runtime service from worker config.
// It uses Wheel-managed runtime files and config by default.
func NewFromConfig(cfg *config.Config) (*Service, error) {
	if cfg == nil {
		return nil, fmt.Errorf("nil config")
	}
	if err := ensureManagedConfig(cfg.CodexRuntimeManagementKey); err != nil {
		return nil, err
	}

	configPath := ManagedConfigPath()
	ownedCfg, err := loadRuntimeConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("load codex runtime config: %w", err)
	}

	run, err := buildRuntimeService(ownedCfg, configPath, cfg.CodexRuntimeManagementKey)
	if err != nil {
		return nil, fmt.Errorf("build codex runtime service: %w", err)
	}

	return &Service{run: run}, nil
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
