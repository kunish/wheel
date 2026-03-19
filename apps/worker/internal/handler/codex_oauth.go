package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// managementAuthEndpoint returns the management API path for initiating OAuth
// based on the runtime channel type.
func managementAuthEndpoint(t types.OutboundType) string {
	contract, ok := runtimeOAuthChannelContract(t)
	if !ok {
		return ""
	}
	return contract.managementAuthEndpoint
}

func codexOAuthStartError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{
		"success": false,
		"data": gin.H{
			"status": "error",
			"phase":  "failed",
			"code":   code,
			"error":  code,
		},
	})
}

func (h *Handler) validateCodexOAuthStartChannel(c *gin.Context) (*types.Channel, error) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "invalid channel ID")
		return nil, err
	}
	channel, err := dal.GetChannel(c.Request.Context(), h.DB, id)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return nil, err
	}
	if channel == nil {
		errorJSON(c, http.StatusNotFound, "channel not found")
		return nil, errors.New("channel not found")
	}
	if !isRuntimeChannel(channel.Type) {
		codexOAuthStartError(c, http.StatusBadRequest, "unsupported_runtime_channel")
		return nil, errors.New("unsupported runtime channel")
	}
	return channel, nil
}

// StartCodexOAuth initiates an OAuth flow via the runtime management API.
// It selects the correct management endpoint based on the channel type and returns {url, state}.
// For device-flow providers (e.g. Copilot), user_code is forwarded to the caller.
func (h *Handler) StartCodexOAuth(c *gin.Context) {
	channel, err := h.validateCodexOAuthStartChannel(c)
	if err != nil {
		return
	}

	if err := h.ensureCodexManagementConfigured(); err != nil {
		codexOAuthStartError(c, http.StatusBadRequest, "runtime_not_configured")
		return
	}

	var req struct {
		ForceRestart bool `json:"force_restart"`
	}
	if c.Request != nil && c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
			errorJSON(c, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	provider := oauthProviderForChannelType(channel.Type)
	importProvider := runtimeProviderFilter(channel.Type)
	codexOAuthStartMu.Lock()
	defer codexOAuthStartMu.Unlock()
	if !req.ForceRestart {
		if session, ok := findActiveOAuthSessionForImportScope(channel.ID, provider, importProvider); ok {
			successJSON(c, serializeCodexOAuthSession(session))
			return
		}
	}
	if active, ok := findConflictingActiveOAuthSessionForImportScope(channel.ID, provider, importProvider); ok {
		_ = active
		codexOAuthStartError(c, http.StatusConflict, "runtime_session_conflict")
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
		codexOAuthStartError(c, http.StatusBadGateway, "provider_unavailable")
		return
	}
	if strings.TrimSpace(resp.State) == "" {
		codexOAuthStartError(c, http.StatusBadGateway, "provider_unavailable")
		return
	}
	if strings.TrimSpace(resp.URL) == "" {
		codexOAuthStartError(c, http.StatusBadGateway, "provider_unavailable")
		return
	}

	contract, _ := runtimeOAuthChannelContract(channel.Type)
	flowType := "redirect"
	supportsManual := contract.supportsManualCallbackImport
	lastPhase := "awaiting_callback"
	userCode := ""
	verificationURI := ""
	if !supportsManual {
		if strings.TrimSpace(resp.UserCode) == "" || strings.TrimSpace(resp.VerificationURI) == "" {
			codexOAuthStartError(c, http.StatusBadGateway, "provider_unavailable")
			return
		}
		flowType = "device_code"
		lastPhase = "awaiting_browser"
		userCode = resp.UserCode
		verificationURI = resp.VerificationURI
	}

	session := codexOAuthSession{
		ChannelID:       channel.ID,
		Provider:        provider,
		ImportProvider:  importProvider,
		FlowType:        flowType,
		URL:             resp.URL,
		UserCode:        userCode,
		VerificationURI: verificationURI,
		SupportsManual:  supportsManual,
		State:           resp.State,
		ExpiresAt:       time.Now().Add(codexOAuthSessionTTL).UTC().Truncate(time.Second),
		LastStatus:      "waiting",
		LastPhase:       lastPhase,
		Existing:        snapshot,
	}
	supersedeOAuthSessions(channel.ID, provider, importProvider, resp.State)
	storeOAuthSession(resp.State, session)
	successJSON(c, serializeCodexOAuthSession(session))
}

// GetCodexOAuthStatus polls the OAuth flow status from Codex management API.
// It proxies GET /v0/management/get-auth-status?state=xxx and returns {status, error?}.
func (h *Handler) GetCodexOAuthStatus(c *gin.Context) {
	channel, err := h.validateCodexChannel(c)
	if err != nil {
		return
	}

	if err := h.ensureCodexManagementConfigured(); err != nil {
		errorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	state := strings.TrimSpace(c.Query("state"))
	if state == "" {
		errorJSON(c, http.StatusBadRequest, "state parameter is required")
		return
	}

	session, sessionState := loadOAuthSessionState(state)
	if sessionState == "missing" || session.ChannelID != channel.ID {
		successJSON(c, serializeCodexOAuthTransport(codexOAuthMissingSession(channel.Type)))
		return
	}
	if session.Provider != oauthProviderForChannelType(channel.Type) || codexOAuthImportScope(session) != runtimeProviderFilter(channel.Type) {
		successJSON(c, serializeCodexOAuthTransport(codexOAuthMissingSession(channel.Type)))
		return
	}
	if sessionState == "expired" {
		successJSON(c, serializeCodexOAuthTransport(session))
		return
	}
	if codexOAuthPhaseTerminal(session.LastPhase) {
		successJSON(c, serializeCodexOAuthTransport(session))
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
	var payload gin.H
	withCodexOAuthStateLock(state, func() {
		session, sessionState := loadOAuthSessionState(state)
		if sessionState == "missing" || session.ChannelID != channel.ID {
			payload = serializeCodexOAuthTransport(codexOAuthMissingSession(channel.Type))
			return
		}
		if session.Provider != oauthProviderForChannelType(channel.Type) || codexOAuthImportScope(session) != runtimeProviderFilter(channel.Type) {
			payload = serializeCodexOAuthTransport(codexOAuthMissingSession(channel.Type))
			return
		}
		if sessionState == "expired" || codexOAuthPhaseTerminal(session.LastPhase) {
			payload = serializeCodexOAuthTransport(session)
			return
		}
		originalPhase := session.LastPhase
		updated := codexOAuthApplyRuntimeStatus(session, resp.Status, resp.Error)
		if resp.Status == "ok" {
			if !codexOAuthCanCompleteFromRuntimeOK(session) {
				updated = codexOAuthMissingSessionForStoredSession(session)
			} else {
				updated.LastStatus = "waiting"
				updated.LastPhase = "importing_auth_file"
				updated.LastCode = ""
				updated.LastError = ""
				if err := h.importOAuthAuthFilesToDB(c.Request.Context(), session.ChannelID, session.Existing, codexOAuthImportScope(session)); err != nil {
					updated.LastStatus = "error"
					updated.LastPhase = "failed"
					updated.LastCode = "auth_import_failed"
					updated.LastError = err.Error()
					storeOAuthSession(state, updated)
					logCodexOAuthPhaseTransition(updated, originalPhase)
					payload = serializeCodexOAuthTransport(updated)
					return
				}
				updated.LastStatus = "ok"
				updated.LastPhase = "completed"
			}
		}
		storeOAuthSession(state, updated)
		logCodexOAuthPhaseTransition(updated, originalPhase)
		payload = serializeCodexOAuthTransport(updated)
	})
	successJSON(c, payload)
}

// SubmitCodexOAuthCallback accepts a manually pasted OAuth callback URL and forwards
// the extracted code/state to the runtime management POST /oauth-callback endpoint.
// This allows completing OAuth flows when the server is deployed remotely and the
// provider's redirect targets localhost which is unreachable.
func (h *Handler) SubmitCodexOAuthCallback(c *gin.Context) {
	channel, err := h.validateCodexChannel(c)
	if err != nil {
		return
	}

	if err := h.ensureCodexManagementConfigured(); err != nil {
		errorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	var req struct {
		CallbackURL string `json:"callback_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "invalid request body")
		return
	}
	callbackURL := strings.TrimSpace(req.CallbackURL)
	if callbackURL == "" {
		errorJSON(c, http.StatusBadRequest, "callback_url is required")
		return
	}

	// Derive the OAuth provider from the channel type.
	provider := oauthProviderForChannelType(channel.Type)
	parsedCallback, err := url.Parse(callbackURL)
	if err != nil || parsedCallback == nil || strings.TrimSpace(parsedCallback.Scheme) == "" || strings.TrimSpace(parsedCallback.Host) == "" {
		successJSON(c, serializeCodexOAuthCallbackValidationError(codexOAuthSession{}, "invalid_callback_url"))
		return
	}
	scheme := strings.ToLower(strings.TrimSpace(parsedCallback.Scheme))
	if scheme != "http" && scheme != "https" {
		successJSON(c, serializeCodexOAuthCallbackValidationError(codexOAuthSession{}, "invalid_callback_url"))
		return
	}
	host := strings.ToLower(strings.TrimSpace(parsedCallback.Hostname()))
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		successJSON(c, serializeCodexOAuthCallbackValidationError(codexOAuthSession{}, "invalid_callback_url"))
		return
	}
	callbackState := strings.TrimSpace(parsedCallback.Query().Get("state"))
	if callbackState == "" {
		successJSON(c, serializeCodexOAuthCallbackValidationError(codexOAuthSession{}, "missing_state"))
		return
	}
	callbackCode := strings.TrimSpace(parsedCallback.Query().Get("code"))
	callbackError := strings.TrimSpace(parsedCallback.Query().Get("error"))
	if callbackCode == "" && callbackError == "" {
		successJSON(c, serializeCodexOAuthCallbackValidationError(codexOAuthSession{}, "missing_code"))
		return
	}
	var payload gin.H
	var transportErr error
	withCodexOAuthStateLock(callbackState, func() {
		activeSession, hasActive := findActiveOAuthSession(channel.ID, provider)
		if hasActive && activeSession.State != callbackState {
			payload = serializeCodexOAuthCallbackValidationError(activeSession, "state_mismatch")
			return
		}
		session, ok := loadOAuthSession(callbackState)
		if !ok || session.ChannelID != channel.ID {
			payload = serializeCodexOAuthCallbackError(codexOAuthMissingSession(channel.Type), false)
			return
		}
		if session.Provider != provider {
			payload = serializeCodexOAuthCallbackValidationError(session, "provider_mismatch")
			return
		}
		if codexOAuthImportScope(session) != runtimeProviderFilter(channel.Type) {
			payload = serializeCodexOAuthCallbackValidationError(session, "provider_mismatch")
			return
		}
		if !session.SupportsManual {
			payload = serializeCodexOAuthCallbackValidationError(session, "manual_callback_not_supported")
			return
		}
		if callbackError != "" {
			updated := session
			updated.LastStatus = "error"
			updated.LastPhase = "failed"
			updated.LastCode = codexOAuthCodeForRuntimeError(callbackError)
			if updated.LastCode == "" {
				updated.LastCode = "provider_error"
			}
			updated.LastError = humanizeCodexOAuthError(callbackError, "OAuth authorization failed")
			storeOAuthSession(callbackState, updated)
			payload = serializeCodexOAuthCallbackError(updated, false)
			return
		}
		if session.State != callbackState {
			payload = serializeCodexOAuthCallbackValidationError(session, "state_mismatch")
			return
		}
		if session.LastPhase == "callback_received" || session.LastPhase == "importing_auth_file" {
			payload = serializeCodexOAuthCallbackDuplicate(session, true)
			return
		}
		if session.LastPhase == "completed" {
			payload = serializeCodexOAuthCallbackDuplicate(session, false)
			return
		}
		if session.LastPhase == "failed" || session.LastPhase == "expired" {
			payload = serializeCodexOAuthCallbackError(session, false)
			return
		}

		body := map[string]string{
			"provider":     provider,
			"redirect_url": callbackURL,
		}
		originalPhase := session.LastPhase
		preflight := session
		preflight.LastStatus = "waiting"
		preflight.LastPhase = "callback_received"
		preflight.LastCode = ""
		preflight.LastError = ""
		storeOAuthSession(callbackState, preflight)

		var resp struct {
			Status string `json:"status"`
			Error  string `json:"error,omitempty"`
		}
		if err := h.codexManagementCall(c, http.MethodPost, "/oauth-callback", nil, body, &resp); err != nil {
			storeOAuthSession(callbackState, session)
			transportErr = err
			return
		}
		runtimeStatus := strings.ToLower(strings.TrimSpace(resp.Status))
		runtimeError := strings.TrimSpace(resp.Error)
		if runtimeStatus != "" && runtimeStatus != "ok" && runtimeStatus != "waiting" {
			if updated, ok := codexOAuthTerminalCallbackResult(session, runtimeStatus, runtimeError); ok {
				storeOAuthSession(callbackState, updated)
				logCodexOAuthPhaseTransition(updated, originalPhase)
				payload = serializeCodexOAuthCallbackError(updated, false)
				return
			}
			storeOAuthSession(callbackState, session)
			payload = serializeCodexOAuthCallbackRuntimeRejection(session, runtimeError)
			return
		}
		updated := preflight
		storeOAuthSession(callbackState, updated)
		logCodexOAuthPhaseTransition(updated, originalPhase)
		payload = serializeCodexOAuthCallbackAccepted(updated)
	})
	if transportErr != nil {
		errorJSON(c, http.StatusBadGateway, transportErr.Error())
		return
	}
	successJSON(c, payload)
}

// oauthProviderForChannelType maps a channel type to its OAuth provider name
// used by the runtime management layer.
func oauthProviderForChannelType(t types.OutboundType) string {
	contract, ok := runtimeOAuthChannelContract(t)
	if !ok {
		return ""
	}
	return contract.oauthProvider
}

func serializeCodexOAuthSession(session codexOAuthSession) gin.H {
	result := gin.H{
		"url":                          session.URL,
		"state":                        session.State,
		"flowType":                     session.FlowType,
		"supportsManualCallbackImport": session.SupportsManual,
		"expiresAt":                    session.ExpiresAt.UTC().Format(time.RFC3339),
	}
	if session.UserCode != "" {
		result["user_code"] = session.UserCode
	}
	if session.VerificationURI != "" {
		result["verification_uri"] = session.VerificationURI
	}
	return result
}

func serializeCodexOAuthTransport(session codexOAuthSession) gin.H {
	result := gin.H{
		"status":                       session.LastStatus,
		"phase":                        session.LastPhase,
		"expiresAt":                    session.ExpiresAt.UTC().Format(time.RFC3339),
		"canRetry":                     codexOAuthCanRetry(session),
		"supportsManualCallbackImport": session.SupportsManual,
	}
	if session.LastCode != "" {
		result["code"] = session.LastCode
	}
	if session.LastError != "" && codexOAuthPhaseTerminal(session.LastPhase) {
		result["error"] = session.LastError
	}
	return result
}

func serializeCodexOAuthCallbackAccepted(session codexOAuthSession) gin.H {
	return gin.H{
		"status":                "accepted",
		"phase":                 session.LastPhase,
		"shouldContinuePolling": !codexOAuthPhaseTerminal(session.LastPhase),
	}
}

func serializeCodexOAuthCallbackDuplicate(session codexOAuthSession, shouldContinuePolling bool) gin.H {
	result := gin.H{
		"status":                "duplicate",
		"phase":                 session.LastPhase,
		"code":                  "duplicate_callback",
		"shouldContinuePolling": shouldContinuePolling,
	}
	if session.LastError != "" && codexOAuthPhaseTerminal(session.LastPhase) {
		result["error"] = session.LastError
	}
	return result
}

func serializeCodexOAuthCallbackError(session codexOAuthSession, shouldContinuePolling bool) gin.H {
	result := gin.H{
		"status":                "error",
		"phase":                 session.LastPhase,
		"shouldContinuePolling": shouldContinuePolling,
	}
	if session.LastCode != "" {
		result["code"] = session.LastCode
	}
	if session.LastError != "" {
		result["error"] = session.LastError
	}
	return result
}

func serializeCodexOAuthCallbackValidationError(session codexOAuthSession, code string) gin.H {
	phase := "awaiting_callback"
	if code == "manual_callback_not_supported" && session.LastPhase != "" {
		phase = session.LastPhase
	}
	return gin.H{
		"status":                "error",
		"phase":                 phase,
		"code":                  code,
		"error":                 codexOAuthCallbackErrorMessage(code),
		"shouldContinuePolling": false,
	}
}

func serializeCodexOAuthCallbackRuntimeRejection(session codexOAuthSession, message string) gin.H {
	phase := "awaiting_callback"
	if session.LastPhase != "" && !codexOAuthPhaseTerminal(session.LastPhase) {
		phase = session.LastPhase
	}
	return gin.H{
		"status":                "error",
		"phase":                 phase,
		"code":                  "runtime_callback_rejected",
		"error":                 humanizeCodexOAuthError(message, "Runtime worker rejected the callback URL."),
		"shouldContinuePolling": false,
	}
}

func codexOAuthMissingSession(t types.OutboundType) codexOAuthSession {
	supportsManual := true
	if contract, ok := runtimeOAuthChannelContract(t); ok {
		supportsManual = contract.supportsManualCallbackImport
	}
	return codexOAuthSession{
		FlowType:       "redirect",
		SupportsManual: supportsManual,
		ExpiresAt:      time.Now().UTC().Truncate(time.Second),
		LastStatus:     "expired",
		LastPhase:      "expired",
		LastCode:       "session_missing",
		LastError:      "OAuth session expired or is no longer available on this worker",
	}
}

func codexOAuthMissingSessionForStoredSession(session codexOAuthSession) codexOAuthSession {
	missing := codexOAuthMissingSession(types.OutboundCodex)
	missing.ChannelID = session.ChannelID
	missing.Provider = session.Provider
	missing.FlowType = session.FlowType
	missing.SupportsManual = session.SupportsManual
	missing.State = session.State
	missing.ExpiresAt = session.ExpiresAt
	return missing
}

func codexOAuthApplyRuntimeStatus(session codexOAuthSession, runtimeStatus string, runtimeError string) codexOAuthSession {
	next := session
	next.LastCode = ""
	next.LastError = ""
	switch runtimeStatus {
	case "ok":
		next.LastStatus = "ok"
		next.LastPhase = "completed"
	case "expired":
		next.LastStatus = "expired"
		next.LastPhase = "expired"
		next.LastCode = "session_expired"
		next.LastError = humanizeCodexOAuthError(runtimeError, "OAuth session expired")
	case "error":
		next.LastStatus = "error"
		next.LastPhase = "failed"
		next.LastCode = codexOAuthCodeForRuntimeError(runtimeError)
		next.LastError = humanizeCodexOAuthError(runtimeError, "OAuth authorization failed")
	default:
		next.LastStatus = "waiting"
		if next.LastPhase == "callback_received" || next.LastPhase == "importing_auth_file" {
			next.LastPhase = "importing_auth_file"
		} else if next.FlowType == "device_code" {
			next.LastPhase = "awaiting_browser"
		} else {
			next.LastPhase = "awaiting_callback"
		}
	}
	return next
}

func codexOAuthCanCompleteFromRuntimeOK(session codexOAuthSession) bool {
	if session.FlowType == "device_code" {
		return true
	}
	switch session.LastPhase {
	case "callback_received", "importing_auth_file", "completed":
		return true
	default:
		return false
	}
}

func codexOAuthCallbackErrorMessage(code string) string {
	switch code {
	case "invalid_callback_url":
		return "The pasted callback URL is invalid."
	case "missing_state":
		return "This callback is incomplete. Copy the full browser address and try again."
	case "missing_code":
		return "This callback is incomplete. Copy the full browser address and try again."
	case "state_mismatch":
		return "This callback belongs to a different login attempt. Restart OAuth and try again."
	case "provider_mismatch":
		return "This callback belongs to a different provider. Restart OAuth and try again."
	case "manual_callback_not_supported":
		return "This login flow does not use manual callback import. Continue the device-code flow in the browser."
	default:
		return "OAuth callback validation failed."
	}
}

func codexOAuthTerminalCallbackResult(session codexOAuthSession, runtimeStatus string, runtimeError string) (codexOAuthSession, bool) {
	code := codexOAuthCodeForRuntimeError(runtimeError)
	switch runtimeStatus {
	case "expired":
		updated := session
		updated.LastStatus = "expired"
		updated.LastPhase = "expired"
		updated.LastCode = "session_expired"
		updated.LastError = humanizeCodexOAuthError(runtimeError, "OAuth session expired")
		return updated, true
	case "error":
		if code == "" {
			return codexOAuthSession{}, false
		}
		updated := session
		updated.LastStatus = "error"
		updated.LastPhase = "failed"
		updated.LastCode = code
		updated.LastError = humanizeCodexOAuthError(runtimeError, "OAuth authorization failed")
		return updated, true
	default:
		return codexOAuthSession{}, false
	}
}

func codexOAuthCodeForRuntimeError(message string) string {
	msg := strings.ToLower(strings.TrimSpace(message))
	switch {
	case strings.Contains(msg, "access_denied") || strings.Contains(msg, "access denied"):
		return "access_denied"
	case strings.Contains(msg, "invalid_grant") || strings.Contains(msg, "rejected") || strings.Contains(msg, "invalid device code"):
		return "device_code_rejected"
	case strings.Contains(msg, "expired"):
		return "device_code_expired"
	default:
		return ""
	}
}

func humanizeCodexOAuthError(message string, fallback string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return fallback
	}
	return message
}

func codexOAuthCanRetry(session codexOAuthSession) bool {
	return session.LastStatus == "error" || session.LastStatus == "expired"
}

func logCodexOAuthPhaseTransition(session codexOAuthSession, previousPhase string) {
	if previousPhase == session.LastPhase {
		return
	}
	slog.Info("oauth phase transition",
		"channel_id", session.ChannelID,
		"provider", session.Provider,
		"from_phase", previousPhase,
		"to_phase", session.LastPhase,
		"status", session.LastStatus,
		"code", session.LastCode,
	)
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

// bestEffortSyncCodexChannelKeys synchronises auth files as channel keys
// without failing the caller on error. Called automatically after OAuth import.
func (h *Handler) bestEffortSyncCodexChannelKeys(ctx context.Context, channelID int) {
	channel, err := dal.GetChannel(ctx, h.DB, channelID)
	if err != nil || channel == nil {
		return
	}
	files, err := h.listManagedCodexAuthFiles(ctx, channelID)
	if err != nil {
		return
	}
	var matched []codexAuthFile
	for _, f := range files {
		if runtimeProviderMatches(channel.Type, f.Provider) && !f.Disabled {
			matched = append(matched, f)
		}
	}
	keys := make([]types.ChannelKeyInput, 0, len(matched))
	for _, f := range matched {
		remark := f.Email
		if remark == "" {
			remark = f.Name
		}
		key := f.AuthIndex
		if key == "" {
			key = f.Name
		}
		keys = append(keys, types.ChannelKeyInput{ChannelKey: key, Remark: remark})
	}
	if err := dal.SyncChannelKeys(ctx, h.DB, channelID, keys); err != nil {
		return
	}
	h.Cache.Delete("channels")
}
