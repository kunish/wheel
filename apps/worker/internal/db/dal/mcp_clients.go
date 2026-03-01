package dal

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/kunish/wheel/apps/worker/internal/mcp"
)

func ListMCPClients(ctx context.Context, db *bun.DB) ([]mcp.MCPClient, error) {
	var clients []mcp.MCPClient
	err := db.NewSelect().Model(&clients).OrderExpr("id ASC").Scan(ctx)
	if err != nil {
		return nil, err
	}
	if clients == nil {
		clients = []mcp.MCPClient{}
	}
	return clients, nil
}

func GetMCPClient(ctx context.Context, db *bun.DB, id int) (*mcp.MCPClient, error) {
	client := new(mcp.MCPClient)
	err := db.NewSelect().Model(client).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func CreateMCPClient(ctx context.Context, db *bun.DB, client *mcp.MCPClient) error {
	_, err := db.NewInsert().Model(client).Exec(ctx)
	return err
}

func UpdateMCPClient(ctx context.Context, db *bun.DB, id int, data map[string]any) error {
	allowed := map[string]bool{
		"name": true, "connection_type": true, "connection_string": true,
		"stdio_config": true, "auth_type": true, "headers": true,
		"oauth_config": true, "tools_to_execute": true,
		"tools_to_auto_exec": true, "enabled": true,
	}
	q := db.NewUpdate().Table("mcp_clients")
	count := 0
	for col, val := range data {
		if allowed[col] {
			q = q.Set(col+" = ?", val)
			count++
		}
	}
	if count == 0 {
		return nil
	}
	_, err := q.Where("id = ?", id).Exec(ctx)
	return err
}

func DeleteMCPClient(ctx context.Context, db *bun.DB, id int) error {
	_, err := db.NewDelete().Model((*mcp.MCPClient)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}
