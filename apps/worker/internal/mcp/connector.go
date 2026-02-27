package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
)

// Connect establishes a connection to an MCP server based on the client config.
// Returns the mcp-go client and a cancel function (for SSE/STDIO cleanup).
// oauthBearerToken is optional; if non-empty it will be added as Authorization header.
func Connect(ctx context.Context, cfg *MCPClient, oauthBearerToken string) (client.MCPClient, func(), error) {
	switch cfg.ConnectionType {
	case ConnectionTypeHTTP:
		return connectHTTP(cfg, oauthBearerToken)
	case ConnectionTypeSSE:
		return connectSSE(ctx, cfg, oauthBearerToken)
	case ConnectionTypeSTDIO:
		return connectSTDIO(cfg)
	default:
		return nil, nil, fmt.Errorf("unsupported connection type: %s", cfg.ConnectionType)
	}
}

// connectHTTP creates a Streamable HTTP MCP client.
func connectHTTP(cfg *MCPClient, oauthBearerToken string) (client.MCPClient, func(), error) {
	if cfg.ConnectionString == "" {
		return nil, nil, fmt.Errorf("connection string required for HTTP connection")
	}
	opts := buildHTTPHeaders(cfg, oauthBearerToken)
	c, err := client.NewStreamableHttpClient(cfg.ConnectionString, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("http connect failed: %w", err)
	}
	return c, func() {}, nil
}

// connectSSE creates an SSE-based MCP client.
func connectSSE(ctx context.Context, cfg *MCPClient, oauthBearerToken string) (client.MCPClient, func(), error) {
	if cfg.ConnectionString == "" {
		return nil, nil, fmt.Errorf("connection string required for SSE connection")
	}
	opts := buildSSEHeaders(cfg, oauthBearerToken)
	c, err := client.NewSSEMCPClient(cfg.ConnectionString, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("sse connect failed: %w", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	// Start SSE connection in background
	go func() {
		<-ctx.Done()
	}()
	return c, cancel, nil
}

// connectSTDIO creates a STDIO-based MCP client by launching a subprocess.
func connectSTDIO(cfg *MCPClient) (client.MCPClient, func(), error) {
	stdio := MCPStdioConfig(cfg.StdioConfig)
	if stdio.Command == "" {
		return nil, nil, fmt.Errorf("stdio command required for STDIO connection")
	}
	c, err := client.NewStdioMCPClient(stdio.Command, stdio.Envs, stdio.Args...)
	if err != nil {
		return nil, nil, fmt.Errorf("stdio connect failed: %w", err)
	}
	return c, func() {}, nil
}

// buildHeaderMap converts MCPClient headers to a map[string]string.
// It also injects the OAuth Bearer token if provided.
func buildHeaderMap(cfg *MCPClient, oauthBearerToken string) map[string]string {
	headers := make(map[string]string)

	// Add custom headers if auth type is "headers"
	if cfg.AuthType == AuthTypeHeaders && len(cfg.Headers) > 0 {
		for _, h := range cfg.Headers {
			headers[h.Key] = h.Value
		}
	}

	// Add OAuth Bearer token if provided
	if cfg.AuthType == AuthTypeOAuth && oauthBearerToken != "" {
		headers["Authorization"] = oauthBearerToken
	}

	if len(headers) == 0 {
		return nil
	}
	return headers
}

// buildHTTPHeaders returns StreamableHTTP options with auth headers.
func buildHTTPHeaders(cfg *MCPClient, oauthBearerToken string) []transport.StreamableHTTPCOption {
	headers := buildHeaderMap(cfg, oauthBearerToken)
	if headers == nil {
		return nil
	}
	return []transport.StreamableHTTPCOption{transport.WithHTTPHeaders(headers)}
}

// buildSSEHeaders returns SSE client options with auth headers.
func buildSSEHeaders(cfg *MCPClient, oauthBearerToken string) []transport.ClientOption {
	headers := buildHeaderMap(cfg, oauthBearerToken)
	if headers == nil {
		return nil
	}
	return []transport.ClientOption{transport.WithHeaders(headers)}
}
