package cliproxy

import (
	"context"
	"fmt"

	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/auth/copilot"
	sdkconfig "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/config"
)

// CopilotDeviceCode holds the device authorization details for the GitHub Copilot flow.
type CopilotDeviceCode struct {
	DeviceCode      string
	UserCode        string
	VerificationURI string
	ExpiresIn       int
	Interval        int
}

// CopilotTokenData holds the OAuth token returned by GitHub.
type CopilotTokenData struct {
	AccessToken string
	TokenType   string
	Scope       string
}

// CopilotUserInfo holds user profile information from the GitHub API.
type CopilotUserInfo struct {
	Login string
	Email string
	Name  string
}

// CopilotTokenStorage is a re-exported type alias for persistence.
type CopilotTokenStorage = copilot.CopilotTokenStorage

// CopilotAuthProvider wraps the internal GitHub/Copilot device flow client.
type CopilotAuthProvider struct {
	inner *copilot.DeviceFlowClient
}

// NewCopilotAuthProvider creates a CopilotAuthProvider using the SDK config.
func NewCopilotAuthProvider(cfg *sdkconfig.Config) *CopilotAuthProvider {
	return &CopilotAuthProvider{
		inner: copilot.NewDeviceFlowClient(cfg),
	}
}

// RequestDeviceCode initiates the GitHub device authorization flow.
func (p *CopilotAuthProvider) RequestDeviceCode(ctx context.Context) (*CopilotDeviceCode, error) {
	dc, err := p.inner.RequestDeviceCode(ctx)
	if err != nil {
		return nil, err
	}
	return &CopilotDeviceCode{
		DeviceCode:      dc.DeviceCode,
		UserCode:        dc.UserCode,
		VerificationURI: dc.VerificationURI,
		ExpiresIn:       dc.ExpiresIn,
		Interval:        dc.Interval,
	}, nil
}

// PollForToken polls until the user completes GitHub device authorization.
func (p *CopilotAuthProvider) PollForToken(ctx context.Context, dc *CopilotDeviceCode) (*CopilotTokenData, error) {
	internalDC := &copilot.DeviceCodeResponse{
		DeviceCode:      dc.DeviceCode,
		UserCode:        dc.UserCode,
		VerificationURI: dc.VerificationURI,
		ExpiresIn:       dc.ExpiresIn,
		Interval:        dc.Interval,
	}
	td, err := p.inner.PollForToken(ctx, internalDC)
	if err != nil {
		return nil, err
	}
	return &CopilotTokenData{
		AccessToken: td.AccessToken,
		TokenType:   td.TokenType,
		Scope:       td.Scope,
	}, nil
}

// FetchUserInfo fetches the authenticated user's GitHub profile.
func (p *CopilotAuthProvider) FetchUserInfo(ctx context.Context, accessToken string) (*CopilotUserInfo, error) {
	info, err := p.inner.FetchUserInfo(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	return &CopilotUserInfo{
		Login: info.Login,
		Email: info.Email,
		Name:  info.Name,
	}, nil
}

// BuildCopilotTokenStorage builds a CopilotTokenStorage from token data and user info.
func BuildCopilotTokenStorage(td *CopilotTokenData, user *CopilotUserInfo) *CopilotTokenStorage {
	username := user.Login
	if username == "" {
		username = "github-user"
	}
	return &copilot.CopilotTokenStorage{
		AccessToken: td.AccessToken,
		TokenType:   td.TokenType,
		Scope:       td.Scope,
		Username:    username,
		Email:       user.Email,
		Name:        user.Name,
		Type:        "github-copilot",
	}
}

// CopilotCredentialFileName returns the canonical credential file name for a Copilot user.
func CopilotCredentialFileName(username string) string {
	if username == "" {
		username = "github-user"
	}
	return fmt.Sprintf("github-copilot-%s.json", username)
}

// CopilotAPITokenResult holds the result of a Copilot API token exchange.
type CopilotAPITokenResult struct {
	Token       string
	APIEndpoint string
	ExpiresAt   int64
}

// ExchangeCopilotAPIToken exchanges a GitHub access token for a short-lived Copilot API token.
// This performs an HTTP GET to https://api.github.com/copilot_internal/v2/token.
func ExchangeCopilotAPIToken(ctx context.Context, cfg *sdkconfig.Config, githubAccessToken string) (*CopilotAPITokenResult, error) {
	var auth *copilot.CopilotAuth
	if cfg != nil {
		auth = copilot.NewCopilotAuth(cfg)
	} else {
		auth = copilot.NewCopilotAuth(&sdkconfig.Config{})
	}
	apiToken, err := auth.GetCopilotAPIToken(ctx, githubAccessToken)
	if err != nil {
		return nil, fmt.Errorf("copilot token exchange failed: %w", err)
	}
	endpoint := apiToken.Endpoints.API
	if endpoint == "" {
		endpoint = "https://api.githubcopilot.com"
	}
	return &CopilotAPITokenResult{
		Token:       apiToken.Token,
		APIEndpoint: endpoint,
		ExpiresAt:   apiToken.ExpiresAt,
	}, nil
}
