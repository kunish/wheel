## ADDED Requirements

### Requirement: URL-persisted filter state

All log filter values SHALL be persisted in the URL query parameters. The log page SHALL read these parameters on mount to initialize filter state. Only non-default values SHALL be included in the URL.

#### Scenario: Load page with URL filters

- **WHEN** user navigates to `/logs?model=gpt-4&status=error&page=2`
- **THEN** the model filter SHALL be pre-filled with "gpt-4"
- **AND** the status filter SHALL be set to "error"
- **AND** the page SHALL display page 2

#### Scenario: Update URL on filter change

- **WHEN** user selects channel "OpenAI" from the dropdown
- **THEN** the URL SHALL update to include `channel=<channelId>`
- **AND** page navigation history SHALL be preserved (back button works)

#### Scenario: Share filtered view

- **WHEN** user copies the current URL and opens it in a new tab
- **THEN** the same filters SHALL be applied and the same results SHALL be displayed

### Requirement: Dashboard model stats link to logs

Each model name in the Dashboard's Model Usage section SHALL be a clickable link that navigates to the log page filtered by that model.

#### Scenario: Click model in dashboard

- **WHEN** user clicks "claude-3-opus" in the Model Usage section
- **THEN** the browser SHALL navigate to `/logs?model=claude-3-opus`
- **AND** the log page SHALL show logs filtered by that model

### Requirement: Dashboard channel stats link to logs

Each channel name in the Dashboard's Channel Ranking section SHALL be a clickable link that navigates to the log page filtered by that channel.

#### Scenario: Click channel in dashboard

- **WHEN** user clicks "OpenAI-Primary" in the Channel Ranking section
- **THEN** the browser SHALL navigate to `/logs?channel=<channelId>`
- **AND** the log page SHALL show logs filtered by that channel

### Requirement: Dashboard heatmap cell links to logs

Activity heatmap cells (day-level and hour-level) SHALL be clickable to navigate to the log page filtered by the corresponding time range.

#### Scenario: Click day cell in heatmap

- **WHEN** user clicks a day cell showing "2025-02-08" in the activity heatmap
- **THEN** the browser SHALL navigate to `/logs?from=<dayStartTimestamp>&to=<dayEndTimestamp>`
- **AND** the log page SHALL show logs from that entire day

#### Scenario: Click hour cell in weekly heatmap

- **WHEN** user clicks the cell for "Monday 14:00" in the weekly heatmap
- **THEN** the browser SHALL navigate to `/logs?from=<hourStartTimestamp>&to=<hourEndTimestamp>`
- **AND** the log page SHALL show logs from that specific hour

### Requirement: Clear all filters action

The log page SHALL provide a "Clear all" action that resets all filters to defaults and clears the URL query string.

#### Scenario: Clear all filters

- **WHEN** user clicks "Clear all filters"
- **THEN** all filter inputs SHALL be reset to their default values
- **AND** the URL SHALL become `/logs` (no query params)
- **AND** the log list SHALL show unfiltered results on page 1
