package runtimeauth

import sdkauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"

func NewLegacyAuthManager() *sdkauth.Manager {
	return sdkauth.NewManager(
		GetTokenStore(),
		sdkauth.NewGeminiAuthenticator(),
		sdkauth.NewCodexAuthenticator(),
		sdkauth.NewClaudeAuthenticator(),
		sdkauth.NewQwenAuthenticator(),
	)
}
