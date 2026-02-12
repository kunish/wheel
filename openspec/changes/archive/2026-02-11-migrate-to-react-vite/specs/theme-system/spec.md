## ADDED Requirements

### Requirement: Theme provider compatibility

The system SHALL use `next-themes` (or an equivalent) to provide dark/light/system theme toggling via the `class` strategy on the `<html>` element, maintaining full compatibility with the existing shadcn/ui and Tailwind CSS v4 dark mode setup.

#### Scenario: Theme provider wraps the application

- **WHEN** the App component renders
- **THEN** a ThemeProvider SHALL wrap all content with `attribute="class"`, `defaultTheme="system"`, `enableSystem`, and `disableTransitionOnChange` props

#### Scenario: Dark mode class is applied

- **WHEN** the user selects dark mode
- **THEN** the `dark` class SHALL be added to the `<html>` element, activating all `.dark` CSS variables

#### Scenario: System preference is respected

- **WHEN** the theme is set to "system" and the OS prefers dark mode
- **THEN** the `dark` class SHALL be applied automatically

### Requirement: Theme toggle functionality

The system SHALL provide a theme toggle button in the top bar that switches between light and dark modes, using the `useTheme` hook.

#### Scenario: Toggle button switches theme

- **WHEN** the user clicks the theme toggle button
- **THEN** the theme SHALL switch between "light" and "dark"

#### Scenario: Theme persists across sessions

- **WHEN** the user selects a theme and reloads the page
- **THEN** the selected theme SHALL be restored from localStorage

### Requirement: No flash of incorrect theme

The system SHALL prevent a flash of the wrong theme on page load by applying the theme class before React hydrates/renders.

#### Scenario: Script in HTML head sets theme early

- **WHEN** the page loads
- **THEN** a blocking inline script in `index.html` SHALL read the theme from localStorage and apply the `dark` class to `<html>` before the body renders, preventing a flash of light mode when dark mode is selected

### Requirement: CSS variables unchanged

The existing CSS custom properties for light and dark themes (defined in `globals.css`) SHALL remain identical — no changes to color values, variable names, or the neobrutalism design system.

#### Scenario: Light theme variables are preserved

- **WHEN** the light theme is active
- **THEN** all CSS variables (e.g., `--background: #fffdf7`, `--foreground: #1a1a1a`, `--nb-lime: #c8f547`) SHALL match the current values exactly

#### Scenario: Dark theme variables are preserved

- **WHEN** the dark theme is active
- **THEN** all `.dark` CSS variables (e.g., `--background: #151515`, `--foreground: #f5f3ee`) SHALL match the current values exactly
