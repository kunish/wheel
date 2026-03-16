package dal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/uptrace/bun"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

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

// LogsAggregateStats holds server-side aggregated stats across all matching logs.
type LogsAggregateStats struct {
	TotalRequests  int     `json:"totalRequests"`
	SuccessCount   int     `json:"successCount"`
	AverageLatency float64 `json:"averageLatency"`
	TotalTokens    int64   `json:"totalTokens"`
	TotalCost      float64 `json:"totalCost"`
	TokenSpeed     float64 `json:"tokenSpeed"`
}

func ListLogs(ctx context.Context, db *bun.DB, opts ListLogsOpts) ([]types.RelayLog, int, *LogsAggregateStats, error) {
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

	// Aggregate stats across all matching logs (single query)
	var aggResult struct {
		Total        int     `bun:"total"`
		SuccessCount int     `bun:"success_count"`
		AvgLatency   float64 `bun:"avg_latency"`
		TotalTokens  int64   `bun:"total_tokens"`
		TotalCost    float64 `bun:"total_cost"`
		AvgSpeed     float64 `bun:"avg_speed"`
	}
	aggQ := db.NewSelect().TableExpr("relay_logs").
		ColumnExpr("COUNT(*) AS total").
		ColumnExpr("SUM(CASE WHEN error = '' THEN 1 ELSE 0 END) AS success_count").
		ColumnExpr("AVG(use_time) AS avg_latency").
		ColumnExpr("SUM(input_tokens + output_tokens) AS total_tokens").
		ColumnExpr("SUM(cost) AS total_cost").
		ColumnExpr("AVG(CASE WHEN use_time > 0 AND output_tokens > 0 THEN output_tokens / (use_time / 1000.0) ELSE NULL END) AS avg_speed")
	aggQ = applyFilters(aggQ)
	err := aggQ.Scan(ctx, &aggResult)
	if err != nil {
		return nil, 0, nil, err
	}

	total := aggResult.Total
	stats := &LogsAggregateStats{
		TotalRequests:  total,
		SuccessCount:   aggResult.SuccessCount,
		AverageLatency: aggResult.AvgLatency,
		TotalTokens:    aggResult.TotalTokens,
		TotalCost:      aggResult.TotalCost,
		TokenSpeed:     aggResult.AvgSpeed,
	}

	// Fetch page — select only summary fields
	var logs []types.RelayLog
	dataQ := db.NewSelect().TableExpr("relay_logs").
		Column("id", "time", "request_model_name", "actual_model_name", "channel_id",
			"channel_name", "input_tokens", "output_tokens", "cache_read_tokens", "cache_creation_tokens",
			"ftut", "use_time", "cost", "error", "total_attempts", "request_content").
		OrderExpr("time DESC").
		Limit(pageSize).Offset(offset)
	dataQ = applyFilters(dataQ)
	err = dataQ.Scan(ctx, &logs)
	if err != nil {
		return nil, 0, nil, err
	}
	if logs == nil {
		logs = []types.RelayLog{}
	}
	for i := range logs {
		logs[i].LastMessagePreview = extractLastMessagePreview(logs[i].RequestContent)
		logs[i].RequestContent = ""
	}
	return logs, total, stats, nil
}

type previewMessage struct {
	Content   any   `json:"content"`
	ToolCalls []any `json:"tool_calls"`
}

func extractLastMessagePreview(requestContent string) string {
	requestContent = strings.TrimSpace(requestContent)
	if requestContent == "" {
		return ""
	}

	var body struct {
		Messages []previewMessage `json:"messages"`
	}
	if err := json.Unmarshal([]byte(requestContent), &body); err == nil && len(body.Messages) > 0 {
		return previewFromMessage(body.Messages[len(body.Messages)-1])
	}

	var direct []previewMessage
	if err := json.Unmarshal([]byte(requestContent), &direct); err == nil && len(direct) > 0 {
		return previewFromMessage(direct[len(direct)-1])
	}

	return ""
}

func previewFromMessage(msg previewMessage) string {
	text, hasImage := extractMessageContentText(msg.Content)
	text = normalizePreviewWhitespace(text)

	if hasImage {
		if text == "" {
			text = "[image]"
		} else {
			text += " [image]"
		}
	}

	if text == "" && len(msg.ToolCalls) > 0 {
		if len(msg.ToolCalls) == 1 {
			return "[1 tool call]"
		}
		return fmt.Sprintf("[%d tool calls]", len(msg.ToolCalls))
	}

	return truncatePreview(text, 140)
}

func extractMessageContentText(content any) (string, bool) {
	switch v := content.(type) {
	case string:
		return v, false
	case []any:
		parts := make([]string, 0, len(v))
		hasImage := false
		for _, item := range v {
			part, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if typ, _ := part["type"].(string); typ == "image_url" {
				hasImage = true
			}
			if typ, _ := part["type"].(string); typ == "text" {
				if text, _ := part["text"].(string); text != "" {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n"), hasImage
	default:
		return "", false
	}
}

func normalizePreviewWhitespace(s string) string {
	if s == "" {
		return ""
	}
	return strings.Join(strings.Fields(s), " ")
}

func truncatePreview(s string, maxRunes int) string {
	if maxRunes <= 0 || s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxRunes]) + "..."
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
