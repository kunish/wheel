## ADDED Requirements

### Requirement: User can configure API base URL in the UI

The web frontend SHALL provide a UI for the user to set the worker's base URL (e.g., `https://my-wheel-instance.example.com`). This setting SHALL be persisted to `localStorage`.

#### Scenario: Setting API URL on first visit

- **WHEN** a user visits the GitHub Pages deployment for the first time
- **AND** no API base URL is configured
- **THEN** the login page SHALL display an input field to set the worker base URL before login

#### Scenario: Changing API URL from settings

- **WHEN** a user navigates to the settings page while logged in
- **THEN** there SHALL be a field showing the current API base URL
- **AND** the user SHALL be able to update and save a new URL

### Requirement: API requests use configured base URL

The `apiFetch` function and WebSocket connection SHALL prepend the configured base URL to all API requests when it is set. When the base URL is empty or unset, requests SHALL use relative paths (backward compatible with proxy setups).

#### Scenario: Requests with base URL configured

- **WHEN** `apiBaseUrl` is set to `https://api.example.com`
- **AND** the client calls `GET /api/v1/channel/list`
- **THEN** the actual HTTP request SHALL be sent to `https://api.example.com/api/v1/channel/list`

#### Scenario: Requests without base URL (default)

- **WHEN** `apiBaseUrl` is empty or unset
- **AND** the client calls `GET /api/v1/channel/list`
- **THEN** the request SHALL use the relative path `/api/v1/channel/list` (same-origin)

#### Scenario: WebSocket uses configured base URL

- **WHEN** `apiBaseUrl` is set to `https://api.example.com`
- **THEN** the WebSocket connection SHALL connect to `wss://api.example.com/api/v1/ws`

### Requirement: Base URL stored in Zustand auth store

The API base URL SHALL be stored in the Zustand auth store alongside the JWT token, with `localStorage` persistence.

#### Scenario: URL persists across page reloads

- **WHEN** a user sets the API base URL to `https://api.example.com`
- **AND** refreshes the page
- **THEN** the base URL SHALL still be `https://api.example.com`

### Requirement: Connection validation

When the user sets or changes the API base URL, the system SHALL validate the connection by calling the health check endpoint (`GET /`) and display success or error feedback.

#### Scenario: Valid URL connection test

- **WHEN** the user enters a valid worker URL and clicks save/connect
- **THEN** the system SHALL call `GET /` on that URL
- **AND** display a success indicator if the response contains `{"name": "wheel"}`

#### Scenario: Invalid URL connection test

- **WHEN** the user enters an unreachable or invalid URL
- **THEN** the system SHALL display an error message indicating the connection failed
