## ADDED Requirements

### Requirement: CI screenshot job on release

The release workflow SHALL include a screenshot generation job that runs when a new version is released.

#### Scenario: Screenshots generated on release

- **WHEN** release-please creates a new release
- **THEN** the `screenshots` job SHALL run
- **AND** build the worker with seed data support
- **AND** build the web frontend
- **AND** execute the screenshot script
- **AND** commit updated screenshots to the repository

#### Scenario: Screenshot job does not block release

- **WHEN** the screenshot job fails
- **THEN** Docker build and Pages deployment jobs SHALL NOT be affected
- **AND** the release SHALL proceed normally

#### Scenario: Screenshots committed automatically

- **WHEN** the screenshot job completes successfully
- **THEN** it SHALL commit changed screenshots to the `main` branch
- **AND** use a bot commit message following conventional commits format (e.g., `docs: update screenshots for vX.Y.Z`)

### Requirement: CI screenshot job dependencies

The screenshot job SHALL install all required dependencies including Go, Node.js, pnpm, and Playwright browsers.

#### Scenario: CI environment setup

- **WHEN** the screenshot job starts
- **THEN** it SHALL install Go for building the worker
- **AND** install Node.js and pnpm for the web frontend
- **AND** install Playwright Chromium browser
- **AND** complete all setup before running the screenshot script
