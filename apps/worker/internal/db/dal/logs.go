package dal

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

func CreateLog(ctx context.Context, db *bun.DB, log types.RelayLog) (*types.RelayLog, error) {
	_, err := db.NewInsert().Model(&log).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return &log, nil
}

// CreateLogsBatch inserts multiple logs in a single statement within the given transaction.
func CreateLogsBatch(ctx context.Context, tx bun.Tx, logs []types.RelayLog) error {
	if len(logs) == 0 {
		return nil
	}
	_, err := tx.NewInsert().Model(&logs).Exec(ctx)
	return err
}

func GetLog(ctx context.Context, db *bun.DB, id int) (*types.RelayLog, error) {
	log := new(types.RelayLog)
	err := db.NewSelect().Model(log).Where("id = ?", id).Scan(ctx)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	return log, nil
}

type ListLogsOpts struct {
	Page      int
	PageSize  int
	Model     string
	ChannelID int
	HasError  *bool
	StartTime int64
	EndTime   int64
	Keyword   string
}

func ListLogs(ctx context.Context, db *bun.DB, opts ListLogsOpts) ([]types.RelayLog, int, error) {
	page := opts.Page
	if page < 1 {
		page = 1
	}
	pageSize := opts.PageSize
	if pageSize < 1 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	// Build WHERE conditions using a helper to apply to both count and data queries
	applyFilters := func(q *bun.SelectQuery) *bun.SelectQuery {
		if opts.Model != "" {
			q = q.Where("request_model_name LIKE ?", "%"+opts.Model+"%")
		}
		if opts.ChannelID != 0 {
			q = q.Where("channel_id = ?", opts.ChannelID)
		}
		if opts.HasError != nil {
			if *opts.HasError {
				q = q.Where("error != ''")
			} else {
				q = q.Where("error = ''")
			}
		}
		if opts.StartTime > 0 {
			q = q.Where("time >= ?", opts.StartTime)
		}
		if opts.EndTime > 0 {
			q = q.Where("time <= ?", opts.EndTime)
		}
		if opts.Keyword != "" {
			pattern := "%" + opts.Keyword + "%"
			q = q.Where("(request_model_name LIKE ? OR channel_name LIKE ? OR error LIKE ? OR request_content LIKE ? OR response_content LIKE ?)",
				pattern, pattern, pattern, pattern, pattern)
		}
		return q
	}

	// Count total
	var total int
	countQ := db.NewSelect().TableExpr("relay_logs").ColumnExpr("COUNT(*)")
	countQ = applyFilters(countQ)
	err := countQ.Scan(ctx, &total)
	if err != nil {
		return nil, 0, err
	}

	// Fetch page — select only summary fields
	var logs []types.RelayLog
	dataQ := db.NewSelect().TableExpr("relay_logs").
		Column("id", "time", "request_model_name", "actual_model_name", "channel_id",
			"channel_name", "input_tokens", "output_tokens", "ftut", "use_time", "cost", "error", "total_attempts").
		OrderExpr("time DESC").
		Limit(pageSize).Offset(offset)
	dataQ = applyFilters(dataQ)
	err = dataQ.Scan(ctx, &logs)
	if err != nil {
		return nil, 0, err
	}
	if logs == nil {
		logs = []types.RelayLog{}
	}
	return logs, total, nil
}

func DeleteLog(ctx context.Context, db *bun.DB, id int) error {
	_, err := db.NewDelete().Model((*types.RelayLog)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func ClearLogs(ctx context.Context, db *bun.DB) error {
	_, err := db.NewDelete().Model((*types.RelayLog)(nil)).Where("1=1").Exec(ctx)
	return err
}

func CleanupOldLogs(ctx context.Context, db *bun.DB, retentionDays int) (int, error) {
	cutoff := currentUnix() - int64(retentionDays)*86400
	res, err := db.NewDelete().Model((*types.RelayLog)(nil)).Where("time <= ?", cutoff).Exec(ctx)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
