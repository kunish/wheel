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
			"channel_name", "input_tokens", "output_tokens", "ftut", "use_time", "cost", "error", "total_attempts", "request_content").
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
	for i := range logs {
		logs[i].LastMessagePreview = extractLastMessagePreview(logs[i].RequestContent)
		logs[i].RequestContent = ""
	}
	return logs, total, nil
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
