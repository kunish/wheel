# Codex Copilot Runtime Pagination and Form Adaptation Design

## Goal

Improve the runtime-managed Codex and Copilot channel experience in the model UI by:

- adding pagination for auth-file and quota displays
- making channel creation/editing use a dedicated runtime-oriented form flow
- clearly separating Codex and Copilot wording and behavior

## Current State

- The backend already supports pagination for auth files and quota endpoints.
- The frontend detail panel currently reads only the default page and renders the full section without paging controls.
- The channel dialog already has a basic runtime branch for types `33` and `34`, but it still behaves mostly like a generic API-key channel form.
- Codex and Copilot currently reuse the same visual flow and most of the same copy.

## Design Summary

### 1. Paginated runtime detail sections

The runtime detail panel will maintain two independent pagination states:

- auth files pagination
- quota pagination

Each section will:

- keep its own page and page size
- include pagination state in the React Query key
- show total count and current range/page information
- provide previous/next navigation
- preserve the current page after refresh or mutations when possible
- move back one page if the current page becomes empty after deletion

This keeps auth-file and quota navigation independent and avoids cross-section refetch confusion.

### 2. Dedicated runtime form mode

The channel dialog will treat Codex and Copilot as a dedicated runtime form mode instead of a minor variant of the generic channel form.

In runtime mode:

- the dialog will show a runtime-specific section header and explanatory text
- the API key field will be replaced by runtime auth actions
- model fetch behavior will be runtime-aware rather than generic key/base-url preview behavior
- form copy will distinguish Codex from Copilot

The generic provider form remains unchanged for non-runtime channel types.

### 3. Stronger Codex vs Copilot differentiation

Runtime copy and presentation will be provider-specific:

- Codex uses Codex wording
- Copilot uses Copilot wording

This applies to:

- section title
- helper text
- import button labels
- empty state hints
- model management guidance

The underlying auth-file and OAuth flows continue using the existing shared API layer with `channelType`-based routing.

## UI Structure

### Runtime form block

The runtime form block will include:

- provider-specific intro copy
- runtime auth actions (`OAuth`, `Upload JSON`)
- a note that credentials are managed via auth files instead of a pasted API key
- model guidance that explains runtime sync behavior

The base URL field should only remain visible if it still affects runtime channels in practice. If not required, it should be hidden or replaced with a read-only explanation so the form no longer resembles a standard OpenAI-compatible channel setup.

### Runtime detail pagination controls

Each paginated section will display:

- section title
- current item count / total count summary
- previous/next controls
- disabled states when at the bounds or loading

The controls should stay compact and fit the current card layout.

## Data Flow

### Auth files

- UI stores `authPage` and `authPageSize`
- `listCodexAuthFiles()` is called with those values
- mutations invalidate paged query keys
- after delete/toggle/upload/sync, the current page is refetched

### Quota

- UI stores `quotaPage` and `quotaPageSize`
- `listCodexQuota()` is called with those values
- refresh keeps the current page

### Runtime channel creation

- selecting type `33` or `34` switches the dialog into runtime mode
- creating a brand-new runtime channel still uses the existing save-first flow where needed
- once an auth file or OAuth import succeeds, the dialog rehydrates models from the saved channel

## Testing Strategy

- extend frontend tests for runtime API helpers/query key behavior if needed
- add dialog/detail tests covering pagination parameters and runtime-specific UI branches
- run targeted web tests for the modified model-page modules

## Out of Scope

- redesigning backend pagination semantics
- changing the runtime auth storage model
- introducing a completely separate route set for Codex vs Copilot UI
- large-scale visual redesign of the model page
