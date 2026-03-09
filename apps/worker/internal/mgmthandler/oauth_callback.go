package mgmthandler

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	sdkcliproxy "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy"
)

type oauthCallbackRequest struct {
	Provider    string `json:"provider"`
	RedirectURL string `json:"redirect_url"`
	Code        string `json:"code"`
	State       string `json:"state"`
	Error       string `json:"error"`
}

func (h *ManagementHandler) PostOAuthCallback(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "handler not initialized"})
		return
	}

	var req oauthCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid body"})
		return
	}

	canonicalProvider, err := sdkcliproxy.NormalizeOAuthProvider(req.Provider)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "unsupported provider"})
		return
	}

	state := strings.TrimSpace(req.State)
	code := strings.TrimSpace(req.Code)
	errMsg := strings.TrimSpace(req.Error)
	if rawRedirect := strings.TrimSpace(req.RedirectURL); rawRedirect != "" {
		u, errParse := url.Parse(rawRedirect)
		if errParse != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid redirect_url"})
			return
		}
		q := u.Query()
		if state == "" {
			state = strings.TrimSpace(q.Get("state"))
		}
		if code == "" {
			code = strings.TrimSpace(q.Get("code"))
		}
		if errMsg == "" {
			errMsg = strings.TrimSpace(q.Get("error"))
			if errMsg == "" {
				errMsg = strings.TrimSpace(q.Get("error_description"))
			}
		}
	}

	if state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "state is required"})
		return
	}
	if err := sdkcliproxy.ValidateOAuthState(state); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid state"})
		return
	}
	if code == "" && errMsg == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "code or error is required"})
		return
	}

	sessionProvider, sessionStatus, ok := sdkcliproxy.GetOAuthSession(state)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "error": "unknown or expired state"})
		return
	}
	if sessionStatus != "" {
		c.JSON(http.StatusConflict, gin.H{"status": "error", "error": "oauth flow is not pending"})
		return
	}
	if !strings.EqualFold(sessionProvider, canonicalProvider) {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "provider does not match state"})
		return
	}

	if _, err := WriteOAuthCallback(h.cfg.AuthDir, canonicalProvider, state, code, errMsg); err != nil {
		c.JSON(http.StatusConflict, gin.H{"status": "error", "error": "oauth flow is not pending"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *ManagementHandler) GetAuthStatus(c *gin.Context) {
	state := strings.TrimSpace(c.Query("state"))
	if state == "" {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}
	if err := sdkcliproxy.ValidateOAuthState(state); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid state"})
		return
	}

	_, status, ok := sdkcliproxy.GetOAuthSession(state)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}
	if status != "" {
		if strings.HasPrefix(status, "device_code|") {
			parts := strings.SplitN(status, "|", 3)
			if len(parts) == 3 {
				c.JSON(http.StatusOK, gin.H{
					"status":           "device_code",
					"verification_url": parts[1],
					"user_code":        parts[2],
				})
				return
			}
		}
		if strings.HasPrefix(status, "auth_url|") {
			c.JSON(http.StatusOK, gin.H{
				"status": "auth_url",
				"url":    strings.TrimPrefix(status, "auth_url|"),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "error", "error": status})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "wait"})
}

func WriteOAuthCallback(authDir, provider, state, code, errorMessage string) (string, error) {
	canonicalProvider, err := sdkcliproxy.NormalizeOAuthProvider(provider)
	if err != nil {
		return "", err
	}
	if err := sdkcliproxy.ValidateOAuthState(state); err != nil {
		return "", err
	}
	if !sdkcliproxy.IsOAuthSessionPending(state, canonicalProvider) {
		return "", fmt.Errorf("oauth session is not pending")
	}
	return sdkcliproxy.WriteOAuthCallbackFile(authDir, canonicalProvider, strings.TrimSpace(state), strings.TrimSpace(code), strings.TrimSpace(errorMessage))
}
