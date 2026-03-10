package types

// ──── Auth ────

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// ──── Channel ────

type ChannelCreateRequest struct {
	Name          string            `json:"name"`
	Type          int               `json:"type"`
	Enabled       bool              `json:"enabled"`
	BaseUrls      []BaseUrl         `json:"baseUrls"`
	Keys          []ChannelKeyInput `json:"keys"`
	Model         []string          `json:"model"`
	FetchedModel  []string          `json:"fetchedModel,omitempty"`
	CustomModel   string            `json:"customModel,omitempty"`
	AutoSync      *bool             `json:"autoSync,omitempty"`
	AutoGroup     *int              `json:"autoGroup,omitempty"`
	CustomHeader  []CustomHeader    `json:"customHeader,omitempty"`
	ParamOverride *string           `json:"paramOverride,omitempty"`
}

type ChannelKeyInput struct {
	ChannelKey string `json:"channelKey"`
	Remark     string `json:"remark,omitempty"`
}

type ChannelUpdateRequest struct {
	ID            int               `json:"id"`
	Name          *string           `json:"name,omitempty"`
	Type          *int              `json:"type,omitempty"`
	Enabled       *bool             `json:"enabled,omitempty"`
	BaseUrls      []BaseUrl         `json:"baseUrls,omitempty"`
	Keys          []ChannelKeyInput `json:"keys,omitempty"`
	Model         []string          `json:"model,omitempty"`
	CustomModel   *string           `json:"customModel,omitempty"`
	Proxy         *bool             `json:"proxy,omitempty"`
	AutoSync      *bool             `json:"autoSync,omitempty"`
	AutoGroup     *int              `json:"autoGroup,omitempty"`
	CustomHeader  []CustomHeader    `json:"customHeader,omitempty"`
	ParamOverride *string           `json:"paramOverride,omitempty"`
	ChannelProxy  *string           `json:"channelProxy,omitempty"`
	FetchedModel  []string          `json:"fetchedModel,omitempty"`
}

type ChannelEnableRequest struct {
	ID      int  `json:"id"`
	Enabled bool `json:"enabled"`
}

// ──── Group ────

type GroupItemInput struct {
	ChannelID int    `json:"channelId"`
	ModelName string `json:"modelName"`
	Priority  int    `json:"priority,omitempty"`
	Weight    int    `json:"weight,omitempty"`
	Enabled   *bool  `json:"enabled,omitempty"`
}

type GroupCreateRequest struct {
	Name              string           `json:"name"`
	Mode              int              `json:"mode"`
	FirstTokenTimeOut int              `json:"firstTokenTimeOut,omitempty"`
	SessionKeepTime   int              `json:"sessionKeepTime,omitempty"`
	ProfileID         int              `json:"profileId"`
	Items             []GroupItemInput `json:"items"`
}

type GroupUpdateRequest struct {
	ID                int              `json:"id"`
	Name              *string          `json:"name,omitempty"`
	Mode              *int             `json:"mode,omitempty"`
	FirstTokenTimeOut *int             `json:"firstTokenTimeOut,omitempty"`
	SessionKeepTime   *int             `json:"sessionKeepTime,omitempty"`
	Items             []GroupItemInput `json:"items,omitempty"`
}

// ──── API Key ────

type APIKeyCreateRequest struct {
	Name            string  `json:"name"`
	ExpireAt        int64   `json:"expireAt,omitempty"`
	MaxCost         float64 `json:"maxCost,omitempty"`
	SupportedModels string  `json:"supportedModels,omitempty"`
	RPMLimit        int     `json:"rpmLimit,omitempty"`
	TPMLimit        int     `json:"tpmLimit,omitempty"`
}

type APIKeyUpdateRequest struct {
	ID              int      `json:"id"`
	Name            *string  `json:"name,omitempty"`
	Enabled         *bool    `json:"enabled,omitempty"`
	ExpireAt        *int64   `json:"expireAt,omitempty"`
	MaxCost         *float64 `json:"maxCost,omitempty"`
	SupportedModels *string  `json:"supportedModels,omitempty"`
	RPMLimit        *int     `json:"rpmLimit,omitempty"`
	TPMLimit        *int     `json:"tpmLimit,omitempty"`
}

// ──── Virtual Key ────

type VirtualKeyCreateRequest struct {
	Name          string   `json:"name" binding:"required"`
	Description   string   `json:"description"`
	TeamID        *int     `json:"teamId"`
	ApiKeyID      int      `json:"apiKeyId" binding:"required"`
	RateLimitRPM  int      `json:"rateLimitRpm"`
	RateLimitTPM  int      `json:"rateLimitTpm"`
	MaxBudget     float64  `json:"maxBudget"`
	AllowedModels []string `json:"allowedModels"`
	ExpiresAt     *string  `json:"expiresAt"` // ISO 8601
}

type VirtualKeyUpdateRequest struct {
	ID            int       `json:"id" binding:"required"`
	Name          *string   `json:"name,omitempty"`
	Description   *string   `json:"description,omitempty"`
	Enabled       *bool     `json:"enabled,omitempty"`
	RateLimitRPM  *int      `json:"rateLimitRpm,omitempty"`
	RateLimitTPM  *int      `json:"rateLimitTpm,omitempty"`
	MaxBudget     *float64  `json:"maxBudget,omitempty"`
	AllowedModels *[]string `json:"allowedModels,omitempty"`
}

// ──── Routing Rule ────

type RoutingRuleCreateRequest struct {
	Name          string                 `json:"name" binding:"required"`
	Priority      int                    `json:"priority"`
	Enabled       bool                   `json:"enabled"`
	CELExpression string                 `json:"cel_expression"`
	Conditions    []RoutingConditionItem `json:"conditions"`
	Action        RoutingActionItem      `json:"action" binding:"required"`
}

type RoutingRuleUpdateRequest struct {
	ID            int                    `json:"id"`
	Name          *string                `json:"name,omitempty"`
	Priority      *int                   `json:"priority,omitempty"`
	Enabled       *bool                  `json:"enabled,omitempty"`
	CELExpression *string                `json:"cel_expression,omitempty"`
	Conditions    []RoutingConditionItem `json:"conditions,omitempty"`
	Action        *RoutingActionItem     `json:"action,omitempty"`
}

// ──── Log ────

// ──── Audit Log ────

type AuditLogListOpts struct {
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
	User      string `json:"user"`
	Action    string `json:"action"`
	StartTime int64  `json:"startTime"`
	EndTime   int64  `json:"endTime"`
}

// ──── MCP Log ────

type MCPLogListOpts struct {
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
	ClientID  int    `json:"clientId"`
	ToolName  string `json:"toolName"`
	Status    string `json:"status"`
	StartTime int64  `json:"startTime"`
	EndTime   int64  `json:"endTime"`
}

// ──── Model Limit ────

type ModelLimitCreateRequest struct {
	Model         string `json:"model" binding:"required"`
	RPM           int    `json:"rpm"`
	TPM           int    `json:"tpm"`
	DailyRequests int    `json:"dailyRequests"`
	DailyTokens   int    `json:"dailyTokens"`
	Enabled       bool   `json:"enabled"`
}

type ModelLimitUpdateRequest struct {
	ID            int     `json:"id"`
	Model         *string `json:"model,omitempty"`
	RPM           *int    `json:"rpm,omitempty"`
	TPM           *int    `json:"tpm,omitempty"`
	DailyRequests *int    `json:"dailyRequests,omitempty"`
	DailyTokens   *int    `json:"dailyTokens,omitempty"`
	Enabled       *bool   `json:"enabled,omitempty"`
}

// ──── Stats ────

type GlobalStatsResponse struct {
	TotalRequests     int     `json:"totalRequests"`
	TotalInputTokens  int     `json:"totalInputTokens"`
	TotalOutputTokens int     `json:"totalOutputTokens"`
	TotalCost         float64 `json:"totalCost"`
	ActiveChannels    int     `json:"activeChannels"`
	ActiveGroups      int     `json:"activeGroups"`
}

type ChannelStatsItem struct {
	ChannelID     int     `json:"channelId"`
	ChannelName   string  `json:"channelName"`
	TotalRequests int     `json:"totalRequests"`
	TotalCost     float64 `json:"totalCost"`
	AvgLatency    float64 `json:"avgLatency"`
}

type ModelStatsItem struct {
	Model             string  `json:"model"`
	RequestCount      int     `json:"requestCount"`
	InputTokens       int     `json:"inputTokens"`
	OutputTokens      int     `json:"outputTokens"`
	TotalCost         float64 `json:"totalCost"`
	AvgLatency        int     `json:"avgLatency"`
	AvgFirstTokenTime int     `json:"avgFirstTokenTime"`
}

type DailyStatsItem struct {
	Date           string  `json:"date"`
	InputToken     int     `json:"input_token"`
	OutputToken    int     `json:"output_token"`
	InputCost      float64 `json:"input_cost"`
	OutputCost     float64 `json:"output_cost"`
	WaitTime       int     `json:"wait_time"`
	RequestSuccess int     `json:"request_success"`
	RequestFailed  int     `json:"request_failed"`
}

type HourlyStatsItem struct {
	Hour           int     `json:"hour"`
	Date           string  `json:"date"`
	InputToken     int     `json:"input_token"`
	OutputToken    int     `json:"output_token"`
	InputCost      float64 `json:"input_cost"`
	OutputCost     float64 `json:"output_cost"`
	WaitTime       int     `json:"wait_time"`
	RequestSuccess int     `json:"request_success"`
	RequestFailed  int     `json:"request_failed"`
}

// ──── Settings ────

type SettingsUpdateRequest struct {
	Settings map[string]string `json:"settings"`
}

// ──── LLM Price ────

type LLMCreateRequest struct {
	Name        string  `json:"name"`
	InputPrice  float64 `json:"inputPrice"`
	OutputPrice float64 `json:"outputPrice"`
}

type LLMUpdateRequest struct {
	ID          int      `json:"id"`
	Name        *string  `json:"name,omitempty"`
	InputPrice  *float64 `json:"inputPrice,omitempty"`
	OutputPrice *float64 `json:"outputPrice,omitempty"`
}

type LLMDeleteRequest struct {
	ID int `json:"id"`
}

// ──── Model Profile ────

type ProfileCreateRequest struct {
	Name     string   `json:"name"`
	Provider string   `json:"provider,omitempty"`
	Models   []string `json:"models,omitempty"`
}

type ProfileUpdateRequest struct {
	ID       int      `json:"id"`
	Name     string   `json:"name"`
	Provider *string  `json:"provider,omitempty"`
	Models   []string `json:"models,omitempty"`
}

type ProfileDeleteRequest struct {
	ID int `json:"id"`
}

// ──── Guardrail Rule ────

type GuardrailRuleCreateRequest struct {
	Name      string `json:"name" binding:"required"`
	Type      string `json:"type" binding:"required"`
	Target    string `json:"target" binding:"required"`
	Action    string `json:"action" binding:"required"`
	Pattern   string `json:"pattern"`
	MaxLength int    `json:"maxLength"`
	Enabled   bool   `json:"enabled"`
}

type GuardrailRuleUpdateRequest struct {
	ID        int     `json:"id"`
	Name      *string `json:"name,omitempty"`
	Type      *string `json:"type,omitempty"`
	Target    *string `json:"target,omitempty"`
	Action    *string `json:"action,omitempty"`
	Pattern   *string `json:"pattern,omitempty"`
	MaxLength *int    `json:"maxLength,omitempty"`
	Enabled   *bool   `json:"enabled,omitempty"`
}

// ──── Tag ────

type TagCreateRequest struct {
	Name        string `json:"name" binding:"required"`
	Color       string `json:"color" binding:"required"`
	Description string `json:"description"`
}

type TagUpdateRequest struct {
	ID          int     `json:"id"`
	Name        *string `json:"name,omitempty"`
	Color       *string `json:"color,omitempty"`
	Description *string `json:"description,omitempty"`
}

// ──── Sync ────

type SyncResult struct {
	SyncedChannels int      `json:"syncedChannels"`
	NewModels      []string `json:"newModels"`
	RemovedModels  []string `json:"removedModels"`
	Errors         []string `json:"errors"`
}

// ──── Data Export / Import ────

type DBDump struct {
	Version    int        `json:"version"`
	ExportedAt string     `json:"exportedAt"`
	Channels   []Channel  `json:"channels"`
	Groups     []Group    `json:"groups"`
	APIKeys    []APIKey   `json:"apiKeys"`
	Settings   []Setting  `json:"settings"`
	RelayLogs  []RelayLog `json:"relayLogs,omitempty"`
}

type ImportResult struct {
	Channels ImportCount `json:"channels"`
	Groups   ImportCount `json:"groups"`
	APIKeys  ImportCount `json:"apiKeys"`
	Settings ImportCount `json:"settings"`
}

type ImportCount struct {
	Added   int `json:"added"`
	Skipped int `json:"skipped"`
}

// ──── WebSocket / Streaming ────

// BroadcastFunc is the signature for WebSocket broadcast.
type BroadcastFunc func(event string, data ...any)

// StreamTracker tracks active streams so new WS clients get a snapshot.
type StreamTracker interface {
	TrackStream(streamId string, data map[string]any)
	UntrackStream(streamId string)
}
