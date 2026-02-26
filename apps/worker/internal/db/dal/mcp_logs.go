package dal

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

// CreateMCPLog inserts a new MCP tool call log entry.
func CreateMCPLog(ctx context.Context, db *bun.DB, log *types.MCPLog) error {
	_, err := db.NewInsert().Model(log).Exec(ctx)
	return err
}

// ListMCPLogs returns paginated MCP logs with optional filters.
func ListMCPLogs(ctx context.Context, db *bun.DB, opts types.MCPLogListOpts) ([]types.MCPLog, int, error) {
	page := opts.Page
	if page < 1 {
		page = 1
	}
	pageSize := opts.PageSize
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	offset := (page - 1) * pageSize

	applyFilters := func(q *bun.SelectQuery) *bun.SelectQuery {
		if opts.ClientID != 0 {
			q = q.Where("client_id = ?", opts.ClientID)
		}
		if opts.ToolName != "" {
			q = q.Where("tool_name LIKE ?", "%"+opts.ToolName+"%")
		}
		if opts.Status != "" {
			q = q.Where("status = ?", opts.Status)
		}
		if opts.StartTime > 0 {
			q = q.Where("time >= ?", opts.StartTime)
		}
		if opts.EndTime > 0 {
			q = q.Where("time <= ?", opts.EndTime)
		}
		return q
	}

	var total int
	countQ := db.NewSelect().TableExpr("mcp_logs").ColumnExpr("COUNT(*)")
	countQ = applyFilters(countQ)
	err := countQ.Scan(ctx, &total)
	if err != nil {
		return nil, 0, err
	}

	var logs []types.MCPLog
	dataQ := db.NewSelect().Model(&logs).OrderExpr("time DESC").Limit(pageSize).Offset(offset)
	dataQ = applyFilters(dataQ)
	err = dataQ.Scan(ctx)
	if err != nil {
		return nil, 0, err
	}
	if logs == nil {
		logs = []types.MCPLog{}
	}
	return logs, total, nil
}

// ClearMCPLogs deletes all MCP logs.
func ClearMCPLogs(ctx context.Context, db *bun.DB) error {
	_, err := db.NewDelete().Model((*types.MCPLog)(nil)).Where("1=1").Exec(ctx)
	return err
}
