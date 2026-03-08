package cliproxy

import (
	"context"

	"github.com/gin-gonic/gin"
	management "github.com/router-for-me/CLIProxyAPI/v6/internal/api/handlers/management"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func RegisterOAuthSession(state, provider string) {
	management.RegisterOAuthSession(state, provider)
}

func GetOAuthSession(state string) (provider string, status string, ok bool) {
	return management.GetOAuthSession(state)
}

func NormalizeOAuthProvider(provider string) (string, error) {
	return management.NormalizeOAuthProvider(provider)
}

func ValidateOAuthState(state string) error {
	return management.ValidateOAuthState(state)
}

func IsOAuthSessionPending(state, provider string) bool {
	return management.IsOAuthSessionPending(state, provider)
}

func WriteOAuthCallbackFile(authDir, provider, state, code, errorMessage string) (string, error) {
	return management.WriteOAuthCallbackFile(authDir, provider, state, code, errorMessage)
}

// SetOAuthSessionError marks an OAuth session as failed with the given message.
func SetOAuthSessionError(state, message string) {
	management.SetOAuthSessionError(state, message)
}

// CompleteOAuthSession removes a session by state key after successful completion.
func CompleteOAuthSession(state string) {
	management.CompleteOAuthSession(state)
}

// CompleteOAuthSessionsByProvider removes all sessions for the given provider.
func CompleteOAuthSessionsByProvider(provider string) int {
	return management.CompleteOAuthSessionsByProvider(provider)
}

// PopulateAuthContext extracts request info from a gin.Context and adds it
// to the Go context for downstream auth operations.
func PopulateAuthContext(ctx context.Context, c *gin.Context) context.Context {
	info := &coreauth.RequestInfo{
		Query:   c.Request.URL.Query(),
		Headers: c.Request.Header,
	}
	return coreauth.WithRequestInfo(ctx, info)
}
