package cliproxy

import (
	"context"
	"fmt"

	sdkAuth "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/auth"
	coreauth "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy/auth"
)

// TokenStore returns the global token store singleton with its base directory
// set to authDir. The caller can then call Save/List/Delete on the returned store.
func TokenStore(authDir string) coreauth.Store {
	store := sdkAuth.GetTokenStore()
	if store == nil {
		return nil
	}
	if authDir != "" {
		if dirSetter, ok := store.(interface{ SetBaseDir(string) }); ok {
			dirSetter.SetBaseDir(authDir)
		}
	}
	return store
}

// SaveTokenRecord persists an auth record via the global token store,
// optionally calling the provided post-auth hook before writing.
func SaveTokenRecord(ctx context.Context, authDir string, record *coreauth.Auth, hook coreauth.PostAuthHook) (string, error) {
	if record == nil {
		return "", fmt.Errorf("token record is nil")
	}
	store := TokenStore(authDir)
	if store == nil {
		return "", fmt.Errorf("token store unavailable")
	}
	if hook != nil {
		if err := hook(ctx, record); err != nil {
			return "", fmt.Errorf("post-auth hook failed: %w", err)
		}
	}
	return store.Save(ctx, record)
}
