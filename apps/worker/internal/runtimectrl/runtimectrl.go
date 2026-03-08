package runtimectrl

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	sdkconfig "github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/corelib/config"
	sdkcliproxy "github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/sdk/cliproxy"
	sdkcliproxyauth "github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/sdk/cliproxy/auth"
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

type geminiCLIAuthURLRequester interface {
	RequestGeminiCLIToken(*gin.Context)
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
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}

	ctx := context.Background()
	ctx = sdkcliproxy.PopulateAuthContext(ctx, c)

	codexAuth := sdkcliproxy.NewCodexAuthProvider(h.cfg)

	pkceCodes, err := codexAuth.GeneratePKCECodes()
	if err != nil {
		log.Errorf("Failed to generate PKCE codes: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate PKCE codes"})
		return
	}

	state, err := sdkcliproxy.GenerateRandomState()
	if err != nil {
		log.Errorf("Failed to generate state parameter: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate state parameter"})
		return
	}

	authURL, err := codexAuth.GenerateAuthURL(state, pkceCodes)
	if err != nil {
		log.Errorf("Failed to generate authorization URL: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate authorization url"})
		return
	}

	sdkcliproxy.RegisterOAuthSession(state, "codex")

	isWebUI := isWebUIRequest(c)
	var forwarder *callbackForwarder
	if isWebUI {
		targetURL, errTarget := managementCallbackURL(h.cfg.Port, h.cfg.TLS.Enable, "/codex/callback")
		if errTarget != nil {
			log.WithError(errTarget).Error("failed to compute codex callback target")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "callback server unavailable"})
			return
		}
		if forwarder, err = startCallbackForwarder(sdkcliproxy.CodexCallbackPort, "codex", targetURL); err != nil {
			log.WithError(err).Error("failed to start codex callback forwarder")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start callback server"})
			return
		}
	}

	go func() {
		if isWebUI {
			defer stopCallbackForwarderInstance(sdkcliproxy.CodexCallbackPort, forwarder)
		}

		result, errPoll := pollForCallbackFile(h.cfg.AuthDir, "codex", state)
		if errPoll != nil {
			log.Errorf("Codex OAuth callback failed: %v", errPoll)
			return
		}

		bundle, errExchange := codexAuth.ExchangeCodeForTokens(ctx, result.Code, pkceCodes)
		if errExchange != nil {
			sdkcliproxy.SetOAuthSessionError(state, "Failed to exchange authorization code for tokens")
			log.Errorf("Failed to exchange authorization code for tokens: %v", errExchange)
			return
		}

		claims, _ := sdkcliproxy.ParseCodexJWTToken(bundle.TokenData.IDToken)
		planType := ""
		hashAccountID := ""
		if claims != nil {
			planType = strings.TrimSpace(claims.CodexAuthInfo.ChatgptPlanType)
			if accountID := claims.GetAccountID(); accountID != "" {
				digest := sha256.Sum256([]byte(accountID))
				hashAccountID = hex.EncodeToString(digest[:])[:8]
			}
		}

		tokenStorage := codexAuth.CreateTokenStorage(bundle)
		fileName := sdkcliproxy.CodexCredentialFileName(tokenStorage.Email, planType, hashAccountID, true)
		record := &sdkcliproxyauth.Auth{
			ID:       fileName,
			Provider: "codex",
			FileName: fileName,
			Storage:  tokenStorage,
			Metadata: map[string]any{
				"email":      tokenStorage.Email,
				"account_id": tokenStorage.AccountID,
			},
		}

		_, errSave := sdkcliproxy.SaveTokenRecord(ctx, h.cfg.AuthDir, record, h.postAuthHook)
		if errSave != nil {
			sdkcliproxy.SetOAuthSessionError(state, "Failed to save authentication tokens")
			log.Errorf("Failed to save authentication tokens: %v", errSave)
			return
		}
		sdkcliproxy.CompleteOAuthSession(state)
		sdkcliproxy.CompleteOAuthSessionsByProvider("codex")
	}()

	c.JSON(http.StatusOK, gin.H{"status": "ok", "url": authURL, "state": state})
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
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}

	ctx := context.Background()
	ctx = sdkcliproxy.PopulateAuthContext(ctx, c)

	anthropicAuth := sdkcliproxy.NewAnthropicAuthProvider(h.cfg)

	pkceCodes, err := anthropicAuth.GeneratePKCECodes()
	if err != nil {
		log.Errorf("Failed to generate PKCE codes: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate PKCE codes"})
		return
	}

	state, err := sdkcliproxy.GenerateRandomState()
	if err != nil {
		log.Errorf("Failed to generate state parameter: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate state parameter"})
		return
	}

	authURL, _, err := anthropicAuth.GenerateAuthURL(state, pkceCodes)
	if err != nil {
		log.Errorf("Failed to generate authorization URL: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate authorization url"})
		return
	}

	sdkcliproxy.RegisterOAuthSession(state, "anthropic")

	isWebUI := isWebUIRequest(c)
	var forwarder *callbackForwarder
	if isWebUI {
		targetURL, errTarget := managementCallbackURL(h.cfg.Port, h.cfg.TLS.Enable, "/anthropic/callback")
		if errTarget != nil {
			log.WithError(errTarget).Error("failed to compute anthropic callback target")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "callback server unavailable"})
			return
		}
		if forwarder, err = startCallbackForwarder(sdkcliproxy.AnthropicCallbackPort, "anthropic", targetURL); err != nil {
			log.WithError(err).Error("failed to start anthropic callback forwarder")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start callback server"})
			return
		}
	}

	go func() {
		if isWebUI {
			defer stopCallbackForwarderInstance(sdkcliproxy.AnthropicCallbackPort, forwarder)
		}

		result, errPoll := pollForCallbackFile(h.cfg.AuthDir, "anthropic", state)
		if errPoll != nil {
			log.Errorf("Anthropic OAuth callback failed: %v", errPoll)
			return
		}

		bundle, errExchange := anthropicAuth.ExchangeCodeForTokens(ctx, result.Code, state, pkceCodes)
		if errExchange != nil {
			sdkcliproxy.SetOAuthSessionError(state, "Failed to exchange authorization code for tokens")
			log.Errorf("Failed to exchange authorization code for tokens: %v", errExchange)
			return
		}

		tokenStorage := anthropicAuth.CreateTokenStorage(bundle)
		fileName := fmt.Sprintf("claude-%s.json", tokenStorage.Email)
		record := &sdkcliproxyauth.Auth{
			ID:       fileName,
			Provider: "claude",
			FileName: fileName,
			Storage:  tokenStorage,
			Metadata: map[string]any{
				"email": tokenStorage.Email,
			},
		}

		_, errSave := sdkcliproxy.SaveTokenRecord(ctx, h.cfg.AuthDir, record, h.postAuthHook)
		if errSave != nil {
			sdkcliproxy.SetOAuthSessionError(state, "Failed to save authentication tokens")
			log.Errorf("Failed to save authentication tokens: %v", errSave)
			return
		}
		sdkcliproxy.CompleteOAuthSession(state)
		sdkcliproxy.CompleteOAuthSessionsByProvider("anthropic")
	}()

	c.JSON(http.StatusOK, gin.H{"status": "ok", "url": authURL, "state": state})
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
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}

	ctx := context.Background()
	ctx = sdkcliproxy.PopulateAuthContext(ctx, c)

	authSvc := sdkcliproxy.NewAntigravityAuthProvider(h.cfg)

	state, err := sdkcliproxy.GenerateRandomState()
	if err != nil {
		log.Errorf("Failed to generate state parameter: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate state parameter"})
		return
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/oauth-callback", sdkcliproxy.AntigravityCallbackPort)
	authURL := authSvc.BuildAuthURL(state, redirectURI)

	sdkcliproxy.RegisterOAuthSession(state, "antigravity")

	isWebUI := isWebUIRequest(c)
	var forwarder *callbackForwarder
	if isWebUI {
		targetURL, errTarget := managementCallbackURL(h.cfg.Port, h.cfg.TLS.Enable, "/antigravity/callback")
		if errTarget != nil {
			log.WithError(errTarget).Error("failed to compute antigravity callback target")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "callback server unavailable"})
			return
		}
		if forwarder, err = startCallbackForwarder(sdkcliproxy.AntigravityCallbackPort, "antigravity", targetURL); err != nil {
			log.WithError(err).Error("failed to start antigravity callback forwarder")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start callback server"})
			return
		}
	}

	go func() {
		if isWebUI {
			defer stopCallbackForwarderInstance(sdkcliproxy.AntigravityCallbackPort, forwarder)
		}

		result, errPoll := pollForCallbackFile(h.cfg.AuthDir, "antigravity", state)
		if errPoll != nil {
			log.Errorf("Antigravity OAuth callback failed: %v", errPoll)
			return
		}

		tokenResp, errToken := authSvc.ExchangeCodeForTokens(ctx, result.Code, redirectURI)
		if errToken != nil {
			sdkcliproxy.SetOAuthSessionError(state, "Failed to exchange token")
			log.Errorf("Failed to exchange token: %v", errToken)
			return
		}

		accessToken := strings.TrimSpace(tokenResp.AccessToken)
		if accessToken == "" {
			sdkcliproxy.SetOAuthSessionError(state, "Failed to exchange token")
			log.Error("antigravity: token exchange returned empty access token")
			return
		}

		email, errInfo := authSvc.FetchUserInfo(ctx, accessToken)
		if errInfo != nil {
			sdkcliproxy.SetOAuthSessionError(state, "Failed to fetch user info")
			log.Errorf("Failed to fetch user info: %v", errInfo)
			return
		}
		email = strings.TrimSpace(email)
		if email == "" {
			sdkcliproxy.SetOAuthSessionError(state, "Failed to fetch user info")
			log.Error("antigravity: user info returned empty email")
			return
		}

		projectID := ""
		fetchedProjectID, errProject := authSvc.FetchProjectID(ctx, accessToken)
		if errProject != nil {
			log.Warnf("antigravity: failed to fetch project ID: %v", errProject)
		} else {
			projectID = fetchedProjectID
		}

		now := time.Now()
		metadata := map[string]any{
			"type":          "antigravity",
			"access_token":  tokenResp.AccessToken,
			"refresh_token": tokenResp.RefreshToken,
			"expires_in":    tokenResp.ExpiresIn,
			"timestamp":     now.UnixMilli(),
			"expired":       now.Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Format(time.RFC3339),
		}
		if email != "" {
			metadata["email"] = email
		}
		if projectID != "" {
			metadata["project_id"] = projectID
		}

		fileName := sdkcliproxy.AntigravityCredentialFileName(email)
		label := strings.TrimSpace(email)
		if label == "" {
			label = "antigravity"
		}

		record := &sdkcliproxyauth.Auth{
			ID:       fileName,
			Provider: "antigravity",
			FileName: fileName,
			Label:    label,
			Metadata: metadata,
		}

		_, errSave := sdkcliproxy.SaveTokenRecord(ctx, h.cfg.AuthDir, record, h.postAuthHook)
		if errSave != nil {
			sdkcliproxy.SetOAuthSessionError(state, "Failed to save token to file")
			log.Errorf("Failed to save token to file: %v", errSave)
			return
		}
		sdkcliproxy.CompleteOAuthSession(state)
		sdkcliproxy.CompleteOAuthSessionsByProvider("antigravity")
	}()

	c.JSON(http.StatusOK, gin.H{"status": "ok", "url": authURL, "state": state})
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
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}

	ctx := context.Background()
	ctx = sdkcliproxy.PopulateAuthContext(ctx, c)

	state := fmt.Sprintf("ifl-%d", time.Now().UnixNano())
	authSvc := sdkcliproxy.NewIFlowAuthProvider(h.cfg)
	authURL, redirectURI := authSvc.AuthorizationURL(state, sdkcliproxy.IFlowCallbackPort)

	sdkcliproxy.RegisterOAuthSession(state, "iflow")

	isWebUI := isWebUIRequest(c)
	var forwarder *callbackForwarder
	if isWebUI {
		targetURL, errTarget := managementCallbackURL(h.cfg.Port, h.cfg.TLS.Enable, "/iflow/callback")
		if errTarget != nil {
			log.WithError(errTarget).Error("failed to compute iflow callback target")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "callback server unavailable"})
			return
		}
		var err error
		if forwarder, err = startCallbackForwarder(sdkcliproxy.IFlowCallbackPort, "iflow", targetURL); err != nil {
			log.WithError(err).Error("failed to start iflow callback forwarder")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start callback server"})
			return
		}
	}

	go func() {
		if isWebUI {
			defer stopCallbackForwarderInstance(sdkcliproxy.IFlowCallbackPort, forwarder)
		}

		result, errPoll := pollForCallbackFile(h.cfg.AuthDir, "iflow", state)
		if errPoll != nil {
			log.Errorf("IFlow OAuth callback failed: %v", errPoll)
			return
		}

		tokenData, errExchange := authSvc.ExchangeCodeForTokens(ctx, result.Code, redirectURI)
		if errExchange != nil {
			sdkcliproxy.SetOAuthSessionError(state, "Authentication failed")
			log.Errorf("Failed to exchange authorization code for tokens: %v", errExchange)
			return
		}

		tokenStorage := authSvc.CreateTokenStorage(tokenData)
		identifier := strings.TrimSpace(tokenStorage.Email)
		if identifier == "" {
			identifier = fmt.Sprintf("%d", time.Now().UnixMilli())
			tokenStorage.Email = identifier
		}
		record := &sdkcliproxyauth.Auth{
			ID:         fmt.Sprintf("iflow-%s.json", identifier),
			Provider:   "iflow",
			FileName:   fmt.Sprintf("iflow-%s.json", identifier),
			Storage:    tokenStorage,
			Metadata:   map[string]any{"email": identifier, "api_key": tokenStorage.APIKey},
			Attributes: map[string]string{"api_key": tokenStorage.APIKey},
		}

		_, errSave := sdkcliproxy.SaveTokenRecord(ctx, h.cfg.AuthDir, record, h.postAuthHook)
		if errSave != nil {
			sdkcliproxy.SetOAuthSessionError(state, "Failed to save authentication tokens")
			log.Errorf("Failed to save authentication tokens: %v", errSave)
			return
		}
		sdkcliproxy.CompleteOAuthSession(state)
		sdkcliproxy.CompleteOAuthSessionsByProvider("iflow")
	}()

	c.JSON(http.StatusOK, gin.H{"status": "ok", "url": authURL, "state": state})
}

func (h *ManagementHandler) RequestIFlowCookieToken(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}

	ctx := context.Background()

	var payload struct {
		Cookie string `json:"cookie"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "cookie is required"})
		return
	}

	cookieValue := strings.TrimSpace(payload.Cookie)
	if cookieValue == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "cookie is required"})
		return
	}

	cookieValue, errNormalize := sdkcliproxy.NormalizeCookie(cookieValue)
	if errNormalize != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": errNormalize.Error()})
		return
	}

	bxAuth := sdkcliproxy.ExtractBXAuth(cookieValue)
	if existingFile, err := sdkcliproxy.CheckDuplicateBXAuth(h.cfg.AuthDir, bxAuth); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "failed to check duplicate"})
		return
	} else if existingFile != "" {
		existingFileName := existingFile[strings.LastIndex(existingFile, "/")+1:]
		c.JSON(http.StatusConflict, gin.H{"status": "error", "error": "duplicate BXAuth found", "existing_file": existingFileName})
		return
	}

	authSvc := sdkcliproxy.NewIFlowAuthProvider(h.cfg)
	tokenData, errAuth := authSvc.AuthenticateWithCookie(ctx, cookieValue)
	if errAuth != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": errAuth.Error()})
		return
	}

	tokenData.Cookie = cookieValue

	tokenStorage := authSvc.CreateCookieTokenStorage(tokenData)
	email := strings.TrimSpace(tokenStorage.Email)
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "failed to extract email from token"})
		return
	}

	fileName := sdkcliproxy.SanitizeIFlowFileName(email)
	if fileName == "" {
		fileName = fmt.Sprintf("iflow-%d", time.Now().UnixMilli())
	} else {
		fileName = fmt.Sprintf("iflow-%s", fileName)
	}

	tokenStorage.Email = email
	timestamp := time.Now().Unix()

	record := &sdkcliproxyauth.Auth{
		ID:       fmt.Sprintf("%s-%d.json", fileName, timestamp),
		Provider: "iflow",
		FileName: fmt.Sprintf("%s-%d.json", fileName, timestamp),
		Storage:  tokenStorage,
		Metadata: map[string]any{
			"email":        email,
			"api_key":      tokenStorage.APIKey,
			"expired":      tokenStorage.Expire,
			"cookie":       tokenStorage.Cookie,
			"type":         tokenStorage.Type,
			"last_refresh": tokenStorage.LastRefresh,
		},
		Attributes: map[string]string{
			"api_key": tokenStorage.APIKey,
		},
	}

	savedPath, errSave := sdkcliproxy.SaveTokenRecord(ctx, h.cfg.AuthDir, record, h.postAuthHook)
	if errSave != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "failed to save authentication tokens"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "ok",
		"saved_path": savedPath,
		"email":      email,
		"expired":    tokenStorage.Expire,
		"type":       tokenStorage.Type,
	})
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
