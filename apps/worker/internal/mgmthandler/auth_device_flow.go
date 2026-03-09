package mgmthandler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	sdkcliproxy "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy"
	sdkcliproxyauth "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

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
