package cliproxy

import (
	"context"

	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/auth/qwen"
	sdkconfig "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/config"
)

// QwenDeviceFlow holds the device authorization details returned when
// initiating the Qwen device code flow.
type QwenDeviceFlow struct {
	DeviceCode              string
	UserCode                string
	VerificationURI         string
	VerificationURIComplete string
	ExpiresIn               int
	Interval                int
	CodeVerifier            string
}

// QwenTokenData holds OAuth2 credential data returned by the Qwen token endpoint.
type QwenTokenData struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ResourceURL  string
	Expire       string
}

// QwenTokenStorage is a re-exported wrapper so first-party code can reference
// the concrete type without importing internal packages.
type QwenTokenStorage = qwen.QwenTokenStorage

// QwenAuthProvider wraps the internal Qwen auth service so first-party code
// can drive the device-code flow without importing internal packages.
type QwenAuthProvider struct {
	inner *qwen.QwenAuth
}

// NewQwenAuthProvider creates a QwenAuthProvider using the SDK config.
func NewQwenAuthProvider(cfg *sdkconfig.Config) *QwenAuthProvider {
	return &QwenAuthProvider{
		inner: qwen.NewQwenAuth(cfg),
	}
}

// InitiateDeviceFlow starts the OAuth 2.0 device authorization flow and
// returns the device flow details needed by the caller.
func (p *QwenAuthProvider) InitiateDeviceFlow(ctx context.Context) (*QwenDeviceFlow, error) {
	df, err := p.inner.InitiateDeviceFlow(ctx)
	if err != nil {
		return nil, err
	}
	return &QwenDeviceFlow{
		DeviceCode:              df.DeviceCode,
		UserCode:                df.UserCode,
		VerificationURI:         df.VerificationURI,
		VerificationURIComplete: df.VerificationURIComplete,
		ExpiresIn:               df.ExpiresIn,
		Interval:                df.Interval,
		CodeVerifier:            df.CodeVerifier,
	}, nil
}

// PollForToken polls the token endpoint with the device code until the user
// completes authorization or the flow times out.
func (p *QwenAuthProvider) PollForToken(deviceCode, codeVerifier string) (*QwenTokenData, error) {
	td, err := p.inner.PollForToken(deviceCode, codeVerifier)
	if err != nil {
		return nil, err
	}
	return &QwenTokenData{
		AccessToken:  td.AccessToken,
		RefreshToken: td.RefreshToken,
		TokenType:    td.TokenType,
		ResourceURL:  td.ResourceURL,
		Expire:       td.Expire,
	}, nil
}

// CreateTokenStorage builds a QwenTokenStorage suitable for persisting
// via the token store.
func (p *QwenAuthProvider) CreateTokenStorage(td *QwenTokenData) *QwenTokenStorage {
	internal := &qwen.QwenTokenData{
		AccessToken:  td.AccessToken,
		RefreshToken: td.RefreshToken,
		TokenType:    td.TokenType,
		ResourceURL:  td.ResourceURL,
		Expire:       td.Expire,
	}
	return p.inner.CreateTokenStorage(internal)
}
