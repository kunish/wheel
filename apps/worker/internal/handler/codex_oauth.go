package handler

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// managementAuthEndpoint returns the management API path for initiating OAuth
// based on the runtime channel type.
func managementAuthEndpoint(t types.OutboundType) string {
	switch t {
	case types.OutboundCopilot:
		return "/github-auth-url"
	case types.OutboundAntigravity:
		return "/antigravity-auth-url"
	default:
		return "/codex-auth-url"
	}
}

// StartCodexOAuth initiates an OAuth flow via the runtime management API.
// It selects the correct management endpoint based on the channel type and returns {url, state}.
// For device-flow providers (e.g. Copilot), user_code is forwarded to the caller.
func (h *Handler) StartCodexOAuth(c *gin.Context) {
	channel, err := h.validateCodexChannel(c)
	if err != nil {
		return
	}

	if err := h.ensureCodexManagementConfigured(); err != nil {
		errorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	query := url.Values{"is_webui": []string{"true"}}
	if workerPort := strings.TrimSpace(h.Config.Port); workerPort != "" {
		query.Set("callback_port", workerPort)
	}
	authDir, err := h.resolveCodexLocalAuthDir()
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	existingFiles, err := h.listLocalAuthFiles(authDir)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	snapshot := make(map[string]struct{}, len(existingFiles))
	for _, file := range existingFiles {
		snapshot[file.Name] = struct{}{}
	}
	var resp struct {
		URL             string `json:"url"`
		State           string `json:"state"`
		UserCode        string `json:"user_code,omitempty"`
		VerificationURI string `json:"verification_uri,omitempty"`
	}
	authPath := managementAuthEndpoint(channel.Type)
	if err := h.codexManagementCall(c, http.MethodGet, authPath, query, nil, &resp); err != nil {
		errorJSON(c, http.StatusBadGateway, err.Error())
		return
	}
	storeOAuthSession(resp.State, codexOAuthSession{ChannelID: channel.ID, Existing: snapshot})
	result := gin.H{"url": resp.URL, "state": resp.State}
	if resp.UserCode != "" {
		result["user_code"] = resp.UserCode
	}
	if resp.VerificationURI != "" {
		result["verification_uri"] = resp.VerificationURI
	}
	successJSON(c, result)
}

// GetCodexOAuthStatus polls the OAuth flow status from Codex management API.
// It proxies GET /v0/management/get-auth-status?state=xxx and returns {status, error?}.
func (h *Handler) GetCodexOAuthStatus(c *gin.Context) {
	channel, err := h.validateCodexChannel(c)
	if err != nil {
		return
	}
	_ = channel

	if err := h.ensureCodexManagementConfigured(); err != nil {
		errorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	state := strings.TrimSpace(c.Query("state"))
	if state == "" {
		errorJSON(c, http.StatusBadRequest, "state parameter is required")
		return
	}

	query := url.Values{"state": []string{state}}
	var resp struct {
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}
	if err := h.codexManagementCall(c, http.MethodGet, "/get-auth-status", query, nil, &resp); err != nil {
		errorJSON(c, http.StatusBadGateway, err.Error())
		return
	}
	if resp.Status == "ok" {
		if session, ok := loadAndDeleteOAuthSession(state); ok {
			if err := h.importOAuthAuthFilesToDB(c.Request.Context(), session.ChannelID, session.Existing); err != nil {
				errorJSON(c, http.StatusInternalServerError, err.Error())
				return
			}
		}
	}
	if resp.Status == "error" {
		codexOAuthSessions.Delete(state)
	}
	result := gin.H{"status": resp.Status}
	if resp.Error != "" {
		result["error"] = resp.Error
	}
	successJSON(c, result)
}

// SyncCodexKeys fetches auth files from Codex auth management and syncs them as channel keys.
func (h *Handler) SyncCodexKeys(c *gin.Context) {
	channel, err := h.validateCodexChannel(c)
	if err != nil {
		return
	}
	var files []codexAuthFile
	if h.codexCapabilities().LocalEnabled {
		files, err = h.listManagedCodexAuthFiles(c.Request.Context(), channel.ID)
		if err != nil {
			errorJSON(c, http.StatusBadGateway, err.Error())
			return
		}
	} else {
		if err := h.ensureCodexManagementConfigured(); err != nil {
			errorJSON(c, http.StatusBadRequest, err.Error())
			return
		}
		var resp struct {
			Files []map[string]any `json:"files"`
		}
		if err := h.codexManagementCall(c, http.MethodGet, "/auth-files", nil, nil, &resp); err != nil {
			errorJSON(c, http.StatusBadGateway, err.Error())
			return
		}
		files = parseAuthFiles(resp.Files)
	}
	// Filter to matching provider only
	var codexFiles []codexAuthFile
	for _, f := range files {
		if runtimeProviderMatches(channel.Type, f.Provider) && !f.Disabled {
			codexFiles = append(codexFiles, f)
		}
	}

	// Convert auth files to channel keys
	keys := make([]types.ChannelKeyInput, 0, len(codexFiles))
	for _, f := range codexFiles {
		remark := f.Email
		if remark == "" {
			remark = f.Name
		}
		key := f.AuthIndex
		if key == "" {
			key = f.Name
		}
		keys = append(keys, types.ChannelKeyInput{
			ChannelKey: key,
			Remark:     remark,
		})
	}

	// Sync keys to channel
	if err := dal.SyncChannelKeys(c.Request.Context(), h.DB, channel.ID, keys); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.bestEffortSyncCodexChannelModels(c.Request.Context(), channel.ID)

	h.Cache.Delete("channels")
	successJSON(c, gin.H{"synced": len(keys), "authFiles": len(codexFiles)})
}
