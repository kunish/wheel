# Codex Quota Bulk Import Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make Codex quota cards populate much faster after bulk auth-file imports by removing serial quota fetching from the backend quota endpoint.

**Architecture:** Keep the existing frontend query flow and `GET /api/v1/channel/:id/codex/quota` response shape intact. Change `ListCodexQuota` in the worker so it fetches quota for the current page with bounded concurrency, preserves item order, and keeps per-item error reporting. Verify the new behavior with handler tests, then run the worker test suite.

**Tech Stack:** Go, Gin, Bun, sqlmock, standard library concurrency primitives

---

### Task 1: Lock In The Current Quota Aggregation Contract

**Files:**

- Modify: `apps/worker/internal/handler/codex_test.go`
- Modify: `apps/worker/internal/handler/codex.go`

**Step 1: Write the failing test**

Add a handler-level test in `apps/worker/internal/handler/codex_test.go` that:

- builds a Codex channel request for `GET /api/v1/channel/:id/codex/quota`
- seeds multiple auth files for the same channel
- injects quota fetch behavior that completes out of order
- asserts the JSON response still returns items in the original auth-file order

The new test should validate these expectations:

```go
if len(resp.Data.Items) != 3 {
	t.Fatalf("len(items) = %d, want 3", len(resp.Data.Items))
}
if got := []string{resp.Data.Items[0].Name, resp.Data.Items[1].Name, resp.Data.Items[2].Name}; !reflect.DeepEqual(got, []string{"a.json", "b.json", "c.json"}) {
	t.Fatalf("order = %v, want original order", got)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/handler -run TestListCodexQuota_PreservesOrderWithConcurrentFetches`

Expected: FAIL because the quota path does not yet support the injected concurrent execution model and test seam.

**Step 3: Write minimal implementation support for the test**

In `apps/worker/internal/handler/codex.go`, add the smallest test seam needed for quota fetching so the test can drive out-of-order completions without changing the HTTP contract. Keep this seam private to the handler package.

Example shape:

```go
type codexQuotaFetcher func(...) (codexQuotaWindow, codexQuotaWindow, string, error)
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/handler -run TestListCodexQuota_PreservesOrderWithConcurrentFetches`

Expected: PASS.

**Step 5: Commit**

```bash
git add apps/worker/internal/handler/codex.go apps/worker/internal/handler/codex_test.go
git commit -m "test: lock Codex quota ordering behavior"
```

### Task 2: Replace Serial Quota Fetching With Bounded Concurrency

**Files:**

- Modify: `apps/worker/internal/handler/codex.go`
- Modify: `apps/worker/internal/handler/codex_test.go`

**Step 1: Write the failing test**

Add a second test in `apps/worker/internal/handler/codex_test.go` that proves quota fetching is no longer fully serial. Use a blocking fetch stub that records active workers and only completes after multiple requests are in flight.

The test should fail if concurrency never exceeds 1.

Example assertion:

```go
if maxConcurrent.Load() < 2 {
	t.Fatalf("max concurrent fetches = %d, want at least 2", maxConcurrent.Load())
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/handler -run TestListCodexQuota_UsesBoundedConcurrency`

Expected: FAIL because the current implementation fetches quota in a plain serial loop.

**Step 3: Write minimal implementation**

Update `ListCodexQuota` in `apps/worker/internal/handler/codex.go` to:

- preallocate the `items` slice to the paged auth-file length
- launch quota work with a fixed concurrency limit
- preserve index-based writes so response order stays stable
- keep existing missing-`authIndex`, missing-account-id, and per-item fetch error behavior
- avoid changing the endpoint response JSON

Suggested structure:

```go
items := make([]codexQuotaItem, len(paged))
sem := make(chan struct{}, quotaFetchConcurrency)
var wg sync.WaitGroup

for i, file := range paged {
	wg.Add(1)
	go func(i int, file codexAuthFile) {
		defer wg.Done()
		sem <- struct{}{}
		defer func() { <-sem }()
		items[i] = buildQuotaItem(...)
	}(i, file)
}

wg.Wait()
```

Use a small constant for concurrency, such as 4 or 6.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/handler -run 'TestListCodexQuota_(PreservesOrderWithConcurrentFetches|UsesBoundedConcurrency)$'`

Expected: PASS.

**Step 5: Commit**

```bash
git add apps/worker/internal/handler/codex.go apps/worker/internal/handler/codex_test.go
git commit -m "perf: parallelize Codex quota fetching"
```

### Task 3: Regression Coverage For Existing Upload Flow

**Files:**

- Modify: `apps/worker/internal/handler/codex_test.go`

**Step 1: Extend or add a focused regression test**

Add or update a test that exercises the quota listing path after a multi-auth-file upload setup and verifies mixed success/error items still survive the concurrent collector.

Minimum assertions:

```go
if resp.Data.Items[1].Error == "" {
	t.Fatalf("expected per-item error to be preserved")
}
if resp.Data.Items[0].PlanType == "" {
	t.Fatalf("expected successful item quota data to be preserved")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/handler -run TestListCodexQuota_PreservesPerItemErrors`

Expected: FAIL until the concurrent aggregation path preserves both success and error item shapes correctly.

**Step 3: Write minimal implementation**

Adjust the concurrent item builder only if needed so it fills both successful quota fields and per-item error fields exactly like the old serial path.

**Step 4: Run focused tests to verify they pass**

Run: `go test ./internal/handler -run 'TestListCodexQuota_(PreservesOrderWithConcurrentFetches|UsesBoundedConcurrency|PreservesPerItemErrors)$'`

Expected: PASS.

**Step 5: Commit**

```bash
git add apps/worker/internal/handler/codex_test.go apps/worker/internal/handler/codex.go
git commit -m "test: cover concurrent Codex quota error handling"
```

### Task 4: Final Verification

**Files:**

- Verify only

**Step 1: Run focused handler tests**

Run: `go test ./internal/handler -run 'Test(ListCodexQuota|UploadCodexAuthFile)'`

Expected: PASS.

**Step 2: Run full worker test suite**

Run: `go test ./...`

Expected: PASS.

**Step 3: Inspect final diff**

Run: `git diff -- apps/worker/internal/handler/codex.go apps/worker/internal/handler/codex_test.go`

Expected: only bounded-concurrency quota changes and matching tests.
