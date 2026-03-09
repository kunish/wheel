package mcp

import (
	"context"
	"log"
	"net/http"
	"sync"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// Server wraps a mcp-go MCPServer that aggregates tools from all connected MCP clients.
// It exposes Wheel as an MCP Server so external MCP clients can discover and call tools.
type Server struct {
	mu      sync.RWMutex
	inner   *mcpserver.MCPServer
	sse     *mcpserver.SSEServer
	manager *Manager
}

// NewServer creates a new MCP Server backed by the given Manager.
func NewServer(mgr *Manager) *Server {
	inner := mcpserver.NewMCPServer(
		"wheel-mcp-gateway",
		"1.0.0",
		mcpserver.WithToolCapabilities(true),
	)

	s := &Server{
		inner:   inner,
		manager: mgr,
	}

	s.sse = mcpserver.NewSSEServer(inner,
		mcpserver.WithStaticBasePath("/mcp"),
	)
	return s
}

// SyncTools re-registers all aggregated tools from the Manager into the mcp-go server.
func (s *Server) SyncTools() {
	allTools := s.manager.GetAllTools()

	// Build ServerTool list with handlers
	serverTools := make([]mcpserver.ServerTool, 0, len(allTools))
	for sanitizedName, agg := range allTools {
		tool := agg.Tool
		tool.Name = sanitizedName // Use sanitized name for external clients
		clientID := agg.ClientID
		originalName := agg.OriginalName

		serverTools = append(serverTools, mcpserver.ServerTool{
			Tool: tool,
			Handler: func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
				args := req.GetArguments()
				return s.manager.ExecuteTool(ctx, clientID, originalName, args)
			},
		})
	}

	// Atomic replace all tools
	s.inner.SetTools(serverTools...)
	log.Printf("[mcp-server] synced %d tools", len(serverTools))
}

// ServeHTTP returns the SSE server's http.Handler for use with Gin.
func (s *Server) ServeHTTP() http.Handler {
	return s.sse
}
