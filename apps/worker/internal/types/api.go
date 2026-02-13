package types

// ──── Auth ────

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token    string `json:"token"`
	ExpireAt string `json:"expireAt"`
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
	AutoSync      *bool             `json:"autoSync,omitempty"`
	AutoGroup     *int              `json:"autoGroup,omitempty"`
	CustomHeader  []CustomHeader    `json:"customHeader,omitempty"`
	ParamOverride *string           `json:"paramOverride,omitempty"`
}

type ChannelEnableRequest struct {
	ID      int  `json:"id"`
	Enabled bool `json:"enabled"`
}

type ChannelListResponse struct {
	Channels []Channel `json:"channels"`
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

type GroupListResponse struct {
	Groups []Group `json:"groups"`
}

type ModelListResponse struct {
	Models []string `json:"models"`
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

type APIKeyListResponse struct {
	APIKeys []APIKey `json:"apiKeys"`
}

type APIKeyStatsResponse struct {
	TotalRequests    int     `json:"totalRequests"`
	TotalCost        float64 `json:"totalCost"`
	TotalInputTokens int     `json:"totalInputTokens"`
	TotalOutputTokens int    `json:"totalOutputTokens"`
}

// ──── Log ────

type LogListRequest struct {
	Page      int    `json:"page,omitempty"`
	PageSize  int    `json:"pageSize,omitempty"`
	Model     string `json:"model,omitempty"`
	ChannelID int    `json:"channelId,omitempty"`
	Status    string `json:"status,omitempty"`
	StartTime int64  `json:"startTime,omitempty"`
	EndTime   int64  `json:"endTime,omitempty"`
}

type LogListResponse struct {
	Logs     []RelayLog `json:"logs"`
	Total    int        `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"pageSize"`
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
	ChannelID    int     `json:"channelId"`
	ChannelName  string  `json:"channelName"`
	TotalRequests int    `json:"totalRequests"`
	TotalCost    float64 `json:"totalCost"`
	AvgLatency   float64 `json:"avgLatency"`
}

type ChannelStatsResponse struct {
	Channels []ChannelStatsItem `json:"channels"`
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

type SettingsResponse struct {
	Settings map[string]string `json:"settings"`
}

type SettingsUpdateRequest struct {
	Settings map[string]string `json:"settings"`
}

// ──── LLM Price ────

type LLMListResponse struct {
	Models []LLMInfo `json:"models"`
}

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

type LLMPriceSyncResponse struct {
	Synced  int `json:"synced"`
	Updated int `json:"updated"`
}

// ──── Sync ────

type SyncResult struct {
	SyncedChannels int      `json:"syncedChannels"`
	NewModels      []string `json:"newModels"`
	RemovedModels  []string `json:"removedModels"`
	Errors         []string `json:"errors"`
}

type FetchModelResponse struct {
	Models []string `json:"models"`
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

// ──── Generic API Response ────

type ApiResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}
