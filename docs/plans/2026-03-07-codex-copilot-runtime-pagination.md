# Codex Copilot Runtime Pagination Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add pagination for Codex/Copilot auth-file and quota displays, and introduce a dedicated runtime-oriented channel form flow that distinctly adapts Codex and Copilot creation/editing.

**Architecture:** Keep the existing backend pagination contract and move the missing state management into the web UI. Refactor the channel dialog so runtime-managed providers use a dedicated branch with provider-specific copy, reduced generic-channel affordances, and shared runtime auth actions.

**Tech Stack:** React, TypeScript, TanStack Query, existing web model-page components, existing worker pagination endpoints

---

### Task 1: Add query-key and API support for paged runtime sections

**Files:**

- Modify: `apps/web/src/pages/model/codex-query-keys.ts`
- Modify: `apps/web/src/lib/api/codex.ts`
- Test: `apps/web/src/lib/api/codex.test.ts`

**Step 1: Write the failing test**

- Add tests that assert auth-file and quota query keys include pagination inputs.
- Add tests that assert `listCodexAuthFiles()` and `listCodexQuota()` preserve `page`, `pageSize`, `search`, and `channelType` in the generated request URL.

**Step 2: Run test to verify it fails**

Run: `pnpm --filter web test codex`

Expected: FAIL on missing paginated query-key or request-shape coverage.

**Step 3: Write minimal implementation**

- Extend query-key helpers to accept pagination/search inputs.
- Keep the existing API helpers, but make sure tests lock the exact URL contract already expected by the backend.

**Step 4: Re-run test to verify it passes**

Run: `pnpm --filter web test codex`

Expected: PASS.

### Task 2: Implement auth-file and quota pagination in runtime detail panel

**Files:**

- Modify: `apps/web/src/pages/model/codex-channel-detail.tsx`
- Test: `apps/web/src/pages/model/codex-channel-draft.test.ts`
- Test: any nearby model-page test file if a new detail-panel test is more appropriate

**Step 1: Write the failing test**

- Add UI-level coverage for:
  - auth-files section requesting the selected page
  - quota section requesting the selected page
  - previous/next controls updating only their own section state
  - current-page preservation after refresh

**Step 2: Run test to verify it fails**

Run: `pnpm --filter web test channel`

Expected: FAIL because pagination controls and page-aware queries do not exist yet.

**Step 3: Write minimal implementation**

- Add separate `authPage/authPageSize` and `quotaPage/quotaPageSize` state.
- Include those values in the detail-panel queries.
- Render compact paging controls and summary text for each section.
- After delete, if the last row on a non-first page disappears, decrement the page before refetching.

**Step 4: Re-run test to verify it passes**

Run: `pnpm --filter web test channel`

Expected: PASS.

### Task 3: Refactor runtime channel dialog into a dedicated provider-aware mode

**Files:**

- Modify: `apps/web/src/pages/model/channel-dialog.tsx`
- Modify: `apps/web/src/pages/model/codex-channel-draft.ts`
- Test: `apps/web/src/pages/model/codex-channel-draft.test.ts`

**Step 1: Write the failing test**

- Add coverage for runtime channel types (`33`, `34`) that asserts:
  - runtime mode renders provider-specific copy
  - generic API-key input is not shown
  - runtime auth actions remain available
  - runtime-specific model guidance is shown

**Step 2: Run test to verify it fails**

Run: `pnpm --filter web test channel-dialog`

Expected: FAIL because runtime mode is still only a shallow conditional branch.

**Step 3: Write minimal implementation**

- Extract runtime-provider helpers (`isRuntimeChannelType`, provider display metadata, copy selection).
- Replace the generic key section with a provider-aware runtime section.
- Remove or weaken generic-channel affordances that do not make sense for runtime-managed channels.
- Preserve the existing save-first flow for OAuth/file import.

**Step 4: Re-run test to verify it passes**

Run: `pnpm --filter web test channel-dialog`

Expected: PASS.

### Task 4: Tighten runtime model-management behavior in the dialog

**Files:**

- Modify: `apps/web/src/pages/model/channel-dialog.tsx`
- Modify: `apps/web/src/pages/model/codex-channel-draft.ts`
- Test: `apps/web/src/pages/model/codex-channel-draft.test.ts`

**Step 1: Write the failing test**

- Add coverage for runtime channels that locks the intended model behavior:
  - imported/synced runtime models merge into form state
  - runtime guidance explains sync-driven model management
  - generic fetch-model preview is hidden or disabled in runtime mode if it no longer applies

**Step 2: Run test to verify it fails**

Run: `pnpm --filter web test codex-channel-draft`

Expected: FAIL because runtime model management is still mixed with generic fetch behavior.

**Step 3: Write minimal implementation**

- Keep model hydration through the existing post-import refresh path.
- Make the model section runtime-aware so the user understands when models are synced vs manually edited.
- Avoid introducing a second source of truth for runtime-managed models.

**Step 4: Re-run test to verify it passes**

Run: `pnpm --filter web test codex-channel-draft`

Expected: PASS.

### Task 5: Final verification

**Files:**

- No code changes expected

**Step 1: Run targeted web tests**

Run: `pnpm --filter web test codex`

Expected: PASS.

**Step 2: Run model-page tests**

Run: `pnpm --filter web test channel`

Expected: PASS.

**Step 3: Run build verification**

Run: `pnpm --filter web build`

Expected: PASS.

**Step 4: Manual verification**

- Open the model page.
- Create a Codex channel and verify the dedicated runtime form mode.
- Create a Copilot channel and verify provider-specific copy.
- Verify auth-file and quota sections page independently.

**Step 5: Commit**

Do not commit yet unless requested.
