## ADDED Requirements

### Requirement: Response metadata display

The system SHALL parse response metadata fields from the response JSON body. When the ResponseBlock is rendered, the system SHALL display a metadata area below the content/tool_calls section showing: `id`, `model`, `created` (formatted as human-readable timestamp), and `system_fingerprint` (when present).

#### Scenario: Response with full metadata

- **WHEN** the response JSON contains `id`, `model`, `created`, and `system_fingerprint` fields
- **THEN** the system SHALL display all four fields as labeled key-value pairs in a compact row

#### Scenario: Response with partial metadata

- **WHEN** some metadata fields are absent (e.g., `system_fingerprint` is missing)
- **THEN** the system SHALL only display the fields that are present

#### Scenario: Non-JSON or streaming-only response

- **WHEN** the response content is plain text (accumulated streaming) or cannot be parsed as JSON
- **THEN** the system SHALL NOT render the metadata area

### Requirement: Usage details display

The system SHALL parse the `usage` object from the response JSON body. When usage data is present, the system SHALL display a usage summary in the ResponseBlock showing `prompt_tokens`, `completion_tokens`, and `total_tokens`, along with detail breakdowns when available.

#### Scenario: Standard usage data

- **WHEN** the response JSON contains `usage` with `prompt_tokens`, `completion_tokens`, and `total_tokens`
- **THEN** the system SHALL display these three values in a compact inline format

#### Scenario: Usage with token details

- **WHEN** `usage.prompt_tokens_details` contains `cached_tokens` or `usage.completion_tokens_details` contains `reasoning_tokens`
- **THEN** the system SHALL display these sub-values as secondary annotations next to their parent token count (e.g., "1234 (cached: 500)")

#### Scenario: Usage data absent

- **WHEN** the response JSON does not contain a `usage` object
- **THEN** the system SHALL NOT render the usage area

### Requirement: Multiple choices display

The system SHALL parse all choices from the `choices` array in the response JSON, not only `choices[0]`. When multiple choices exist (n > 1), each choice SHALL be rendered as a separate ResponseBlock with its choice index displayed.

#### Scenario: Single choice response

- **WHEN** the response contains exactly one choice in `choices`
- **THEN** the system SHALL render the response as currently (single ResponseBlock, no index indicator)

#### Scenario: Multiple choices response

- **WHEN** the response contains multiple choices in `choices` (n > 1)
- **THEN** the system SHALL render a separate ResponseBlock for each choice, with a "Choice #N" label
