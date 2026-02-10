## ADDED Requirements

### Requirement: Group models by provider

Model lists with 4+ items must be visually grouped by provider. Each group shows a provider header followed by the group's model cards.

#### Scenario: Channel card with 6 models from 3 providers

- **WHEN** a channel card renders 6 models (2 GPT, 2 Claude, 2 Gemini)
- **THEN** models are grouped under "OpenAI" (2), "Anthropic" (2), "Google" (2) headers with provider logo, name, and count

#### Scenario: Channel card with fewer than 4 models

- **WHEN** a channel card renders 3 or fewer models
- **THEN** models are rendered as a flat list without provider headers

#### Scenario: Models without metadata

- **WHEN** some models have no metadata match
- **THEN** they are grouped under "Other" at the end of the list

### Requirement: Provider section header

Each provider group is preceded by a compact inline header.

#### Scenario: Header content

- **WHEN** a provider group header renders
- **THEN** it shows: provider logo (16x16, dark:invert), provider name, and model count in parentheses

### Requirement: Render callback

The GroupedModelList component accepts a `renderModel` callback for location-specific rendering.

#### Scenario: Channel card with draggable models

- **WHEN** GroupedModelList is used in channel card
- **THEN** `renderModel(modelId)` returns a `DraggableModelTag` preserving drag-and-drop

#### Scenario: ModelTagInput with removable models

- **WHEN** GroupedModelList is used in ModelTagInput
- **THEN** `renderModel(modelId)` returns a `ModelCard` with `onRemove` handler

### Requirement: Group ordering

#### Scenario: Multiple providers present

- **WHEN** models span multiple providers
- **THEN** groups are sorted alphabetically by provider name, with "Other" always last
