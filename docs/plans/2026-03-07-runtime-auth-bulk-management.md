# Runtime Auth File Bulk Management Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix runtime auth-file/quota pagination so page changes take effect immediately, and add practical bulk auth-file management including cross-page selection, search, page-size controls, and bulk enable/disable/delete.

**Architecture:** Keep the current runtime detail panel and evolve the auth-file section into a lightweight admin surface. Use page-aware query keys and selection state in the frontend, and add minimal batch-capable handler/API support for auth-file updates and deletions by explicit names or filtered-all scope.

**Tech Stack:** React, TypeScript, TanStack Query, Go Gin handlers, Bun DAL, existing runtime auth-file management routes

---

### Task 1: Lock down pagination behavior and query-key correctness

**Files:**

- Modify: `apps/web/src/pages/model/codex-query-keys.ts`
- Modify: `apps/web/src/pages/model/codex-query-keys.test.ts`
- Modify: `apps/web/src/pages/model/codex-channel-detail.tsx`

**Step 1: Write the failing test**

- Add tests that prove paginated auth-file/quota queries are invalidated and refreshed using the same page-aware key shape used by `useQuery`.
- Add a UI-level test if available that captures the “next page needs two clicks” regression.

**Step 2: Run test to verify it fails**

Run: `pnpm --filter web test codex-query-keys`

Expected: FAIL due to stale invalidation assumptions or missing page-aware behavior coverage.

**Step 3: Write minimal implementation**

- Make refresh/invalidation logic target the full paginated query family.
- Keep page state stable across refreshes.

**Step 4: Re-run test to verify it passes**

Run: `pnpm --filter web test codex-query-keys`

Expected: PASS.

### Task 2: Add auth-file search and page-size controls

**Files:**

- Modify: `apps/web/src/pages/model/codex-channel-detail.tsx`
- Modify: `apps/web/src/lib/api/codex.ts`
- Modify: `apps/web/src/i18n/locales/en/model.json`
- Modify: `apps/web/src/i18n/locales/zh-CN/model.json`
- Test: nearby model-page tests or new tests as appropriate

**Step 1: Write the failing test**

- Add tests for auth-file search parameter propagation.
- Add tests for page-size changes resetting to page 1.
- Add tests that changing search clears bulk selection state.

**Step 2: Run test to verify it fails**

Run: `pnpm --filter web test channel`

Expected: FAIL because search/page-size controls do not exist yet.

**Step 3: Write minimal implementation**

- Add auth-file search input.
- Add page-size selector.
- Reset auth page to 1 when either changes.
- Keep quota pagination unchanged except for stable query behavior.

**Step 4: Re-run test to verify it passes**

Run: `pnpm --filter web test channel`

Expected: PASS.

### Task 3: Add backend batch auth-file operations

**Files:**

- Modify: `apps/worker/internal/handler/codex.go`
- Modify: `apps/worker/internal/handler/routes.go`
- Modify: `apps/worker/internal/handler/codex_test.go`
- Modify: `apps/web/src/lib/api/codex.ts`

**Step 1: Write the failing test**

- Add handler tests for batch status changes by explicit auth-file names.
- Add handler tests for batch deletion by explicit auth-file names.
- Add handler tests for filtered-all scope (search + provider filter) if you choose a dedicated request shape for it.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/handler -run 'Test(Batch|ListCodexAuthFiles|DeleteCodexAuthFile|PatchCodexAuthFileStatus)'`

Expected: FAIL because batch request handling does not exist yet.

**Step 3: Write minimal implementation**

- Add minimal batch-capable request handling for enable/disable and delete.
- Support either explicit `names` or filtered-all scope using the same search/provider semantics as listing.
- Return a summary payload suitable for UI feedback.

**Step 4: Re-run test to verify it passes**

Run: `go test ./internal/handler -run 'Test(Batch|ListCodexAuthFiles|DeleteCodexAuthFile|PatchCodexAuthFileStatus)'`

Expected: PASS.

### Task 4: Add frontend bulk-selection and cross-page select-all flow

**Files:**

- Modify: `apps/web/src/pages/model/codex-channel-detail.tsx`
- Possibly create: `apps/web/src/pages/model/runtime-auth-selection.ts` if selection logic needs extraction
- Modify: `apps/web/src/i18n/locales/en/model.json`
- Modify: `apps/web/src/i18n/locales/zh-CN/model.json`
- Test: model-page tests covering selection flow

**Step 1: Write the failing test**

- Add tests for:
  - select current page
  - escalate to select all matching items across pages
  - clearing selection on search/page-size change
  - bulk toolbar visibility and counts

**Step 2: Run test to verify it fails**

Run: `pnpm --filter web test channel`

Expected: FAIL because selection model and cross-page flow do not exist yet.

**Step 3: Write minimal implementation**

- Add row checkboxes and header checkbox.
- Add page-selection vs all-matching selection state.
- Show a selection banner with counts, clear action, and “select all matching” CTA.

**Step 4: Re-run test to verify it passes**

Run: `pnpm --filter web test channel`

Expected: PASS.

### Task 5: Wire bulk enable/disable/delete actions into the UI

**Files:**

- Modify: `apps/web/src/pages/model/codex-channel-detail.tsx`
- Modify: `apps/web/src/lib/api/codex.ts`
- Test: model-page tests and/or API helper tests

**Step 1: Write the failing test**

- Add tests that bulk enable/disable/delete actions send the right payloads for:
  - explicit page selection
  - all-matching selection

**Step 2: Run test to verify it fails**

Run: `pnpm --filter web test codex`

Expected: FAIL because the new batch API helpers and selection-aware action wiring are not in place.

**Step 3: Write minimal implementation**

- Add batch API helpers.
- Wire bulk action buttons to current selection semantics.
- Refresh relevant query families after success.
- Clamp current page after bulk deletion if the page becomes empty.

**Step 4: Re-run test to verify it passes**

Run: `pnpm --filter web test codex`

Expected: PASS.

### Task 6: Final verification

**Files:**

- No code changes expected

**Step 1: Run targeted web tests**

Run: `pnpm --filter web test codex`

Expected: PASS.

**Step 2: Run broader model-page tests**

Run: `pnpm --filter web test channel`

Expected: PASS.

**Step 3: Run worker handler tests for runtime auth-file management**

Run: `go test ./internal/handler -run 'Test(Batch|ListCodexAuthFiles|DeleteCodexAuthFile|PatchCodexAuthFileStatus|ListCodexQuota)'`

Expected: PASS.

**Step 4: Run build verification**

Run: `pnpm --filter web build`

Expected: PASS.

**Step 5: Commit**

Do not commit yet unless requested.
