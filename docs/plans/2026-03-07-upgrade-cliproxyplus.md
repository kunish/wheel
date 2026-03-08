# Upgrade CLIProxyAPIPlus Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Upgrade the vendored `CLIProxyAPIPlus` runtime to the latest upstream version and keep Wheel's worker runtime integrations working, including exposing newly added models such as `gpt-5.4`.

**Architecture:** Treat the vendored runtime source tree as the real dependency and refresh it from upstream first. Then repair only the Wheel-side integration breakages introduced by the upgrade and verify the runtime model listing path still works end-to-end.

**Tech Stack:** Go modules, vendored `CLIProxyAPIPlus`, Wheel worker handlers/tests, runtime auth/model management paths

---

### Task 1: Capture current runtime expectations before upgrade

**Files:**

- Modify: `apps/worker/internal/handler/codex_test.go`
- Test: `apps/worker/internal/handler/codex_test.go`

**Step 1: Write the failing or lock-in test**

- Add or tighten a test that validates runtime model sync/listing behavior around auth-file models.
- Prefer a test that makes it easy to detect whether a newly introduced upstream model (for example `gpt-5.4`) is surfaced through the current handler path.

**Step 2: Run test to verify current behavior**

Run: `go test ./internal/handler -run 'Test(GetCodexAuthFileModels|ListCodexQuota|SyncCodex)'`

Expected: PASS on current baseline or reveal the exact missing expectation you want to lock.

**Step 3: Adjust the test to reflect intended post-upgrade behavior**

- Keep the test narrowly scoped to runtime model-list exposure, not broad runtime refactors.

**Step 4: Re-run test**

Run: `go test ./internal/handler -run 'Test(GetCodexAuthFileModels|ListCodexQuota|SyncCodex)'`

Expected: PASS if it is a lock-in baseline, or FAIL in a controlled way if it encodes the missing new model expectation.

### Task 2: Upgrade vendored CLIProxyAPIPlus to latest upstream

**Files:**

- Modify: `apps/worker/internal/runtime/**`
- Modify: `apps/worker/go.mod`
- Modify: `apps/worker/go.sum`

**Step 1: Refresh vendored source from upstream**

- Replace or sync `apps/worker/internal/runtime` to the latest upstream source.
- Keep module path and local `replace` workflow intact.

**Step 2: Align module metadata**

- Update `apps/worker/go.mod` required version if needed to match the new vendored upstream version.
- Refresh `go.sum` only as needed.

**Step 3: Run compile-focused verification**

Run: `go test ./internal/handler -run 'Test(GetCodexAuthFileModels|ListCodexQuota|SyncCodex)'`

Expected: likely FAIL if integration drift exists, with actionable compile/runtime errors.

### Task 3: Repair Wheel integration breakages caused by the upgrade

**Files:**

- Modify: `apps/worker/internal/handler/codex.go`
- Modify: any directly impacted Wheel worker files only if required by the upgrade
- Test: `apps/worker/internal/handler/codex_test.go`

**Step 1: Fix the smallest breakage first**

- Address compile errors or changed upstream behavior in Wheel's handler integration.
- Avoid refactoring unrelated runtime code.

**Step 2: Re-run focused tests after each fix**

Run: `go test ./internal/handler -run 'Test(GetCodexAuthFileModels|ListCodexQuota|SyncCodex)'`

Expected: fewer failures after each minimal fix.

**Step 3: Verify upgraded model exposure**

- Confirm the runtime model-list path now surfaces `gpt-5.4` if upstream includes it.

**Step 4: Re-run focused tests until green**

Run: `go test ./internal/handler -run 'Test(GetCodexAuthFileModels|ListCodexQuota|SyncCodex)'`

Expected: PASS.

### Task 4: Broader worker verification

**Files:**

- No code changes expected unless verification reveals an upgrade regression

**Step 1: Run handler package tests**

Run: `go test ./internal/handler`

Expected: PASS.

**Step 2: Run wider worker verification**

Run: `go test ./...`

Expected: PASS, or reveal targeted regressions to fix.

**Step 3: Inspect vendored model definitions for the new model**

- Verify the upgraded vendored source actually contains `gpt-5.4` or equivalent latest model entries.

### Task 5: Final verification and wrap-up

**Files:**

- No code changes expected

**Step 1: Run final targeted proof command**

Run: `go test ./internal/handler -run 'Test(GetCodexAuthFileModels|ListCodexQuota|SyncCodex)'`

Expected: PASS.

**Step 2: Run full worker verification again if Task 4 required fixes**

Run: `go test ./...`

Expected: PASS.

**Step 3: Summarize upgrade outcome**

- Record the new upstream version/tag or commit source.
- Note any Wheel-side compatibility fixes that were necessary.

**Step 4: Commit**

Do not commit yet unless requested.
