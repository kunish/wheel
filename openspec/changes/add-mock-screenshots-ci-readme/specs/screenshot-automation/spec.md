## ADDED Requirements

### Requirement: Automated screenshot capture

A Playwright-based script SHALL capture screenshots of key UI pages using mock data, producing PNG files for documentation.

#### Scenario: Capture all key pages

- **WHEN** the screenshot script runs with mock data loaded
- **THEN** it SHALL capture screenshots of Dashboard, Channels, Groups, Models, Logs, and API Keys pages
- **AND** save them as PNG files in `docs/screenshots/`

#### Scenario: Capture both light and dark themes

- **WHEN** the screenshot script runs
- **THEN** it SHALL capture each page in both light and dark theme
- **AND** name files with theme suffix (e.g., `dashboard-light.png`, `dashboard-dark.png`)

#### Scenario: Consistent viewport size

- **WHEN** screenshots are captured
- **THEN** all screenshots SHALL use a 1280x800 viewport
- **AND** wait for page content to fully render before capture

### Requirement: Screenshot script is self-contained

The screenshot script SHALL handle starting and stopping the required services (worker + web dev server) automatically.

#### Scenario: Run screenshot script standalone

- **WHEN** user runs the screenshot script via pnpm
- **THEN** it SHALL start the worker with seed data
- **AND** start the web dev server
- **AND** capture all screenshots
- **AND** shut down both services after completion

#### Scenario: Screenshot script handles service startup failures

- **WHEN** the worker or web dev server fails to start
- **THEN** the script SHALL exit with a non-zero code
- **AND** clean up any started processes
