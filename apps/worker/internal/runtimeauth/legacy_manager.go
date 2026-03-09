package runtimeauth

import (
	sdkauth "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/auth"
	coreauth "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy/auth"
)

// NewLegacyAuthManager creates a legacy auth manager backed by the given token store.
func NewLegacyAuthManager(store coreauth.Store) *sdkauth.Manager {
	return sdkauth.NewManager(
		store,
		sdkauth.NewGeminiAuthenticator(),
		sdkauth.NewCodexAuthenticator(),
		sdkauth.NewClaudeAuthenticator(),
		sdkauth.NewQwenAuthenticator(),
	)
}
