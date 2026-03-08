package cliproxy

import (
	"context"

	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/auth/kilo"
)

// KiloDeviceFlow holds the device authorization details for the Kilo device code flow.
type KiloDeviceFlow struct {
	VerificationURL string
	Code            string
}

// KiloTokenStatus holds the result of a successful Kilo device token poll.
type KiloTokenStatus struct {
	Token     string
	UserEmail string
}

// KiloProfile holds user profile information returned by the Kilo API.
type KiloProfile struct {
	Email string
	Orgs  []KiloOrganization
}

// KiloOrganization represents a Kilo organization.
type KiloOrganization struct {
	ID   string
	Name string
}

// KiloDefaults holds the default model/org returned by the Kilo API.
type KiloDefaults struct {
	Model string
}

// KiloTokenStorage is a re-exported type alias for persistence.
type KiloTokenStorage = kilo.KiloTokenStorage

// KiloCredentialFileName returns the canonical credential file name for a Kilo user.
func KiloCredentialFileName(email string) string {
	return kilo.CredentialFileName(email)
}

// KiloAuthProvider wraps the internal Kilo auth service.
type KiloAuthProvider struct {
	inner *kilo.KiloAuth
}

// NewKiloAuthProvider creates a KiloAuthProvider.
func NewKiloAuthProvider() *KiloAuthProvider {
	return &KiloAuthProvider{
		inner: kilo.NewKiloAuth(),
	}
}

// InitiateDeviceFlow starts the Kilo device authorization flow.
func (p *KiloAuthProvider) InitiateDeviceFlow(ctx context.Context) (*KiloDeviceFlow, error) {
	resp, err := p.inner.InitiateDeviceFlow(ctx)
	if err != nil {
		return nil, err
	}
	return &KiloDeviceFlow{
		VerificationURL: resp.VerificationURL,
		Code:            resp.Code,
	}, nil
}

// PollForToken polls until the user completes device authorization.
func (p *KiloAuthProvider) PollForToken(ctx context.Context, code string) (*KiloTokenStatus, error) {
	status, err := p.inner.PollForToken(ctx, code)
	if err != nil {
		return nil, err
	}
	return &KiloTokenStatus{
		Token:     status.Token,
		UserEmail: status.UserEmail,
	}, nil
}

// GetProfile fetches the user profile from the Kilo API.
func (p *KiloAuthProvider) GetProfile(ctx context.Context, token string) (*KiloProfile, error) {
	profile, err := p.inner.GetProfile(ctx, token)
	if err != nil {
		return nil, err
	}
	result := &KiloProfile{Email: profile.Email}
	for _, org := range profile.Orgs {
		result.Orgs = append(result.Orgs, KiloOrganization{ID: org.ID, Name: org.Name})
	}
	return result, nil
}

// GetDefaults fetches the default model and org settings from the Kilo API.
func (p *KiloAuthProvider) GetDefaults(ctx context.Context, token, orgID string) (*KiloDefaults, error) {
	defaults, err := p.inner.GetDefaults(ctx, token, orgID)
	if err != nil {
		return nil, err
	}
	return &KiloDefaults{Model: defaults.Model}, nil
}
