package cliproxy

import (
	"context"

	"github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/corelib/auth/claude"
	"github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/corelib/config"
)

// --- Anthropic (Claude) Auth Provider Seam ---

// AnthropicPKCECodes wraps the PKCE verifier and challenge for Anthropic OAuth.
type AnthropicPKCECodes = claude.PKCECodes

// AnthropicTokenData holds OAuth token information from Anthropic.
type AnthropicTokenData = claude.ClaudeTokenData

// AnthropicAuthBundle aggregates all auth data after the Anthropic OAuth flow.
type AnthropicAuthBundle = claude.ClaudeAuthBundle

// AnthropicTokenStorage is the persistent token storage for Anthropic/Claude credentials.
type AnthropicTokenStorage = claude.ClaudeTokenStorage

// AnthropicAuthProvider wraps the internal Claude auth service for first-party use.
type AnthropicAuthProvider struct {
	inner *claude.ClaudeAuth
}

// NewAnthropicAuthProvider creates a new Anthropic auth service using the given config.
func NewAnthropicAuthProvider(cfg *config.Config) *AnthropicAuthProvider {
	return &AnthropicAuthProvider{inner: claude.NewClaudeAuth(cfg)}
}

// GeneratePKCECodes creates a new PKCE verifier and challenge pair.
func (p *AnthropicAuthProvider) GeneratePKCECodes() (*AnthropicPKCECodes, error) {
	return claude.GeneratePKCECodes()
}

// GenerateAuthURL builds the OAuth authorization URL for Anthropic.
// Returns (authURL, state, error). The state may differ from input if the
// provider appends additional info.
func (p *AnthropicAuthProvider) GenerateAuthURL(state string, pkceCodes *AnthropicPKCECodes) (string, string, error) {
	return p.inner.GenerateAuthURL(state, pkceCodes)
}

// ExchangeCodeForTokens exchanges an authorization code for tokens.
func (p *AnthropicAuthProvider) ExchangeCodeForTokens(ctx context.Context, code, state string, pkceCodes *AnthropicPKCECodes) (*AnthropicAuthBundle, error) {
	return p.inner.ExchangeCodeForTokens(ctx, code, state, pkceCodes)
}

// CreateTokenStorage creates an AnthropicTokenStorage from an auth bundle.
func (p *AnthropicAuthProvider) CreateTokenStorage(bundle *AnthropicAuthBundle) *AnthropicTokenStorage {
	return p.inner.CreateTokenStorage(bundle)
}

// AnthropicCallbackPort is the port the Claude CLI expects for OAuth redirect.
const AnthropicCallbackPort = 54545
