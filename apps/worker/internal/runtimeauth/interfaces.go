package runtimeauth

import (
	"context"
	"errors"
	"time"

	coreauth "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy/auth"
	runtimeconfig "github.com/kunish/wheel/apps/worker/internal/runtimecore/config"
)

var errRefreshNotSupported = errors.New("cliproxy auth: refresh not supported")

// loginOptions captures generic knobs shared across authenticators.
type loginOptions struct {
	NoBrowser    bool
	ProjectID    string
	CallbackPort int
	Metadata     map[string]string
	Prompt       func(prompt string) (string, error)
}

// authenticator manages login and optional refresh flows for a provider.
type authenticator interface {
	Provider() string
	Login(ctx context.Context, cfg *runtimeconfig.Config, opts *loginOptions) (*coreauth.Auth, error)
	RefreshLead() *time.Duration
}
