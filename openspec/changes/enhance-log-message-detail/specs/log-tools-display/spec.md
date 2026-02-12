## ADDED Requirements

### Requirement: Tools definition list display

The system SHALL parse the `tools` array from the request JSON body. When tools are present, the system SHALL display a collapsible "Tools" section between the request parameters summary and the messages list. Each tool SHALL be rendered as a collapsible card showing the function name and description, with the full JSON Schema parameters available on expand.

#### Scenario: Request with tools array

- **WHEN** the request JSON contains a non-empty `tools` array
- **THEN** the system SHALL render a "Tools" section header with the count of tools, followed by a collapsible card for each tool showing `function.name` and `function.description`

#### Scenario: Tool card expanded

- **WHEN** a user expands a tool card
- **THEN** the system SHALL display the `function.parameters` JSON Schema in a formatted code block

#### Scenario: Request without tools

- **WHEN** the request JSON does not contain a `tools` field or the array is empty
- **THEN** the system SHALL NOT render the Tools section

### Requirement: Tool choice display

The system SHALL parse the `tool_choice` field from the request JSON body. When present, the system SHALL display the tool_choice setting alongside the tools section header.

#### Scenario: tool_choice is a string value

- **WHEN** `tool_choice` is `"auto"`, `"required"`, or `"none"`
- **THEN** the system SHALL display it as a badge next to the Tools section header

#### Scenario: tool_choice is a specific tool object

- **WHEN** `tool_choice` is an object like `{"type": "function", "function": {"name": "my_func"}}`
- **THEN** the system SHALL display the function name as the badge text

#### Scenario: tool_choice is absent

- **WHEN** the request JSON does not contain a `tool_choice` field
- **THEN** the system SHALL NOT display any tool_choice indicator
