## ADDED Requirements

### Requirement: Mock data seed command

Worker SHALL provide a `seed` subcommand that populates SQLite databases with realistic demo data, covering channels, groups, API keys, pricing, and request logs.

#### Scenario: Run seed command on empty database

- **WHEN** user runs `./worker seed`
- **THEN** the system SHALL create demo channels for OpenAI, Anthropic, and Google Gemini providers
- **AND** create demo groups with channel-model pairings and load balancing configurations
- **AND** create demo API keys with varying quotas and expiration settings
- **AND** create demo pricing entries for common models
- **AND** create demo request logs spanning the past 30 days with realistic token usage and cost data

#### Scenario: Seed command is idempotent

- **WHEN** user runs `./worker seed` on a database that already contains seed data
- **THEN** the system SHALL skip existing records without error
- **AND** NOT duplicate any data

#### Scenario: Seed data produces realistic dashboard

- **WHEN** seed data is loaded and user opens the dashboard
- **THEN** the heatmap SHALL show activity across the past 30 days
- **AND** cost trend charts SHALL display meaningful data
- **AND** channel rankings and model statistics SHALL be populated

### Requirement: Mock data covers all entity types

The seed command SHALL populate data for every major entity type in the system to ensure all UI pages have content to display.

#### Scenario: All management pages have data

- **WHEN** seed data is loaded
- **THEN** the Channels page SHALL show at least 3 channels with different providers
- **AND** the Groups page SHALL show at least 2 groups with channel-model pairings
- **AND** the API Keys page SHALL show at least 3 keys with different quota/expiration states
- **AND** the Models page SHALL show models from multiple providers with pricing
- **AND** the Logs page SHALL show request logs with varied statuses (success, error, retry)
