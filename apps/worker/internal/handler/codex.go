package handler

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

const (
	codexQuotaEndpoint         = "https://chatgpt.com/backend-api/wham/usage"
	codexQuotaFetchConcurrency = 4
	codexModelSyncRetryWindow  = time.Second
	codexModelSyncRetryDelay   = 100 * time.Millisecond
)

type codexAuthFile struct {
	ID         int            `json:"-"`
	ChannelID  int            `json:"-"`
	Name       string         `json:"name"`
	Provider   string         `json:"provider"`
	Type       string         `json:"type"`
	Email      string         `json:"email,omitempty"`
	Disabled   bool           `json:"disabled"`
	AuthIndex  string         `json:"authIndex,omitempty"`
	Path       string         `json:"-"`
	RawContent string         `json:"-"`
	Raw        map[string]any `json:"-"`
}

type codexCapabilities struct {
	LocalEnabled      bool `json:"localEnabled"`
	ManagementEnabled bool `json:"managementEnabled"`
	OAuthEnabled      bool `json:"oauthEnabled"`
	ModelsEnabled     bool `json:"modelsEnabled"`
}

type codexQuotaWindow struct {
	UsedPercent        float64 `json:"usedPercent"`
	LimitWindowSeconds int64   `json:"limitWindowSeconds"`
	ResetAfterSeconds  int64   `json:"resetAfterSeconds"`
	ResetAt            string  `json:"resetAt"`
	Allowed            bool    `json:"allowed"`
	LimitReached       bool    `json:"limitReached"`
}

type codexQuotaItem struct {
	Name       string           `json:"name"`
	Email      string           `json:"email,omitempty"`
	AuthIndex  string           `json:"authIndex,omitempty"`
	PlanType   string           `json:"planType,omitempty"`
	Weekly     codexQuotaWindow `json:"weekly"`
	CodeReview codexQuotaWindow `json:"codeReview"`
	Snapshots  []quotaSnapshot  `json:"snapshots,omitempty"`
	ResetAt    string           `json:"resetAt,omitempty"`
	Error      string           `json:"error,omitempty"`
}

type quotaSnapshot struct {
	ID               string  `json:"id"`
	Label            string  `json:"label"`
	PercentRemaining float64 `json:"percentRemaining"`
	Remaining        float64 `json:"remaining,omitempty"`
	Entitlement      float64 `json:"entitlement,omitempty"`
	Unlimited        bool    `json:"unlimited,omitempty"`
}

type codexOAuthSession struct {
	ChannelID       int
	Provider        string
	ImportProvider  string
	FlowType        string
	URL             string
	UserCode        string
	VerificationURI string
	SupportsManual  bool
	State           string
	ExpiresAt       time.Time
	LastStatus      string
	LastPhase       string
	LastCode        string
	LastError       string
	Existing        map[string]struct{}
	createdAt       time.Time
}

const codexOAuthSessionTTL = 15 * time.Minute

// quotaCacheTTL is the time-to-live for cached quota entries.
const quotaCacheTTL = 5 * time.Minute

// quotaCacheEntry wraps a codexQuotaItem with a timestamp for TTL-based expiry.
type quotaCacheEntry struct {
	Item      codexQuotaItem
	FetchedAt time.Time
}

type codexUploadFile struct {
	Name    string
	Content []byte
}

type codexAuthUploadResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type codexAuthUploadResponse struct {
	Total        int                     `json:"total"`
	SuccessCount int                     `json:"successCount"`
	FailedCount  int                     `json:"failedCount"`
	Results      []codexAuthUploadResult `json:"results"`
}

type codexAuthBatchScope struct {
	Names        []string `json:"names"`
	AllMatching  bool     `json:"allMatching"`
	Search       string   `json:"search"`
	Provider     string   `json:"provider"`
	ExcludeNames []string `json:"excludeNames"`
}

var codexOAuthSessions sync.Map
var codexOAuthStartMu sync.Mutex

// storeOAuthSession stores a session with a creation timestamp and sweeps expired entries.
func storeOAuthSession(state string, session codexOAuthSession) {
	session.createdAt = time.Now()
	if session.State == "" {
		session.State = state
	}
	if session.ExpiresAt.IsZero() {
		session.ExpiresAt = session.createdAt.Add(codexOAuthSessionTTL)
	}
	codexOAuthSessions.Store(state, session)

	// Best-effort sweep: delete any expired sessions.
	codexOAuthSessions.Range(func(key, value any) bool {
		if s, ok := value.(codexOAuthSession); ok {
			if time.Since(s.createdAt) > codexOAuthSessionTTL {
				codexOAuthSessions.Delete(key)
			}
		}
		return true
	})
}

func loadOAuthSession(state string) (codexOAuthSession, bool) {
	v, ok := codexOAuthSessions.Load(state)
	if !ok {
		return codexOAuthSession{}, false
	}
	session, ok := v.(codexOAuthSession)
	if !ok || time.Since(session.createdAt) > codexOAuthSessionTTL || time.Now().After(session.ExpiresAt) {
		codexOAuthSessions.Delete(state)
		return codexOAuthSession{}, false
	}
	return session, true
}

// loadAndDeleteOAuthSession retrieves and deletes a session, returning false if missing or expired.
func loadAndDeleteOAuthSession(state string) (codexOAuthSession, bool) {
	v, ok := codexOAuthSessions.LoadAndDelete(state)
	if !ok {
		return codexOAuthSession{}, false
	}
	session, ok := v.(codexOAuthSession)
	if !ok || time.Since(session.createdAt) > codexOAuthSessionTTL || time.Now().After(session.ExpiresAt) {
		return codexOAuthSession{}, false
	}
	return session, true
}

func findActiveOAuthSession(channelID int, provider string) (codexOAuthSession, bool) {
	var latest codexOAuthSession
	var found bool
	codexOAuthSessions.Range(func(key, value any) bool {
		state, _ := key.(string)
		session, ok := value.(codexOAuthSession)
		if !ok {
			codexOAuthSessions.Delete(key)
			return true
		}
		if time.Since(session.createdAt) > codexOAuthSessionTTL || time.Now().After(session.ExpiresAt) {
			codexOAuthSessions.Delete(state)
			return true
		}
		if session.ChannelID != channelID || session.Provider != provider {
			return true
		}
		if codexOAuthPhaseTerminal(session.LastPhase) {
			return true
		}
		if !found || session.createdAt.After(latest.createdAt) {
			latest = session
			found = true
		}
		return true
	})
	return latest, found
}

func findLatestActiveOAuthSession() (codexOAuthSession, bool) {
	var latest codexOAuthSession
	var found bool
	codexOAuthSessions.Range(func(key, value any) bool {
		state, _ := key.(string)
		session, ok := value.(codexOAuthSession)
		if !ok {
			codexOAuthSessions.Delete(key)
			return true
		}
		if time.Since(session.createdAt) > codexOAuthSessionTTL || time.Now().After(session.ExpiresAt) {
			codexOAuthSessions.Delete(state)
			return true
		}
		if codexOAuthPhaseTerminal(session.LastPhase) {
			return true
		}
		if !found || session.createdAt.After(latest.createdAt) {
			latest = session
			found = true
		}
		return true
	})
	return latest, found
}

func findConflictingActiveOAuthSession(channelID int, provider string) (codexOAuthSession, bool) {
	return findConflictingActiveOAuthSessionForImportScope(channelID, provider, provider)
}

func codexOAuthImportScope(session codexOAuthSession) string {
	if scope := canonicalRuntimeProvider(session.ImportProvider); scope != "" {
		return scope
	}
	return canonicalRuntimeProvider(session.Provider)
}

func findConflictingActiveOAuthSessionForImportScope(channelID int, provider string, importProvider string) (codexOAuthSession, bool) {
	var latest codexOAuthSession
	var found bool
	importScope := canonicalRuntimeProvider(importProvider)
	codexOAuthSessions.Range(func(key, value any) bool {
		state, _ := key.(string)
		session, ok := value.(codexOAuthSession)
		if !ok {
			codexOAuthSessions.Delete(key)
			return true
		}
		if time.Since(session.createdAt) > codexOAuthSessionTTL || time.Now().After(session.ExpiresAt) {
			codexOAuthSessions.Delete(state)
			return true
		}
		if codexOAuthPhaseTerminal(session.LastPhase) {
			return true
		}
		if session.ChannelID == channelID && session.Provider == provider {
			return true
		}
		if codexOAuthImportScope(session) != importScope {
			return true
		}
		if !found || session.createdAt.After(latest.createdAt) {
			latest = session
			found = true
		}
		return true
	})
	return latest, found
}

func supersedeOAuthSessions(channelID int, provider string, keepState string) {
	codexOAuthSessions.Range(func(key, value any) bool {
		state, _ := key.(string)
		session, ok := value.(codexOAuthSession)
		if !ok {
			codexOAuthSessions.Delete(key)
			return true
		}
		if session.ChannelID == channelID && session.Provider == provider && state != keepState {
			session.LastStatus = "expired"
			session.LastPhase = "expired"
			session.LastCode = "session_superseded"
			session.LastError = "OAuth session expired because a newer sign-in attempt replaced it"
			codexOAuthSessions.Store(state, session)
		}
		return true
	})
}

func codexOAuthPhaseTerminal(phase string) bool {
	switch phase {
	case "completed", "expired", "failed":
		return true
	default:
		return false
	}
}

func (h *Handler) codexCapabilities() codexCapabilities {
	managementEnabled := h != nil && h.Config != nil && strings.TrimSpace(h.Config.CodexRuntimeManagementKey) != ""
	localEnabled := h != nil && h.DB != nil
	return codexCapabilities{
		LocalEnabled:      localEnabled,
		ManagementEnabled: managementEnabled,
		OAuthEnabled:      managementEnabled,
		ModelsEnabled:     managementEnabled,
	}
}

// isRuntimeChannel returns true if the channel type uses the embedded CLIProxyAPI runtime.
func isRuntimeChannel(t types.OutboundType) bool {
	return t == types.OutboundCodex || t == types.OutboundCopilot ||
		t == types.OutboundCodexCLI || t == types.OutboundAntigravity
}

// runtimeProviderFilter returns the auth file provider filter for the given runtime channel type.
func runtimeProviderFilter(t types.OutboundType) string {
	switch t {
	case types.OutboundCopilot:
		return "copilot"
	case types.OutboundCodexCLI:
		return "codex-cli"
	case types.OutboundAntigravity:
		return "antigravity"
	default:
		return "codex"
	}
}

func canonicalRuntimeProvider(provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	switch provider {
	case "github-copilot", "github", "copilot":
		return "copilot"
	case "codex-cli", "openai-codex-cli":
		return "codex-cli"
	case "antigravity", "google-antigravity":
		return "antigravity"
	default:
		return provider
	}
}

func runtimeProviderMatches(channelType types.OutboundType, provider string) bool {
	return canonicalRuntimeProvider(provider) == runtimeProviderFilter(channelType)
}

// validateCodexChannel verifies the channel exists and is a runtime-managed type (Codex or Copilot).
// On failure it writes an error response and returns nil.
func (h *Handler) validateCodexChannel(c *gin.Context) (*types.Channel, error) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		errorJSON(c, 400, "invalid channel ID")
		return nil, err
	}
	channel, err := dal.GetChannel(c.Request.Context(), h.DB, id)
	if err != nil {
		errorJSON(c, 500, err.Error())
		return nil, err
	}
	if channel == nil {
		errorJSON(c, 404, "channel not found")
		return nil, fmt.Errorf("channel not found")
	}
	if !isRuntimeChannel(channel.Type) {
		errorJSON(c, 400, "channel is not a Codex/Copilot channel")
		return nil, fmt.Errorf("not a runtime channel")
	}
	return channel, nil
}

func (h *Handler) ensureCodexManagementConfigured() error {
	if h == nil || h.Config == nil {
		return fmt.Errorf("handler config not initialized")
	}
	if strings.TrimSpace(h.Config.CodexRuntimeManagementURL) == "" {
		return fmt.Errorf("codex runtime management URL is not configured")
	}
	if strings.TrimSpace(h.Config.CodexRuntimeManagementKey) == "" {
		return fmt.Errorf("codex runtime management key is not configured")
	}
	return nil
}
