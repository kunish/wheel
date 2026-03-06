# Codex Quota Bulk Import Design

**Goal:** Make Codex quota cards appear much sooner after bulk auth-file imports without changing the frontend API contract.

**Problem:** After large auth-file imports, the quota section stays empty or loading for too long because `ListCodexQuota` fetches quota data for the current page serially. Each account waits for the previous account's upstream quota request to finish.

**Recommendation:** Keep the existing endpoint and UI contract, but change backend quota collection from serial execution to bounded concurrency.

## Approaches Considered

### 1. Bounded backend concurrency (recommended)

- Keep `GET /api/v1/channel/:id/codex/quota` unchanged.
- For the paged auth files, fetch quota in parallel with a small fixed concurrency limit.
- Preserve response ordering so the frontend keeps rendering quota cards in the same order.
- Keep per-item error handling so one bad auth file does not fail the whole page.

**Why this wins:** It reduces wall-clock latency the most for the least code change and does not force frontend refactors.

### 2. Frontend progressive quota rendering

- Return auth files first, then fetch quota per account from the browser.
- Show cards immediately and fill in quota progressively.

**Why not now:** Better UX, but it requires new API shape or many per-account requests and spreads complexity across both frontend and backend.

### 3. Background quota cache warming

- Warm and cache quota immediately after import.
- Serve quota endpoint from fresh or stale cache.

**Why not now:** Strong long-term option, but it introduces cache invalidation, freshness rules, and more operational complexity than needed for this issue.

## Design

### Backend

- Update `apps/worker/internal/handler/codex.go` so `ListCodexQuota` processes the paged auth files with bounded concurrency instead of a plain `for` loop.
- Use a fixed worker limit (for example 4 or 6) to avoid creating one upstream request per auth file without control.
- Keep the current quota fetch helpers (`fetchLocalCodexQuota` and `fetchCodexQuota`) unchanged unless a small signature adjustment improves reuse.

### Ordering And Error Handling

- Allocate the result slice up front with the same length as the paged auth-file slice.
- Each worker writes its computed `codexQuotaItem` back to the original index.
- If a file is missing `authIndex` or `accountID`, keep current per-item error behavior.
- If a quota request fails, only that item gets an `error` field; the endpoint still returns the rest of the page.

### Frontend Impact

- No API contract changes.
- Existing invalidation in `apps/web/src/pages/model/codex-channel-detail.tsx` remains sufficient.
- The visible improvement comes from the quota endpoint responding sooner after import.

## Testing

- Add a handler test that proves quota items keep their original order even when quota fetches complete out of order.
- Add a regression test that would be noticeably slow or blocked under serial behavior and passes with bounded concurrency.
- Run `go test ./...` in `apps/worker`.

## Non-Goals

- No frontend state-management redesign.
- No persistent quota cache.
- No API schema changes.
