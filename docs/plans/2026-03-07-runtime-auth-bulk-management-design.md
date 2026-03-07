# Runtime Auth File Bulk Management Design

## Goal

Improve runtime-managed auth-file and quota UX by:

- fixing pagination so page changes take effect immediately
- adding scalable auth-file management for larger account sets
- supporting cross-page selection for bulk auth-file actions

## Current State

- Runtime auth-file and quota sections already support backend pagination.
- The frontend detail panel stores page state, but refresh and invalidation still use older query-key patterns in some places.
- Auth-file operations are single-item oriented: single toggle, single delete, upload, OAuth import.
- Large auth-file sets are cumbersome to manage because there is no search, page-size control, or bulk action flow.

## Problems To Solve

### 1. Pagination interaction feels broken

The current implementation can require a second click before the next page visibly takes effect. The likely cause is stale cache invalidation and page-state coupling around paginated query keys.

### 2. Large auth-file sets are hard to operate

Users with many auth files need a faster way to:

- find files by name or email
- change enabled/disabled state in bulk
- remove multiple files at once
- operate across multiple pages of filtered results

## Recommended Approach

Keep the current detail-panel layout, but upgrade the auth-file section into a lightweight bulk-management surface.

This approach avoids a large page redesign while still making large account sets practical to manage.

## Design Details

### 1. Stable pagination state

Both auth-file and quota sections will use fully page-aware query keys and refresh logic.

The frontend should:

- always invalidate the paginated query family, not just a page-less base key assumption
- avoid page resets unless the current page becomes invalid after deletion or filtering
- update button disabled states based on loading and boundary conditions

This should make next/previous page clicks work on the first interaction.

### 2. Auth-file search and page size

The auth-file section will add:

- text search by file name or email
- page-size selector

Search and page-size changes will:

- reset auth-file pagination back to page 1
- clear any active selection state

Quota keeps stable pagination but does not gain bulk-management controls in this phase.

### 3. Selection model

Auth-file selection supports:

- selecting individual rows
- selecting the current page
- escalating to select all results across pages for the current search/filter scope

Cross-page selection semantics:

- first action selects the current page only
- then the UI offers “select all N matching items”
- once enabled, the selection applies to all current search results, not just the visible page
- changing search text, provider scope, or page size clears the global selection

This mirrors familiar admin-dashboard bulk action behavior while reducing accidental mass changes.

### 4. Bulk actions

Auth-file bulk actions in this phase:

- bulk enable
- bulk disable
- bulk delete

These actions should work for:

- explicit row selections on the current page
- cross-page “all matching results” selection

The UI will show:

- selected item count
- whether the selection is page-only or all matching results
- an explicit clear-selection action

### 5. Backend support

The current backend already supports:

- list with search + pagination
- single-item status patch
- delete by name or delete all

To support efficient bulk management cleanly, add backend batch endpoints or batch request shapes for:

- status updates by explicit names or filtered-all scope
- deletion by explicit names or filtered-all scope

The filtered-all scope should use the same search/provider filter semantics as listing.

## UI Structure

### Auth-file toolbar

Toolbar contents:

- search input
- page-size selector
- refresh
- sync keys/models
- upload
- OAuth import

### Selection banner

Shown when selection exists:

- “Selected X items on this page” or “Selected all Y matching items”
- action buttons for enable / disable / delete
- clear selection
- cross-page select-all callout when relevant

### Auth-file list rows

Each row gains:

- checkbox
- existing provider/name/email metadata
- keep single-row actions available for convenience

## Error Handling

- bulk operations should return per-item failures if possible, or at minimum a summary count
- selection should clear only when the action succeeds decisively
- page state should clamp safely if bulk delete empties the current page

## Testing Strategy

- frontend tests for pagination state transitions and selection behavior
- frontend tests for cross-page select-all semantics
- handler tests for new batch auth-file operations if backend endpoints are added
- verification that search + page-size changes clear selection and reset page 1

## Out of Scope

- quota bulk operations
- cross-provider combined selection
- export/reporting workflows
- separate full-screen auth-file management page
