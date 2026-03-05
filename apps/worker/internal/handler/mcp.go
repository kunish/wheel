package handler

import (
	"context"
	"log"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	mcpmcp "github.com/mark3labs/mcp-go/mcp"

	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	mcpgw "github.com/kunish/wheel/apps/worker/internal/mcp"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// ──── List MCP Clients ────

func (h *RelayHandler) ListMCPClients(c *gin.Context) {
	clients, err := dal.ListMCPClients(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	states := h.MCPManager.GetAllClientStates()
	resp := make([]mcpgw.MCPClientResponse, 0, len(clients))

	for _, cfg := range clients {
		r := buildClientResponse(&cfg, states)
		resp = append(resp, r)
	}

	successJSON(c, gin.H{
		"clients":   resp,
		"serverUrl": buildPublicMCPServerURL(c.Request),
	})
}

// buildClientResponse merges DB config with runtime state into an API response.
func buildClientResponse(cfg *mcpgw.MCPClient, states map[int]*mcpgw.ClientState) mcpgw.MCPClientResponse {
	r := mcpgw.MCPClientResponse{
		ID:               cfg.ID,
		Name:             cfg.Name,
		ConnectionType:   cfg.ConnectionType,
		ConnectionString: cfg.ConnectionString,
		AuthType:         cfg.AuthType,
		Headers:          []mcpgw.MCPHeaderEntry(cfg.Headers),
		ToolsToExecute:   []string(cfg.ToolsToExecute),
		ToolsToAutoExec:  []string(cfg.ToolsToAutoExec),
		Enabled:          cfg.Enabled,
		State:            mcpgw.StateDisconnected,
		Tools:            []mcpgw.ToolInfo{},
		CreatedAt:        cfg.CreatedAt,
		UpdatedAt:        cfg.UpdatedAt,
	}

	if cfg.ConnectionType == mcpgw.ConnectionTypeSTDIO {
		stdio := mcpgw.MCPStdioConfig(cfg.StdioConfig)
		r.StdioConfig = &stdio
	}

	if cfg.AuthType == mcpgw.AuthTypeOAuth {
		oauthCfg := mcpgw.MCPOAuthConfig(cfg.OAuthConfig)
		// Mask client secret in API responses
		if oauthCfg.ClientSecret != "" {
			oauthCfg.ClientSecret = "***"
		}
		if oauthCfg.AccessToken != "" {
			oauthCfg.AccessToken = "***"
		}
		r.OAuthConfig = &oauthCfg
	}

	if state, ok := states[cfg.ID]; ok {
		r.State = state.GetState()
		r.ErrorMsg = state.ErrorMsg
		tools := state.GetTools()
		toolInfos := make([]mcpgw.ToolInfo, 0, len(tools))
		for _, t := range tools {
			toolInfos = append(toolInfos, mcpgw.ToolInfo{
				Name:        t.Name,
				Description: t.Description,
			})
		}
		sort.Slice(toolInfos, func(i, j int) bool {
			return toolInfos[i].Name < toolInfos[j].Name
		})
		r.Tools = toolInfos
	}

	return r
}

// ──── Create MCP Client ────

func (h *RelayHandler) CreateMCPClient(c *gin.Context) {
	var req mcpgw.MCPClientCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate connection type
	switch req.ConnectionType {
	case mcpgw.ConnectionTypeHTTP, mcpgw.ConnectionTypeSSE, mcpgw.ConnectionTypeSTDIO:
	default:
		errorJSON(c, http.StatusBadRequest, "Invalid connection type")
		return
	}

	client := &mcpgw.MCPClient{
		Name:             req.Name,
		ConnectionType:   req.ConnectionType,
		ConnectionString: req.ConnectionString,
		AuthType:         req.AuthType,
		Headers:          mcpgw.HeaderListJSON(req.Headers),
		ToolsToExecute:   mcpgw.StringListJSON(req.ToolsToExecute),
		ToolsToAutoExec:  mcpgw.StringListJSON(req.ToolsToAutoExec),
		Enabled:          req.Enabled,
	}
	if req.StdioConfig != nil {
		client.StdioConfig = mcpgw.StdioConfigJSON(*req.StdioConfig)
	}
	if req.OAuthConfig != nil {
		client.OAuthConfig = mcpgw.OAuthConfigJSON(*req.OAuthConfig)
	}

	// Default: allow all tools for newly created clients.
	if client.ToolsToExecute == nil {
		client.ToolsToExecute = mcpgw.StringListJSON{"*"}
	}
	if client.ToolsToAutoExec == nil {
		client.ToolsToAutoExec = mcpgw.StringListJSON{}
	}

	// Persist to DB first
	if err := dal.CreateMCPClient(c.Request.Context(), h.DB, client); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Connect if enabled
	if client.Enabled {
		if err := h.MCPManager.AddClient(c.Request.Context(), client); err != nil {
			log.Printf("[mcp] connect after create failed (id=%d): %v", client.ID, err)
			// Don't rollback DB — client is saved but disconnected
		}
	}
	h.syncMCPServerTools()

	successJSON(c, client)
}

// ──── Update MCP Client ────

func (h *RelayHandler) UpdateMCPClient(c *gin.Context) {
	var req mcpgw.MCPClientUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.ID == 0 {
		errorJSON(c, http.StatusBadRequest, "id is required")
		return
	}

	data := make(map[string]any)
	if req.Name != nil {
		data["name"] = *req.Name
	}
	if req.ConnectionType != nil {
		data["connection_type"] = *req.ConnectionType
	}
	if req.ConnectionString != nil {
		data["connection_string"] = *req.ConnectionString
	}
	if req.StdioConfig != nil {
		data["stdio_config"] = mcpgw.StdioConfigJSON(*req.StdioConfig)
	}
	if req.AuthType != nil {
		data["auth_type"] = *req.AuthType
	}
	if req.Headers != nil {
		data["headers"] = mcpgw.HeaderListJSON(req.Headers)
	}
	if req.OAuthConfig != nil {
		// If secret fields contain the mask placeholder "***", preserve the original values from DB
		if req.OAuthConfig.ClientSecret == "***" || req.OAuthConfig.AccessToken == "***" {
			existing, err := dal.GetMCPClient(c.Request.Context(), h.DB, req.ID)
			if err == nil {
				origCfg := mcpgw.MCPOAuthConfig(existing.OAuthConfig)
				if req.OAuthConfig.ClientSecret == "***" {
					req.OAuthConfig.ClientSecret = origCfg.ClientSecret
				}
				if req.OAuthConfig.AccessToken == "***" {
					req.OAuthConfig.AccessToken = origCfg.AccessToken
				}
			}
		}
		data["oauth_config"] = mcpgw.OAuthConfigJSON(*req.OAuthConfig)
	}
	if req.ToolsToExecute != nil {
		data["tools_to_execute"] = mcpgw.StringListJSON(req.ToolsToExecute)
	}
	if req.ToolsToAutoExec != nil {
		data["tools_to_auto_exec"] = mcpgw.StringListJSON(req.ToolsToAutoExec)
	}
	if req.Enabled != nil {
		data["enabled"] = *req.Enabled
	}

	if err := dal.UpdateMCPClient(c.Request.Context(), h.DB, req.ID, data); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Reconnect with updated config
	updated, err := dal.GetMCPClient(c.Request.Context(), h.DB, req.ID)
	if err != nil {
		log.Printf("[mcp] fetch updated client failed (id=%d): %v", req.ID, err)
	} else {
		if err := h.MCPManager.UpdateClient(c.Request.Context(), updated); err != nil {
			log.Printf("[mcp] reconnect after update failed (id=%d): %v", req.ID, err)
		}
		h.syncMCPServerTools()
	}

	successNoData(c)
}

// ──── Delete MCP Client ────

func (h *RelayHandler) DeleteMCPClient(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid client ID")
		return
	}

	// Delete from DB first
	if err := dal.DeleteMCPClient(c.Request.Context(), h.DB, id); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Disconnect from runtime
	h.MCPManager.RemoveClient(id)
	h.syncMCPServerTools()
	successNoData(c)
}

// ──── Reconnect MCP Client ────

func (h *RelayHandler) ReconnectMCPClient(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid client ID")
		return
	}

	// Fetch fresh config from DB
	cfg, err := dal.GetMCPClient(c.Request.Context(), h.DB, id)
	if err != nil {
		errorJSON(c, http.StatusNotFound, "Client not found")
		return
	}

	// Remove old connection and reconnect
	h.MCPManager.RemoveClient(id)
	if err := h.MCPManager.AddClient(c.Request.Context(), cfg); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.syncMCPServerTools()

	successNoData(c)
}

// ──── Execute Tool (REST API) ────

func (h *RelayHandler) ExecuteMCPTool(c *gin.Context) {
	var req mcpgw.ToolExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Resolve client name for logging
	clientName := ""
	if cfg, err := dal.GetMCPClient(c.Request.Context(), h.DB, req.ClientID); err == nil && cfg != nil {
		clientName = cfg.Name
	}

	start := time.Now()
	result, err := h.MCPManager.ExecuteTool(
		c.Request.Context(), req.ClientID, req.ToolName, req.Arguments,
	)
	duration := int(time.Since(start).Milliseconds())

	// Log the MCP tool call
	mcpLog := &types.MCPLog{
		Time:       time.Now().Unix(),
		ClientID:   req.ClientID,
		ClientName: clientName,
		ToolName:   req.ToolName,
		Duration:   duration,
	}
	if err != nil {
		mcpLog.Status = "error"
		mcpLog.Error = err.Error()
		h.writeMCPLogAsync(mcpLog)
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	if result.IsError {
		mcpLog.Status = "error"
	} else {
		mcpLog.Status = "ok"
	}
	h.writeMCPLogAsync(mcpLog)

	resp := mcpgw.ToolExecuteResponse{IsError: result.IsError}
	for _, content := range result.Content {
		if tc, ok := mcpmcp.AsTextContent(content); ok {
			resp.Content = append(resp.Content, mcpgw.ToolContent{
				Type: "text",
				Text: tc.Text,
			})
		}
	}

	successJSON(c, resp)
}

func (h *RelayHandler) syncMCPServerTools() {
	if h.MCPServer == nil {
		return
	}
	h.MCPServer.SyncTools()
}

func (h *RelayHandler) writeMCPLogAsync(entry *types.MCPLog) {
	if entry == nil || h.DB == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := dal.CreateMCPLog(ctx, h.DB, entry); err != nil {
			log.Printf("[mcp] write log failed: %v", err)
		}
	}()
}

// ──── Discover OAuth Metadata ────

// DiscoverOAuthMetadataRequest is the request body for OAuth metadata discovery.
type DiscoverOAuthMetadataRequest struct {
	ServerURL string `json:"serverUrl" binding:"required"`
}

func (h *RelayHandler) DiscoverOAuthMetadata(c *gin.Context) {
	var req DiscoverOAuthMetadataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "serverUrl is required")
		return
	}

	result, err := mcpgw.DiscoverOAuthMetadata(req.ServerURL)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	successJSON(c, result)
}
