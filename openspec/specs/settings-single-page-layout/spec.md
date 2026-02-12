## ADDED Requirements

### Requirement: Settings page displays all sections on a single page

The settings page SHALL render all settings sections (API Keys, Account, System Configuration, Backup) vertically on a single scrollable page without tab navigation.

#### Scenario: All sections visible on page load

- **WHEN** user navigates to the settings page
- **THEN** all four sections (API Keys, Account, System Configuration, Backup) SHALL be visible as vertically stacked cards without requiring tab switching

### Requirement: API Keys section wrapped in Card

The API Keys section SHALL be wrapped in a Card component with a CardHeader containing the title "API Keys", consistent with other sections' visual style.

#### Scenario: API Keys section displays with Card wrapper

- **WHEN** the settings page renders
- **THEN** the API Keys section SHALL appear inside a Card with a "API Keys" CardTitle header, matching the visual pattern of System Configuration, Account, and Backup sections

### Requirement: Tab navigation removed

The settings page SHALL NOT use Tabs, TabsList, TabsTrigger, or TabsContent components.

#### Scenario: No tab UI elements present

- **WHEN** user views the settings page
- **THEN** there SHALL be no tab navigation bar or tab switching controls

### Requirement: Section ordering

Sections SHALL be ordered top-to-bottom as: API Keys, Account, System Configuration, Backup.

#### Scenario: Section display order

- **WHEN** user scrolls through the settings page
- **THEN** sections SHALL appear in order: API Keys first, then Account, then System Configuration, then Backup

### Requirement: All existing functionality preserved

All existing CRUD operations, form submissions, dialogs, animations, and interactions within each section SHALL continue to function identically.

#### Scenario: API Key CRUD operations work

- **WHEN** user creates, edits, or deletes an API key
- **THEN** the operation SHALL complete successfully with appropriate toast notifications

#### Scenario: Account changes work

- **WHEN** user updates username or password
- **THEN** the operation SHALL complete successfully with appropriate toast notifications

#### Scenario: Export and import work

- **WHEN** user exports or imports data
- **THEN** the operation SHALL complete successfully as before
