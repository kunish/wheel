// Package cliproxy provides the core service implementation for the CLI Proxy API.
// It includes service lifecycle management, authentication handling, file watching,
// and integration with various AI service providers through a unified interface.
package cliproxy

import (
	"fmt"
	"strings"

	configaccess "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/access/config_access"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/api"
	sdkaccess "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/access"
	sdkAuth "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/auth"
	coreauth "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy/auth"
	"github.com/kunish/wheel/apps/worker/internal/runtime/sdk/config"
)

// Builder constructs a Service instance with customizable providers.
// It provides a fluent interface for configuring all aspects of the service
// including authentication, file watching, HTTP server options, and lifecycle hooks.
type Builder struct {
	// cfg holds the application configuration.
	cfg *config.Config

	// configPath is the path to the configuration file.
	configPath string

	// tokenProvider handles loading token-based clients.
	tokenProvider TokenClientProvider

	// apiKeyProvider handles loading API key-based clients.
	apiKeyProvider APIKeyClientProvider

	// watcherFactory creates file watcher instances.
	watcherFactory WatcherFactory

	// executorBinder allows host-owned code to override provider executor binding.
	executorBinder ExecutorBinder

	// serverFactory creates the embedded HTTP API server.
	serverFactory ServerFactory

	// websocketGatewayFactory creates the embedded websocket relay gateway.
	websocketGatewayFactory WebsocketGatewayFactory

	// openAIHandlerFactory overrides OpenAI chat/completions route construction.
	openAIHandlerFactory OpenAIHandlerFactory

	// openAIResponsesFactory overrides OpenAI responses route construction.
	openAIResponsesFactory OpenAIResponsesHandlerFactory

	// managementHandlerFactory overrides management route handler construction.
	managementHandlerFactory ManagementHandlerFactory

	// oauthCallbackWriter overrides OAuth callback persistence.
	oauthCallbackWriter OAuthCallbackWriter

	// hooks provides lifecycle callbacks.
	hooks Hooks

	// authManager handles legacy authentication operations.
	authManager *sdkAuth.Manager

	// accessManager handles request authentication providers.
	accessManager *sdkaccess.Manager

	// coreManager handles core authentication and execution.
	coreManager *coreauth.Manager

	// serverOptions contains additional server configuration options.
	serverOptions []api.ServerOption
}

// Hooks allows callers to plug into service lifecycle stages.
// These callbacks provide opportunities to perform custom initialization
// and cleanup operations during service startup and shutdown.
type Hooks struct {
	// OnBeforeStart is called before the service starts, allowing configuration
	// modifications or additional setup.
	OnBeforeStart func(*config.Config)

	// OnAfterStart is called after the service has started successfully,
	// providing access to the service instance for additional operations.
	OnAfterStart func(*Service)
}

// NewBuilder creates a Builder with default dependencies left unset.
// Use the fluent interface methods to configure the service before calling Build().
//
// Returns:
//   - *Builder: A new builder instance ready for configuration
func NewBuilder() *Builder {
	return &Builder{}
}

// WithConfig sets the configuration instance used by the service.
//
// Parameters:
//   - cfg: The application configuration
//
// Returns:
//   - *Builder: The builder instance for method chaining
func (b *Builder) WithConfig(cfg *config.Config) *Builder {
	b.cfg = cfg
	return b
}

// WithConfigPath sets the absolute configuration file path used for reload watching.
//
// Parameters:
//   - path: The absolute path to the configuration file
//
// Returns:
//   - *Builder: The builder instance for method chaining
func (b *Builder) WithConfigPath(path string) *Builder {
	b.configPath = path
	return b
}

// WithTokenClientProvider overrides the provider responsible for token-backed clients.
func (b *Builder) WithTokenClientProvider(provider TokenClientProvider) *Builder {
	b.tokenProvider = provider
	return b
}

// WithAPIKeyClientProvider overrides the provider responsible for API key-backed clients.
func (b *Builder) WithAPIKeyClientProvider(provider APIKeyClientProvider) *Builder {
	b.apiKeyProvider = provider
	return b
}

// WithWatcherFactory allows customizing the watcher factory that handles reloads.
func (b *Builder) WithWatcherFactory(factory WatcherFactory) *Builder {
	b.watcherFactory = factory
	return b
}

// WithExecutorBinder allows host-owned code to intercept provider executor binding.
func (b *Builder) WithExecutorBinder(binder ExecutorBinder) *Builder {
	b.executorBinder = binder
	return b
}

// WithServerFactory allows host-owned code to override embedded API server construction.
func (b *Builder) WithServerFactory(factory ServerFactory) *Builder {
	b.serverFactory = factory
	return b
}

// WithWebsocketGatewayFactory allows host-owned code to override websocket gateway construction.
func (b *Builder) WithWebsocketGatewayFactory(factory WebsocketGatewayFactory) *Builder {
	b.websocketGatewayFactory = factory
	return b
}

// WithOpenAIHandlerFactory allows host-owned code to override OpenAI chat/completions handlers.
func (b *Builder) WithOpenAIHandlerFactory(factory OpenAIHandlerFactory) *Builder {
	b.openAIHandlerFactory = factory
	return b
}

// WithOpenAIResponsesHandlerFactory allows host-owned code to override OpenAI responses handlers.
func (b *Builder) WithOpenAIResponsesHandlerFactory(factory OpenAIResponsesHandlerFactory) *Builder {
	b.openAIResponsesFactory = factory
	return b
}

// WithManagementHandlerFactory allows host-owned code to override management handler construction.
func (b *Builder) WithManagementHandlerFactory(factory ManagementHandlerFactory) *Builder {
	b.managementHandlerFactory = factory
	return b
}

// WithOAuthCallbackWriter allows host-owned code to override OAuth callback persistence.
func (b *Builder) WithOAuthCallbackWriter(writer OAuthCallbackWriter) *Builder {
	b.oauthCallbackWriter = writer
	return b
}

// WithHooks registers lifecycle hooks executed around service startup.
func (b *Builder) WithHooks(h Hooks) *Builder {
	b.hooks = h
	return b
}

// WithAuthManager overrides the authentication manager used for token lifecycle operations.
func (b *Builder) WithAuthManager(mgr *sdkAuth.Manager) *Builder {
	b.authManager = mgr
	return b
}

// WithRequestAccessManager overrides the request authentication manager.
func (b *Builder) WithRequestAccessManager(mgr *sdkaccess.Manager) *Builder {
	b.accessManager = mgr
	return b
}

// WithCoreAuthManager overrides the runtime auth manager responsible for request execution.
func (b *Builder) WithCoreAuthManager(mgr *coreauth.Manager) *Builder {
	b.coreManager = mgr
	return b
}

// WithServerOptions appends server configuration options used during construction.
func (b *Builder) WithServerOptions(opts ...api.ServerOption) *Builder {
	b.serverOptions = append(b.serverOptions, opts...)
	return b
}

// WithLocalManagementPassword configures a password that is only accepted from localhost management requests.
func (b *Builder) WithLocalManagementPassword(password string) *Builder {
	if password == "" {
		return b
	}
	b.serverOptions = append(b.serverOptions, api.WithLocalManagementPassword(password))
	return b
}

// WithPostAuthHook registers a hook to be called after an Auth record is created
// but before it is persisted to storage.
func (b *Builder) WithPostAuthHook(hook coreauth.PostAuthHook) *Builder {
	if hook == nil {
		return b
	}
	b.serverOptions = append(b.serverOptions, api.WithPostAuthHook(hook))
	return b
}

// Build validates inputs, applies defaults, and returns a ready-to-run service.
func (b *Builder) Build() (*Service, error) {
	if b.cfg == nil {
		return nil, fmt.Errorf("cliproxy: configuration is required")
	}
	if b.configPath == "" {
		return nil, fmt.Errorf("cliproxy: configuration path is required")
	}

	tokenProvider := b.tokenProvider
	if tokenProvider == nil {
		tokenProvider = NewFileTokenClientProvider()
	}

	apiKeyProvider := b.apiKeyProvider
	if apiKeyProvider == nil {
		apiKeyProvider = NewAPIKeyClientProvider()
	}

	watcherFactory := b.watcherFactory
	if watcherFactory == nil {
		watcherFactory = defaultWatcherFactory
	}

	authManager := b.authManager
	if authManager == nil {
		authManager = newDefaultAuthManager()
	}

	accessManager := b.accessManager
	if accessManager == nil {
		accessManager = sdkaccess.NewManager()
	}

	configaccess.Register(&b.cfg.SDKConfig)
	accessManager.SetProviders(sdkaccess.RegisteredProviders())

	coreManager := b.coreManager
	if coreManager == nil {
		tokenStore := sdkAuth.GetTokenStore()
		if dirSetter, ok := tokenStore.(interface{ SetBaseDir(string) }); ok && b.cfg != nil {
			dirSetter.SetBaseDir(b.cfg.AuthDir)
		}

		strategy := ""
		if b.cfg != nil {
			strategy = strings.ToLower(strings.TrimSpace(b.cfg.Routing.Strategy))
		}
		var selector coreauth.Selector
		switch strategy {
		case "fill-first", "fillfirst", "ff":
			selector = &coreauth.FillFirstSelector{}
		default:
			selector = &coreauth.RoundRobinSelector{}
		}

		coreManager = coreauth.NewManager(tokenStore, selector, nil)
	}
	// Attach a default RoundTripper provider so providers can opt-in per-auth transports.
	coreManager.SetRoundTripperProvider(newDefaultRoundTripperProvider())
	coreManager.SetConfig(b.cfg)
	coreManager.SetOAuthModelAlias(b.cfg.OAuthModelAlias)

	service := &Service{
		cfg:                      b.cfg,
		configPath:               b.configPath,
		tokenProvider:            tokenProvider,
		apiKeyProvider:           apiKeyProvider,
		watcherFactory:           watcherFactory,
		executorBinder:           b.executorBinder,
		serverFactory:            b.serverFactory,
		websocketGatewayFactory:  b.websocketGatewayFactory,
		openAIHandlerFactory:     b.openAIHandlerFactory,
		openAIResponsesFactory:   b.openAIResponsesFactory,
		managementHandlerFactory: b.managementHandlerFactory,
		oauthCallbackWriter:      b.oauthCallbackWriter,
		hooks:                    b.hooks,
		authManager:              authManager,
		accessManager:            accessManager,
		coreManager:              coreManager,
		serverOptions:            append([]api.ServerOption(nil), b.serverOptions...),
	}
	return service, nil
}
