## ADDED Requirements

### Requirement: Popover-based time range picker

The system SHALL provide a TimeRangePicker component that combines quick presets and custom datetime range selection in a single Popover UI.

#### Scenario: Popover opens on trigger click

- **WHEN** user clicks the time range trigger button
- **THEN** a popover opens showing preset buttons and custom range inputs

#### Scenario: Popover closes on outside click

- **WHEN** popover is open and user clicks outside the popover
- **THEN** the popover closes without applying changes

### Requirement: Quick preset selection

The system SHALL support preset time range buttons: 1h, 6h, 24h, 7d, 30d. Clicking a preset SHALL set `from` to `now - preset.seconds` and `to` to `now` (both as unix seconds), then close the popover immediately.

#### Scenario: User selects 24h preset

- **WHEN** user clicks the "24h" preset button
- **THEN** `from` is set to current time minus 86400 seconds, `to` is set to current time, and the popover closes

#### Scenario: User selects 30d preset

- **WHEN** user clicks the "30d" preset button
- **THEN** `from` is set to current time minus 2592000 seconds, `to` is set to current time, and the popover closes

### Requirement: Custom datetime range selection

The system SHALL provide two `datetime-local` inputs (From and To) for selecting an arbitrary time range. The range SHALL only be applied when the user clicks an explicit "Apply" button.

#### Scenario: User enters custom range and applies

- **WHEN** user enters a start datetime in the "From" input, an end datetime in the "To" input, and clicks "Apply"
- **THEN** `from` and `to` are set to the corresponding unix second timestamps and the popover closes

#### Scenario: User enters only a start datetime

- **WHEN** user enters a start datetime but leaves "To" empty, then clicks "Apply"
- **THEN** `from` is set to the entered timestamp, `to` remains undefined, and the popover closes

#### Scenario: Custom inputs initialize from current filter state

- **WHEN** the popover opens and `from`/`to` are already set (e.g., from a preset or deep link)
- **THEN** the datetime-local inputs SHALL be pre-filled with the corresponding datetime values

### Requirement: Range summary display

The trigger button SHALL display a human-readable summary of the active time range. When no range is selected, it SHALL show placeholder text "Time Range" with a calendar icon.

#### Scenario: Preset range active

- **WHEN** the active range matches a known preset (duration equals preset seconds and `to` is within 60s of current time)
- **THEN** the trigger displays "Last {preset.label}" (e.g., "Last 24h")

#### Scenario: Custom range active

- **WHEN** both `from` and `to` are set and do not match any preset
- **THEN** the trigger displays "{from formatted} – {to formatted}" in compact MM/DD HH:mm format

#### Scenario: No range selected

- **WHEN** neither `from` nor `to` is set
- **THEN** the trigger displays "Time Range" as placeholder text

### Requirement: Clear time range

The system SHALL provide a way to clear the active time range from within the picker or via the trigger button.

#### Scenario: User clears time range via clear button

- **WHEN** a time range is active and user clicks the clear (×) button on the trigger
- **THEN** both `from` and `to` are cleared and the trigger reverts to placeholder text

### Requirement: Component interface

The TimeRangePicker component SHALL accept `from` and `to` props (unix seconds or undefined) and an `onChange(from: number | undefined, to: number | undefined)` callback. It SHALL NOT manage URL state internally.

#### Scenario: Parent controls state via props

- **WHEN** parent passes updated `from`/`to` props (e.g., from URL search params)
- **THEN** the component reflects the new values in both the trigger summary and the popover inputs

### Requirement: Popover UI primitive

The system SHALL add a Popover component to the UI component library at `components/ui/popover.tsx`, based on Radix UI Popover, following the existing shadcn/ui pattern.

#### Scenario: Popover primitive is available

- **WHEN** a developer imports `Popover, PopoverTrigger, PopoverContent` from `@/components/ui/popover`
- **THEN** the imports resolve and the components render a functional popover
