package mcp

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"sync"

	mcpclient "github.com/mark3labs/mcp-go/client"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/uptrace/bun"
)

// ──── Enums ────

// MCPConnectionType defines the transport protocol for MCP connections.
type MCPConnectionType string

const (
	ConnectionTypeHTTP  MCPConnectionType = "http"
	ConnectionTypeSSE   MCPConnectionType = "sse"
	ConnectionTypeSTDIO MCPConnectionType = "stdio"
)

// MCPConnectionState represents the runtime state of an MCP client connection.
type MCPConnectionState string

const (
	StateConnected    MCPConnectionState = "connected"
	StateDisconnected MCPConnectionState = "disconnected"
	StateError        MCPConnectionState = "error"
)

// MCPAuthType defines the authentication method for MCP connections.
type MCPAuthType string

const (
	AuthTypeNone    MCPAuthType = "none"
	AuthTypeHeaders MCPAuthType = "headers"
	AuthTypeOAuth   MCPAuthType = "oauth"
)

// ──── STDIO Config ────

// MCPStdioConfig defines how to launch a STDIO-based MCP server process.
type MCPStdioConfig struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Envs    []string `json:"envs"`
}

// ──── Header Entry ────

// MCPHeaderEntry is a key-value pair for custom auth headers.
type MCPHeaderEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ──── OAuth Config ────

// MCPOAuthConfig defines the OAuth 2.0 configuration for connecting to an OAuth-protected MCP server.
type MCPOAuthConfig struct {
	ClientID         string `json:"clientId"`
	ClientSecret     string `json:"clientSecret,omitempty"`
	TokenURL         string `json:"tokenUrl"`
	AuthorizationURL string `json:"authorizationUrl,omitempty"` // For reference / UI display
	Scopes           string `json:"scopes,omitempty"`           // Space-separated scopes
	AccessToken      string `json:"accessToken,omitempty"`      // Pre-configured access token (skip client_credentials)
}

// ──── DB Model ────

// MCPClient is the persistent DB model for an MCP client configuration.
type MCPClient struct {
	bun.BaseModel `bun:"table:mcp_clients"`

	ID               int               `bun:"id,pk,autoincrement"  json:"id"`
	Name             string            `bun:"name"                 json:"name"`
	ConnectionType   MCPConnectionType `bun:"connection_type"      json:"connectionType"`
	ConnectionString string            `bun:"connection_string"    json:"connectionString"`
	StdioConfig      StdioConfigJSON   `bun:"stdio_config"         json:"stdioConfig"`
	AuthType         MCPAuthType       `bun:"auth_type"            json:"authType"`
	Headers          HeaderListJSON    `bun:"headers"              json:"headers"`
	OAuthConfig      OAuthConfigJSON   `bun:"oauth_config"         json:"oauthConfig"`
	ToolsToExecute   StringListJSON    `bun:"tools_to_execute"     json:"toolsToExecute"`
	ToolsToAutoExec  StringListJSON    `bun:"tools_to_auto_exec"   json:"toolsToAutoExec"`
	Enabled          bool              `bun:"enabled"              json:"enabled"`
	CreatedAt        *string           `bun:"created_at"           json:"createdAt,omitempty"`
	UpdatedAt        *string           `bun:"updated_at"           json:"updatedAt,omitempty"`
}

// ──── JSON Scanner/Valuer types for DB TEXT columns ────

// StdioConfigJSON stores MCPStdioConfig as JSON TEXT in DB.
type StdioConfigJSON MCPStdioConfig

func (s *StdioConfigJSON) Scan(src any) error {
	if src == nil {
		*s = StdioConfigJSON{}
		return nil
	}
	var data []byte
	switch v := src.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		return fmt.Errorf("StdioConfigJSON.Scan: unsupported type %T", src)
	}
	return json.Unmarshal(data, s)
}

func (s StdioConfigJSON) Value() (driver.Value, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

// OAuthConfigJSON stores MCPOAuthConfig as JSON TEXT in DB.
type OAuthConfigJSON MCPOAuthConfig

func (o *OAuthConfigJSON) Scan(src any) error {
	if src == nil {
		*o = OAuthConfigJSON{}
		return nil
	}
	var data []byte
	switch v := src.(type) {
	case string:
		if v == "" {
			*o = OAuthConfigJSON{}
			return nil
		}
		data = []byte(v)
	case []byte:
		if len(v) == 0 {
			*o = OAuthConfigJSON{}
			return nil
		}
		data = v
	default:
		return fmt.Errorf("OAuthConfigJSON.Scan: unsupported type %T", src)
	}
	return json.Unmarshal(data, o)
}

func (o OAuthConfigJSON) Value() (driver.Value, error) {
	data, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

// HeaderListJSON stores []MCPHeaderEntry as JSON TEXT in DB.
type HeaderListJSON []MCPHeaderEntry

func (h *HeaderListJSON) Scan(src any) error {
	if src == nil {
		*h = []MCPHeaderEntry{}
		return nil
	}
	var data []byte
	switch v := src.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		return fmt.Errorf("HeaderListJSON.Scan: unsupported type %T", src)
	}
	return json.Unmarshal(data, h)
}

func (h HeaderListJSON) Value() (driver.Value, error) {
	if h == nil {
		return "[]", nil
	}
	data, err := json.Marshal(h)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

// StringListJSON stores []string as JSON TEXT in DB.
type StringListJSON []string

func (s *StringListJSON) Scan(src any) error {
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
		return fmt.Errorf("StringListJSON.Scan: unsupported type %T", src)
	}
	return json.Unmarshal(data, s)
}

func (s StringListJSON) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	data, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

// ──── Runtime State (in-memory only) ────

// ClientState holds the live connection and discovered tools for a connected MCP client.
type ClientState struct {
	mu              sync.RWMutex
	Config          *MCPClient             // Persisted config from DB
	Conn            mcpclient.MCPClient    // Active mcp-go client connection
	State           MCPConnectionState     // Current connection state
	Tools           map[string]mcplib.Tool // Discovered tools keyed by name
	ToolNameMapping map[string]string      // sanitized_name -> original_mcp_name
	CancelFunc      func()                 // Cancel function for SSE/STDIO connections
	ErrorMsg        string                 // Last error message if State == StateError
	OAuthToken      *OAuthToken            // Cached OAuth token (in-memory only)
}

// GetState returns the current connection state thread-safely.
func (cs *ClientState) GetState() MCPConnectionState {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.State
}

// SetState updates the connection state thread-safely.
func (cs *ClientState) SetState(state MCPConnectionState, errMsg string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.State = state
	cs.ErrorMsg = errMsg
}

// GetTools returns a snapshot of the current tool map.
func (cs *ClientState) GetTools() map[string]mcplib.Tool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	cp := make(map[string]mcplib.Tool, len(cs.Tools))
	for k, v := range cs.Tools {
		cp[k] = v
	}
	return cp
}

// ──── API Request/Response Types ────

// MCPClientCreateRequest is the request body for creating an MCP client.
type MCPClientCreateRequest struct {
	Name             string            `json:"name" binding:"required"`
	ConnectionType   MCPConnectionType `json:"connectionType" binding:"required"`
	ConnectionString string            `json:"connectionString,omitempty"`
	StdioConfig      *MCPStdioConfig   `json:"stdioConfig,omitempty"`
	AuthType         MCPAuthType       `json:"authType"`
	Headers          []MCPHeaderEntry  `json:"headers,omitempty"`
	OAuthConfig      *MCPOAuthConfig   `json:"oauthConfig,omitempty"`
	ToolsToExecute   []string          `json:"toolsToExecute,omitempty"`
	ToolsToAutoExec  []string          `json:"toolsToAutoExec,omitempty"`
	Enabled          bool              `json:"enabled"`
}

// MCPClientUpdateRequest is the request body for updating an MCP client.
type MCPClientUpdateRequest struct {
	ID               int                `json:"id"`
	Name             *string            `json:"name,omitempty"`
	ConnectionType   *MCPConnectionType `json:"connectionType,omitempty"`
	ConnectionString *string            `json:"connectionString,omitempty"`
	StdioConfig      *MCPStdioConfig    `json:"stdioConfig,omitempty"`
	AuthType         *MCPAuthType       `json:"authType,omitempty"`
	Headers          []MCPHeaderEntry   `json:"headers,omitempty"`
	OAuthConfig      *MCPOAuthConfig    `json:"oauthConfig,omitempty"`
	ToolsToExecute   []string           `json:"toolsToExecute,omitempty"`
	ToolsToAutoExec  []string           `json:"toolsToAutoExec,omitempty"`
	Enabled          *bool              `json:"enabled,omitempty"`
}

// MCPClientResponse is the API response for a single MCP client, including runtime state.
type MCPClientResponse struct {
	ID               int                `json:"id"`
	Name             string             `json:"name"`
	ConnectionType   MCPConnectionType  `json:"connectionType"`
	ConnectionString string             `json:"connectionString"`
	StdioConfig      *MCPStdioConfig    `json:"stdioConfig,omitempty"`
	AuthType         MCPAuthType        `json:"authType"`
	Headers          []MCPHeaderEntry   `json:"headers,omitempty"`
	OAuthConfig      *MCPOAuthConfig    `json:"oauthConfig,omitempty"`
	ToolsToExecute   []string           `json:"toolsToExecute"`
	ToolsToAutoExec  []string           `json:"toolsToAutoExec"`
	Enabled          bool               `json:"enabled"`
	State            MCPConnectionState `json:"state"`
	ErrorMsg         string             `json:"errorMsg,omitempty"`
	Tools            []ToolInfo         `json:"tools"`
	CreatedAt        *string            `json:"createdAt,omitempty"`
	UpdatedAt        *string            `json:"updatedAt,omitempty"`
}

// ToolInfo is a simplified tool description for API responses.
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ToolExecuteRequest is the request body for REST tool execution (/v1/mcp/tool/execute).
type ToolExecuteRequest struct {
	ClientID  int            `json:"clientId" binding:"required"`
	ToolName  string         `json:"toolName" binding:"required"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// ToolExecuteResponse is the response for REST tool execution.
type ToolExecuteResponse struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ToolContent represents a single content item in a tool execution result.
type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// AggregatedTool represents a tool from a specific MCP client, used by the Manager
// to track which client owns which tool.
type AggregatedTool struct {
	ClientID     int         `json:"clientId"`
	ClientName   string      `json:"clientName"`
	Tool         mcplib.Tool `json:"tool"`
	OriginalName string      `json:"originalName"` // Original MCP tool name before sanitization
}
