package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/uptrace/bun"
)

// BaseUrl represents an upstream URL with an optional delay.
type BaseUrl struct {
	URL   string `json:"url"`
	Delay int    `json:"delay"`
}

// CustomHeader is a key-value pair for custom HTTP headers.
type CustomHeader struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ChannelKey represents an API key associated with a channel.
type ChannelKey struct {
	bun.BaseModel    `bun:"table:channel_keys"`
	ID               int     `bun:"id,pk,autoincrement" json:"id"`
	ChannelID        int     `bun:"channel_id"          json:"channelId"`
	Enabled          bool    `bun:"enabled"             json:"enabled"`
	ChannelKey       string  `bun:"channel_key"         json:"channelKey"`
	StatusCode       int     `bun:"status_code"         json:"statusCode"`
	LastUseTimestamp int64   `bun:"last_use_timestamp"  json:"lastUseTimestamp"`
	TotalCost        float64 `bun:"total_cost"          json:"totalCost"`
	Remark           string  `bun:"remark"              json:"remark"`
}

// Channel represents an upstream provider channel.
type Channel struct {
	bun.BaseModel `bun:"table:channels"`
	ID            int              `bun:"id,pk,autoincrement" json:"id"`
	Name          string           `bun:"name"                json:"name"`
	Type          OutboundType     `bun:"type"                json:"type"`
	Enabled       bool             `bun:"enabled"             json:"enabled"`
	BaseUrls      BaseUrlList      `bun:"base_urls"           json:"baseUrls"`
	Keys          []ChannelKey     `bun:"-"                   json:"keys"`
	Model         StringList       `bun:"model"               json:"model"`
	FetchedModel  StringList       `bun:"fetched_model"       json:"fetchedModel"`
	CustomModel   string           `bun:"custom_model"        json:"customModel"`
	Proxy         bool             `bun:"proxy"               json:"proxy"`
	AutoSync      bool             `bun:"auto_sync"           json:"autoSync"`
	AutoGroup     AutoGroupType    `bun:"auto_group"          json:"autoGroup"`
	CustomHeader  CustomHeaderList `bun:"custom_header"       json:"customHeader"`
	ParamOverride *string          `bun:"param_override"      json:"paramOverride"`
	ChannelProxy  *string          `bun:"channel_proxy"       json:"channelProxy"`
	Order         int              `bun:"order,default:0" json:"order"`
}

// GroupItem links a channel to a group with routing metadata.
type GroupItem struct {
	bun.BaseModel `bun:"table:group_items"`
	ID            int    `bun:"id,pk,autoincrement" json:"id"`
	GroupID       int    `bun:"group_id"            json:"groupId"`
	ChannelID     int    `bun:"channel_id"          json:"channelId"`
	ModelName     string `bun:"model_name"          json:"modelName"`
	Priority      int    `bun:"priority"            json:"priority"`
	Weight        int    `bun:"weight"              json:"weight"`
	Enabled       bool   `bun:"enabled"             json:"enabled"`
}

// Group defines a routing group of channels.
type Group struct {
	bun.BaseModel     `bun:"table:groups"`
	ID                int         `bun:"id,pk,autoincrement"    json:"id"`
	Name              string      `bun:"name"                   json:"name"`
	Mode              GroupMode   `bun:"mode"                   json:"mode"`
	FirstTokenTimeOut int         `bun:"first_token_time_out"   json:"firstTokenTimeOut"`
	SessionKeepTime   int         `bun:"session_keep_time"      json:"sessionKeepTime"`
	ProfileID         int         `bun:"profile_id"             json:"profileId"`
	Order             int         `bun:"order"                  json:"order,omitempty"`
	Items             []GroupItem `bun:"-"                      json:"items"`
}

// APIKey represents a user-facing API key.
type APIKey struct {
	bun.BaseModel   `bun:"table:api_keys"`
	ID              int     `bun:"id,pk,autoincrement" json:"id"`
	Name            string  `bun:"name"                json:"name"`
	APIKey          string  `bun:"api_key"             json:"apiKey"`
	Enabled         bool    `bun:"enabled"             json:"enabled"`
	ExpireAt        int64   `bun:"expire_at"           json:"expireAt"`
	MaxCost         float64 `bun:"max_cost"            json:"maxCost"`
	SupportedModels string  `bun:"supported_models"    json:"supportedModels"`
	TotalCost       float64 `bun:"total_cost"          json:"totalCost"`
}

// ChannelAttempt records a single relay attempt.
type ChannelAttempt struct {
	ChannelID    int           `json:"channelId"`
	ChannelKeyID *int          `json:"channelKeyId,omitempty"`
	ChannelName  string        `json:"channelName"`
	ModelName    string        `json:"modelName"`
	AttemptNum   int           `json:"attemptNum"`
	Status       AttemptStatus `json:"status"`
	Duration     int           `json:"duration"`
	Sticky       *bool         `json:"sticky,omitempty"`
	Msg          *string       `json:"msg,omitempty"`
}

// RelayLog records a single relay request with its outcome.
type RelayLog struct {
	bun.BaseModel    `bun:"table:relay_logs"`
	ID               int         `bun:"id,pk,autoincrement"  json:"id"`
	Time             int64       `bun:"time"                 json:"time"`
	RequestModelName string      `bun:"request_model_name"   json:"requestModelName"`
	ChannelID        int         `bun:"channel_id"           json:"channelId"`
	ChannelName      string      `bun:"channel_name"         json:"channelName"`
	ActualModelName  string      `bun:"actual_model_name"    json:"actualModelName"`
	InputTokens      int         `bun:"input_tokens"         json:"inputTokens"`
	OutputTokens     int         `bun:"output_tokens"        json:"outputTokens"`
	FTUT             int         `bun:"ftut"                 json:"ftut"`
	UseTime          int         `bun:"use_time"             json:"useTime"`
	Cost             float64     `bun:"cost"                 json:"cost"`
	RequestContent   string      `bun:"request_content"      json:"requestContent"`
	UpstreamContent  *string     `bun:"upstream_content"     json:"upstreamContent,omitempty"`
	ResponseContent  string      `bun:"response_content"     json:"responseContent"`
	Error            string      `bun:"error"                json:"error"`
	Attempts         AttemptList `bun:"attempts"             json:"attempts"`
	TotalAttempts    int         `bun:"total_attempts"       json:"totalAttempts"`
}

// User represents an admin user.
type User struct {
	bun.BaseModel `bun:"table:users"`
	ID            int    `bun:"id,pk,autoincrement" json:"id"`
	Username      string `bun:"username"            json:"username"`
	Password      string `bun:"password"            json:"password"`
}

// Setting is a key-value configuration entry.
type Setting struct {
	bun.BaseModel `bun:"table:settings"`
	Key           string `bun:"key,pk" json:"key"`
	Value         string `bun:"value"  json:"value"`
}

// LLMPrice holds pricing info for a model (per million tokens).
type LLMPrice struct {
	bun.BaseModel   `bun:"table:llm_prices"`
	ID              int     `bun:"id,pk,autoincrement" json:"id"`
	Name            string  `bun:"name"                json:"name"`
	InputPrice      float64 `bun:"input_price"         json:"inputPrice"`
	OutputPrice     float64 `bun:"output_price"        json:"outputPrice"`
	CacheReadPrice  float64 `bun:"cache_read_price"    json:"cacheReadPrice"`
	CacheWritePrice float64 `bun:"cache_write_price"   json:"cacheWritePrice"`
	Source          string  `bun:"source"              json:"source"`
	CreatedAt       *string `bun:"created_at"          json:"createdAt,omitempty"`
	UpdatedAt       *string `bun:"updated_at"          json:"updatedAt,omitempty"`
}

// LLMInfo is the public-facing model pricing info.
type LLMInfo struct {
	ID          *int    `json:"id,omitempty"`
	Name        string  `json:"name"`
	InputPrice  float64 `json:"inputPrice"`
	OutputPrice float64 `json:"outputPrice"`
	Source      string  `json:"source"`
	CreatedAt   *string `json:"createdAt,omitempty"`
	UpdatedAt   *string `json:"updatedAt,omitempty"`
}

// ModelProfile represents a workspace for organizing groups.
type ModelProfile struct {
	bun.BaseModel `bun:"table:model_profiles"`
	ID            int        `bun:"id,pk,autoincrement" json:"id"`
	Name          string     `bun:"name"                json:"name"`
	Provider      string     `bun:"provider"            json:"provider"`
	Models        StringList `bun:"models"           json:"models"`
	IsBuiltin     bool       `bun:"is_builtin"          json:"isBuiltin"`
	CreatedAt     *string    `bun:"created_at"          json:"createdAt,omitempty"`
	UpdatedAt     *string    `bun:"updated_at"          json:"updatedAt,omitempty"`
	GroupCount    int        `bun:"-"                   json:"groupCount"`
}

// ──── JSON Scanner/Valuer types for SQLite TEXT columns ────

// BaseUrlList is a []BaseUrl that scans/values as JSON TEXT.
type BaseUrlList []BaseUrl

func (b *BaseUrlList) Scan(src any) error {
	if src == nil {
		*b = []BaseUrl{}
		return nil
	}
	var data []byte
	switch v := src.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		return fmt.Errorf("BaseUrlList.Scan: unsupported type %T", src)
	}
	return json.Unmarshal(data, b)
}

func (b BaseUrlList) Value() (driver.Value, error) {
	if b == nil {
		return "[]", nil
	}
	data, err := json.Marshal(b)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

// StringList is a []string that scans/values as JSON TEXT.
type StringList []string

func (s *StringList) Scan(src any) error {
	if src == nil {
		*s = []string{}
		return nil
	}
	var data []byte
	switch v := src.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		return fmt.Errorf("StringList.Scan: unsupported type %T", src)
	}
	return json.Unmarshal(data, s)
}

func (s StringList) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	data, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

// CustomHeaderList is a []CustomHeader that scans/values as JSON TEXT.
type CustomHeaderList []CustomHeader

func (c *CustomHeaderList) Scan(src any) error {
	if src == nil {
		*c = []CustomHeader{}
		return nil
	}
	var data []byte
	switch v := src.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		return fmt.Errorf("CustomHeaderList.Scan: unsupported type %T", src)
	}
	return json.Unmarshal(data, c)
}

func (c CustomHeaderList) Value() (driver.Value, error) {
	if c == nil {
		return "[]", nil
	}
	data, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

// AttemptList is a []ChannelAttempt that scans/values as JSON TEXT.
type AttemptList []ChannelAttempt

func (a *AttemptList) Scan(src any) error {
	if src == nil {
		*a = []ChannelAttempt{}
		return nil
	}
	var data []byte
	switch v := src.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		return fmt.Errorf("AttemptList.Scan: unsupported type %T", src)
	}
	return json.Unmarshal(data, a)
}

func (a AttemptList) Value() (driver.Value, error) {
	if a == nil {
		return "[]", nil
	}
	data, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}
