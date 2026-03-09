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
	ChannelID int
	Existing  map[string]struct{}
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
	return t == types.OutboundCodex || t == types.OutboundCopilot
}

// runtimeProviderFilter returns the auth file provider filter for the given runtime channel type.
func runtimeProviderFilter(t types.OutboundType) string {
	switch t {
	case types.OutboundCopilot:
		return "copilot"
	default:
		return "codex"
	}
}

func canonicalRuntimeProvider(provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	switch provider {
	case "github-copilot", "github", "copilot":
		return "copilot"
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
