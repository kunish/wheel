# Codex Bulk Auth Upload Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow one Codex upload action to import multiple auth JSON files with partial-success handling and immediate UI refresh.

**Architecture:** Reuse the existing Codex auth upload endpoint and extend it to accept repeated multipart file parts. Process files independently on the backend, return a batch result payload, and update the frontend upload helpers to send many files and present one summarized result.

**Tech Stack:** React, TypeScript, TanStack Query, Gin, Go, Bun, Vitest

---

### Task 1: Add backend regression test for batch upload

**Files:**

- Modify: `apps/worker/internal/handler/codex_test.go`

**Step 1: Write the failing test**

- Add a handler test that posts multipart data with multiple auth files.
- Include at least:
  - one valid `.json`
  - one invalid `.json`
  - one duplicate filename or malformed file
- Assert response includes `total`, `successCount`, `failedCount`, and `results[]`.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/handler -run TestUploadCodexAuthFileBatch`

Expected: FAIL because the handler currently only accepts one `file`.

**Step 3: Write minimal implementation**

- None yet.

**Step 4: Run test to confirm red state is correct**

Run: `go test ./internal/handler -run TestUploadCodexAuthFileBatch`

Expected: FAIL for the expected reason.

**Step 5: Commit**

Do not commit yet unless requested.

### Task 2: Implement backend batch parsing and result aggregation

**Files:**

- Modify: `apps/worker/internal/handler/codex.go`

**Step 1: Update handler input parsing**

- Extend `UploadCodexAuthFile` to read repeated multipart `files` entries.
- Support existing `file` field as fallback.
- Reject requests with no files.

**Step 2: Implement per-file processing helper**

- Extract single-file local upload logic into a helper that returns a structured result.
- Return status and error instead of writing HTTP responses mid-loop.

**Step 3: Aggregate batch response**

- Process all files.
- Count successes and failures.
- Return one JSON response with batch metadata and per-file results.

**Step 4: Sync channel models once per successful batch**

- If `successCount > 0`, run `bestEffortSyncCodexChannelModels` once after processing.

**Step 5: Run focused tests**

Run: `go test ./internal/handler -run TestUploadCodexAuthFileBatch`

Expected: PASS.

### Task 3: Preserve single-file compatibility with tests

**Files:**

- Modify: `apps/worker/internal/handler/codex_test.go`

**Step 1: Add single-file compatibility assertion**

- Add or update a test to ensure a request with only `file` still succeeds.

**Step 2: Run focused tests**

Run: `go test ./internal/handler -run TestUploadCodexAuthFile`

Expected: PASS.

**Step 3: Run package tests**

Run: `go test ./internal/handler`

Expected: PASS.

### Task 4: Update frontend API helper for multi-file upload

**Files:**

- Modify: `apps/web/src/lib/api/codex.ts`

**Step 1: Write the failing test or extractable helper if useful**

- If practical, extract request-building logic into a small helper and test repeated `files` appends.
- If not practical, proceed with implementation and verify through integration-style build checks.

**Step 2: Implement multi-file request payload**

- Change upload helper to accept `File[]`.
- Append each file to `FormData` under `files`.
- Normalize server response type to batch result payload.

**Step 3: Verify TypeScript compiles**

Run: `pnpm --filter @wheel/web build`

Expected: initial failures in callers that still pass a single file.

### Task 5: Update Codex detail upload UI to support multi-select

**Files:**

- Modify: `apps/web/src/pages/model/codex-channel-detail.tsx`

**Step 1: Update file input and handlers**

- Add `multiple` to the hidden file input.
- Convert `FileList` to array.
- Filter or fail fast on non-JSON files before sending.

**Step 2: Update mutation success messaging**

- Show one summary toast using returned counts.
- Keep existing query invalidation of auth files, quota, and channels.

**Step 3: Run targeted checks**

Run: `pnpm --filter @wheel/web exec vitest run src/pages/model/codex-query-keys.test.ts`

Expected: PASS.

### Task 6: Update channel dialog upload UI to support multi-select

**Files:**

- Modify: `apps/web/src/pages/model/channel-dialog.tsx`

**Step 1: Update file input and handlers**

- Add `multiple`.
- Pass selected files as an array to the API helper.

**Step 2: Match summary behavior**

- Reuse the same success/error summary style as `codex-channel-detail.tsx`.
- Keep existing query invalidation behavior.

**Step 3: Verify compilation**

Run: `pnpm --filter @wheel/web build`

Expected: PASS.

### Task 7: Run full verification

**Files:**

- No code changes expected

**Step 1: Run backend test suite**

Run: `go test ./...`

Expected: PASS.

**Step 2: Run frontend test/build verification**

Run: `pnpm --filter @wheel/web exec vitest run src/pages/model/codex-query-keys.test.ts && pnpm --filter @wheel/web build`

Expected: PASS.

**Step 3: Manual verification**

- Open the Codex channel UI.
- Select multiple auth files in one action.
- Confirm mixed batches show summarized results.
- Confirm auth list, quota, and channel models refresh after successful uploads.

**Step 4: Commit**

Do not commit yet unless requested.
