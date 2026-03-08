package runtimeauth

import (
	"context"
	"errors"
	"time"

	runtimeconfig "github.com/kunish/wheel/apps/worker/internal/runtimecore/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

var ErrRefreshNotSupported = errors.New("cliproxy auth: refresh not supported")

// LoginOptions captures generic knobs shared across authenticators.
type LoginOptions struct {
	NoBrowser    bool
	ProjectID    string
	CallbackPort int
	Metadata     map[string]string
	Prompt       func(prompt string) (string, error)
}

// Authenticator manages login and optional refresh flows for a provider.
type Authenticator interface {
	Provider() string
	Login(ctx context.Context, cfg *runtimeconfig.Config, opts *LoginOptions) (*coreauth.Auth, error)
	RefreshLead() *time.Duration
}
