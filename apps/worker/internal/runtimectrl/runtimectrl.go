package runtimectrl

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	sdkcliproxy "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy"
	sdkcliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
	log "github.com/sirupsen/logrus"
)

type ManagementHandler struct {
	cfg          *sdkconfig.Config
	inner        sdkcliproxy.ManagementHandlerRoutes
	postAuthHook sdkcliproxyauth.PostAuthHook
}

func NewManagementHandler(cfg *sdkconfig.Config, configFilePath string, authManager *sdkcliproxyauth.Manager) *ManagementHandler {
	return &ManagementHandler{cfg: cfg, inner: sdkcliproxy.DefaultManagementHandlerFactory(cfg, configFilePath, authManager)}
}

type oauthRouteRegistrarWithoutCallback interface {
	RegisterRoutesWithoutOAuthCallback(*gin.RouterGroup)
}

type oauthRouteRegistrarWithoutSessionRoutes interface {
	RegisterRoutesWithoutOAuthSessionRoutes(*gin.RouterGroup)
}

type codexAuthURLRequester interface {
	RequestCodexToken(*gin.Context)
}

type gitHubAuthURLRequester interface {
	RequestGitHubToken(*gin.Context)
}

type anthropicAuthURLRequester interface {
	RequestAnthropicToken(*gin.Context)
}

type geminiCLIAuthURLRequester interface {
	RequestGeminiCLIToken(*gin.Context)
}

type antigravityAuthURLRequester interface {
	RequestAntigravityToken(*gin.Context)
}

type qwenAuthURLRequester interface {
	RequestQwenToken(*gin.Context)
}

type kiloAuthURLRequester interface {
	RequestKiloToken(*gin.Context)
}

type kimiAuthURLRequester interface {
	RequestKimiToken(*gin.Context)
}

type iflowAuthURLRequester interface {
	RequestIFlowToken(*gin.Context)
}

type iflowCookieAuthURLRequester interface {
	RequestIFlowCookieToken(*gin.Context)
}

type kiroAuthURLRequester interface {
	RequestKiroToken(*gin.Context)
}

type oauthCallbackRequest struct {
	Provider    string `json:"provider"`
	RedirectURL string `json:"redirect_url"`
	Code        string `json:"code"`
	State       string `json:"state"`
	Error       string `json:"error"`
}

func (h *ManagementHandler) Middleware() gin.HandlerFunc {
	if h == nil || h.inner == nil {
		return func(c *gin.Context) {
			c.Abort()
		}
	}
	return h.inner.Middleware()
}

func (h *ManagementHandler) RegisterRoutes(group *gin.RouterGroup) {
	if h == nil || h.inner == nil {
		return
	}
	if registrar, ok := h.inner.(oauthRouteRegistrarWithoutSessionRoutes); ok {
		registrar.RegisterRoutesWithoutOAuthSessionRoutes(group)
		group.GET("/anthropic-auth-url", h.RequestAnthropicToken)
		group.GET("/codex-auth-url", h.RequestCodexToken)
		group.GET("/github-auth-url", h.RequestGitHubToken)
		group.GET("/gemini-cli-auth-url", h.RequestGeminiCLIToken)
		group.GET("/antigravity-auth-url", h.RequestAntigravityToken)
		group.GET("/qwen-auth-url", h.RequestQwenToken)
		group.GET("/kilo-auth-url", h.RequestKiloToken)
		group.GET("/kimi-auth-url", h.RequestKimiToken)
		group.GET("/iflow-auth-url", h.RequestIFlowToken)
		group.POST("/iflow-auth-url", h.RequestIFlowCookieToken)
		group.GET("/kiro-auth-url", h.RequestKiroToken)
		group.POST("/oauth-callback", h.PostOAuthCallback)
		group.GET("/get-auth-status", h.GetAuthStatus)
		return
	}
	if registrar, ok := h.inner.(oauthRouteRegistrarWithoutCallback); ok {
		registrar.RegisterRoutesWithoutOAuthCallback(group)
		group.GET("/anthropic-auth-url", h.RequestAnthropicToken)
		group.GET("/codex-auth-url", h.RequestCodexToken)
		group.GET("/github-auth-url", h.RequestGitHubToken)
		group.GET("/gemini-cli-auth-url", h.RequestGeminiCLIToken)
		group.GET("/antigravity-auth-url", h.RequestAntigravityToken)
		group.GET("/qwen-auth-url", h.RequestQwenToken)
		group.GET("/kilo-auth-url", h.RequestKiloToken)
		group.GET("/kimi-auth-url", h.RequestKimiToken)
		group.GET("/iflow-auth-url", h.RequestIFlowToken)
		group.POST("/iflow-auth-url", h.RequestIFlowCookieToken)
		group.GET("/kiro-auth-url", h.RequestKiroToken)
		group.POST("/oauth-callback", h.PostOAuthCallback)
		group.GET("/get-auth-status", h.GetAuthStatus)
		return
	}
	h.inner.RegisterRoutes(group)
}

func (h *ManagementHandler) SetConfig(cfg *sdkconfig.Config) {
	h.cfg = cfg
	if h == nil || h.inner == nil {
		return
	}
	h.inner.SetConfig(cfg)
}

func (h *ManagementHandler) SetAuthManager(manager *sdkcliproxyauth.Manager) {
	if h == nil || h.inner == nil {
		return
	}
	h.inner.SetAuthManager(manager)
}

func (h *ManagementHandler) SetLocalPassword(password string) {
	if h == nil || h.inner == nil {
		return
	}
	h.inner.SetLocalPassword(password)
}

func (h *ManagementHandler) SetLogDirectory(dir string) {
	if h == nil || h.inner == nil {
		return
	}
	h.inner.SetLogDirectory(dir)
}

func (h *ManagementHandler) SetPostAuthHook(hook sdkcliproxyauth.PostAuthHook) {
	h.postAuthHook = hook
	if h == nil || h.inner == nil {
		return
	}
	h.inner.SetPostAuthHook(hook)
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

func (h *ManagementHandler) RequestCodexToken(c *gin.Context) {
	requester, ok := h.inner.(codexAuthURLRequester)
	if !ok || requester == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	requester.RequestCodexToken(c)
}

func (h *ManagementHandler) RequestGitHubToken(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}

	ctx := context.Background()
	state := fmt.Sprintf("gh-%d", time.Now().UnixNano())

	deviceClient := sdkcliproxy.NewCopilotAuthProvider(h.cfg)
	deviceCode, err := deviceClient.RequestDeviceCode(ctx)
	if err != nil {
		log.Errorf("Failed to initiate device flow: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to initiate device flow"})
		return
	}

	authURL := deviceCode.VerificationURI
	userCode := deviceCode.UserCode

	sdkcliproxy.RegisterOAuthSession(state, "github-copilot")

	go func() {
		tokenData, errPoll := deviceClient.PollForToken(ctx, deviceCode)
		if errPoll != nil {
			sdkcliproxy.SetOAuthSessionError(state, "Authentication failed")
			log.Errorf("GitHub Copilot authentication failed: %v", errPoll)
			return
		}

		userInfo, errUser := deviceClient.FetchUserInfo(ctx, tokenData.AccessToken)
		if errUser != nil {
			log.Warnf("Failed to fetch user info: %v", errUser)
			userInfo = &sdkcliproxy.CopilotUserInfo{Login: "github-user"}
		}

		username := userInfo.Login
		if username == "" {
			username = "github-user"
		}

		tokenStorage := sdkcliproxy.BuildCopilotTokenStorage(tokenData, userInfo)
		fileName := sdkcliproxy.CopilotCredentialFileName(username)
		label := userInfo.Email
		if label == "" {
			label = username
		}
		record := &sdkcliproxyauth.Auth{
			ID:       fileName,
			Provider: "github-copilot",
			Label:    label,
			FileName: fileName,
			Storage:  tokenStorage,
			Metadata: map[string]any{
				"email":    userInfo.Email,
				"username": username,
				"name":     userInfo.Name,
			},
		}

		_, errSave := sdkcliproxy.SaveTokenRecord(ctx, h.cfg.AuthDir, record, h.postAuthHook)
		if errSave != nil {
			log.Errorf("Failed to save authentication tokens: %v", errSave)
			sdkcliproxy.SetOAuthSessionError(state, "Failed to save authentication tokens")
			return
		}
		sdkcliproxy.CompleteOAuthSession(state)
		sdkcliproxy.CompleteOAuthSessionsByProvider("github-copilot")
	}()

	c.JSON(http.StatusOK, gin.H{
		"status":           "ok",
		"url":              authURL,
		"state":            state,
		"user_code":        userCode,
		"verification_uri": authURL,
	})
}

func (h *ManagementHandler) RequestAnthropicToken(c *gin.Context) {
	requester, ok := h.inner.(anthropicAuthURLRequester)
	if !ok || requester == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	requester.RequestAnthropicToken(c)
}

func (h *ManagementHandler) RequestGeminiCLIToken(c *gin.Context) {
	requester, ok := h.inner.(geminiCLIAuthURLRequester)
	if !ok || requester == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	requester.RequestGeminiCLIToken(c)
}

func (h *ManagementHandler) RequestAntigravityToken(c *gin.Context) {
	requester, ok := h.inner.(antigravityAuthURLRequester)
	if !ok || requester == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	requester.RequestAntigravityToken(c)
}

func (h *ManagementHandler) RequestQwenToken(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}

	ctx := context.Background()
	ctx = sdkcliproxy.PopulateAuthContext(ctx, c)

	state := fmt.Sprintf("gem-%d", time.Now().UnixNano())

	qwenAuth := sdkcliproxy.NewQwenAuthProvider(h.cfg)
	deviceFlow, err := qwenAuth.InitiateDeviceFlow(ctx)
	if err != nil {
		log.Errorf("Failed to generate authorization URL: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate authorization url"})
		return
	}

	authURL := deviceFlow.VerificationURIComplete
	sdkcliproxy.RegisterOAuthSession(state, "qwen")

	go func() {
		tokenData, errPoll := qwenAuth.PollForToken(deviceFlow.DeviceCode, deviceFlow.CodeVerifier)
		if errPoll != nil {
			sdkcliproxy.SetOAuthSessionError(state, "Authentication failed")
			log.Errorf("Qwen authentication failed: %v", errPoll)
			return
		}

		tokenStorage := qwenAuth.CreateTokenStorage(tokenData)
		tokenStorage.Email = fmt.Sprintf("%d", time.Now().UnixMilli())
		record := &sdkcliproxyauth.Auth{
			ID:       fmt.Sprintf("qwen-%s.json", tokenStorage.Email),
			Provider: "qwen",
			FileName: fmt.Sprintf("qwen-%s.json", tokenStorage.Email),
			Storage:  tokenStorage,
			Metadata: map[string]any{"email": tokenStorage.Email},
		}
		_, errSave := sdkcliproxy.SaveTokenRecord(ctx, h.cfg.AuthDir, record, h.postAuthHook)
		if errSave != nil {
			log.Errorf("Failed to save authentication tokens: %v", errSave)
			sdkcliproxy.SetOAuthSessionError(state, "Failed to save authentication tokens")
			return
		}
		sdkcliproxy.CompleteOAuthSession(state)
	}()

	c.JSON(http.StatusOK, gin.H{"status": "ok", "url": authURL, "state": state})
}

func (h *ManagementHandler) RequestKiloToken(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}

	ctx := context.Background()
	state := fmt.Sprintf("kil-%d", time.Now().UnixNano())

	kiloAuth := sdkcliproxy.NewKiloAuthProvider()
	resp, err := kiloAuth.InitiateDeviceFlow(ctx)
	if err != nil {
		log.Errorf("Failed to initiate device flow: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to initiate device flow"})
		return
	}

	sdkcliproxy.RegisterOAuthSession(state, "kilo")

	go func() {
		status, errPoll := kiloAuth.PollForToken(ctx, resp.Code)
		if errPoll != nil {
			sdkcliproxy.SetOAuthSessionError(state, "Authentication failed")
			log.Errorf("Kilo authentication failed: %v", errPoll)
			return
		}

		profile, errProfile := kiloAuth.GetProfile(ctx, status.Token)
		if errProfile != nil {
			log.Warnf("Failed to fetch profile: %v", errProfile)
			profile = &sdkcliproxy.KiloProfile{Email: status.UserEmail}
		}

		var orgID string
		if len(profile.Orgs) > 0 {
			orgID = profile.Orgs[0].ID
		}

		defaults, errDefaults := kiloAuth.GetDefaults(ctx, status.Token, orgID)
		if errDefaults != nil {
			defaults = &sdkcliproxy.KiloDefaults{}
		}

		ts := &sdkcliproxy.KiloTokenStorage{
			Token:          status.Token,
			OrganizationID: orgID,
			Model:          defaults.Model,
			Email:          status.UserEmail,
			Type:           "kilo",
		}

		fileName := sdkcliproxy.KiloCredentialFileName(status.UserEmail)
		record := &sdkcliproxyauth.Auth{
			ID:       fileName,
			Provider: "kilo",
			FileName: fileName,
			Storage:  ts,
			Metadata: map[string]any{
				"email":           status.UserEmail,
				"organization_id": orgID,
				"model":           defaults.Model,
			},
		}

		_, errSave := sdkcliproxy.SaveTokenRecord(ctx, h.cfg.AuthDir, record, h.postAuthHook)
		if errSave != nil {
			log.Errorf("Failed to save authentication tokens: %v", errSave)
			sdkcliproxy.SetOAuthSessionError(state, "Failed to save authentication tokens")
			return
		}
		sdkcliproxy.CompleteOAuthSession(state)
		sdkcliproxy.CompleteOAuthSessionsByProvider("kilo")
	}()

	c.JSON(http.StatusOK, gin.H{
		"status":           "ok",
		"url":              resp.VerificationURL,
		"state":            state,
		"user_code":        resp.Code,
		"verification_uri": resp.VerificationURL,
	})
}

func (h *ManagementHandler) RequestKimiToken(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}

	ctx := context.Background()
	ctx = sdkcliproxy.PopulateAuthContext(ctx, c)

	state := fmt.Sprintf("kmi-%d", time.Now().UnixNano())
	kimiAuth := sdkcliproxy.NewKimiAuthProvider(h.cfg)

	deviceFlow, err := kimiAuth.StartDeviceFlow(ctx)
	if err != nil {
		log.Errorf("Failed to generate authorization URL: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate authorization url"})
		return
	}

	authURL := deviceFlow.VerificationURIComplete
	if authURL == "" {
		authURL = deviceFlow.VerificationURI
	}

	sdkcliproxy.RegisterOAuthSession(state, "kimi")

	go func() {
		bundle, errWait := kimiAuth.WaitForAuthorization(ctx, deviceFlow)
		if errWait != nil {
			sdkcliproxy.SetOAuthSessionError(state, "Authentication failed")
			log.Errorf("Kimi authentication failed: %v", errWait)
			return
		}

		tokenStorage := kimiAuth.CreateTokenStorage(bundle)
		metadata := sdkcliproxy.KimiAuthBundleMetadata(bundle)

		fileName := fmt.Sprintf("kimi-%d.json", time.Now().UnixMilli())
		record := &sdkcliproxyauth.Auth{
			ID:       fileName,
			Provider: "kimi",
			FileName: fileName,
			Label:    "Kimi User",
			Storage:  tokenStorage,
			Metadata: metadata,
		}

		_, errSave := sdkcliproxy.SaveTokenRecord(ctx, h.cfg.AuthDir, record, h.postAuthHook)
		if errSave != nil {
			log.Errorf("Failed to save authentication tokens: %v", errSave)
			sdkcliproxy.SetOAuthSessionError(state, "Failed to save authentication tokens")
			return
		}
		sdkcliproxy.CompleteOAuthSession(state)
		sdkcliproxy.CompleteOAuthSessionsByProvider("kimi")
	}()

	c.JSON(http.StatusOK, gin.H{"status": "ok", "url": authURL, "state": state})
}

func (h *ManagementHandler) RequestIFlowToken(c *gin.Context) {
	requester, ok := h.inner.(iflowAuthURLRequester)
	if !ok || requester == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	requester.RequestIFlowToken(c)
}

func (h *ManagementHandler) RequestIFlowCookieToken(c *gin.Context) {
	requester, ok := h.inner.(iflowCookieAuthURLRequester)
	if !ok || requester == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	requester.RequestIFlowCookieToken(c)
}

func (h *ManagementHandler) RequestKiroToken(c *gin.Context) {
	requester, ok := h.inner.(kiroAuthURLRequester)
	if !ok || requester == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	requester.RequestKiroToken(c)
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
