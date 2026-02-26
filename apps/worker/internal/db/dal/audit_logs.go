package dal

import (
	"context"
	"time"

	"github.com/uptrace/bun"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

// CreateAuditLog inserts a new audit log entry.
func CreateAuditLog(ctx context.Context, db *bun.DB, user, action, target, detail string) error {
	log := &types.AuditLog{
		Time:   time.Now().Unix(),
		User:   user,
		Action: action,
		Target: target,
		Detail: detail,
	}
	_, err := db.NewInsert().Model(log).Exec(ctx)
	return err
}

// ListAuditLogs returns paginated audit logs with optional filters.
func ListAuditLogs(ctx context.Context, db *bun.DB, opts types.AuditLogListOpts) ([]types.AuditLog, int, error) {
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
		if opts.User != "" {
			q = q.Where("user = ?", opts.User)
		}
		if opts.Action != "" {
			q = q.Where("action = ?", opts.Action)
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
	countQ := db.NewSelect().TableExpr("audit_logs").ColumnExpr("COUNT(*)")
	countQ = applyFilters(countQ)
	err := countQ.Scan(ctx, &total)
	if err != nil {
		return nil, 0, err
	}

	var logs []types.AuditLog
	dataQ := db.NewSelect().Model(&logs).OrderExpr("time DESC").Limit(pageSize).Offset(offset)
	dataQ = applyFilters(dataQ)
	err = dataQ.Scan(ctx)
	if err != nil {
		return nil, 0, err
	}
	if logs == nil {
		logs = []types.AuditLog{}
	}
	return logs, total, nil
}

// ClearAuditLogs deletes all audit logs.
func ClearAuditLogs(ctx context.Context, db *bun.DB) error {
	_, err := db.NewDelete().Model((*types.AuditLog)(nil)).Where("1=1").Exec(ctx)
	return err
}
