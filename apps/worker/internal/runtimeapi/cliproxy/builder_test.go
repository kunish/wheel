package cliproxy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	sdkconfig "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/config"
	sdkaccess "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/access"
	sdkauth "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/auth"
	sdkcliproxy "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy"
	sdkcliproxyauth "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy/auth"
	runtimeconfig "github.com/kunish/wheel/apps/worker/internal/runtimecore/config"
)

func TestBuilderBuildForwardsRuntimeInputs(t *testing.T) {
	original := newSDKBuilder
	fakeService := &fakeSDKService{}
	fakeBuilder := &fakeSDKBuilder{service: fakeService}
	newSDKBuilder = func() sdkBuilder { return fakeBuilder }
	t.Cleanup(func() {
		newSDKBuilder = original
	})

	cfg := &runtimeconfig.Config{AuthDir: "/tmp/runtime-auth"}
	svc, err := NewBuilder().
		WithConfig(cfg).
		WithConfigPath("/tmp/runtime-config.yaml").
		WithLocalManagementPassword("secret").
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if fakeBuilder.cfg == nil {
		t.Fatal("expected config to be forwarded")
	}
	if got, want := fakeBuilder.cfg.AuthDir, cfg.AuthDir; got != want {
		t.Fatalf("forwarded auth dir = %q, want %q", got, want)
	}
	if got, want := fakeBuilder.configPath, "/tmp/runtime-config.yaml"; got != want {
		t.Fatalf("forwarded config path = %q, want %q", got, want)
	}
	if got, want := fakeBuilder.localManagementPassword, "secret"; got != want {
		t.Fatalf("forwarded local management password = %q, want %q", got, want)
	}
}

func TestBuilderBuildInjectsOwnedDefaultCoreAuthManager(t *testing.T) {
	originalBuilder := newSDKBuilder
	t.Cleanup(func() {
		newSDKBuilder = originalBuilder
	})

	store := &fakeRuntimeStore{}

	fakeBuilder := &fakeSDKBuilder{service: &fakeSDKService{}}
	newSDKBuilder = func() sdkBuilder { return fakeBuilder }

	_, err := NewBuilder().
		WithConfig(&runtimeconfig.Config{AuthDir: "/tmp/runtime-auth"}).
		WithConfigPath("/tmp/runtime-config.yaml").
		WithTokenStore(store).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if fakeBuilder.coreManager == nil {
		t.Fatal("expected owned default core auth manager to be injected")
	}

	_, err = fakeBuilder.coreManager.Register(context.Background(), &sdkcliproxyauth.Auth{
		ID:       "github-copilot.json",
		Provider: "github-copilot",
		FileName: "github-copilot.json",
		Metadata: map[string]any{"type": "github-copilot"},
	})
	if err != nil {
		t.Fatalf("core manager Register() error = %v", err)
	}
	if got, want := store.baseDir, "/tmp/runtime-auth"; got != want {
		t.Fatalf("runtime store base dir = %q, want %q", got, want)
	}
	if len(store.saved) != 1 {
		t.Fatalf("runtime store save count = %d, want 1", len(store.saved))
	}
	if got, want := store.saved[0].FileName, "github-copilot.json"; got != want {
		t.Fatalf("saved auth filename = %q, want %q", got, want)
	}
	if got, want := store.savedPaths[0], filepath.Join("/tmp/runtime-auth", "github-copilot.json"); got != want {
		t.Fatalf("saved path = %q, want %q", got, want)
	}
}

func TestBuilderBuildInjectsOwnedDefaultAuthManager(t *testing.T) {
	originalBuilder := newSDKBuilder
	t.Cleanup(func() {
		newSDKBuilder = originalBuilder
	})

	fakeBuilder := &fakeSDKBuilder{service: &fakeSDKService{}}
	newSDKBuilder = func() sdkBuilder { return fakeBuilder }

	_, err := NewBuilder().
		WithConfig(&runtimeconfig.Config{AuthDir: t.TempDir()}).
		WithConfigPath(filepath.Join(t.TempDir(), "runtime-config.yaml")).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if fakeBuilder.authManager == nil {
		t.Fatal("expected owned default auth manager to be injected")
	}
}

func TestBuilderBuildInjectsOwnedDefaultRequestAccessManager(t *testing.T) {
	originalBuilder := newSDKBuilder
	t.Cleanup(func() {
		newSDKBuilder = originalBuilder
	})

	fakeBuilder := &fakeSDKBuilder{service: &fakeSDKService{}}
	newSDKBuilder = func() sdkBuilder { return fakeBuilder }

	_, err := NewBuilder().
		WithConfig(&runtimeconfig.Config{AuthDir: t.TempDir()}).
		WithConfigPath(filepath.Join(t.TempDir(), "runtime-config.yaml")).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if fakeBuilder.accessManager == nil {
		t.Fatal("expected owned default request access manager to be injected")
	}
}

func TestBuilderBuildInjectsOwnedDefaultWatcherFactory(t *testing.T) {
	originalBuilder := newSDKBuilder
	t.Cleanup(func() {
		newSDKBuilder = originalBuilder
	})

	fakeBuilder := &fakeSDKBuilder{service: &fakeSDKService{}}
	newSDKBuilder = func() sdkBuilder { return fakeBuilder }

	_, err := NewBuilder().
		WithConfig(&runtimeconfig.Config{AuthDir: t.TempDir()}).
		WithConfigPath(filepath.Join(t.TempDir(), "runtime-config.yaml")).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if fakeBuilder.watcherFactory == nil {
		t.Fatal("expected owned default watcher factory to be injected")
	}
}

func TestBuilderBuildInjectsOwnedServerFactory(t *testing.T) {
	originalBuilder := newSDKBuilder
	t.Cleanup(func() {
		newSDKBuilder = originalBuilder
	})

	fakeBuilder := &fakeSDKBuilder{service: &fakeSDKService{}}
	newSDKBuilder = func() sdkBuilder { return fakeBuilder }

	_, err := NewBuilder().
		WithConfig(&runtimeconfig.Config{AuthDir: t.TempDir()}).
		WithConfigPath(filepath.Join(t.TempDir(), "runtime-config.yaml")).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if fakeBuilder.serverFactory == nil {
		t.Fatal("expected owned server factory to be injected")
	}
}

func TestBuilderBuildInjectsOwnedOpenAIHandlerFactories(t *testing.T) {
	originalBuilder := newSDKBuilder
	t.Cleanup(func() {
		newSDKBuilder = originalBuilder
	})

	fakeBuilder := &fakeSDKBuilder{service: &fakeSDKService{}}
	newSDKBuilder = func() sdkBuilder { return fakeBuilder }

	_, err := NewBuilder().
		WithConfig(&runtimeconfig.Config{AuthDir: t.TempDir()}).
		WithConfigPath(filepath.Join(t.TempDir(), "runtime-config.yaml")).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if fakeBuilder.openAIHandlerFactory == nil {
		t.Fatal("expected owned OpenAI handler factory to be injected")
	}
	if fakeBuilder.openAIResponsesFactory == nil {
		t.Fatal("expected owned OpenAI responses handler factory to be injected")
	}
}

func TestBuilderBuildInjectsOwnedWebsocketGatewayFactory(t *testing.T) {
	originalBuilder := newSDKBuilder
	t.Cleanup(func() {
		newSDKBuilder = originalBuilder
	})

	fakeBuilder := &fakeSDKBuilder{service: &fakeSDKService{}}
	newSDKBuilder = func() sdkBuilder { return fakeBuilder }

	_, err := NewBuilder().
		WithConfig(&runtimeconfig.Config{AuthDir: t.TempDir()}).
		WithConfigPath(filepath.Join(t.TempDir(), "runtime-config.yaml")).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if fakeBuilder.websocketGatewayFactory == nil {
		t.Fatal("expected owned websocket gateway factory to be injected")
	}
}

func TestBuilderBuildInjectsOwnedManagementSeams(t *testing.T) {
	originalBuilder := newSDKBuilder
	t.Cleanup(func() {
		newSDKBuilder = originalBuilder
	})

	fakeBuilder := &fakeSDKBuilder{service: &fakeSDKService{}}
	newSDKBuilder = func() sdkBuilder { return fakeBuilder }

	_, err := NewBuilder().
		WithConfig(&runtimeconfig.Config{AuthDir: t.TempDir()}).
		WithConfigPath(filepath.Join(t.TempDir(), "runtime-config.yaml")).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if fakeBuilder.managementHandlerFactory == nil {
		t.Fatal("expected owned management handler factory to be injected")
	}
	if fakeBuilder.oauthCallbackWriter == nil {
		t.Fatal("expected owned oauth callback writer to be injected")
	}
}

func TestBuilderBuildInjectsOwnedExecutorBinder(t *testing.T) {
	originalBuilder := newSDKBuilder
	t.Cleanup(func() {
		newSDKBuilder = originalBuilder
	})

	fakeBuilder := &fakeSDKBuilder{service: &fakeSDKService{}}
	newSDKBuilder = func() sdkBuilder { return fakeBuilder }

	_, err := NewBuilder().
		WithConfig(&runtimeconfig.Config{AuthDir: t.TempDir()}).
		WithConfigPath(filepath.Join(t.TempDir(), "runtime-config.yaml")).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if fakeBuilder.executorBinder == nil {
		t.Fatal("expected owned executor binder to be injected")
	}

	manager := sdkcliproxyauth.NewManager(nil, nil, nil)
	handled := fakeBuilder.executorBinder(&sdkconfig.Config{}, manager, &sdkcliproxyauth.Auth{Provider: "github-copilot"}, false)
	if !handled {
		t.Fatal("expected default executor binder to handle github-copilot")
	}
	exec, ok := manager.Executor("github-copilot")
	if !ok || exec == nil {
		t.Fatal("expected github-copilot executor to be registered")
	}
	if got, want := exec.Identifier(), "github-copilot"; got != want {
		t.Fatalf("registered executor = %q, want %q", got, want)
	}

	handled = fakeBuilder.executorBinder(&sdkconfig.Config{}, manager, &sdkcliproxyauth.Auth{Provider: "claude"}, false)
	if handled {
		t.Fatal("expected executor binder to leave non-owned providers untouched")
	}

	handled = fakeBuilder.executorBinder(&sdkconfig.Config{}, manager, &sdkcliproxyauth.Auth{
		Provider: "openai-compatibility",
		Attributes: map[string]string{
			"compat_name":  "openrouter",
			"provider_key": "openrouter",
		},
	}, false)
	if !handled {
		t.Fatal("expected default executor binder to handle openai-compatibility")
	}
	exec, ok = manager.Executor("openrouter")
	if !ok || exec == nil {
		t.Fatal("expected openai-compat executor to be registered")
	}
	if got, want := exec.Identifier(), "openrouter"; got != want {
		t.Fatalf("registered executor = %q, want %q", got, want)
	}
}

func TestDefaultWatcherFactoryBuildsUsableWrapper(t *testing.T) {
	authDir := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "runtime-config.yaml")
	if err := os.WriteFile(configPath, []byte("auth-dir: "+authDir+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	wrapper, err := defaultWatcherFactory(configPath, authDir, func(*sdkconfig.Config) {})
	if err != nil {
		t.Fatalf("defaultWatcherFactory() error = %v", err)
	}
	if wrapper == nil {
		t.Fatal("expected non-nil watcher wrapper")
	}
	wrapper.SetConfig(&sdkconfig.Config{AuthDir: authDir})
	_ = wrapper.SnapshotAuths()
	if err := wrapper.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestNewDefaultCoreAuthManagerUsesFillFirstSelector(t *testing.T) {
	manager := newDefaultCoreAuthManager(&runtimeconfig.Config{Routing: runtimeconfig.RoutingConfig{Strategy: "fill-first"}}, &fakeRuntimeStore{})
	if manager == nil {
		t.Fatal("expected non-nil manager")
	}
	if got := coreSelectorTypeName(manager); !strings.HasSuffix(got, ".FillFirstSelector") {
		t.Fatalf("selector type = %q, want FillFirstSelector", got)
	}
}

func TestNewDefaultCoreAuthManagerDefaultsToRoundRobinSelector(t *testing.T) {
	manager := newDefaultCoreAuthManager(&runtimeconfig.Config{Routing: runtimeconfig.RoutingConfig{Strategy: "round-robin"}}, &fakeRuntimeStore{})
	if manager == nil {
		t.Fatal("expected non-nil manager")
	}
	if got := coreSelectorTypeName(manager); !strings.HasSuffix(got, ".RoundRobinSelector") {
		t.Fatalf("selector type = %q, want RoundRobinSelector", got)
	}
}

func TestServiceRunDelegatesToUnderlyingRunner(t *testing.T) {
	expectedErr := errors.New("run failed")
	runner := &fakeSDKService{err: expectedErr}
	svc := &Service{runner: runner}

	err := svc.Run(context.Background())
	if !errors.Is(err, expectedErr) {
		t.Fatalf("Run() error = %v, want %v", err, expectedErr)
	}
	if runner.runCalls != 1 {
		t.Fatalf("Run() calls = %d, want 1", runner.runCalls)
	}
}

func TestBuilderBuildRequiresConfig(t *testing.T) {
	_, err := NewBuilder().WithConfigPath("/tmp/runtime-config.yaml").Build()
	if err == nil {
		t.Fatal("expected error when config is missing")
	}
	if !strings.Contains(err.Error(), "configuration is required") {
		t.Fatalf("Build() error = %v, want missing config error", err)
	}
}

func TestBuilderBuildRequiresConfigPath(t *testing.T) {
	_, err := NewBuilder().WithConfig(&runtimeconfig.Config{}).Build()
	if err == nil {
		t.Fatal("expected error when config path is missing")
	}
	if !strings.Contains(err.Error(), "configuration path is required") {
		t.Fatalf("Build() error = %v, want missing config path error", err)
	}
}

func TestServiceRunErrorsWhenRunnerMissing(t *testing.T) {
	err := (&Service{}).Run(context.Background())
	if err == nil {
		t.Fatal("expected error when runner is missing")
	}
	if !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("Run() error = %v, want not initialized error", err)
	}
}

type fakeSDKBuilder struct {
	cfg                      *sdkconfig.Config
	configPath               string
	localManagementPassword  string
	authManager              *sdkauth.Manager
	accessManager            *sdkaccess.Manager
	coreManager              *sdkcliproxyauth.Manager
	watcherFactory           sdkcliproxy.WatcherFactory
	executorBinder           sdkcliproxy.ExecutorBinder
	serverFactory            sdkcliproxy.ServerFactory
	openAIHandlerFactory     sdkcliproxy.OpenAIHandlerFactory
	openAIResponsesFactory   sdkcliproxy.OpenAIResponsesHandlerFactory
	websocketGatewayFactory  sdkcliproxy.WebsocketGatewayFactory
	managementHandlerFactory sdkcliproxy.ManagementHandlerFactory
	oauthCallbackWriter      sdkcliproxy.OAuthCallbackWriter
	service                  sdkServiceRunner
	err                      error
}

func (b *fakeSDKBuilder) WithConfig(cfg *sdkconfig.Config) sdkBuilder {
	b.cfg = cfg
	return b
}

func (b *fakeSDKBuilder) WithConfigPath(path string) sdkBuilder {
	b.configPath = path
	return b
}

func (b *fakeSDKBuilder) WithLocalManagementPassword(password string) sdkBuilder {
	b.localManagementPassword = password
	return b
}

func (b *fakeSDKBuilder) WithAuthManager(manager *sdkauth.Manager) sdkBuilder {
	b.authManager = manager
	return b
}

func (b *fakeSDKBuilder) WithRequestAccessManager(manager *sdkaccess.Manager) sdkBuilder {
	b.accessManager = manager
	return b
}

func (b *fakeSDKBuilder) WithCoreAuthManager(manager *sdkcliproxyauth.Manager) sdkBuilder {
	b.coreManager = manager
	return b
}

func (b *fakeSDKBuilder) WithWatcherFactory(factory sdkcliproxy.WatcherFactory) sdkBuilder {
	b.watcherFactory = factory
	return b
}

func (b *fakeSDKBuilder) WithExecutorBinder(binder sdkcliproxy.ExecutorBinder) sdkBuilder {
	b.executorBinder = binder
	return b
}

func (b *fakeSDKBuilder) WithServerFactory(factory sdkcliproxy.ServerFactory) sdkBuilder {
	b.serverFactory = factory
	return b
}

func (b *fakeSDKBuilder) WithOpenAIHandlerFactory(factory sdkcliproxy.OpenAIHandlerFactory) sdkBuilder {
	b.openAIHandlerFactory = factory
	return b
}

func (b *fakeSDKBuilder) WithOpenAIResponsesHandlerFactory(factory sdkcliproxy.OpenAIResponsesHandlerFactory) sdkBuilder {
	b.openAIResponsesFactory = factory
	return b
}

func (b *fakeSDKBuilder) WithWebsocketGatewayFactory(factory sdkcliproxy.WebsocketGatewayFactory) sdkBuilder {
	b.websocketGatewayFactory = factory
	return b
}

func (b *fakeSDKBuilder) WithManagementHandlerFactory(factory sdkcliproxy.ManagementHandlerFactory) sdkBuilder {
	b.managementHandlerFactory = factory
	return b
}

func (b *fakeSDKBuilder) WithOAuthCallbackWriter(writer sdkcliproxy.OAuthCallbackWriter) sdkBuilder {
	b.oauthCallbackWriter = writer
	return b
}

func (b *fakeSDKBuilder) WithHandlerOnly() sdkBuilder {
	return b
}

func (b *fakeSDKBuilder) Build() (sdkServiceRunner, error) {
	return b.service, b.err
}

type fakeSDKService struct {
	runCalls int
	err      error
}

func (s *fakeSDKService) Run(context.Context) error {
	s.runCalls++
	return s.err
}

type fakeRuntimeStore struct {
	baseDir    string
	saved      []*sdkcliproxyauth.Auth
	savedPaths []string
}

func (s *fakeRuntimeStore) SetBaseDir(dir string) {
	s.baseDir = dir
}

func (s *fakeRuntimeStore) List(context.Context) ([]*sdkcliproxyauth.Auth, error) {
	return nil, nil
}

func (s *fakeRuntimeStore) Save(_ context.Context, auth *sdkcliproxyauth.Auth) (string, error) {
	s.saved = append(s.saved, auth.Clone())
	path := filepath.Join(s.baseDir, auth.FileName)
	s.savedPaths = append(s.savedPaths, path)
	return path, nil
}

func (s *fakeRuntimeStore) Delete(context.Context, string) error {
	return nil
}

func coreSelectorTypeName(manager *sdkcliproxyauth.Manager) string {
	if manager == nil {
		return ""
	}
	selector := reflect.ValueOf(manager).Elem().FieldByName("selector")
	if !selector.IsValid() || selector.IsNil() {
		return ""
	}
	return selector.Elem().Type().String()
}
