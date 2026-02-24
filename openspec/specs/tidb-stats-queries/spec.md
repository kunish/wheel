## ADDED Requirements

### Requirement: MySQL time-based date grouping

The system SHALL use `DATE_FORMAT(DATE_ADD(FROM_UNIXTIME(time), INTERVAL ? SECOND), '%Y%m%d')` for date grouping in statistics queries, replacing SQLite's `strftime('%%Y%%m%%d', time, 'unixepoch', modifier)`.

#### Scenario: Daily stats grouping with timezone

- **WHEN** `GetDailyStats` is called with timezone `+08:00`
- **THEN** the query SHALL group by `DATE_FORMAT(DATE_ADD(FROM_UNIXTIME(time), INTERVAL 28800 SECOND), '%Y%m%d')`

#### Scenario: Today stats filtering with timezone

- **WHEN** `GetTodayStats` is called with timezone `+08:00`
- **THEN** the query SHALL filter using `DATE_FORMAT(DATE_ADD(FROM_UNIXTIME(time), INTERVAL 28800 SECOND), '%Y%m%d') = ?`

### Requirement: MySQL hour extraction

The system SHALL use `HOUR(DATE_ADD(FROM_UNIXTIME(time), INTERVAL ? SECOND))` for hourly stats, replacing SQLite's `CAST(strftime('%%H', ...) AS INTEGER)`.

#### Scenario: Hourly stats grouping

- **WHEN** `GetHourlyStats` is called
- **THEN** the query SHALL extract hour using `HOUR(DATE_ADD(FROM_UNIXTIME(time), INTERVAL ? SECOND))`

### Requirement: MySQL time range filtering

The system SHALL use `UNIX_TIMESTAMP(DATE_SUB(NOW(), INTERVAL 365 DAY))` for time range filters, replacing SQLite's `unixepoch('now', '-365 days')`.

#### Scenario: Daily stats 365-day window

- **WHEN** `GetDailyStats` queries the last year of data
- **THEN** the WHERE clause SHALL use `time >= UNIX_TIMESTAMP(DATE_SUB(NOW(), INTERVAL 365 DAY))`

### Requirement: CAST AS DOUBLE for aggregations

The system SHALL use `CAST(... AS DOUBLE)` in all aggregation expressions, replacing SQLite's `CAST(... AS REAL)`.

#### Scenario: Metrics column expressions

- **WHEN** `metricsColumns()` builds aggregation expressions
- **THEN** all CAST expressions SHALL use `DOUBLE` (e.g., `CAST(COALESCE(SUM(cost), 0) AS DOUBLE)`)

### Requirement: Timezone offset helper returns seconds

The `tzModifier` function SHALL be refactored to return an integer (seconds offset) instead of a SQLite modifier string, for use with MySQL `INTERVAL ? SECOND`.

#### Scenario: Positive timezone offset

- **WHEN** timezone is `+08:00`
- **THEN** the function SHALL return `28800`

#### Scenario: Negative timezone offset

- **WHEN** timezone is `-05:00`
- **THEN** the function SHALL return `-18000`

#### Scenario: Empty timezone

- **WHEN** timezone is empty string
- **THEN** the function SHALL return `0`
