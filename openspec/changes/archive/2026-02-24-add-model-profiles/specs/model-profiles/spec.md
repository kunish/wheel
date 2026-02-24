## ADDED Requirements

### Requirement: Profile data model

The system SHALL store model profiles in a `model_profiles` database table with the following fields: id (auto-increment), name (text), provider (text), models (JSON text array of model IDs), is_builtin (boolean), created_at (timestamp), updated_at (timestamp).

#### Scenario: Profile record structure

- **WHEN** a profile is created
- **THEN** it SHALL contain a unique id, name, provider key, a JSON array of model ID strings, a builtin flag, and timestamps

### Requirement: Profile CRUD API

The system SHALL provide RESTful API endpoints for managing model profiles under `/api/v1/model/profiles`.

#### Scenario: List all profiles

- **WHEN** a GET request is made to `/api/v1/model/profiles`
- **THEN** the system SHALL return all profiles ordered by is_builtin DESC, name ASC

#### Scenario: Create a custom profile

- **WHEN** a POST request is made to `/api/v1/model/profiles` with name, provider, and models fields
- **THEN** the system SHALL create a new profile with `is_builtin=false` and return the created record

#### Scenario: Update a custom profile

- **WHEN** a PUT request is made to `/api/v1/model/profiles/:id` for a profile with `is_builtin=false`
- **THEN** the system SHALL update the name, provider, and models fields and return the updated record

#### Scenario: Reject update of builtin profile

- **WHEN** a PUT request is made to `/api/v1/model/profiles/:id` for a profile with `is_builtin=true`
- **THEN** the system SHALL return a 403 error with message indicating builtin profiles cannot be modified

#### Scenario: Delete a custom profile

- **WHEN** a DELETE request is made to `/api/v1/model/profiles/:id` for a profile with `is_builtin=false`
- **THEN** the system SHALL delete the profile and return success

#### Scenario: Reject delete of builtin profile

- **WHEN** a DELETE request is made to `/api/v1/model/profiles/:id` for a profile with `is_builtin=true`
- **THEN** the system SHALL return a 403 error with message indicating builtin profiles cannot be deleted

### Requirement: Builtin profile generation from models.dev

The system SHALL generate builtin profiles for Anthropic, OpenAI, and Google from models.dev metadata.

#### Scenario: Generate builtin profiles on metadata refresh

- **WHEN** the model metadata refresh is triggered (via API or cron)
- **THEN** the system SHALL extract model IDs for providers "anthropic", "openai", and "google" from the models.dev data
- **THEN** the system SHALL upsert `is_builtin=true` profiles named "Anthropic", "OpenAI", and "Google" with the corresponding model ID lists

#### Scenario: Builtin profile update preserves IDs

- **WHEN** builtin profiles are regenerated
- **THEN** existing builtin profile records SHALL be updated in-place (same id) rather than deleted and recreated

### Requirement: Profile application to channel

The system SHALL support applying a profile's model list to a channel's model configuration.

#### Scenario: Apply profile merges models

- **WHEN** a user applies a profile to a channel in the edit dialog
- **THEN** the profile's model list SHALL be merged with the channel's existing model list with duplicates removed

#### Scenario: Apply profile does not remove existing models

- **WHEN** a channel has models ["gpt-4o", "custom-model"] and a profile with ["gpt-4o", "gpt-4o-mini"] is applied
- **THEN** the resulting model list SHALL be ["gpt-4o", "custom-model", "gpt-4o-mini"]

### Requirement: Profile selector in channel dialog

The channel edit dialog SHALL include a profile selector that allows quick loading of model presets.

#### Scenario: Profile selector displays available profiles

- **WHEN** the channel edit dialog is open
- **THEN** a profile selector SHALL be displayed above the model input area showing all available profiles

#### Scenario: Selecting a profile merges models

- **WHEN** the user selects a profile from the selector
- **THEN** the profile's models SHALL be merged into the current model list

#### Scenario: Profile selector shows model count

- **WHEN** profiles are listed in the selector
- **THEN** each profile entry SHALL display the profile name and the number of models it contains

### Requirement: Profile management UI

The system SHALL provide a profile management interface accessible from the model page toolbar.

#### Scenario: Open profile management

- **WHEN** the user clicks the "Profiles" button in the model page toolbar
- **THEN** a dialog SHALL open showing all profiles with builtin profiles marked

#### Scenario: Create custom profile from management dialog

- **WHEN** the user clicks "Add Profile" in the management dialog
- **THEN** a form SHALL appear for entering name, provider, and model list

#### Scenario: Builtin profiles are read-only

- **WHEN** the management dialog displays a builtin profile
- **THEN** edit and delete actions SHALL be disabled for that profile

#### Scenario: Duplicate builtin profile as custom

- **WHEN** the user clicks "Duplicate" on a builtin profile
- **THEN** a new custom profile SHALL be created with the same models but editable name and model list

### Requirement: Profile i18n support

All profile-related UI strings SHALL be available in English (en) and Chinese Simplified (zh-CN).

#### Scenario: English locale profile strings

- **WHEN** the locale is "en"
- **THEN** profile UI strings SHALL include "Profiles", "Add Profile", "Apply", "Duplicate", "models"

#### Scenario: Chinese locale profile strings

- **WHEN** the locale is "zh-CN"
- **THEN** profile UI strings SHALL include "预设", "添加预设", "应用", "复制", "个模型"
