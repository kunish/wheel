// Package management provides the management API handlers and middleware
// for configuring the server and managing auth files.
package management

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/buildinfo"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"golang.org/x/crypto/bcrypt"
)

type attemptInfo struct {
	count        int
	blockedUntil time.Time
	lastActivity time.Time // track last activity for cleanup
}

// attemptCleanupInterval controls how often stale IP entries are purged
const attemptCleanupInterval = 1 * time.Hour

// attemptMaxIdleTime controls how long an IP can be idle before cleanup
const attemptMaxIdleTime = 2 * time.Hour

// Handler aggregates config reference, persistence path and helpers.
type Handler struct {
	cfg                 *config.Config
	configFilePath      string
	mu                  sync.Mutex
	attemptsMu          sync.Mutex
	failedAttempts      map[string]*attemptInfo // keyed by client IP
	authManager         *coreauth.Manager
	usageStats          *usage.RequestStatistics
	tokenStore          coreauth.Store
	localPassword       string
	allowRemoteOverride bool
	envSecret           string
	logDir              string
	postAuthHook        coreauth.PostAuthHook
}

// NewHandler creates a new management handler instance.
func NewHandler(cfg *config.Config, configFilePath string, manager *coreauth.Manager) *Handler {
	envSecret, _ := os.LookupEnv("MANAGEMENT_PASSWORD")
	envSecret = strings.TrimSpace(envSecret)

	h := &Handler{
		cfg:                 cfg,
		configFilePath:      configFilePath,
		failedAttempts:      make(map[string]*attemptInfo),
		authManager:         manager,
		usageStats:          usage.GetRequestStatistics(),
		tokenStore:          sdkAuth.GetTokenStore(),
		allowRemoteOverride: envSecret != "",
		envSecret:           envSecret,
	}
	h.startAttemptCleanup()
	return h
}

// startAttemptCleanup launches a background goroutine that periodically
// removes stale IP entries from failedAttempts to prevent memory leaks.
func (h *Handler) startAttemptCleanup() {
	go func() {
		ticker := time.NewTicker(attemptCleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			h.purgeStaleAttempts()
		}
	}()
}

// purgeStaleAttempts removes IP entries that have been idle beyond attemptMaxIdleTime
// and whose ban (if any) has expired.
func (h *Handler) purgeStaleAttempts() {
	now := time.Now()
	h.attemptsMu.Lock()
	defer h.attemptsMu.Unlock()
	for ip, ai := range h.failedAttempts {
		// Skip if still banned
		if !ai.blockedUntil.IsZero() && now.Before(ai.blockedUntil) {
			continue
		}
		// Remove if idle too long
		if now.Sub(ai.lastActivity) > attemptMaxIdleTime {
			delete(h.failedAttempts, ip)
		}
	}
}

// NewHandler creates a new management handler instance.
func NewHandlerWithoutConfigFilePath(cfg *config.Config, manager *coreauth.Manager) *Handler {
	return NewHandler(cfg, "", manager)
}

// SetConfig updates the in-memory config reference when the server hot-reloads.
func (h *Handler) SetConfig(cfg *config.Config) { h.cfg = cfg }

// SetAuthManager updates the auth manager reference used by management endpoints.
func (h *Handler) SetAuthManager(manager *coreauth.Manager) { h.authManager = manager }

// SetUsageStatistics allows replacing the usage statistics reference.
func (h *Handler) SetUsageStatistics(stats *usage.RequestStatistics) { h.usageStats = stats }

// SetLocalPassword configures the runtime-local password accepted for localhost requests.
func (h *Handler) SetLocalPassword(password string) { h.localPassword = password }

// SetLogDirectory updates the directory where main.log should be looked up.
func (h *Handler) SetLogDirectory(dir string) {
	if dir == "" {
		return
	}
	if !filepath.IsAbs(dir) {
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
	}
	h.logDir = dir
}

// SetPostAuthHook registers a hook to be called after auth record creation but before persistence.
func (h *Handler) SetPostAuthHook(hook coreauth.PostAuthHook) {
	h.postAuthHook = hook
}

// RegisterRoutes attaches management endpoints to the provided router group.
func (h *Handler) RegisterRoutes(mgmt *gin.RouterGroup) {
	if h == nil || mgmt == nil {
		return
	}
	h.RegisterRoutesWithoutOAuthSessionRoutes(mgmt)
	mgmt.GET("/anthropic-auth-url", h.RequestAnthropicToken)
	mgmt.GET("/codex-auth-url", h.RequestCodexToken)
	mgmt.GET("/github-auth-url", h.RequestGitHubToken)
	mgmt.GET("/gemini-cli-auth-url", h.RequestGeminiCLIToken)
	mgmt.GET("/antigravity-auth-url", h.RequestAntigravityToken)
	mgmt.GET("/qwen-auth-url", h.RequestQwenToken)
	mgmt.GET("/kilo-auth-url", h.RequestKiloToken)
	mgmt.GET("/kimi-auth-url", h.RequestKimiToken)
	mgmt.GET("/iflow-auth-url", h.RequestIFlowToken)
	mgmt.POST("/iflow-auth-url", h.RequestIFlowCookieToken)
	mgmt.GET("/kiro-auth-url", h.RequestKiroToken)
	mgmt.POST("/oauth-callback", h.PostOAuthCallback)
	mgmt.GET("/get-auth-status", h.GetAuthStatus)
}

// RegisterRoutesWithoutOAuthCallback attaches management endpoints except OAuth callback and status polling routes.
func (h *Handler) RegisterRoutesWithoutOAuthCallback(mgmt *gin.RouterGroup) {
	h.RegisterRoutesWithoutOAuthSessionRoutes(mgmt)
	mgmt.GET("/anthropic-auth-url", h.RequestAnthropicToken)
	mgmt.GET("/codex-auth-url", h.RequestCodexToken)
	mgmt.GET("/github-auth-url", h.RequestGitHubToken)
	mgmt.GET("/gemini-cli-auth-url", h.RequestGeminiCLIToken)
	mgmt.GET("/antigravity-auth-url", h.RequestAntigravityToken)
	mgmt.GET("/qwen-auth-url", h.RequestQwenToken)
	mgmt.GET("/kilo-auth-url", h.RequestKiloToken)
	mgmt.GET("/kimi-auth-url", h.RequestKimiToken)
	mgmt.GET("/iflow-auth-url", h.RequestIFlowToken)
	mgmt.POST("/iflow-auth-url", h.RequestIFlowCookieToken)
	mgmt.GET("/kiro-auth-url", h.RequestKiroToken)
	mgmt.GET("/get-auth-status", h.GetAuthStatus)
}

// RegisterRoutesWithoutOAuthSessionRoutes attaches management endpoints except OAuth session routes.
func (h *Handler) RegisterRoutesWithoutOAuthSessionRoutes(mgmt *gin.RouterGroup) {
	if h == nil || mgmt == nil {
		return
	}

	mgmt.GET("/usage", h.GetUsageStatistics)
	mgmt.GET("/usage/export", h.ExportUsageStatistics)
	mgmt.POST("/usage/import", h.ImportUsageStatistics)
	mgmt.GET("/config", h.GetConfig)
	mgmt.GET("/config.yaml", h.GetConfigYAML)
	mgmt.PUT("/config.yaml", h.PutConfigYAML)
	mgmt.GET("/latest-version", h.GetLatestVersion)

	mgmt.GET("/debug", h.GetDebug)
	mgmt.PUT("/debug", h.PutDebug)
	mgmt.PATCH("/debug", h.PutDebug)

	mgmt.GET("/logging-to-file", h.GetLoggingToFile)
	mgmt.PUT("/logging-to-file", h.PutLoggingToFile)
	mgmt.PATCH("/logging-to-file", h.PutLoggingToFile)

	mgmt.GET("/logs-max-total-size-mb", h.GetLogsMaxTotalSizeMB)
	mgmt.PUT("/logs-max-total-size-mb", h.PutLogsMaxTotalSizeMB)
	mgmt.PATCH("/logs-max-total-size-mb", h.PutLogsMaxTotalSizeMB)

	mgmt.GET("/error-logs-max-files", h.GetErrorLogsMaxFiles)
	mgmt.PUT("/error-logs-max-files", h.PutErrorLogsMaxFiles)
	mgmt.PATCH("/error-logs-max-files", h.PutErrorLogsMaxFiles)

	mgmt.GET("/usage-statistics-enabled", h.GetUsageStatisticsEnabled)
	mgmt.PUT("/usage-statistics-enabled", h.PutUsageStatisticsEnabled)
	mgmt.PATCH("/usage-statistics-enabled", h.PutUsageStatisticsEnabled)

	mgmt.GET("/proxy-url", h.GetProxyURL)
	mgmt.PUT("/proxy-url", h.PutProxyURL)
	mgmt.PATCH("/proxy-url", h.PutProxyURL)
	mgmt.DELETE("/proxy-url", h.DeleteProxyURL)

	mgmt.POST("/api-call", h.APICall)

	mgmt.GET("/quota-exceeded/switch-project", h.GetSwitchProject)
	mgmt.PUT("/quota-exceeded/switch-project", h.PutSwitchProject)
	mgmt.PATCH("/quota-exceeded/switch-project", h.PutSwitchProject)

	mgmt.GET("/quota-exceeded/switch-preview-model", h.GetSwitchPreviewModel)
	mgmt.PUT("/quota-exceeded/switch-preview-model", h.PutSwitchPreviewModel)
	mgmt.PATCH("/quota-exceeded/switch-preview-model", h.PutSwitchPreviewModel)

	mgmt.GET("/api-keys", h.GetAPIKeys)
	mgmt.PUT("/api-keys", h.PutAPIKeys)
	mgmt.PATCH("/api-keys", h.PatchAPIKeys)
	mgmt.DELETE("/api-keys", h.DeleteAPIKeys)

	mgmt.GET("/gemini-api-key", h.GetGeminiKeys)
	mgmt.PUT("/gemini-api-key", h.PutGeminiKeys)
	mgmt.PATCH("/gemini-api-key", h.PatchGeminiKey)
	mgmt.DELETE("/gemini-api-key", h.DeleteGeminiKey)

	mgmt.GET("/logs", h.GetLogs)
	mgmt.DELETE("/logs", h.DeleteLogs)
	mgmt.GET("/request-error-logs", h.GetRequestErrorLogs)
	mgmt.GET("/request-error-logs/:name", h.DownloadRequestErrorLog)
	mgmt.GET("/request-log-by-id/:id", h.GetRequestLogByID)
	mgmt.GET("/request-log", h.GetRequestLog)
	mgmt.PUT("/request-log", h.PutRequestLog)
	mgmt.PATCH("/request-log", h.PutRequestLog)
	mgmt.GET("/ws-auth", h.GetWebsocketAuth)
	mgmt.PUT("/ws-auth", h.PutWebsocketAuth)
	mgmt.PATCH("/ws-auth", h.PutWebsocketAuth)

	mgmt.GET("/ampcode", h.GetAmpCode)
	mgmt.GET("/ampcode/upstream-url", h.GetAmpUpstreamURL)
	mgmt.PUT("/ampcode/upstream-url", h.PutAmpUpstreamURL)
	mgmt.PATCH("/ampcode/upstream-url", h.PutAmpUpstreamURL)
	mgmt.DELETE("/ampcode/upstream-url", h.DeleteAmpUpstreamURL)
	mgmt.GET("/ampcode/upstream-api-key", h.GetAmpUpstreamAPIKey)
	mgmt.PUT("/ampcode/upstream-api-key", h.PutAmpUpstreamAPIKey)
	mgmt.PATCH("/ampcode/upstream-api-key", h.PutAmpUpstreamAPIKey)
	mgmt.DELETE("/ampcode/upstream-api-key", h.DeleteAmpUpstreamAPIKey)
	mgmt.GET("/ampcode/restrict-management-to-localhost", h.GetAmpRestrictManagementToLocalhost)
	mgmt.PUT("/ampcode/restrict-management-to-localhost", h.PutAmpRestrictManagementToLocalhost)
	mgmt.PATCH("/ampcode/restrict-management-to-localhost", h.PutAmpRestrictManagementToLocalhost)
	mgmt.GET("/ampcode/model-mappings", h.GetAmpModelMappings)
	mgmt.PUT("/ampcode/model-mappings", h.PutAmpModelMappings)
	mgmt.PATCH("/ampcode/model-mappings", h.PatchAmpModelMappings)
	mgmt.DELETE("/ampcode/model-mappings", h.DeleteAmpModelMappings)
	mgmt.GET("/ampcode/force-model-mappings", h.GetAmpForceModelMappings)
	mgmt.PUT("/ampcode/force-model-mappings", h.PutAmpForceModelMappings)
	mgmt.PATCH("/ampcode/force-model-mappings", h.PutAmpForceModelMappings)
	mgmt.GET("/ampcode/upstream-api-keys", h.GetAmpUpstreamAPIKeys)
	mgmt.PUT("/ampcode/upstream-api-keys", h.PutAmpUpstreamAPIKeys)
	mgmt.PATCH("/ampcode/upstream-api-keys", h.PatchAmpUpstreamAPIKeys)
	mgmt.DELETE("/ampcode/upstream-api-keys", h.DeleteAmpUpstreamAPIKeys)

	mgmt.GET("/request-retry", h.GetRequestRetry)
	mgmt.PUT("/request-retry", h.PutRequestRetry)
	mgmt.PATCH("/request-retry", h.PutRequestRetry)
	mgmt.GET("/max-retry-interval", h.GetMaxRetryInterval)
	mgmt.PUT("/max-retry-interval", h.PutMaxRetryInterval)
	mgmt.PATCH("/max-retry-interval", h.PutMaxRetryInterval)

	mgmt.GET("/force-model-prefix", h.GetForceModelPrefix)
	mgmt.PUT("/force-model-prefix", h.PutForceModelPrefix)
	mgmt.PATCH("/force-model-prefix", h.PutForceModelPrefix)

	mgmt.GET("/routing/strategy", h.GetRoutingStrategy)
	mgmt.PUT("/routing/strategy", h.PutRoutingStrategy)
	mgmt.PATCH("/routing/strategy", h.PutRoutingStrategy)

	mgmt.GET("/claude-api-key", h.GetClaudeKeys)
	mgmt.PUT("/claude-api-key", h.PutClaudeKeys)
	mgmt.PATCH("/claude-api-key", h.PatchClaudeKey)
	mgmt.DELETE("/claude-api-key", h.DeleteClaudeKey)

	mgmt.GET("/codex-api-key", h.GetCodexKeys)
	mgmt.PUT("/codex-api-key", h.PutCodexKeys)
	mgmt.PATCH("/codex-api-key", h.PatchCodexKey)
	mgmt.DELETE("/codex-api-key", h.DeleteCodexKey)

	mgmt.GET("/openai-compatibility", h.GetOpenAICompat)
	mgmt.PUT("/openai-compatibility", h.PutOpenAICompat)
	mgmt.PATCH("/openai-compatibility", h.PatchOpenAICompat)
	mgmt.DELETE("/openai-compatibility", h.DeleteOpenAICompat)

	mgmt.GET("/vertex-api-key", h.GetVertexCompatKeys)
	mgmt.PUT("/vertex-api-key", h.PutVertexCompatKeys)
	mgmt.PATCH("/vertex-api-key", h.PatchVertexCompatKey)
	mgmt.DELETE("/vertex-api-key", h.DeleteVertexCompatKey)

	mgmt.GET("/oauth-excluded-models", h.GetOAuthExcludedModels)
	mgmt.PUT("/oauth-excluded-models", h.PutOAuthExcludedModels)
	mgmt.PATCH("/oauth-excluded-models", h.PatchOAuthExcludedModels)
	mgmt.DELETE("/oauth-excluded-models", h.DeleteOAuthExcludedModels)

	mgmt.GET("/oauth-model-alias", h.GetOAuthModelAlias)
	mgmt.PUT("/oauth-model-alias", h.PutOAuthModelAlias)
	mgmt.PATCH("/oauth-model-alias", h.PatchOAuthModelAlias)
	mgmt.DELETE("/oauth-model-alias", h.DeleteOAuthModelAlias)

	mgmt.GET("/auth-files", h.ListAuthFiles)
	mgmt.GET("/auth-files/models", h.GetAuthFileModels)
	mgmt.GET("/model-definitions/:channel", h.GetStaticModelDefinitions)
	mgmt.GET("/auth-files/download", h.DownloadAuthFile)
	mgmt.POST("/auth-files", h.UploadAuthFile)
	mgmt.DELETE("/auth-files", h.DeleteAuthFile)
	mgmt.PATCH("/auth-files/status", h.PatchAuthFileStatus)
	mgmt.PATCH("/auth-files/fields", h.PatchAuthFileFields)
	mgmt.POST("/vertex/import", h.ImportVertexCredential)

}

// Middleware enforces access control for management endpoints.
// All requests (local and remote) require a valid management key.
// Additionally, remote access requires allow-remote-management=true.
func (h *Handler) Middleware() gin.HandlerFunc {
	const maxFailures = 5
	const banDuration = 30 * time.Minute

	return func(c *gin.Context) {
		c.Header("X-CPA-VERSION", buildinfo.Version)
		c.Header("X-CPA-COMMIT", buildinfo.Commit)
		c.Header("X-CPA-BUILD-DATE", buildinfo.BuildDate)

		clientIP := c.ClientIP()
		localClient := clientIP == "127.0.0.1" || clientIP == "::1"
		cfg := h.cfg
		var (
			allowRemote bool
			secretHash  string
		)
		if cfg != nil {
			allowRemote = cfg.RemoteManagement.AllowRemote
			secretHash = cfg.RemoteManagement.SecretKey
		}
		if h.allowRemoteOverride {
			allowRemote = true
		}
		envSecret := h.envSecret

		fail := func() {}
		if !localClient {
			h.attemptsMu.Lock()
			ai := h.failedAttempts[clientIP]
			if ai != nil {
				if !ai.blockedUntil.IsZero() {
					if time.Now().Before(ai.blockedUntil) {
						remaining := time.Until(ai.blockedUntil).Round(time.Second)
						h.attemptsMu.Unlock()
						c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": fmt.Sprintf("IP banned due to too many failed attempts. Try again in %s", remaining)})
						return
					}
					// Ban expired, reset state
					ai.blockedUntil = time.Time{}
					ai.count = 0
				}
			}
			h.attemptsMu.Unlock()

			if !allowRemote {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "remote management disabled"})
				return
			}

			fail = func() {
				h.attemptsMu.Lock()
				aip := h.failedAttempts[clientIP]
				if aip == nil {
					aip = &attemptInfo{}
					h.failedAttempts[clientIP] = aip
				}
				aip.count++
				aip.lastActivity = time.Now()
				if aip.count >= maxFailures {
					aip.blockedUntil = time.Now().Add(banDuration)
					aip.count = 0
				}
				h.attemptsMu.Unlock()
			}
		}
		if secretHash == "" && envSecret == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "remote management key not set"})
			return
		}

		// Accept either Authorization: Bearer <key> or X-Management-Key
		var provided string
		if ah := c.GetHeader("Authorization"); ah != "" {
			parts := strings.SplitN(ah, " ", 2)
			if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
				provided = parts[1]
			} else {
				provided = ah
			}
		}
		if provided == "" {
			provided = c.GetHeader("X-Management-Key")
		}

		if provided == "" {
			if !localClient {
				fail()
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing management key"})
			return
		}

		if localClient {
			if lp := h.localPassword; lp != "" {
				if subtle.ConstantTimeCompare([]byte(provided), []byte(lp)) == 1 {
					c.Next()
					return
				}
			}
		}

		if envSecret != "" && subtle.ConstantTimeCompare([]byte(provided), []byte(envSecret)) == 1 {
			if !localClient {
				h.attemptsMu.Lock()
				if ai := h.failedAttempts[clientIP]; ai != nil {
					ai.count = 0
					ai.blockedUntil = time.Time{}
				}
				h.attemptsMu.Unlock()
			}
			c.Next()
			return
		}

		if secretHash == "" || bcrypt.CompareHashAndPassword([]byte(secretHash), []byte(provided)) != nil {
			if !localClient {
				fail()
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid management key"})
			return
		}

		if !localClient {
			h.attemptsMu.Lock()
			if ai := h.failedAttempts[clientIP]; ai != nil {
				ai.count = 0
				ai.blockedUntil = time.Time{}
			}
			h.attemptsMu.Unlock()
		}

		c.Next()
	}
}

// persist saves the current in-memory config to disk.
func (h *Handler) persist(c *gin.Context) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	// Preserve comments when writing
	if err := config.SaveConfigPreserveComments(h.configFilePath, h.cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to save config: %v", err)})
		return false
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
	return true
}

// Helper methods for simple types
func (h *Handler) updateBoolField(c *gin.Context, set func(bool)) {
	var body struct {
		Value *bool `json:"value"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Value == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	set(*body.Value)
	h.persist(c)
}

func (h *Handler) updateIntField(c *gin.Context, set func(int)) {
	var body struct {
		Value *int `json:"value"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Value == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	set(*body.Value)
	h.persist(c)
}

func (h *Handler) updateStringField(c *gin.Context, set func(string)) {
	var body struct {
		Value *string `json:"value"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Value == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	set(*body.Value)
	h.persist(c)
}
