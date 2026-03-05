package mcp

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

// Manager manages the lifecycle of MCP client connections.
type Manager struct {
	mu      sync.RWMutex
	clients map[int]*ClientState

	tokenManager     *TokenManager
	toolSyncInterval time.Duration
	stopSync         chan struct{}
}

// NewManager creates a new MCP Manager.
func NewManager() *Manager {
	return &Manager{
		clients:          make(map[int]*ClientState),
		tokenManager:     NewTokenManager(),
		toolSyncInterval: 10 * time.Minute,
		stopSync:         make(chan struct{}),
	}
}

// AddClient connects to an MCP server, discovers tools, and tracks the client.
func (m *Manager) AddClient(ctx context.Context, cfg *MCPClient) error {
	// Acquire OAuth token if needed
	var oauthBearerToken string
	if cfg.AuthType == AuthTypeOAuth {
		oauthCfg := MCPOAuthConfig(cfg.OAuthConfig)
		token, err := m.tokenManager.GetBearerToken(cfg.ID, &oauthCfg)
		if err != nil {
			return fmt.Errorf("OAuth token acquisition for %s failed: %w", cfg.Name, err)
		}
		oauthBearerToken = token
	}

	conn, cancel, err := Connect(ctx, cfg, oauthBearerToken)
	if err != nil {
		return fmt.Errorf("connect to %s failed: %w", cfg.Name, err)
	}

	// Initialize the MCP connection
	initReq := mcplib.InitializeRequest{
		Params: mcplib.InitializeParams{
			ProtocolVersion: mcplib.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcplib.Implementation{
				Name:    "wheel",
				Version: "1.0.0",
			},
		},
	}
	if _, err := conn.Initialize(ctx, initReq); err != nil {
		cancel()
		return fmt.Errorf("initialize %s failed: %w", cfg.Name, err)
	}

	// Discover tools
	tools, nameMapping, err := discoverTools(ctx, conn, cfg)
	if err != nil {
		cancel()
		return fmt.Errorf("discover tools for %s failed: %w", cfg.Name, err)
	}

	state := &ClientState{
		Config:          cfg,
		Conn:            conn,
		State:           StateConnected,
		Tools:           tools,
		ToolNameMapping: nameMapping,
		CancelFunc:      cancel,
	}

	m.mu.Lock()
	m.clients[cfg.ID] = state
	m.mu.Unlock()

	log.Printf("[mcp] client %q (id=%d) connected, %d tools discovered", cfg.Name, cfg.ID, len(tools))
	return nil
}

// discoverTools lists tools from a connected MCP client and applies filtering.
func discoverTools(ctx context.Context, conn interface {
	ListTools(context.Context, mcplib.ListToolsRequest) (*mcplib.ListToolsResult, error)
}, cfg *MCPClient) (map[string]mcplib.Tool, map[string]string, error) {
	result, err := conn.ListTools(ctx, mcplib.ListToolsRequest{})
	if err != nil {
		return nil, nil, err
	}

	tools := make(map[string]mcplib.Tool, len(result.Tools))
	nameMapping := make(map[string]string, len(result.Tools))

	for _, tool := range result.Tools {
		if !isToolAllowed(tool.Name, cfg.ToolsToExecute) {
			continue
		}
		sanitized := sanitizeToolName(cfg.Name, tool.Name)
		tools[sanitized] = tool
		nameMapping[sanitized] = tool.Name
	}
	return tools, nameMapping, nil
}

// isToolAllowed checks if a tool name is permitted by the ToolsToExecute filter.
// [] or nil = deny all, ["*"] = allow all, ["a","b"] = only those.
func isToolAllowed(toolName string, filter StringListJSON) bool {
	if len(filter) == 0 {
		return false
	}
	if len(filter) == 1 && filter[0] == "*" {
		return true
	}
	for _, name := range filter {
		if name == toolName {
			return true
		}
	}
	return false
}

// sanitizeToolName creates a unique, LLM-friendly tool name: "clientName_toolName".
// Replaces hyphens and spaces with underscores.
func sanitizeToolName(clientName, toolName string) string {
	s := clientName + "_" + toolName
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

// RemoveClient disconnects and removes a client from the manager.
func (m *Manager) RemoveClient(id int) {
	m.mu.Lock()
	state, ok := m.clients[id]
	if ok {
		delete(m.clients, id)
	}
	m.mu.Unlock()

	if ok && state.CancelFunc != nil {
		state.CancelFunc()
	}
	if ok && state.Conn != nil {
		if err := state.Conn.Close(); err != nil {
			log.Printf("[mcp] close client id=%d failed: %v", id, err)
		}
	}
	// Clean up cached OAuth token
	m.tokenManager.InvalidateToken(id)
	if ok {
		log.Printf("[mcp] client id=%d removed", id)
	}
}

// UpdateClient disconnects the old connection and reconnects with updated config.
func (m *Manager) UpdateClient(ctx context.Context, cfg *MCPClient) error {
	m.RemoveClient(cfg.ID)
	if !cfg.Enabled {
		return nil
	}
	return m.AddClient(ctx, cfg)
}

// ReconnectClient reconnects an existing client using its stored config.
func (m *Manager) ReconnectClient(ctx context.Context, id int) error {
	m.mu.RLock()
	state, ok := m.clients[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("client id=%d not found", id)
	}
	cfg := state.Config
	m.RemoveClient(id)
	return m.AddClient(ctx, cfg)
}

// GetClientState returns the runtime state for a specific client.
func (m *Manager) GetClientState(id int) (*ClientState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.clients[id]
	return s, ok
}

// GetAllClientStates returns a snapshot of all client states.
func (m *Manager) GetAllClientStates() map[int]*ClientState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cp := make(map[int]*ClientState, len(m.clients))
	for k, v := range m.clients {
		cp[k] = v
	}
	return cp
}

// GetAllTools returns all discovered tools across all connected clients.
// Keys are sanitized tool names, values include the client ID and tool.
func (m *Manager) GetAllTools() map[string]AggregatedTool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]AggregatedTool)
	for _, state := range m.clients {
		if state.GetState() != StateConnected {
			continue
		}
		for name, tool := range state.GetTools() {
			result[name] = AggregatedTool{
				ClientID:     state.Config.ID,
				ClientName:   state.Config.Name,
				Tool:         tool,
				OriginalName: state.ToolNameMapping[name],
			}
		}
	}
	return result
}

// ExecuteTool calls a tool on a specific MCP client by client ID and original tool name.
func (m *Manager) ExecuteTool(ctx context.Context, clientID int, toolName string, args map[string]any) (*mcplib.CallToolResult, error) {
	m.mu.RLock()
	state, ok := m.clients[clientID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("client id=%d not found", clientID)
	}
	if state.GetState() != StateConnected {
		return nil, fmt.Errorf("client id=%d is not connected", clientID)
	}
	if !isToolAllowed(toolName, state.Config.ToolsToExecute) {
		return nil, fmt.Errorf("tool %q is not allowed for client id=%d", toolName, clientID)
	}

	req := mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name:      toolName,
			Arguments: args,
		},
	}
	return state.Conn.CallTool(ctx, req)
}

// SyncTools re-discovers tools for all connected clients.
func (m *Manager) SyncTools(ctx context.Context) {
	m.mu.RLock()
	ids := make([]int, 0, len(m.clients))
	for id := range m.clients {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	for _, id := range ids {
		m.mu.RLock()
		state, ok := m.clients[id]
		m.mu.RUnlock()
		if !ok || state.GetState() != StateConnected {
			continue
		}
		tools, nameMapping, err := discoverTools(ctx, state.Conn, state.Config)
		if err != nil {
			log.Printf("[mcp] sync tools for %q failed: %v", state.Config.Name, err)
			state.SetState(StateError, err.Error())
			continue
		}
		state.mu.Lock()
		state.Tools = tools
		state.ToolNameMapping = nameMapping
		state.mu.Unlock()
	}
}

// StartToolSync starts a background goroutine that periodically re-discovers tools.
func (m *Manager) StartToolSync(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(m.toolSyncInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.SyncTools(ctx)
			case <-m.stopSync:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop shuts down the manager: stops tool sync and disconnects all clients.
func (m *Manager) Stop() {
	close(m.stopSync)

	m.mu.Lock()
	clients := m.clients
	m.clients = make(map[int]*ClientState)
	m.mu.Unlock()

	for _, state := range clients {
		if state.CancelFunc != nil {
			state.CancelFunc()
		}
		if state.Conn != nil {
			if err := state.Conn.Close(); err != nil {
				log.Printf("[mcp] close client during stop failed (id=%d): %v", state.Config.ID, err)
			}
		}
	}
	log.Printf("[mcp] manager stopped, %d clients disconnected", len(clients))
}
