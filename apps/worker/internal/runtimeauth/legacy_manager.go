package runtimeauth

import sdkauth "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/auth"

func NewLegacyAuthManager() *sdkauth.Manager {
	return sdkauth.NewManager(
		GetTokenStore(),
		sdkauth.NewGeminiAuthenticator(),
		sdkauth.NewCodexAuthenticator(),
		sdkauth.NewClaudeAuthenticator(),
		sdkauth.NewQwenAuthenticator(),
	)
}
