package runtimeauth

import (
	"context"
	"fmt"

	coreauth "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy/auth"
	runtimeconfig "github.com/kunish/wheel/apps/worker/internal/runtimecore/config"
)

// manager aggregates authenticators and coordinates persistence via a token store.
type manager struct {
	authenticators map[string]authenticator
	store          coreauth.Store
}

// newManager constructs a manager with the provided token store and authenticators.
func newManager(store coreauth.Store, authenticators ...authenticator) *manager {
	mgr := &manager{
		authenticators: make(map[string]authenticator),
		store:          store,
	}
	for i := range authenticators {
		mgr.Register(authenticators[i])
	}
	return mgr
}

// Register adds or replaces an authenticator keyed by its provider identifier.
func (m *manager) Register(a authenticator) {
	if a == nil {
		return
	}
	if m.authenticators == nil {
		m.authenticators = make(map[string]authenticator)
	}
	m.authenticators[a.Provider()] = a
}

// SetStore updates the token store used for persistence.
func (m *manager) SetStore(store coreauth.Store) {
	m.store = store
}

// Login executes the provider login flow and persists the resulting auth record.
func (m *manager) Login(ctx context.Context, provider string, cfg *runtimeconfig.Config, opts *loginOptions) (*coreauth.Auth, string, error) {
	auth, ok := m.authenticators[provider]
	if !ok {
		return nil, "", fmt.Errorf("cliproxy auth: authenticator %s not registered", provider)
	}

	record, err := auth.Login(ctx, cfg, opts)
	if err != nil {
		return nil, "", err
	}
	if record == nil {
		return nil, "", fmt.Errorf("cliproxy auth: authenticator %s returned nil record", provider)
	}

	if m.store == nil {
		return record, "", nil
	}

	if cfg != nil {
		if dirSetter, ok := m.store.(interface{ SetBaseDir(string) }); ok {
			dirSetter.SetBaseDir(cfg.AuthDir)
		}
	}

	savedPath, err := m.store.Save(ctx, record)
	if err != nil {
		return record, "", err
	}
	return record, savedPath, nil
}

// SaveAuth persists an auth record directly without going through the login flow.
func (m *manager) SaveAuth(record *coreauth.Auth, cfg *runtimeconfig.Config) (string, error) {
	if m.store == nil {
		return "", fmt.Errorf("no store configured")
	}
	if cfg != nil {
		if dirSetter, ok := m.store.(interface{ SetBaseDir(string) }); ok {
			dirSetter.SetBaseDir(cfg.AuthDir)
		}
	}
	return m.store.Save(context.Background(), record)
}
