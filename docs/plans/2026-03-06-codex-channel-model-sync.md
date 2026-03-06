# Codex Channel Model Sync Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make Codex channels stop showing `noModels` after auth files are uploaded or imported by syncing auth-derived models back into the channel record.

**Architecture:** Reuse the embedded Codex runtime management endpoint to fetch models for each enabled Codex auth file, merge them into a unique ordered channel model list, and persist that list back to `channels.model` and `channels.fetched_model`. Trigger this sync after local auth mutations and invalidate the `channels` query on the frontend so the card UI updates immediately.

**Tech Stack:** Go worker, Gin, Bun DAL, React, TanStack Query, Vitest

---

### Task 1: Backend Aggregation Test

**Files:**

- Modify: `apps/worker/internal/handler/codex_test.go`
- Modify: `apps/worker/internal/handler/codex.go`

**Step 1: Write the failing test**

- Mock the Codex management `/auth-files/models` endpoint.
- Feed two enabled auth files into the aggregation helper.
- Assert the merged model list is unique and stable.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/handler -run TestCollectCodexChannelModels`
Expected: FAIL because the helper does not exist yet.

**Step 3: Write minimal implementation**

- Add a context-based management call helper.
- Add model aggregation helper.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/handler -run TestCollectCodexChannelModels`
Expected: PASS.

### Task 2: Persist Channel Models After Auth Mutations

**Files:**

- Modify: `apps/worker/internal/handler/codex.go`

**Step 1: Implement channel model persistence**

- Add a helper that lists enabled auth files, fetches auth models, and updates `channels.model` + `channels.fetched_model`.
- Call it after upload, OAuth import, delete, disable toggle, and sync keys.
- Keep auth mutations successful even if model sync fails; log the sync error instead of rolling back auth persistence.

**Step 2: Verify worker tests**

Run: `go test ./internal/handler`
Expected: PASS.

### Task 3: Frontend Channel Refresh

**Files:**

- Modify: `apps/web/src/pages/model/codex-query-keys.ts`
- Modify: `apps/web/src/pages/model/codex-query-keys.test.ts`
- Modify: `apps/web/src/pages/model/codex-channel-detail.tsx`
- Modify: `apps/web/src/pages/model/channel-dialog.tsx`

**Step 1: Write/update the failing test**

- Assert upload-triggered refresh keys include `channels` in addition to auth files and quota.

**Step 2: Run test to verify it fails**

Run: `pnpm --filter @wheel/web exec vitest run src/pages/model/codex-query-keys.test.ts`
Expected: FAIL because `channels` is missing.

**Step 3: Write minimal implementation**

- Extend shared refresh keys to include `channels`.
- Reuse those keys from both Codex upload entry points.

**Step 4: Run test to verify it passes**

Run: `pnpm --filter @wheel/web exec vitest run src/pages/model/codex-query-keys.test.ts`
Expected: PASS.

### Task 4: Final Verification

**Files:**

- Verify only

**Step 1: Run handler tests**

Run: `go test ./internal/handler`
Expected: PASS.

**Step 2: Run focused frontend test**

Run: `pnpm --filter @wheel/web exec vitest run src/pages/model/codex-query-keys.test.ts`
Expected: PASS.

**Step 3: Run web build**

Run: `pnpm --filter @wheel/web build`
Expected: PASS.
