## ADDED Requirements

### Requirement: Request parameters summary display

The system SHALL parse the full request JSON body and extract non-messages configuration parameters. When the Conversation view is active, the system SHALL display a collapsible summary area above the messages list showing the following parameters (when present and non-null): `model`, `stream`, `temperature`, `max_tokens` / `max_completion_tokens`, `top_p`, `frequency_penalty`, `presence_penalty`, `response_format`, `seed`, `stop`, `n`, `user`.

#### Scenario: Request with standard parameters

- **WHEN** the request JSON contains `temperature`, `max_tokens`, and `stream` fields
- **THEN** the system SHALL display each parameter as a labeled key-value pair in a grid layout above the messages list

#### Scenario: Request with missing or null parameters

- **WHEN** a parameter field is absent or null in the request JSON
- **THEN** the system SHALL NOT render that parameter in the summary area

#### Scenario: Request with response_format

- **WHEN** the request JSON contains a `response_format` field (e.g., `{"type": "json_object"}`)
- **THEN** the system SHALL display the response_format type as a badge

#### Scenario: Non-parseable request content

- **WHEN** the request content cannot be parsed as JSON or does not contain recognizable fields
- **THEN** the system SHALL NOT render the parameters summary area and SHALL NOT throw errors
