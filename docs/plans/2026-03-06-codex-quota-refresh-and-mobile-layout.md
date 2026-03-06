# Codex Quota Refresh And Mobile Layout Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make newly uploaded Codex auth files appear in the remaining-quota section immediately, and make the Codex detail UI usable on narrow screens.

**Architecture:** Keep the existing Codex detail page and backend quota source intact. Fix the stale quota symptom by invalidating the quota query on upload success, and adjust the existing layout containers so action buttons wrap, auth-file rows stack cleanly on small screens, and quota cards fall back to a single column below medium breakpoints.

**Tech Stack:** React, TypeScript, TanStack Query, Tailwind CSS, Vitest

---

### Task 1: Upload Refresh Regression Test

**Files:**

- Create: `apps/web/src/pages/model/codex-channel-detail.test.ts`
- Modify: `apps/web/src/pages/model/codex-channel-detail.tsx`

**Step 1: Write the failing test**

- Render `CodexChannelDetail` with mocked queries/mutations.
- Trigger upload success flow.
- Assert both `codex-auth-files` and `codex-quota` queries are invalidated.

**Step 2: Run test to verify it fails**

Run: `pnpm --filter @wheel/web test -- codex-channel-detail.test.ts`
Expected: FAIL because upload success currently only invalidates auth files.

**Step 3: Write minimal implementation**

- Add quota query invalidation to upload success.

**Step 4: Run test to verify it passes**

Run: `pnpm --filter @wheel/web test -- codex-channel-detail.test.ts`
Expected: PASS.

### Task 2: Narrow-Screen Layout Fix

**Files:**

- Modify: `apps/web/src/pages/model/codex-channel-detail.tsx`

**Step 1: Make action area wrap**

- Allow the toolbar button row to wrap onto multiple lines.
- Keep button sizing consistent with current design system.

**Step 2: Make auth-file rows stack safely**

- On small screens, let row content and actions break into multiple lines.
- Preserve compact single-line layout on larger screens.

**Step 3: Make quota cards responsive**

- Use one column by default and two columns from medium screens upward.

**Step 4: Verify visually via build**

Run: `pnpm --filter @wheel/web build`
Expected: PASS.

### Task 3: Final Verification

**Files:**

- Verify only

**Step 1: Run focused frontend test**

Run: `pnpm --filter @wheel/web test -- codex-channel-detail.test.ts`
Expected: PASS.

**Step 2: Run frontend build**

Run: `pnpm --filter @wheel/web build`
Expected: PASS.
