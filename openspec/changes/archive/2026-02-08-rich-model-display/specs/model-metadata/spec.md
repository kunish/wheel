## ADDED Requirements

### Requirement: Backend model metadata API

The system SHALL provide a GET endpoint `/api/v1/model/metadata` that returns a map of model ID to model metadata. The metadata SHALL be fetched from `models.dev/api.json`, flattened into `Record<modelId, { name, provider, providerName, logoUrl }>`, and cached in KV with a 24-hour TTL.

#### Scenario: First request with empty cache

- **WHEN** the frontend calls `GET /api/v1/model/metadata` and the KV cache is empty
- **THEN** the system SHALL fetch from `models.dev/api.json`, process and cache the result, and return the metadata map

#### Scenario: Subsequent request with valid cache

- **WHEN** the frontend calls `GET /api/v1/model/metadata` and the KV cache contains valid metadata
- **THEN** the system SHALL return the cached metadata without fetching from models.dev

#### Scenario: models.dev is unavailable

- **WHEN** the fetch to `models.dev/api.json` fails and no cache exists
- **THEN** the system SHALL return an empty metadata map with a success response

### Requirement: Frontend model metadata hook

The system SHALL provide a `useModelMeta(modelId)` hook that returns `{ name, provider, providerName, logoUrl } | null` for a given model ID. The hook SHALL use TanStack Query with a staleTime of 1 hour.

#### Scenario: Model exists in metadata

- **WHEN** `useModelMeta("gpt-4o")` is called and metadata is loaded
- **THEN** the hook SHALL return `{ name: "GPT-4o", provider: "openai", providerName: "OpenAI", logoUrl: "https://models.dev/logos/openai.svg" }`

#### Scenario: Model not in metadata

- **WHEN** `useModelMeta("unknown-model")` is called
- **THEN** the hook SHALL return `null`

### Requirement: ModelBadge component

The system SHALL provide a `ModelBadge` component that accepts a `modelId` prop and displays the model's provider logo and display name. If the model is not found in metadata, it SHALL fallback to displaying the raw model ID.

#### Scenario: Known model display

- **WHEN** `ModelBadge` renders with `modelId="gpt-4o"`
- **THEN** it SHALL display the OpenAI logo (16x16) followed by "GPT-4o"

#### Scenario: Unknown model fallback

- **WHEN** `ModelBadge` renders with `modelId="custom-model-xyz"`
- **THEN** it SHALL display "custom-model-xyz" without a logo

#### Scenario: Logo load failure

- **WHEN** the provider logo image fails to load
- **THEN** the component SHALL hide the image and display only the text name

### Requirement: Channels page model display

The system SHALL use `ModelBadge` to display models in channel cards and in the channel dialog's fetched model results.

#### Scenario: Channel card model list

- **WHEN** a channel card displays its models
- **THEN** each model SHALL be rendered using `ModelBadge` instead of plain text

### Requirement: Prices page model display

The system SHALL use `ModelBadge` in the prices table's model name column.

#### Scenario: Price table model column

- **WHEN** the prices table renders a model row
- **THEN** the model name cell SHALL display `ModelBadge` with the model's logo and display name

### Requirement: Logs page model display

The system SHALL use `ModelBadge` in the logs table's model column.

#### Scenario: Log entry model display

- **WHEN** a log entry row renders
- **THEN** the model column SHALL display `ModelBadge`

### Requirement: Dashboard model display

The system SHALL use `ModelBadge` in dashboard sections where model names appear (e.g., channel ranking tooltips).

#### Scenario: Dashboard model reference

- **WHEN** the dashboard displays a model name in any context
- **THEN** it SHALL use `ModelBadge` for the display
