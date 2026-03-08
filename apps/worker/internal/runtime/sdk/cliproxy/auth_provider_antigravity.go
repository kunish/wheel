package cliproxy

import (
	"context"

	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/auth/antigravity"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/config"
)

// --- Antigravity Auth Provider Seam ---

// AntigravityTokenResponse holds the OAuth token response from Google for Antigravity.
type AntigravityTokenResponse = antigravity.TokenResponse

// AntigravityAuthProvider wraps the internal Antigravity auth service for first-party use.
type AntigravityAuthProvider struct {
	inner *antigravity.AntigravityAuth
}

// NewAntigravityAuthProvider creates a new Antigravity auth service using the given config.
func NewAntigravityAuthProvider(cfg *config.Config) *AntigravityAuthProvider {
	return &AntigravityAuthProvider{inner: antigravity.NewAntigravityAuth(cfg, nil)}
}

// BuildAuthURL constructs the OAuth authorization URL for Antigravity.
func (p *AntigravityAuthProvider) BuildAuthURL(state, redirectURI string) string {
	return p.inner.BuildAuthURL(state, redirectURI)
}

// ExchangeCodeForTokens exchanges an authorization code for tokens.
func (p *AntigravityAuthProvider) ExchangeCodeForTokens(ctx context.Context, code, redirectURI string) (*AntigravityTokenResponse, error) {
	return p.inner.ExchangeCodeForTokens(ctx, code, redirectURI)
}

// FetchUserInfo retrieves the user's email from the Google user info endpoint.
func (p *AntigravityAuthProvider) FetchUserInfo(ctx context.Context, accessToken string) (string, error) {
	return p.inner.FetchUserInfo(ctx, accessToken)
}

// FetchProjectID retrieves the Cloud AI Companion project ID.
func (p *AntigravityAuthProvider) FetchProjectID(ctx context.Context, accessToken string) (string, error) {
	return p.inner.FetchProjectID(ctx, accessToken)
}

// AntigravityCredentialFileName generates the credential filename for Antigravity.
func AntigravityCredentialFileName(email string) string {
	return antigravity.CredentialFileName(email)
}

// AntigravityCallbackPort is the port the Antigravity CLI expects for OAuth redirect.
const AntigravityCallbackPort = antigravity.CallbackPort
