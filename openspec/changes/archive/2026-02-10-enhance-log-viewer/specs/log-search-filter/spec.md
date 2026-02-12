## ADDED Requirements

### Requirement: Unified keyword search across log fields

The system SHALL provide a single search input that searches across `requestModelName`, `channelName`, `error`, `requestContent`, and `responseContent` fields using case-insensitive substring matching.

#### Scenario: Search by error keyword

- **WHEN** user types "rate limit" in the search bar
- **THEN** the log list SHALL display only logs where any of the searchable fields contain "rate limit"

#### Scenario: Search by model name via keyword

- **WHEN** user types "gpt-4" in the search bar
- **THEN** the log list SHALL include logs where `requestModelName`, `actualModelName`, or content fields contain "gpt-4"

#### Scenario: Empty search

- **WHEN** user clears the search bar
- **THEN** the keyword filter SHALL be removed and all logs (subject to other active filters) SHALL be displayed

### Requirement: Backend keyword search parameter

The `listLogs` API SHALL accept an optional `keyword` query parameter. When provided, the backend SHALL filter logs using SQL `LIKE '%keyword%'` across `requestModelName`, `channelName`, `error`, `requestContent`, and `responseContent` columns combined with OR logic.

#### Scenario: API keyword filtering

- **WHEN** `GET /api/v1/log/list?keyword=timeout` is called
- **THEN** the response SHALL contain only logs where at least one of the searchable fields contains "timeout"

### Requirement: Channel filter dropdown

The system SHALL provide a dropdown select that lists all channels (fetched from `listChannels` API). Selecting a channel SHALL filter logs to only those processed through that channel.

#### Scenario: Filter by channel

- **WHEN** user selects "OpenAI-Primary" from the channel dropdown
- **THEN** the log list SHALL show only logs with that channel's ID
- **AND** the `channelId` query parameter SHALL be sent to the backend

#### Scenario: Clear channel filter

- **WHEN** user selects "All Channels" (default option)
- **THEN** the channel filter SHALL be removed

### Requirement: Time range filter with presets

The system SHALL provide quick preset buttons for common time ranges (1h, 6h, 24h, 7d) and custom date-time inputs for arbitrary ranges. The selected range SHALL be sent as `startTime` and `endTime` unix timestamp parameters to the backend.

#### Scenario: Quick preset selection

- **WHEN** user clicks the "1h" preset button
- **THEN** `startTime` SHALL be set to current time minus 3600 seconds
- **AND** `endTime` SHALL not be set (or set to current time)
- **AND** the log list SHALL refresh showing only logs from the last hour

#### Scenario: Custom time range

- **WHEN** user sets a custom start and end datetime
- **THEN** the log list SHALL show only logs within that time range

#### Scenario: Clear time range

- **WHEN** user clicks the active preset button again or clears the custom range
- **THEN** the time range filter SHALL be removed

### Requirement: Active filter chips display

The system SHALL display currently active filters as removable Badge chips below the filter bar. Each chip SHALL show the filter type and value. Clicking the remove icon on a chip SHALL clear that specific filter.

#### Scenario: Multiple active filters

- **WHEN** user has model="gpt-4", status="error", and time range="1h" active
- **THEN** three filter chips SHALL be displayed: "Model: gpt-4", "Status: Error", "Time: Last 1h"
- **AND** each chip SHALL have a clickable remove (×) icon

#### Scenario: Remove single filter via chip

- **WHEN** user clicks the × on the "Status: Error" chip
- **THEN** the status filter SHALL be cleared
- **AND** the remaining chips SHALL still be displayed
- **AND** the log list SHALL refresh without the status filter

### Requirement: Debounced search input

The keyword search input SHALL debounce API calls by 300ms to avoid excessive requests during typing.

#### Scenario: Rapid typing

- **WHEN** user types "timeout" character by character
- **THEN** the API SHALL be called once (after 300ms of no typing), not six times
