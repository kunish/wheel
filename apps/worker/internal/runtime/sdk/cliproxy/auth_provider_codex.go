package cliproxy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/auth/codex"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/config"
)

// GenerateRandomState produces a cryptographic random hex string suitable
// for OAuth state parameters.
func GenerateRandomState() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// --- Codex Auth Provider Seam ---

// CodexPKCECodes wraps the PKCE verifier and challenge for Codex OAuth.
type CodexPKCECodes = codex.PKCECodes

// CodexTokenData holds OAuth token information from OpenAI.
type CodexTokenData = codex.CodexTokenData

// CodexAuthBundle aggregates all auth data after the Codex OAuth flow.
type CodexAuthBundle = codex.CodexAuthBundle

// CodexTokenStorage is the persistent token storage for Codex credentials.
type CodexTokenStorage = codex.CodexTokenStorage

// CodexJWTClaims holds parsed JWT claims from the Codex ID token.
type CodexJWTClaims = codex.JWTClaims

// CodexAuthProvider wraps the internal Codex auth service for first-party use.
type CodexAuthProvider struct {
	inner *codex.CodexAuth
}

// NewCodexAuthProvider creates a new Codex auth service using the given config.
func NewCodexAuthProvider(cfg *config.Config) *CodexAuthProvider {
	return &CodexAuthProvider{inner: codex.NewCodexAuth(cfg)}
}

// GeneratePKCECodes creates a new PKCE verifier and challenge pair.
func (p *CodexAuthProvider) GeneratePKCECodes() (*CodexPKCECodes, error) {
	return codex.GeneratePKCECodes()
}

// GenerateAuthURL builds the OAuth authorization URL for Codex.
func (p *CodexAuthProvider) GenerateAuthURL(state string, pkceCodes *CodexPKCECodes) (string, error) {
	return p.inner.GenerateAuthURL(state, pkceCodes)
}

// ExchangeCodeForTokens exchanges an authorization code for tokens.
func (p *CodexAuthProvider) ExchangeCodeForTokens(ctx context.Context, code string, pkceCodes *CodexPKCECodes) (*CodexAuthBundle, error) {
	return p.inner.ExchangeCodeForTokens(ctx, code, pkceCodes)
}

// CreateTokenStorage creates a CodexTokenStorage from an auth bundle.
func (p *CodexAuthProvider) CreateTokenStorage(bundle *CodexAuthBundle) *CodexTokenStorage {
	return p.inner.CreateTokenStorage(bundle)
}

// ParseCodexJWTToken parses a Codex ID token and returns the claims.
func ParseCodexJWTToken(token string) (*CodexJWTClaims, error) {
	return codex.ParseJWTToken(token)
}

// CodexCredentialFileName generates the credential filename for Codex.
func CodexCredentialFileName(email, planType, hashAccountID string, includeProviderPrefix bool) string {
	return codex.CredentialFileName(email, planType, hashAccountID, includeProviderPrefix)
}

// CodexCallbackPort is the port the Codex CLI expects for OAuth redirect.
const CodexCallbackPort = 1455
