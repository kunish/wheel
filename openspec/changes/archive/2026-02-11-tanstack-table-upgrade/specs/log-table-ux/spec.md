## MODIFIED Requirements

### Requirement: Client-side column sorting

The Latency, Cost, Input Tokens, Output Tokens, and TTFT column headers SHALL be clickable to sort the current page's data. Clicking SHALL cycle through three states: ascending, descending, and no sort. A sort indicator (arrow icon) SHALL show the current sort state: ArrowUp for ascending, ArrowDown for descending, ArrowUpDown for unsorted.

#### Scenario: Sort by latency descending

- **WHEN** user clicks the "Latency" column header (currently unsorted)
- **THEN** the current page rows SHALL be reordered by latency ascending
- **AND** an upward arrow icon SHALL appear next to "Latency"

#### Scenario: Toggle sort direction

- **WHEN** user clicks the same column header again (currently ascending)
- **THEN** the sort direction SHALL change to descending
- **AND** the arrow icon SHALL change to downward

#### Scenario: Clear sort

- **WHEN** user clicks the same column header a third time (currently descending)
- **THEN** the sort SHALL be removed and rows SHALL return to their original order
- **AND** the arrow icon SHALL change to ArrowUpDown (unsorted indicator)

#### Scenario: Sort by TTFT

- **WHEN** user clicks the "TTFT" column header
- **THEN** the current page rows SHALL be reordered by TTFT (ftut field) ascending
- **AND** an upward arrow icon SHALL appear next to "TTFT"
- **AND** rows with TTFT value of 0 SHALL sort as lowest values
