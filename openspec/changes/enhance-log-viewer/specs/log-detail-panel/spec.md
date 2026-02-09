## ADDED Requirements

### Requirement: Side panel replaces modal dialog

Log detail SHALL open in a right-side Sheet (slide-out panel) instead of a centered Dialog. The panel SHALL have a max width of `max-w-2xl` and the log table SHALL remain visible and interactive behind it.

#### Scenario: Open log detail

- **WHEN** user clicks a log row or the eye icon
- **THEN** a Sheet panel SHALL slide in from the right
- **AND** the log table SHALL remain visible in the background

#### Scenario: Close panel

- **WHEN** user clicks outside the panel, presses Escape, or clicks the close button
- **THEN** the Sheet SHALL close
- **AND** the table SHALL return to full width

### Requirement: Prev/next log navigation

The detail panel SHALL include previous (↑) and next (↓) navigation buttons to browse adjacent logs within the current filtered list without closing the panel.

#### Scenario: Navigate to next log

- **WHEN** user clicks the "next" button in the detail panel
- **THEN** the panel SHALL load the next log entry in the current list
- **AND** the table row highlight SHALL move to the corresponding row

#### Scenario: Boundary navigation

- **WHEN** user is viewing the last log on the current page and clicks "next"
- **THEN** the next button SHALL be disabled

### Requirement: In-content keyword search

The Request and Response tabs SHALL include a search input that highlights matching text within the displayed content.

#### Scenario: Search within request JSON

- **WHEN** user types "system" in the in-content search input on the Request tab
- **THEN** all occurrences of "system" in the displayed content SHALL be highlighted
- **AND** a match count (e.g., "3 / 12 matches") SHALL be displayed

#### Scenario: No matches

- **WHEN** user searches for a term not present in the content
- **THEN** the match count SHALL show "0 matches"

### Requirement: Backend request replay endpoint

The system SHALL provide a `POST /api/v1/log/replay/:id` endpoint that reads the stored log's `requestContent`, parses it, and forwards it through the relay pipeline. The response SHALL be returned to the caller (supporting both streaming and non-streaming).

#### Scenario: Replay a successful log

- **WHEN** `POST /api/v1/log/replay/42` is called
- **THEN** the backend SHALL read log #42's `requestContent` and `requestModelName`
- **AND** forward the request through the relay pipeline
- **AND** return the relay response (streamed if the original was streamed)

#### Scenario: Replay a log with truncated content

- **WHEN** the log's `requestContent` was truncated during storage
- **THEN** the response SHALL include a warning header or field indicating content was truncated
- **AND** the replay SHALL proceed with the available (truncated) content

#### Scenario: Replay a non-existent log

- **WHEN** `POST /api/v1/log/replay/99999` is called for a log that doesn't exist
- **THEN** the endpoint SHALL return 404 with `{"success": false, "error": "Log not found"}`

### Requirement: Replay UI in detail panel

The detail panel SHALL include a "Replay" button in the Overview tab. Clicking it SHALL trigger the backend replay endpoint and display the response in a new "Replay Result" tab.

#### Scenario: Trigger replay

- **WHEN** user clicks the "Replay" button
- **THEN** the button SHALL show a loading state
- **AND** a `POST /api/v1/log/replay/:id` request SHALL be sent
- **AND** on completion, a "Replay Result" tab SHALL appear with the response content

#### Scenario: Replay with truncation warning

- **WHEN** the log content was truncated
- **THEN** a warning message SHALL be shown before replay: "Request content was truncated during storage. Replay may produce different results."

### Requirement: Copy individual fields

Each metric field in the Overview tab (Model, Channel, Time, Tokens, Cost, Error) SHALL have a copy button that copies the field value to clipboard.

#### Scenario: Copy error message

- **WHEN** user clicks the copy button next to the Error field
- **THEN** the full error text SHALL be copied to clipboard
- **AND** a brief "Copied" confirmation SHALL appear
