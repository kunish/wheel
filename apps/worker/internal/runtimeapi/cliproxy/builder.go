package cliproxy

import (
	"context"
	"fmt"
	"strings"

	runtimeopenai "github.com/kunish/wheel/apps/worker/internal/runtimeapi/openai"
	"github.com/kunish/wheel/apps/worker/internal/runtimeauth"
	runtimeconfig "github.com/kunish/wheel/apps/worker/internal/runtimecore/config"
	runtimeexecutor "github.com/kunish/wheel/apps/worker/internal/runtimecore/executor"
	"github.com/kunish/wheel/apps/worker/internal/runtimectrl"
	sdkconfig "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/config"
	sdkaccess "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/access"
	sdkapihandlers "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/api/handlers"
	sdkauth "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/auth"
	sdkcliproxy "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy"
	sdkcliproxyauth "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy/auth"
)

type sdkServiceRunner interface {
	Run(context.Context) error
}

type sdkBuilder interface {
	WithConfig(*sdkconfig.Config) sdkBuilder
	WithConfigPath(string) sdkBuilder
	WithLocalManagementPassword(string) sdkBuilder
	WithAuthManager(*sdkauth.Manager) sdkBuilder
	WithRequestAccessManager(*sdkaccess.Manager) sdkBuilder
	WithCoreAuthManager(*sdkcliproxyauth.Manager) sdkBuilder
	WithWatcherFactory(sdkcliproxy.WatcherFactory) sdkBuilder
	WithExecutorBinder(sdkcliproxy.ExecutorBinder) sdkBuilder
	WithServerFactory(sdkcliproxy.ServerFactory) sdkBuilder
	WithWebsocketGatewayFactory(sdkcliproxy.WebsocketGatewayFactory) sdkBuilder
	WithOpenAIHandlerFactory(sdkcliproxy.OpenAIHandlerFactory) sdkBuilder
	WithOpenAIResponsesHandlerFactory(sdkcliproxy.OpenAIResponsesHandlerFactory) sdkBuilder
	WithManagementHandlerFactory(sdkcliproxy.ManagementHandlerFactory) sdkBuilder
	WithOAuthCallbackWriter(sdkcliproxy.OAuthCallbackWriter) sdkBuilder
	Build() (sdkServiceRunner, error)
}

type sdkBuilderAdapter struct {
	inner *sdkcliproxy.Builder
}

func (a *sdkBuilderAdapter) WithConfig(cfg *sdkconfig.Config) sdkBuilder {
	a.inner = a.inner.WithConfig(cfg)
	return a
}

func (a *sdkBuilderAdapter) WithConfigPath(path string) sdkBuilder {
	a.inner = a.inner.WithConfigPath(path)
	return a
}

func (a *sdkBuilderAdapter) WithLocalManagementPassword(password string) sdkBuilder {
	a.inner = a.inner.WithLocalManagementPassword(password)
	return a
}

func (a *sdkBuilderAdapter) WithAuthManager(manager *sdkauth.Manager) sdkBuilder {
	a.inner = a.inner.WithAuthManager(manager)
	return a
}

func (a *sdkBuilderAdapter) WithRequestAccessManager(manager *sdkaccess.Manager) sdkBuilder {
	a.inner = a.inner.WithRequestAccessManager(manager)
	return a
}

func (a *sdkBuilderAdapter) WithCoreAuthManager(manager *sdkcliproxyauth.Manager) sdkBuilder {
	a.inner = a.inner.WithCoreAuthManager(manager)
	return a
}

func (a *sdkBuilderAdapter) WithWatcherFactory(factory sdkcliproxy.WatcherFactory) sdkBuilder {
	a.inner = a.inner.WithWatcherFactory(factory)
	return a
}

func (a *sdkBuilderAdapter) WithExecutorBinder(binder sdkcliproxy.ExecutorBinder) sdkBuilder {
	a.inner = a.inner.WithExecutorBinder(binder)
	return a
}

func (a *sdkBuilderAdapter) WithServerFactory(factory sdkcliproxy.ServerFactory) sdkBuilder {
	a.inner = a.inner.WithServerFactory(factory)
	return a
}

func (a *sdkBuilderAdapter) WithWebsocketGatewayFactory(factory sdkcliproxy.WebsocketGatewayFactory) sdkBuilder {
	a.inner = a.inner.WithWebsocketGatewayFactory(factory)
	return a
}

func (a *sdkBuilderAdapter) WithOpenAIHandlerFactory(factory sdkcliproxy.OpenAIHandlerFactory) sdkBuilder {
	a.inner = a.inner.WithOpenAIHandlerFactory(factory)
	return a
}

func (a *sdkBuilderAdapter) WithOpenAIResponsesHandlerFactory(factory sdkcliproxy.OpenAIResponsesHandlerFactory) sdkBuilder {
	a.inner = a.inner.WithOpenAIResponsesHandlerFactory(factory)
	return a
}

func (a *sdkBuilderAdapter) WithManagementHandlerFactory(factory sdkcliproxy.ManagementHandlerFactory) sdkBuilder {
	a.inner = a.inner.WithManagementHandlerFactory(factory)
	return a
}

func (a *sdkBuilderAdapter) WithOAuthCallbackWriter(writer sdkcliproxy.OAuthCallbackWriter) sdkBuilder {
	a.inner = a.inner.WithOAuthCallbackWriter(writer)
	return a
}

func (a *sdkBuilderAdapter) Build() (sdkServiceRunner, error) {
	return a.inner.Build()
}

var newSDKBuilder = func() sdkBuilder {
	return &sdkBuilderAdapter{inner: sdkcliproxy.NewBuilder()}
}

// Builder owns the production runtime boot seam for embedded runtime startup.
type Builder struct {
	cfg                     *runtimeconfig.Config
	configPath              string
	localManagementPassword string
}

func NewBuilder() *Builder {
	return &Builder{}
}

func (b *Builder) WithConfig(cfg *runtimeconfig.Config) *Builder {
	b.cfg = cfg
	return b
}

func (b *Builder) WithConfigPath(path string) *Builder {
	b.configPath = path
	return b
}

func (b *Builder) WithLocalManagementPassword(password string) *Builder {
	b.localManagementPassword = password
	return b
}

func (b *Builder) Build() (*Service, error) {
	if b == nil {
		return nil, fmt.Errorf("cliproxy runtime builder is nil")
	}
	if b.cfg == nil {
		return nil, fmt.Errorf("cliproxy runtime builder: configuration is required")
	}
	if strings.TrimSpace(b.configPath) == "" {
		return nil, fmt.Errorf("cliproxy runtime builder: configuration path is required")
	}

	sdkCfg, err := b.cfg.ToSDKConfig()
	if err != nil {
		return nil, err
	}
	authManager := runtimeauth.NewLegacyAuthManager()
	accessManager := sdkaccess.NewManager()
	coreManager := newDefaultCoreAuthManager(b.cfg)

	runner, err := newSDKBuilder().
		WithConfig(sdkCfg).
		WithConfigPath(b.configPath).
		WithLocalManagementPassword(b.localManagementPassword).
		WithAuthManager(authManager).
		WithRequestAccessManager(accessManager).
		WithCoreAuthManager(coreManager).
		WithWatcherFactory(defaultWatcherFactory).
		WithExecutorBinder(newDefaultExecutorBinder(b.cfg)).
		WithServerFactory(sdkcliproxy.DefaultServerFactory).
		WithWebsocketGatewayFactory(sdkcliproxy.DefaultWebsocketGatewayFactory).
		WithOpenAIHandlerFactory(func(base *sdkapihandlers.BaseAPIHandler) sdkcliproxy.OpenAIHandlerRoutes {
			return runtimeopenai.NewAPIHandler(base)
		}).
		WithOpenAIResponsesHandlerFactory(func(base *sdkapihandlers.BaseAPIHandler) sdkcliproxy.OpenAIResponsesHandlerRoutes {
			return runtimeopenai.NewResponsesHandler(base)
		}).
		WithManagementHandlerFactory(func(cfg *sdkconfig.Config, configFilePath string, authManager *sdkcliproxyauth.Manager) sdkcliproxy.ManagementHandlerRoutes {
			return runtimectrl.NewManagementHandler(cfg, configFilePath, authManager)
		}).
		WithOAuthCallbackWriter(runtimectrl.WriteOAuthCallback).
		Build()
	if err != nil {
		return nil, err
	}

	return &Service{runner: runner}, nil
}

// Service wraps the runtime boot seam used by Wheel-owned embedding code.
type Service struct {
	runner sdkServiceRunner
}

func (s *Service) Run(ctx context.Context) error {
	if s == nil || s.runner == nil {
		return fmt.Errorf("cliproxy runtime service is not initialized")
	}
	return s.runner.Run(ctx)
}

func newDefaultCoreAuthManager(cfg *runtimeconfig.Config) *sdkcliproxyauth.Manager {
	store := runtimeauth.GetTokenStore()
	if dirSetter, ok := store.(interface{ SetBaseDir(string) }); ok && cfg != nil {
		dirSetter.SetBaseDir(cfg.AuthDir)
	}
	return sdkcliproxyauth.NewManager(store, newCoreAuthSelector(cfg), nil)
}

func newCoreAuthSelector(cfg *runtimeconfig.Config) sdkcliproxyauth.Selector {
	strategy := ""
	if cfg != nil {
		strategy = strings.ToLower(strings.TrimSpace(cfg.Routing.Strategy))
	}
	switch strategy {
	case "fill-first", "fillfirst", "ff":
		return &sdkcliproxyauth.FillFirstSelector{}
	default:
		return &sdkcliproxyauth.RoundRobinSelector{}
	}
}

func newDefaultExecutorBinder(cfg *runtimeconfig.Config) sdkcliproxy.ExecutorBinder {
	return func(_ *sdkconfig.Config, manager *sdkcliproxyauth.Manager, auth *sdkcliproxyauth.Auth, _ bool) bool {
		if manager == nil || auth == nil {
			return false
		}
		provider := strings.ToLower(strings.TrimSpace(auth.Provider))
		switch {
		case strings.EqualFold(provider, "github-copilot"):
			manager.RegisterExecutor(runtimeexecutor.NewGitHubCopilotExecutor(cfg))
			return true
		case isOpenAICompatBindingTarget(auth):
			manager.RegisterExecutor(runtimeexecutor.NewOpenAICompatExecutor(openAICompatProviderKey(auth), cfg))
			return true
		default:
			return false
		}
	}
}

func isOpenAICompatBindingTarget(auth *sdkcliproxyauth.Auth) bool {
	if auth == nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(auth.Provider), "openai-compatibility") {
		return true
	}
	if auth.Attributes == nil {
		return false
	}
	return strings.TrimSpace(auth.Attributes["compat_name"]) != ""
}

func openAICompatProviderKey(auth *sdkcliproxyauth.Auth) string {
	if auth == nil {
		return "openai-compatibility"
	}
	if auth.Attributes != nil {
		if key := strings.ToLower(strings.TrimSpace(auth.Attributes["provider_key"])); key != "" {
			return key
		}
		if key := strings.ToLower(strings.TrimSpace(auth.Attributes["compat_name"])); key != "" {
			return key
		}
	}
	if key := strings.ToLower(strings.TrimSpace(auth.Provider)); key != "" {
		return key
	}
	return "openai-compatibility"
}
