## ADDED Requirements

### Requirement: Error row visual highlighting

Log table rows with errors SHALL be visually distinguished with a red left border (`border-l-2 border-destructive`) and a subtle red background tint (`bg-destructive/5`).

#### Scenario: Error row appearance

- **WHEN** a log entry has a non-empty `error` field
- **THEN** its table row SHALL display a red left border and tinted background
- **AND** success rows SHALL have no additional styling

### Requirement: Error message preview tooltip

The error status Badge SHALL display a Tooltip on hover showing the first 200 characters of the error message.

#### Scenario: Hover error badge

- **WHEN** user hovers over an "Error" badge in the status column
- **THEN** a tooltip SHALL appear showing a truncated preview of the error text
- **AND** the tooltip SHALL not appear for "OK" badges

### Requirement: Adjustable page size

The pagination area SHALL include a page size selector allowing users to choose between 20, 50, and 100 items per page. Changing the page size SHALL reset to page 1.

#### Scenario: Change page size

- **WHEN** user selects "50" from the page size dropdown
- **THEN** the log list SHALL reload with `pageSize=50`
- **AND** pagination SHALL reset to page 1
- **AND** the total pages count SHALL update accordingly

### Requirement: Client-side column sorting

The Latency, Cost, Input Tokens, and Output Tokens column headers SHALL be clickable to sort the current page's data. Clicking toggles between ascending and descending order. A sort indicator (arrow icon) SHALL show the current sort direction.

#### Scenario: Sort by latency descending

- **WHEN** user clicks the "Latency" column header
- **THEN** the current page rows SHALL be reordered by latency descending
- **AND** an downward arrow icon SHALL appear next to "Latency"

#### Scenario: Toggle sort direction

- **WHEN** user clicks the same column header again
- **THEN** the sort direction SHALL toggle from descending to ascending
- **AND** the arrow icon SHALL flip direction

#### Scenario: Sort different column

- **WHEN** user clicks a different sortable column
- **THEN** sorting SHALL switch to that column in descending order
- **AND** the previous column's sort indicator SHALL be removed

### Requirement: Empty state with guidance

When filters produce no results, the table SHALL display a contextual empty state message instead of a blank table.

#### Scenario: No results from filters

- **WHEN** the current filter combination returns zero logs
- **THEN** a message like "No logs match your filters" SHALL be displayed
- **AND** a "Clear all filters" button SHALL be shown
