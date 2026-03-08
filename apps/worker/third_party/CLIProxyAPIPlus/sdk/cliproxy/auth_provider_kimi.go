package cliproxy

import (
	"context"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kimi"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

// KimiDeviceFlow holds the device authorization details for the Kimi device code flow.
type KimiDeviceFlow struct {
	DeviceCode              string
	UserCode                string
	VerificationURI         string
	VerificationURIComplete string
	ExpiresIn               int
	Interval                int
}

// KimiAuthBundle holds the result of a successful Kimi device authorization.
type KimiAuthBundle struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	Scope        string
	ExpiresAt    int64
	DeviceID     string
}

// KimiTokenStorage is a re-exported type alias for persistence.
type KimiTokenStorage = kimi.KimiTokenStorage

// KimiAuthProvider wraps the internal Kimi auth service.
type KimiAuthProvider struct {
	inner *kimi.KimiAuth
}

// NewKimiAuthProvider creates a KimiAuthProvider using the SDK config.
func NewKimiAuthProvider(cfg *sdkconfig.Config) *KimiAuthProvider {
	return &KimiAuthProvider{
		inner: kimi.NewKimiAuth(cfg),
	}
}

// StartDeviceFlow initiates the Kimi device authorization flow.
func (p *KimiAuthProvider) StartDeviceFlow(ctx context.Context) (*KimiDeviceFlow, error) {
	df, err := p.inner.StartDeviceFlow(ctx)
	if err != nil {
		return nil, err
	}
	return &KimiDeviceFlow{
		DeviceCode:              df.DeviceCode,
		UserCode:                df.UserCode,
		VerificationURI:         df.VerificationURI,
		VerificationURIComplete: df.VerificationURIComplete,
		ExpiresIn:               df.ExpiresIn,
		Interval:                df.Interval,
	}, nil
}

// WaitForAuthorization polls until the user completes authorization.
func (p *KimiAuthProvider) WaitForAuthorization(ctx context.Context, df *KimiDeviceFlow) (*KimiAuthBundle, error) {
	internalDF := &kimi.DeviceCodeResponse{
		DeviceCode:              df.DeviceCode,
		UserCode:                df.UserCode,
		VerificationURI:         df.VerificationURI,
		VerificationURIComplete: df.VerificationURIComplete,
		ExpiresIn:               df.ExpiresIn,
		Interval:                df.Interval,
	}
	bundle, err := p.inner.WaitForAuthorization(ctx, internalDF)
	if err != nil {
		return nil, err
	}
	return &KimiAuthBundle{
		AccessToken:  bundle.TokenData.AccessToken,
		RefreshToken: bundle.TokenData.RefreshToken,
		TokenType:    bundle.TokenData.TokenType,
		Scope:        bundle.TokenData.Scope,
		ExpiresAt:    bundle.TokenData.ExpiresAt,
		DeviceID:     bundle.DeviceID,
	}, nil
}

// CreateTokenStorage builds a KimiTokenStorage from a KimiAuthBundle.
func (p *KimiAuthProvider) CreateTokenStorage(b *KimiAuthBundle) *KimiTokenStorage {
	internalBundle := &kimi.KimiAuthBundle{
		TokenData: &kimi.KimiTokenData{
			AccessToken:  b.AccessToken,
			RefreshToken: b.RefreshToken,
			TokenType:    b.TokenType,
			Scope:        b.Scope,
			ExpiresAt:    b.ExpiresAt,
		},
		DeviceID: b.DeviceID,
	}
	return p.inner.CreateTokenStorage(internalBundle)
}

// KimiAuthBundleMetadata builds the standard metadata map for a Kimi auth record.
func KimiAuthBundleMetadata(b *KimiAuthBundle) map[string]any {
	metadata := map[string]any{
		"type":          "kimi",
		"access_token":  b.AccessToken,
		"refresh_token": b.RefreshToken,
		"token_type":    b.TokenType,
		"scope":         b.Scope,
		"timestamp":     time.Now().UnixMilli(),
	}
	if b.ExpiresAt > 0 {
		expired := time.Unix(b.ExpiresAt, 0).UTC().Format(time.RFC3339)
		metadata["expired"] = expired
	}
	if strings.TrimSpace(b.DeviceID) != "" {
		metadata["device_id"] = strings.TrimSpace(b.DeviceID)
	}
	return metadata
}
