package cliproxy

import (
	kiroauth "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/auth/kiro"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/config"
)

// --- Kiro Auth Provider Seam ---
// Thin wrappers exposing the internal kiro auth package types for first-party use.

// KiroCallbackPort is the port used for Kiro social auth callback.
const KiroCallbackPort = 9876

// KiroAuthServiceEndpoint is the Kiro auth service base URL.
const KiroAuthServiceEndpoint = "https://prod.us-east-1.auth.desktop.kiro.dev"

// KiroRedirectURI is the protocol redirect URI for Kiro auth.
const KiroRedirectURI = kiroauth.KiroRedirectURI

// --- SSO OIDC Client (device code flow: AWS / Builder ID) ---

// KiroSSOOIDCClient wraps the SSO OIDC client for device code flow.
type KiroSSOOIDCClient = kiroauth.SSOOIDCClient

// KiroRegisterClientResponse is the response from RegisterClient.
type KiroRegisterClientResponse = kiroauth.RegisterClientResponse

// KiroStartDeviceAuthResponse is the response from StartDeviceAuthorization.
type KiroStartDeviceAuthResponse = kiroauth.StartDeviceAuthResponse

// KiroSSOCreateTokenResponse is the SSO OIDC CreateToken response.
type KiroSSOCreateTokenResponse = kiroauth.CreateTokenResponse

// NewKiroSSOOIDCClient creates a new SSO OIDC client.
func NewKiroSSOOIDCClient(cfg *config.Config) *KiroSSOOIDCClient {
	return kiroauth.NewSSOOIDCClient(cfg)
}

// --- Social Auth Client (Google / GitHub) ---

// KiroSocialAuthClient wraps the social auth client.
type KiroSocialAuthClient = kiroauth.SocialAuthClient

// KiroCreateTokenRequest is the request to exchange a social auth code for tokens.
type KiroCreateTokenRequest = kiroauth.CreateTokenRequest

// KiroSocialTokenResponse is the response from social auth token exchange.
type KiroSocialTokenResponse = kiroauth.SocialTokenResponse

// NewKiroSocialAuthClient creates a new social auth client.
func NewKiroSocialAuthClient(cfg *config.Config) *KiroSocialAuthClient {
	return kiroauth.NewSocialAuthClient(cfg)
}

// --- Token utilities ---

// KiroExtractEmailFromJWT extracts email from a JWT access token.
func KiroExtractEmailFromJWT(accessToken string) string {
	return kiroauth.ExtractEmailFromJWT(accessToken)
}

// KiroSanitizeEmailForFilename sanitizes an email address for use in filenames.
func KiroSanitizeEmailForFilename(email string) string {
	return kiroauth.SanitizeEmailForFilename(email)
}
