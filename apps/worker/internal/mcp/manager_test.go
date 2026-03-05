package mcp

import (
	"context"
	"strings"
	"testing"

	mcpclient "github.com/mark3labs/mcp-go/client"
	mcplib "github.com/mark3labs/mcp-go/mcp"
)

type trackingClient struct {
	mcpclient.MCPClient
	callCount int
	closed    bool
}

func (c *trackingClient) CallTool(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	c.callCount++
	return &mcplib.CallToolResult{}, nil
}

func (c *trackingClient) Close() error {
	c.closed = true
	return nil
}

func TestIsToolAllowed_DenyByDefault(t *testing.T) {
	if isToolAllowed("ping", nil) {
		t.Fatal("expected nil filter to deny tools by default")
	}

	if isToolAllowed("ping", StringListJSON{}) {
		t.Fatal("expected empty filter to deny tools by default")
	}

	if !isToolAllowed("ping", StringListJSON{"*"}) {
		t.Fatal("expected wildcard filter to allow all tools")
	}

	if !isToolAllowed("ping", StringListJSON{"ping"}) {
		t.Fatal("expected explicit allow-list to permit matching tool")
	}
}

func TestExecuteTool_RejectsDisallowedTool(t *testing.T) {
	mgr := NewManager()
	conn := &trackingClient{}

	mgr.clients[1] = &ClientState{
		Config: &MCPClient{
			ID:             1,
			Name:           "calc",
			ToolsToExecute: StringListJSON{"allowed"},
		},
		Conn:  conn,
		State: StateConnected,
	}

	_, err := mgr.ExecuteTool(context.Background(), 1, "blocked", map[string]any{"x": 1})
	if err == nil {
		t.Fatal("expected disallowed tool to return an error")
	}
	if !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected not allowed error, got: %v", err)
	}
	if conn.callCount != 0 {
		t.Fatalf("expected upstream tool call to be blocked, got %d calls", conn.callCount)
	}
}

func TestRemoveClient_ClosesConnection(t *testing.T) {
	mgr := NewManager()
	conn := &trackingClient{}

	mgr.clients[2] = &ClientState{
		Config: &MCPClient{ID: 2, Name: "search"},
		Conn:   conn,
		State:  StateConnected,
	}

	mgr.RemoveClient(2)

	if !conn.closed {
		t.Fatal("expected RemoveClient to close connection")
	}
	if _, ok := mgr.GetClientState(2); ok {
		t.Fatal("expected client to be removed from manager")
	}
}
