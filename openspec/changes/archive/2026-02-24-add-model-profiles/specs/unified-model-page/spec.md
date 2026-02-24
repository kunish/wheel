## MODIFIED Requirements

### Requirement: Integrated price management toolbar

The Model page SHALL include price management actions and profile management in the top toolbar area, alongside the existing Models button.

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

#### Scenario: Profiles button in toolbar

- **WHEN** the Model page renders
- **THEN** a "Profiles" button SHALL be displayed in the toolbar
- **WHEN** the user clicks "Profiles"
- **THEN** the profile management dialog SHALL open
