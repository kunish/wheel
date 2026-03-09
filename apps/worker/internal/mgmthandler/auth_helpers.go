package mgmthandler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	sdkcliproxy "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy"
	sdkcliproxyauth "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

// guardHandler checks that the handler is initialized and returns false if not.
func (h *ManagementHandler) guardHandler(c *gin.Context) bool {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return false
	}
	return true
}

// newAuthContext creates a context derived from the request, populated with auth info.
func newAuthContext(c *gin.Context) context.Context {
	ctx := c.Request.Context()
	return sdkcliproxy.PopulateAuthContext(ctx, c)
}

// saveAndCompleteAuth saves the auth record and completes the OAuth session.
// This is the standard finalisation sequence shared by all auth handlers.
func (h *ManagementHandler) saveAndCompleteAuth(ctx context.Context, state, providerName string, record *sdkcliproxyauth.Auth) {
	_, errSave := sdkcliproxy.SaveTokenRecord(ctx, h.cfg.AuthDir, record, h.postAuthHook)
	if errSave != nil {
		log.Errorf("Failed to save authentication tokens: %v", errSave)
		sdkcliproxy.SetOAuthSessionError(state, "Failed to save authentication tokens")
		return
	}
	sdkcliproxy.CompleteOAuthSession(state)
	sdkcliproxy.CompleteOAuthSessionsByProvider(providerName)
}

// respondAuthURL sends the standard JSON response for a successfully initiated auth flow.
func respondAuthURL(c *gin.Context, authURL, state string) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "url": authURL, "state": state})
}

// respondDeviceFlow sends the JSON response for a device flow auth (includes user_code).
func respondDeviceFlow(c *gin.Context, authURL, state, userCode string) {
	c.JSON(http.StatusOK, gin.H{
		"status":           "ok",
		"url":              authURL,
		"state":            state,
		"user_code":        userCode,
		"verification_uri": authURL,
	})
}

// callbackForwarderSetup holds the state needed for the PKCE callback forwarder lifecycle.
type callbackForwarderSetup struct {
	isWebUI   bool
	forwarder *callbackForwarder
	port      int
}

// startPKCECallbackForwarder conditionally starts a callback forwarder if the request
// comes from WebUI. Returns nil setup (no-op) if the request is not from WebUI.
func (h *ManagementHandler) startPKCECallbackForwarder(c *gin.Context, port int, providerName, callbackPath string) (*callbackForwarderSetup, error) {
	setup := &callbackForwarderSetup{isWebUI: isWebUIRequest(c), port: port}
	if !setup.isWebUI {
		return setup, nil
	}

	targetURL, err := managementCallbackURL(h.cfg.Port, h.cfg.TLS.Enable, callbackPath)
	if err != nil {
		log.WithError(err).Errorf("failed to compute %s callback target", providerName)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "callback server unavailable"})
		return nil, err
	}

	forwarder, err := startCallbackForwarder(port, providerName, targetURL)
	if err != nil {
		log.WithError(err).Errorf("failed to start %s callback forwarder", providerName)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start callback server"})
		return nil, err
	}
	setup.forwarder = forwarder
	return setup, nil
}

// deferStop should be called at the start of the async goroutine to ensure the
// callback forwarder is stopped when the goroutine exits.
func (s *callbackForwarderSetup) deferStop() {
	if s != nil && s.isWebUI {
		stopCallbackForwarderInstance(s.port, s.forwarder)
	}
}
