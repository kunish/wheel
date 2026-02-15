package types

// ──── Auth ────

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

type ChangeUsernameRequest struct {
	Username string `json:"username"`
}

// ──── Channel ────

type ChannelCreateRequest struct {
	Name          string           `json:"name"`
	Type          int              `json:"type"`
	Enabled       bool             `json:"enabled"`
	BaseUrls      []BaseUrl        `json:"baseUrls"`
	Keys          []ChannelKeyInput `json:"keys"`
	Model         []string         `json:"model"`
	CustomModel   string           `json:"customModel,omitempty"`
	AutoSync      *bool            `json:"autoSync,omitempty"`
	AutoGroup     *int             `json:"autoGroup,omitempty"`
	CustomHeader  []CustomHeader   `json:"customHeader,omitempty"`
	ParamOverride *string          `json:"paramOverride,omitempty"`
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
	Items             []GroupItemInput  `json:"items"`
}

type GroupUpdateRequest struct {
	ID                int              `json:"id"`
	Name              *string          `json:"name,omitempty"`
	Mode              *int             `json:"mode,omitempty"`
	FirstTokenTimeOut *int             `json:"firstTokenTimeOut,omitempty"`
	SessionKeepTime   *int             `json:"sessionKeepTime,omitempty"`
	Items             []GroupItemInput  `json:"items,omitempty"`
}

// ──── API Key ────

type APIKeyCreateRequest struct {
	Name            string  `json:"name"`
	ExpireAt        int64   `json:"expireAt,omitempty"`
	MaxCost         float64 `json:"maxCost,omitempty"`
	SupportedModels string  `json:"supportedModels,omitempty"`
}

type APIKeyUpdateRequest struct {
	ID              int     `json:"id"`
	Name            *string `json:"name,omitempty"`
	Enabled         *bool   `json:"enabled,omitempty"`
	ExpireAt        *int64  `json:"expireAt,omitempty"`
	MaxCost         *float64 `json:"maxCost,omitempty"`
	SupportedModels *string `json:"supportedModels,omitempty"`
}

// ──── Log ────

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
	ChannelID    int     `json:"channelId"`
	ChannelName  string  `json:"channelName"`
	TotalRequests int    `json:"totalRequests"`
	TotalCost    float64 `json:"totalCost"`
	AvgLatency   float64 `json:"avgLatency"`
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
