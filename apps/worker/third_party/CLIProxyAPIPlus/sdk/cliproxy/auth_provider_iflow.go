package cliproxy

import (
	"context"

	"github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/corelib/auth/iflow"
	"github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/corelib/config"
)

// --- IFlow Auth Provider Seam ---

// IFlowTokenData holds OAuth/cookie token information from IFlow.
type IFlowTokenData = iflow.IFlowTokenData

// IFlowTokenStorage is the persistent token storage for IFlow credentials.
type IFlowTokenStorage = iflow.IFlowTokenStorage

// IFlowAuthProvider wraps the internal IFlow auth service for first-party use.
type IFlowAuthProvider struct {
	inner *iflow.IFlowAuth
}

// NewIFlowAuthProvider creates a new IFlow auth service using the given config.
func NewIFlowAuthProvider(cfg *config.Config) *IFlowAuthProvider {
	return &IFlowAuthProvider{inner: iflow.NewIFlowAuth(cfg)}
}

// AuthorizationURL builds the OAuth authorization URL and redirect URI for IFlow.
func (p *IFlowAuthProvider) AuthorizationURL(state string, port int) (authURL, redirectURI string) {
	return p.inner.AuthorizationURL(state, port)
}

// ExchangeCodeForTokens exchanges an authorization code for tokens.
func (p *IFlowAuthProvider) ExchangeCodeForTokens(ctx context.Context, code, redirectURI string) (*IFlowTokenData, error) {
	return p.inner.ExchangeCodeForTokens(ctx, code, redirectURI)
}

// CreateTokenStorage creates an IFlowTokenStorage from OAuth token data.
func (p *IFlowAuthProvider) CreateTokenStorage(data *IFlowTokenData) *IFlowTokenStorage {
	return p.inner.CreateTokenStorage(data)
}

// AuthenticateWithCookie performs cookie-based authentication with IFlow.
func (p *IFlowAuthProvider) AuthenticateWithCookie(ctx context.Context, cookie string) (*IFlowTokenData, error) {
	return p.inner.AuthenticateWithCookie(ctx, cookie)
}

// CreateCookieTokenStorage creates an IFlowTokenStorage from cookie auth data.
func (p *IFlowAuthProvider) CreateCookieTokenStorage(data *IFlowTokenData) *IFlowTokenStorage {
	return p.inner.CreateCookieTokenStorage(data)
}

// NormalizeCookie validates and normalizes an IFlow cookie string.
func NormalizeCookie(raw string) (string, error) {
	return iflow.NormalizeCookie(raw)
}

// SanitizeIFlowFileName makes a filename-safe string from the given input.
func SanitizeIFlowFileName(raw string) string {
	return iflow.SanitizeIFlowFileName(raw)
}

// ExtractBXAuth extracts the BXAuth value from a cookie string.
func ExtractBXAuth(cookie string) string {
	return iflow.ExtractBXAuth(cookie)
}

// CheckDuplicateBXAuth checks if a BXAuth value already exists in the auth directory.
func CheckDuplicateBXAuth(authDir, bxAuth string) (string, error) {
	return iflow.CheckDuplicateBXAuth(authDir, bxAuth)
}

// IFlowCallbackPort is the port the IFlow CLI expects for OAuth redirect.
const IFlowCallbackPort = iflow.CallbackPort
