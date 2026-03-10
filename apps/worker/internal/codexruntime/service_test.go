package codexruntime

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/kunish/wheel/apps/worker/internal/config"
	runtimeconfig "github.com/kunish/wheel/apps/worker/internal/runtimecore/config"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

func TestNewFromConfigManagedDefaults(t *testing.T) {
	cfg := &config.Config{}

	svc, err := NewFromConfig(cfg)
	if err != nil {
		t.Fatalf("expected nil error with managed defaults, got %v", err)
	}
	if svc == nil {
		t.Fatalf("expected non-nil service with managed defaults")
	}
}

func TestServiceStartPropagatesRunError(t *testing.T) {
	expectedErr := errors.New("run failed")
	svc := &Service{run: func(context.Context) error { return expectedErr }}

	errCh := svc.Start(context.Background())
	err := <-errCh
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestManagedAuthFilePathFlattensChannelName(t *testing.T) {
	path := managedAuthFilePath(&types.CodexAuthFile{
		ChannelID: 7,
		Name:      "first.json",
	})

	if got, want := filepath.Dir(path), ManagedAuthDir(); got != want {
		t.Fatalf("managed auth dir = %q, want %q", got, want)
	}
	if got, want := filepath.Base(path), "channel-7--first.json"; got != want {
		t.Fatalf("managed auth file name = %q, want %q", got, want)
	}
}

func TestNewFromConfigUsesOwnedRuntimeConfigLoader(t *testing.T) {
	originalEnsure := ensureManagedConfigFn
	originalLoad := loadRuntimeConfig
	originalBuild := buildRuntimeService
	t.Cleanup(func() {
		ensureManagedConfigFn = originalEnsure
		loadRuntimeConfig = originalLoad
		buildRuntimeService = originalBuild
	})

	var gotManagementKey string
	ensureManagedConfigFn = func(managementKey string) error {
		gotManagementKey = managementKey
		return nil
	}

	var gotConfigPath string
	loadRuntimeConfig = func(path string) (*runtimeconfig.Config, error) {
		gotConfigPath = path
		return &runtimeconfig.Config{AuthDir: "/tmp/runtime-auth"}, nil
	}

	var gotOwned *runtimeconfig.Config
	var gotBuildConfigPath string
	var gotBuildManagementKey string
	buildRuntimeService = func(cfg *runtimeconfig.Config, configPath string, managementKey string) (*Service, error) {
		gotOwned = cfg
		gotBuildConfigPath = configPath
		gotBuildManagementKey = managementKey
		return &Service{run: func(context.Context) error { return nil }}, nil
	}

	svc, err := NewFromConfig(&config.Config{CodexRuntimeManagementKey: "secret"})
	if err != nil {
		t.Fatalf("NewFromConfig() error = %v", err)
	}
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if gotConfigPath != ManagedConfigPath() {
		t.Fatalf("loadRuntimeConfig path = %q, want %q", gotConfigPath, ManagedConfigPath())
	}
	if gotOwned == nil {
		t.Fatal("expected owned config to be converted for builder")
	}
	if got, want := gotManagementKey, "secret"; got != want {
		t.Fatalf("ensureManagedConfig management key = %q, want %q", got, want)
	}
	if got, want := gotBuildConfigPath, ManagedConfigPath(); got != want {
		t.Fatalf("buildRuntimeService config path = %q, want %q", got, want)
	}
	if got, want := gotBuildManagementKey, "secret"; got != want {
		t.Fatalf("buildRuntimeService management key = %q, want %q", got, want)
	}
}

func TestNewFromConfigBuildsThroughOwnedRuntimeAPI(t *testing.T) {
	originalEnsure := ensureManagedConfigFn
	originalLoad := loadRuntimeConfig
	originalBuilder := newRuntimeBuilder
	t.Cleanup(func() {
		ensureManagedConfigFn = originalEnsure
		loadRuntimeConfig = originalLoad
		newRuntimeBuilder = originalBuilder
	})

	var gotManagementKey string
	ensureManagedConfigFn = func(managementKey string) error {
		gotManagementKey = managementKey
		return nil
	}

	loadedCfg := &runtimeconfig.Config{AuthDir: "/tmp/runtime-auth"}
	loadRuntimeConfig = func(path string) (*runtimeconfig.Config, error) {
		return loadedCfg, nil
	}

	fakeService := &fakeOwnedRuntimeService{}
	fakeBuilder := &fakeOwnedRuntimeBuilder{service: fakeService}
	newRuntimeBuilder = func() runtimeBuilder { return fakeBuilder }

	svc, err := NewFromConfig(&config.Config{CodexRuntimeManagementKey: "secret"})
	if err != nil {
		t.Fatalf("NewFromConfig() error = %v", err)
	}
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if got, want := fakeBuilder.cfg, loadedCfg; got != want {
		t.Fatalf("builder config = %p, want %p", got, want)
	}
	if got, want := fakeBuilder.configPath, ManagedConfigPath(); got != want {
		t.Fatalf("builder config path = %q, want %q", got, want)
	}
	if got, want := fakeBuilder.localManagementPassword, "secret"; got != want {
		t.Fatalf("builder local management password = %q, want %q", got, want)
	}
	if got, want := gotManagementKey, "secret"; got != want {
		t.Fatalf("ensureManagedConfig management key = %q, want %q", got, want)
	}

	errCh := svc.Start(context.Background())
	if err := <-errCh; err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if fakeService.runCalls != 1 {
		t.Fatalf("owned runtime Run() calls = %d, want 1", fakeService.runCalls)
	}
}

type fakeOwnedRuntimeBuilder struct {
	cfg                     *runtimeconfig.Config
	configPath              string
	localManagementPassword string
	service                 runtimeService
	err                     error
}

func (b *fakeOwnedRuntimeBuilder) WithConfig(cfg *runtimeconfig.Config) runtimeBuilder {
	b.cfg = cfg
	return b
}

func (b *fakeOwnedRuntimeBuilder) WithConfigPath(path string) runtimeBuilder {
	b.configPath = path
	return b
}

func (b *fakeOwnedRuntimeBuilder) WithLocalManagementPassword(password string) runtimeBuilder {
	b.localManagementPassword = password
	return b
}

func (b *fakeOwnedRuntimeBuilder) WithHandlerOnly() runtimeBuilder {
	return b
}

func (b *fakeOwnedRuntimeBuilder) Build() (runtimeService, error) {
	return b.service, b.err
}

type fakeOwnedRuntimeService struct {
	runCalls int
	err      error
}

func (s *fakeOwnedRuntimeService) Run(context.Context) error {
	s.runCalls++
	return s.err
}
