## ADDED Requirements

### Requirement: Unified Model page route

The application SHALL serve the unified Model page at the `/model` route, replacing the previous `/channels` route.

#### Scenario: User navigates to /model

- **WHEN** an authenticated user navigates to `/model`
- **THEN** the system SHALL display the unified Model page with Channels, Groups, and Prices functionality

#### Scenario: Legacy /channels redirect

- **WHEN** a user navigates to `/channels`
- **THEN** the system SHALL redirect to `/model` preserving any query parameters (e.g., `?highlight=123`)

#### Scenario: Legacy /prices redirect

- **WHEN** a user navigates to `/prices`
- **THEN** the system SHALL redirect to `/model`

#### Scenario: Legacy /groups redirect

- **WHEN** a user navigates to `/groups`
- **THEN** the system SHALL redirect to `/model`

### Requirement: Updated bottom navigation

The bottom navigation bar SHALL display 4 items: Dashboard, Model, Logs, Settings — removing the separate Prices entry.

#### Scenario: Navigation displays Model instead of Channels & Groups

- **WHEN** the bottom navigation renders
- **THEN** it SHALL show "Model" (EN) / "模型" (ZH-CN) with a `Boxes` icon at the `/model` route position

#### Scenario: Prices navigation entry removed

- **WHEN** the bottom navigation renders
- **THEN** it SHALL NOT include a separate "Prices" navigation item

#### Scenario: Active state on Model page

- **WHEN** the user is on the `/model` page
- **THEN** the "Model" navigation item SHALL be highlighted as active

### Requirement: Route transition animation order

The `protected-layout.tsx` transition order array SHALL reflect the new route structure: `["/dashboard", "/model", "/logs", "/settings"]`.

#### Scenario: Transition direction from Dashboard to Model

- **WHEN** user navigates from `/dashboard` to `/model`
- **THEN** the page SHALL animate with a downward slide (direction = 1)

#### Scenario: Transition direction from Model to Logs

- **WHEN** user navigates from `/model` to `/logs`
- **THEN** the page SHALL animate with a downward slide (direction = 1)

### Requirement: Page title and header

The unified Model page SHALL display "Model" (EN) / "模型" (ZH-CN) as the page title.

#### Scenario: English locale

- **WHEN** the page renders with English locale
- **THEN** the page header SHALL display "Model"

#### Scenario: Chinese locale

- **WHEN** the page renders with Chinese locale
- **THEN** the page header SHALL display "模型"

### Requirement: Integrated price management toolbar

The Model page SHALL include price management actions in the top toolbar area, alongside the existing Models button.

#### Scenario: Sync Prices button

- **WHEN** the Model page renders
- **THEN** a "Sync Prices" button SHALL be displayed in the toolbar
- **WHEN** the user clicks "Sync Prices"
- **THEN** the system SHALL call the `syncModelPrices` API and refresh price data

#### Scenario: Add Price button

- **WHEN** the user clicks the "Add Price" button in the toolbar
- **THEN** a dialog SHALL open allowing the user to enter model name, input price, and output price
- **WHEN** the user submits the form
- **THEN** the system SHALL call `createModelPrice` and refresh the price list

#### Scenario: Last sync time display

- **WHEN** price data has been synced before
- **THEN** the toolbar SHALL display the last sync time

### Requirement: Price display in ModelCard

The `ModelCard` component SHALL display model pricing information when available.

#### Scenario: Model has price data

- **WHEN** a model is displayed in a Channel or Group card and price data exists for that model
- **THEN** the ModelCard SHALL show the input and output price in a compact format (e.g., "↓0.15 ↑0.60" for $0.15/$0.60 per M tokens)

#### Scenario: Model has no price data

- **WHEN** a model is displayed but no price data exists for that model
- **THEN** the ModelCard SHALL render without price information (no empty space or placeholder)

#### Scenario: Price is editable from ModelCard

- **WHEN** the user clicks on the price display within a ModelCard
- **THEN** an edit price dialog SHALL open pre-filled with the current price values

### Requirement: Price editing dialog

The system SHALL provide a dialog for editing existing model prices.

#### Scenario: Edit price from inline display

- **WHEN** the user triggers price editing for a model
- **THEN** a dialog SHALL open with the model name (read-only), input price, and output price fields pre-filled
- **WHEN** the user submits changes
- **THEN** the system SHALL call `updateModelPrice` and refresh the price data

#### Scenario: Delete price

- **WHEN** the user deletes a price entry
- **THEN** a confirmation dialog SHALL appear
- **WHEN** the user confirms deletion
- **THEN** the system SHALL call `deleteModelPrice` and refresh the price data

### Requirement: Model list dialog with prices

The "Models" button in the toolbar SHALL open a dialog showing all available models with their pricing information.

#### Scenario: Model list shows prices

- **WHEN** the user opens the model list dialog
- **THEN** each model entry SHALL display the model name and its pricing (input/output) if available

#### Scenario: Model list with no prices

- **WHEN** a model in the list has no price data
- **THEN** that model entry SHALL display the model name without pricing (showing "-" or similar)

### Requirement: i18n translation updates

All user-facing strings SHALL be available in both English (en) and Chinese Simplified (zh-CN) locales.

#### Scenario: Navigation translation

- **WHEN** the locale is "en"
- **THEN** `nav.model` SHALL return "Model"
- **WHEN** the locale is "zh-CN"
- **THEN** `nav.model` SHALL return "模型"

#### Scenario: Price-related translations in model namespace

- **WHEN** price management UI renders
- **THEN** all strings (sync, add, edit, delete, form labels) SHALL use the `model` translation namespace

### Requirement: File structure reorganization

The frontend file structure SHALL be reorganized to reflect the unified Model page.

#### Scenario: pages/channels.tsx renamed

- **WHEN** the codebase is inspected
- **THEN** the main page component SHALL be at `pages/model.tsx`

#### Scenario: pages/channels/ directory renamed

- **WHEN** the codebase is inspected
- **THEN** sub-components SHALL be under `pages/model/` directory

#### Scenario: pages/prices.tsx removed

- **WHEN** the codebase is inspected
- **THEN** `pages/prices.tsx` SHALL NOT exist (its functionality merged into `pages/model.tsx` and `pages/model/price-dialog.tsx`)

#### Scenario: Translation files reorganized

- **WHEN** the codebase is inspected
- **THEN** `i18n/locales/{locale}/model.json` SHALL exist containing merged translations from the former `channels.json` and `prices.json`
- **THEN** `channels.json` and `prices.json` SHALL be removed
