package dal

import (
	"context"
	"database/sql"
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

// tzModifier converts a timezone offset string (e.g. "+08:00") to a SQLite modifier.
// The output is validated to only contain digits and +/- to prevent SQL injection.
func tzModifier(tz string) string {
	if tz == "" {
		return "+0 seconds"
	}
	re := regexp.MustCompile(`^([+-])(\d{1,2}):(\d{2})$`)
	m := re.FindStringSubmatch(tz)
	if m == nil {
		return "+0 seconds"
	}
	sign := 1
	if m[1] == "-" {
		sign = -1
	}
	hours, _ := strconv.Atoi(m[2])
	mins, _ := strconv.Atoi(m[3])
	// Clamp to valid UTC offset range (-12:00 to +14:00)
	if hours > 14 || (hours == 14 && mins > 0) {
		return "+0 seconds"
	}
	secs := sign * (hours*3600 + mins*60)
	if secs >= 0 {
		return fmt.Sprintf("+%d seconds", secs)
	}
	return fmt.Sprintf("%d seconds", secs)
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

// metricsRow holds raw aggregation results from SQL queries.
type metricsRow struct {
	InputTokens  sql.NullFloat64
	OutputTokens sql.NullFloat64
	Cost         sql.NullFloat64
	WaitTime     sql.NullFloat64
	SuccessCount sql.NullFloat64
	FailedCount  sql.NullFloat64
}

func (r metricsRow) toDailyStats(date string) types.DailyStatsItem {
	cost := r.Cost.Float64
	return types.DailyStatsItem{
		Date:           date,
		InputToken:     int(r.InputTokens.Float64),
		OutputToken:    int(r.OutputTokens.Float64),
		InputCost:      cost * 0.6,
		OutputCost:     cost * 0.4,
		WaitTime:       int(r.WaitTime.Float64),
		RequestSuccess: int(r.SuccessCount.Float64),
		RequestFailed:  int(r.FailedCount.Float64),
	}
}

func (r metricsRow) toHourlyStats(hour int, date string) types.HourlyStatsItem {
	cost := r.Cost.Float64
	return types.HourlyStatsItem{
		Hour:           hour,
		Date:           date,
		InputToken:     int(r.InputTokens.Float64),
		OutputToken:    int(r.OutputTokens.Float64),
		InputCost:      cost * 0.6,
		OutputCost:     cost * 0.4,
		WaitTime:       int(r.WaitTime.Float64),
		RequestSuccess: int(r.SuccessCount.Float64),
		RequestFailed:  int(r.FailedCount.Float64),
	}
}

const metricsSelectSQL = `
	SUM(input_tokens),
	SUM(output_tokens),
	SUM(cost),
	SUM(use_time),
	SUM(CASE WHEN error = '' THEN 1 ELSE 0 END),
	SUM(CASE WHEN error != '' THEN 1 ELSE 0 END)
`

func scanMetrics(scanner interface{ Scan(...any) error }) (metricsRow, error) {
	var m metricsRow
	err := scanner.Scan(&m.InputTokens, &m.OutputTokens, &m.Cost, &m.WaitTime, &m.SuccessCount, &m.FailedCount)
	return m, err
}

func GetTotalStats(ctx context.Context, db *bun.DB) (*types.DailyStatsItem, error) {
	row := db.QueryRowContext(ctx, "SELECT "+metricsSelectSQL+" FROM relay_logs")
	m, err := scanMetrics(row)
	if err != nil {
		return nil, err
	}
	result := m.toDailyStats("")
	return &result, nil
}

func GetTodayStats(ctx context.Context, db *bun.DB, tz string) (*types.DailyStatsItem, error) {
	mod := tzModifier(tz)
	todayStr := localDateStr(time.Now(), parseTzMinutes(tz))
	query := fmt.Sprintf(`SELECT %s FROM relay_logs
		WHERE strftime('%%Y%%m%%d', time, 'unixepoch', '%s') = ?`, metricsSelectSQL, mod)
	row := db.QueryRowContext(ctx, query, todayStr)
	m, err := scanMetrics(row)
	if err != nil {
		return nil, err
	}
	result := m.toDailyStats(todayStr)
	return &result, nil
}

func GetDailyStats(ctx context.Context, db *bun.DB, tz string) ([]types.DailyStatsItem, error) {
	mod := tzModifier(tz)
	query := fmt.Sprintf(`SELECT strftime('%%Y%%m%%d', time, 'unixepoch', '%s') as date, %s
		FROM relay_logs
		WHERE time >= unixepoch('now', '-365 days')
		GROUP BY date ORDER BY date`, mod, metricsSelectSQL)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []types.DailyStatsItem
	for rows.Next() {
		var date string
		var m metricsRow
		if err := rows.Scan(&date, &m.InputTokens, &m.OutputTokens, &m.Cost, &m.WaitTime, &m.SuccessCount, &m.FailedCount); err != nil {
			return nil, err
		}
		results = append(results, m.toDailyStats(date))
	}
	if results == nil {
		results = []types.DailyStatsItem{}
	}
	return results, nil
}

func GetHourlyStats(ctx context.Context, db *bun.DB, startDate, endDate, tz string) ([]types.HourlyStatsItem, error) {
	mod := tzModifier(tz)
	if startDate == "" {
		startDate = localDateStr(time.Now(), parseTzMinutes(tz))
	}
	if endDate == "" {
		endDate = startDate
	}

	query := fmt.Sprintf(`SELECT
		CAST(strftime('%%H', time, 'unixepoch', '%s') AS INTEGER) as hour,
		strftime('%%Y%%m%%d', time, 'unixepoch', '%s') as date,
		%s
		FROM relay_logs
		WHERE strftime('%%Y%%m%%d', time, 'unixepoch', '%s') >= ?
		  AND strftime('%%Y%%m%%d', time, 'unixepoch', '%s') <= ?
		GROUP BY date, hour ORDER BY date, hour`, mod, mod, metricsSelectSQL, mod, mod)

	rows, err := db.QueryContext(ctx, query, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []types.HourlyStatsItem
	for rows.Next() {
		var hour int
		var date string
		var m metricsRow
		if err := rows.Scan(&hour, &date, &m.InputTokens, &m.OutputTokens, &m.Cost, &m.WaitTime, &m.SuccessCount, &m.FailedCount); err != nil {
			return nil, err
		}
		results = append(results, m.toHourlyStats(hour, date))
	}
	if results == nil {
		results = []types.HourlyStatsItem{}
	}
	return results, nil
}

func GetGlobalStats(ctx context.Context, db *bun.DB) (*types.GlobalStatsResponse, error) {
	var totalRequests int
	var totalInputTokens, totalOutputTokens, totalCost sql.NullFloat64
	err := db.QueryRowContext(ctx, `SELECT COUNT(*), SUM(input_tokens), SUM(output_tokens), SUM(cost) FROM relay_logs`).
		Scan(&totalRequests, &totalInputTokens, &totalOutputTokens, &totalCost)
	if err != nil {
		return nil, err
	}

	var activeChannels int
	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM channels WHERE enabled = 1").Scan(&activeChannels)

	var activeGroups int
	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM groups").Scan(&activeGroups)

	return &types.GlobalStatsResponse{
		TotalRequests:     totalRequests,
		TotalInputTokens:  int(totalInputTokens.Float64),
		TotalOutputTokens: int(totalOutputTokens.Float64),
		TotalCost:         totalCost.Float64,
		ActiveChannels:    activeChannels,
		ActiveGroups:      activeGroups,
	}, nil
}

func GetChannelStats(ctx context.Context, db *bun.DB) ([]types.ChannelStatsItem, error) {
	rows, err := db.QueryContext(ctx, `SELECT channel_id, channel_name, COUNT(*) as total_requests,
		SUM(cost) as total_cost, AVG(use_time) as avg_latency
		FROM relay_logs WHERE channel_id > 0
		GROUP BY channel_id, channel_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []types.ChannelStatsItem
	for rows.Next() {
		var s types.ChannelStatsItem
		var totalCost, avgLatency sql.NullFloat64
		if err := rows.Scan(&s.ChannelID, &s.ChannelName, &s.TotalRequests, &totalCost, &avgLatency); err != nil {
			return nil, err
		}
		s.TotalCost = totalCost.Float64
		s.AvgLatency = avgLatency.Float64
		results = append(results, s)
	}
	if results == nil {
		results = []types.ChannelStatsItem{}
	}
	return results, nil
}

func GetModelStats(ctx context.Context, db *bun.DB) ([]types.ModelStatsItem, error) {
	rows, err := db.QueryContext(ctx, `SELECT request_model_name, COUNT(*) as cnt,
		SUM(input_tokens), SUM(output_tokens), SUM(cost),
		AVG(use_time), AVG(ftut)
		FROM relay_logs
		GROUP BY request_model_name ORDER BY cnt DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []types.ModelStatsItem
	for rows.Next() {
		var s types.ModelStatsItem
		var inputTokens, outputTokens, totalCost, avgLatency, avgFtut sql.NullFloat64
		if err := rows.Scan(&s.Model, &s.RequestCount, &inputTokens, &outputTokens, &totalCost, &avgLatency, &avgFtut); err != nil {
			return nil, err
		}
		s.InputTokens = int(inputTokens.Float64)
		s.OutputTokens = int(outputTokens.Float64)
		s.TotalCost = totalCost.Float64
		s.AvgLatency = int(math.Round(avgLatency.Float64))
		s.AvgFirstTokenTime = int(math.Round(avgFtut.Float64))
		results = append(results, s)
	}
	if results == nil {
		results = []types.ModelStatsItem{}
	}
	return results, nil
}

func GetApiKeyStats(ctx context.Context, db *bun.DB) ([]map[string]any, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, name, enabled, total_cost, max_cost, expire_at FROM api_keys`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var id int
		var name string
		var enabled bool
		var totalCost, maxCost float64
		var expireAt int64
		if err := rows.Scan(&id, &name, &enabled, &totalCost, &maxCost, &expireAt); err != nil {
			return nil, err
		}
		results = append(results, map[string]any{
			"id":        id,
			"name":      name,
			"enabled":   enabled,
			"totalCost": totalCost,
			"maxCost":   maxCost,
			"expireAt":  expireAt,
		})
	}
	if results == nil {
		results = []map[string]any{}
	}
	return results, nil
}
