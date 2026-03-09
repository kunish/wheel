package mgmthandler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	sdkcliproxy "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy"
	sdkcliproxyauth "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

func (h *ManagementHandler) RequestCodexToken(c *gin.Context) {
	if !h.guardHandler(c) {
		return
	}

	ctx := newAuthContext(c)
	codexAuth := sdkcliproxy.NewCodexAuthProvider(h.cfg)

	pkceCodes, err := codexAuth.GeneratePKCECodes()
	if err != nil {
		log.Errorf("Failed to generate PKCE codes: %v", err)
		c.JSON(500, gin.H{"error": "failed to generate PKCE codes"})
		return
	}

	state, err := sdkcliproxy.GenerateRandomState()
	if err != nil {
		log.Errorf("Failed to generate state parameter: %v", err)
		c.JSON(500, gin.H{"error": "failed to generate state parameter"})
		return
	}

	authURL, err := codexAuth.GenerateAuthURL(state, pkceCodes)
	if err != nil {
		log.Errorf("Failed to generate authorization URL: %v", err)
		c.JSON(500, gin.H{"error": "failed to generate authorization url"})
		return
	}

	sdkcliproxy.RegisterOAuthSession(state, "codex")

	fwd, err := h.startPKCECallbackForwarder(c, sdkcliproxy.CodexCallbackPort, "codex", "/codex/callback")
	if err != nil {
		return
	}

	go func() {
		defer fwd.deferStop()

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

		h.saveAndCompleteAuth(ctx, state, "codex", record)
	}()

	respondAuthURL(c, authURL, state)
}

func (h *ManagementHandler) RequestAnthropicToken(c *gin.Context) {
	if !h.guardHandler(c) {
		return
	}

	ctx := newAuthContext(c)
	anthropicAuth := sdkcliproxy.NewAnthropicAuthProvider(h.cfg)

	pkceCodes, err := anthropicAuth.GeneratePKCECodes()
	if err != nil {
		log.Errorf("Failed to generate PKCE codes: %v", err)
		c.JSON(500, gin.H{"error": "failed to generate PKCE codes"})
		return
	}

	state, err := sdkcliproxy.GenerateRandomState()
	if err != nil {
		log.Errorf("Failed to generate state parameter: %v", err)
		c.JSON(500, gin.H{"error": "failed to generate state parameter"})
		return
	}

	authURL, _, err := anthropicAuth.GenerateAuthURL(state, pkceCodes)
	if err != nil {
		log.Errorf("Failed to generate authorization URL: %v", err)
		c.JSON(500, gin.H{"error": "failed to generate authorization url"})
		return
	}

	sdkcliproxy.RegisterOAuthSession(state, "anthropic")

	fwd, err := h.startPKCECallbackForwarder(c, sdkcliproxy.AnthropicCallbackPort, "anthropic", "/anthropic/callback")
	if err != nil {
		return
	}

	go func() {
		defer fwd.deferStop()

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

		h.saveAndCompleteAuth(ctx, state, "anthropic", record)
	}()

	respondAuthURL(c, authURL, state)
}

func (h *ManagementHandler) RequestAntigravityToken(c *gin.Context) {
	if !h.guardHandler(c) {
		return
	}

	ctx := newAuthContext(c)
	authSvc := sdkcliproxy.NewAntigravityAuthProvider(h.cfg)

	state, err := sdkcliproxy.GenerateRandomState()
	if err != nil {
		log.Errorf("Failed to generate state parameter: %v", err)
		c.JSON(500, gin.H{"error": "failed to generate state parameter"})
		return
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/oauth-callback", sdkcliproxy.AntigravityCallbackPort)
	authURL := authSvc.BuildAuthURL(state, redirectURI)

	sdkcliproxy.RegisterOAuthSession(state, "antigravity")

	fwd, err := h.startPKCECallbackForwarder(c, sdkcliproxy.AntigravityCallbackPort, "antigravity", "/antigravity/callback")
	if err != nil {
		return
	}

	go func() {
		defer fwd.deferStop()

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

		h.saveAndCompleteAuth(ctx, state, "antigravity", record)
	}()

	respondAuthURL(c, authURL, state)
}

func (h *ManagementHandler) RequestIFlowToken(c *gin.Context) {
	if !h.guardHandler(c) {
		return
	}

	ctx := newAuthContext(c)

	state := fmt.Sprintf("ifl-%d", time.Now().UnixNano())
	authSvc := sdkcliproxy.NewIFlowAuthProvider(h.cfg)
	authURL, redirectURI := authSvc.AuthorizationURL(state, sdkcliproxy.IFlowCallbackPort)

	sdkcliproxy.RegisterOAuthSession(state, "iflow")

	fwd, err := h.startPKCECallbackForwarder(c, sdkcliproxy.IFlowCallbackPort, "iflow", "/iflow/callback")
	if err != nil {
		return
	}

	go func() {
		defer fwd.deferStop()

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

		h.saveAndCompleteAuth(ctx, state, "iflow", record)
	}()

	respondAuthURL(c, authURL, state)
}
