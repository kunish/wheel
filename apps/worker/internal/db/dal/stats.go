package dal

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"time"

	"github.com/uptrace/bun"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

// currentUnix returns the current Unix epoch in seconds.
func currentUnix() int64 {
	return time.Now().Unix()
}

// tzOffsetSeconds converts a timezone offset string (e.g. "+08:00") to seconds.
func tzOffsetSeconds(tz string) int {
	if tz == "" {
		return 0
	}
	re := regexp.MustCompile(`^([+-])(\d{1,2}):(\d{2})$`)
	m := re.FindStringSubmatch(tz)
	if m == nil {
		return 0
	}
	sign := 1
	if m[1] == "-" {
		sign = -1
	}
	hours, _ := strconv.Atoi(m[2])
	mins, _ := strconv.Atoi(m[3])
	if hours > 14 || (hours == 14 && mins > 0) {
		return 0
	}
	return sign * (hours*3600 + mins*60)
}

// parseTzMinutes parses "+08:00" to offset in minutes.
func parseTzMinutes(tz string) int {
	if tz == "" {
		return 0
	}
	re := regexp.MustCompile(`^([+-])(\d{1,2}):(\d{2})$`)
	m := re.FindStringSubmatch(tz)
	if m == nil {
		return 0
	}
	sign := 1
	if m[1] == "-" {
		sign = -1
	}
	hours, _ := strconv.Atoi(m[2])
	mins, _ := strconv.Atoi(m[3])
	return sign * (hours*60 + mins)
}

// localDateStr returns YYYYMMDD for the given time in the specified tz offset.
func localDateStr(t time.Time, tzOffsetMinutes int) string {
	loc := time.FixedZone("custom", tzOffsetMinutes*60)
	local := t.In(loc)
	return fmt.Sprintf("%04d%02d%02d", local.Year(), local.Month(), local.Day())
}

// metricsResult holds aggregation results scanned via Bun ORM.
type metricsResult struct {
	InputTokens  float64 `bun:"input_tokens"`
	OutputTokens float64 `bun:"output_tokens"`
	Cost         float64 `bun:"cost"`
	WaitTime     float64 `bun:"wait_time"`
	SuccessCount float64 `bun:"success_count"`
	FailedCount  float64 `bun:"failed_count"`
}

func (r metricsResult) toDailyStats(date string) types.DailyStatsItem {
	return types.DailyStatsItem{
		Date:           date,
		InputToken:     int(r.InputTokens),
		OutputToken:    int(r.OutputTokens),
		InputCost:      r.Cost * 0.6,
		OutputCost:     r.Cost * 0.4,
		WaitTime:       int(r.WaitTime),
		RequestSuccess: int(r.SuccessCount),
		RequestFailed:  int(r.FailedCount),
	}
}

func (r metricsResult) toHourlyStats(hour int, date string) types.HourlyStatsItem {
	return types.HourlyStatsItem{
		Hour:           hour,
		Date:           date,
		InputToken:     int(r.InputTokens),
		OutputToken:    int(r.OutputTokens),
		InputCost:      r.Cost * 0.6,
		OutputCost:     r.Cost * 0.4,
		WaitTime:       int(r.WaitTime),
		RequestSuccess: int(r.SuccessCount),
		RequestFailed:  int(r.FailedCount),
	}
}

func metricsColumns(q *bun.SelectQuery) *bun.SelectQuery {
	return q.
		ColumnExpr("CAST(COALESCE(SUM(input_tokens), 0) AS DOUBLE) AS input_tokens").
		ColumnExpr("CAST(COALESCE(SUM(output_tokens), 0) AS DOUBLE) AS output_tokens").
		ColumnExpr("CAST(COALESCE(SUM(cost), 0) AS DOUBLE) AS cost").
		ColumnExpr("CAST(COALESCE(SUM(use_time), 0) AS DOUBLE) AS wait_time").
		ColumnExpr("CAST(COALESCE(SUM(CASE WHEN error = '' THEN 1 ELSE 0 END), 0) AS DOUBLE) AS success_count").
		ColumnExpr("CAST(COALESCE(SUM(CASE WHEN error != '' THEN 1 ELSE 0 END), 0) AS DOUBLE) AS failed_count")
}

func GetTotalStats(ctx context.Context, db *bun.DB) (*types.DailyStatsItem, error) {
	var m metricsResult
	err := metricsColumns(db.NewSelect().TableExpr("relay_logs")).Scan(ctx, &m)
	if err != nil {
		return nil, err
	}
	result := m.toDailyStats("")
	return &result, nil
}

func GetTodayStats(ctx context.Context, db *bun.DB, tz string) (*types.DailyStatsItem, error) {
	offset := tzOffsetSeconds(tz)
	todayStr := localDateStr(time.Now(), parseTzMinutes(tz))

	q := db.NewSelect().TableExpr("relay_logs").
		Where(fmt.Sprintf("DATE_FORMAT(DATE_ADD(FROM_UNIXTIME(time), INTERVAL %d SECOND), '%%Y%%m%%d') = ?", offset), todayStr)
	var m metricsResult
	err := metricsColumns(q).Scan(ctx, &m)
	if err != nil {
		return nil, err
	}
	result := m.toDailyStats(todayStr)
	return &result, nil
}

type dailyRow struct {
	metricsResult
	Date string `bun:"date"`
}

func GetDailyStats(ctx context.Context, db *bun.DB, tz string) ([]types.DailyStatsItem, error) {
	offset := tzOffsetSeconds(tz)

	dateExpr := fmt.Sprintf("DATE_FORMAT(DATE_ADD(FROM_UNIXTIME(time), INTERVAL %d SECOND), '%%Y%%m%%d')", offset)
	q := db.NewSelect().TableExpr("relay_logs").
		ColumnExpr(fmt.Sprintf("%s AS date", dateExpr)).
		Where("time >= UNIX_TIMESTAMP(DATE_SUB(NOW(), INTERVAL 365 DAY))").
		GroupExpr("date").
		OrderExpr("date")
	q = metricsColumns(q)

	var rows []dailyRow
	err := q.Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}

	results := make([]types.DailyStatsItem, 0, len(rows))
	for _, r := range rows {
		results = append(results, r.toDailyStats(r.Date))
	}
	return results, nil
}

type hourlyRow struct {
	metricsResult
	Hour int    `bun:"hour"`
	Date string `bun:"date"`
}

func GetHourlyStats(ctx context.Context, db *bun.DB, startDate, endDate, tz string) ([]types.HourlyStatsItem, error) {
	offset := tzOffsetSeconds(tz)
	if startDate == "" {
		startDate = localDateStr(time.Now(), parseTzMinutes(tz))
	}
	if endDate == "" {
		endDate = startDate
	}

	dateExpr := fmt.Sprintf("DATE_FORMAT(DATE_ADD(FROM_UNIXTIME(time), INTERVAL %d SECOND), '%%Y%%m%%d')", offset)

	q := db.NewSelect().TableExpr("relay_logs").
		ColumnExpr(fmt.Sprintf("HOUR(DATE_ADD(FROM_UNIXTIME(time), INTERVAL %d SECOND)) AS hour", offset)).
		ColumnExpr(fmt.Sprintf("%s AS date", dateExpr)).
		Where(fmt.Sprintf("%s >= ?", dateExpr), startDate).
		Where(fmt.Sprintf("%s <= ?", dateExpr), endDate).
		GroupExpr("date, hour").
		OrderExpr("date, hour")
	q = metricsColumns(q)

	var rows []hourlyRow
	err := q.Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}

	results := make([]types.HourlyStatsItem, 0, len(rows))
	for _, r := range rows {
		results = append(results, r.toHourlyStats(r.Hour, r.Date))
	}
	return results, nil
}

func GetGlobalStats(ctx context.Context, db *bun.DB) (*types.GlobalStatsResponse, error) {
	var logStats struct {
		TotalRequests     int     `bun:"total_requests"`
		TotalInputTokens  float64 `bun:"total_input_tokens"`
		TotalOutputTokens float64 `bun:"total_output_tokens"`
		TotalCost         float64 `bun:"total_cost"`
	}
	err := db.NewSelect().TableExpr("relay_logs").
		ColumnExpr("COUNT(*) AS total_requests").
		ColumnExpr("CAST(COALESCE(SUM(input_tokens), 0) AS DOUBLE) AS total_input_tokens").
		ColumnExpr("CAST(COALESCE(SUM(output_tokens), 0) AS DOUBLE) AS total_output_tokens").
		ColumnExpr("CAST(COALESCE(SUM(cost), 0) AS DOUBLE) AS total_cost").
		Scan(ctx, &logStats)
	if err != nil {
		return nil, err
	}

	var activeChannels int
	_ = db.NewSelect().TableExpr("channels").
		ColumnExpr("COUNT(*)").
		Where("enabled = 1").
		Scan(ctx, &activeChannels)

	var activeGroups int
	_ = db.NewSelect().TableExpr("groups").
		ColumnExpr("COUNT(*)").
		Scan(ctx, &activeGroups)

	return &types.GlobalStatsResponse{
		TotalRequests:     logStats.TotalRequests,
		TotalInputTokens:  int(logStats.TotalInputTokens),
		TotalOutputTokens: int(logStats.TotalOutputTokens),
		TotalCost:         logStats.TotalCost,
		ActiveChannels:    activeChannels,
		ActiveGroups:      activeGroups,
	}, nil
}

type channelStatsRow struct {
	ChannelID     int     `bun:"channel_id"`
	ChannelName   string  `bun:"channel_name"`
	TotalRequests int     `bun:"total_requests"`
	TotalCost     float64 `bun:"total_cost"`
	AvgLatency    float64 `bun:"avg_latency"`
}

func GetChannelStats(ctx context.Context, db *bun.DB) ([]types.ChannelStatsItem, error) {
	var rows []channelStatsRow
	err := db.NewSelect().TableExpr("relay_logs").
		Column("channel_id", "channel_name").
		ColumnExpr("COUNT(*) AS total_requests").
		ColumnExpr("CAST(COALESCE(SUM(cost), 0) AS DOUBLE) AS total_cost").
		ColumnExpr("CAST(COALESCE(AVG(use_time), 0) AS DOUBLE) AS avg_latency").
		Where("channel_id > 0").
		GroupExpr("channel_id, channel_name").
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}

	results := make([]types.ChannelStatsItem, 0, len(rows))
	for _, r := range rows {
		results = append(results, types.ChannelStatsItem{
			ChannelID:     r.ChannelID,
			ChannelName:   r.ChannelName,
			TotalRequests: r.TotalRequests,
			TotalCost:     r.TotalCost,
			AvgLatency:    r.AvgLatency,
		})
	}
	return results, nil
}

type modelStatsRow struct {
	Model        string  `bun:"request_model_name"`
	RequestCount int     `bun:"cnt"`
	InputTokens  float64 `bun:"input_tokens"`
	OutputTokens float64 `bun:"output_tokens"`
	TotalCost    float64 `bun:"total_cost"`
	AvgLatency   float64 `bun:"avg_latency"`
	AvgFtut      float64 `bun:"avg_ftut"`
}

func GetModelStats(ctx context.Context, db *bun.DB) ([]types.ModelStatsItem, error) {
	var rows []modelStatsRow
	err := db.NewSelect().TableExpr("relay_logs").
		Column("request_model_name").
		ColumnExpr("COUNT(*) AS cnt").
		ColumnExpr("CAST(COALESCE(SUM(input_tokens), 0) AS DOUBLE) AS input_tokens").
		ColumnExpr("CAST(COALESCE(SUM(output_tokens), 0) AS DOUBLE) AS output_tokens").
		ColumnExpr("CAST(COALESCE(SUM(cost), 0) AS DOUBLE) AS total_cost").
		ColumnExpr("CAST(COALESCE(AVG(use_time), 0) AS DOUBLE) AS avg_latency").
		ColumnExpr("CAST(COALESCE(AVG(ftut), 0) AS DOUBLE) AS avg_ftut").
		GroupExpr("request_model_name").
		OrderExpr("cnt DESC").
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}

	results := make([]types.ModelStatsItem, 0, len(rows))
	for _, r := range rows {
		results = append(results, types.ModelStatsItem{
			Model:             r.Model,
			RequestCount:      r.RequestCount,
			InputTokens:       int(r.InputTokens),
			OutputTokens:      int(r.OutputTokens),
			TotalCost:         r.TotalCost,
			AvgLatency:        int(math.Round(r.AvgLatency)),
			AvgFirstTokenTime: int(math.Round(r.AvgFtut)),
		})
	}
	return results, nil
}

type apiKeyStatsRow struct {
	ID        int     `bun:"id"`
	Name      string  `bun:"name"`
	Enabled   bool    `bun:"enabled"`
	TotalCost float64 `bun:"total_cost"`
	MaxCost   float64 `bun:"max_cost"`
	ExpireAt  int64   `bun:"expire_at"`
}

func GetApiKeyStats(ctx context.Context, db *bun.DB) ([]map[string]any, error) {
	var rows []apiKeyStatsRow
	err := db.NewSelect().TableExpr("api_keys").
		Column("id", "name", "enabled", "total_cost", "max_cost", "expire_at").
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}

	results := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		results = append(results, map[string]any{
			"id":        r.ID,
			"name":      r.Name,
			"enabled":   r.Enabled,
			"totalCost": r.TotalCost,
			"maxCost":   r.MaxCost,
			"expireAt":  r.ExpireAt,
		})
	}
	return results, nil
}
