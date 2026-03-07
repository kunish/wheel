# OpenAI Anthropic Inference Test Matrix Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a protocol-matrix test suite that verifies OpenAI and Anthropic inference interfaces, including request/response conversion, streaming conversion, and key edge-case semantics.

**Architecture:** Keep one conceptual test matrix, but place each test at the narrowest layer that can validate it well: translation rules in relay adapter tests, SSE conversion in relay streaming tests, and end-to-end contract checks in handler integration tests. Reuse fixtures and helpers so the matrix is explicit without duplicating large payloads.

**Tech Stack:** Go, Gin, net/http/httptest, existing relay conversion helpers, SSE event parsing helpers

---

### Task 1: Add failing non-streaming OpenAI/Anthropic translation matrix tests

**Files:**

- Modify: `apps/worker/internal/relay/adapter_test.go`

**Step 1: Write the failing test**

- Add table-driven tests for non-streaming translation gaps that are not currently locked down, including at least:
  - OpenAI `chat/completions` -> Anthropic request with system + tools + tool result
  - OpenAI `responses` -> Anthropic request body shape
  - Anthropic response -> OpenAI response mapping of usage and finish reason
  - OpenAI response -> Anthropic response mapping of usage and stop reason

**Step 2: Run test to verify it fails**

Run: `go test ./internal/relay -run 'Test(BuildAnthropicRequest|ConvertAnthropicResponse|ConvertToAnthropicResponse)'`

Expected: FAIL on at least one newly added matrix case.

**Step 3: Write minimal implementation**

- None yet.

**Step 4: Re-run to confirm red state**

Run: `go test ./internal/relay -run 'Test(BuildAnthropicRequest|ConvertAnthropicResponse|ConvertToAnthropicResponse)'`

Expected: FAIL for the intended translation gap.

**Step 5: Commit**

Do not commit yet unless requested.

### Task 2: Implement minimal non-streaming translation fixes

**Files:**

- Modify: `apps/worker/internal/relay/adapter.go`
- Modify: `apps/worker/internal/relay/adapter_test.go`

**Step 1: Implement minimal fixes for failing translation cases**

- Update request/response conversion helpers only where the new tests prove behavior is missing or wrong.

**Step 2: Run focused relay tests**

Run: `go test ./internal/relay -run 'Test(BuildAnthropicRequest|ConvertAnthropicResponse|ConvertToAnthropicResponse)'`

Expected: PASS.

**Step 3: Keep behavior explicit**

- For any unsupported field, add assertions that lock the current degradation path rather than leaving it implicit.

### Task 3: Add failing streaming conversion matrix tests

**Files:**

- Create or Modify: `apps/worker/internal/relay/proxy_stream_test.go`

**Step 1: Write the failing test**

- Add sequence-aware SSE tests for:
  - OpenAI SSE -> Anthropic SSE
  - Anthropic SSE -> OpenAI SSE
  - tool delta behavior
  - thinking/reasoning delta behavior
  - final done-event behavior

**Step 2: Run test to verify it fails**

Run: `go test ./internal/relay -run 'Test(Stream|SSE|Anthropic|OpenAI)'`

Expected: FAIL on at least one new event-sequence case.

**Step 3: Write minimal implementation**

- None yet.

**Step 4: Re-run to confirm red state**

Run: `go test ./internal/relay -run 'Test(Stream|SSE|Anthropic|OpenAI)'`

Expected: FAIL for the intended streaming gap.

### Task 4: Implement minimal streaming conversion fixes

**Files:**

- Modify: `apps/worker/internal/relay/proxy.go`
- Modify: `apps/worker/internal/relay/proxy_stream_test.go`

**Step 1: Fix only the failing SSE conversion gaps**

- Update streaming conversion logic so the new sequence-aware tests pass.

**Step 2: Run focused relay streaming tests**

Run: `go test ./internal/relay -run 'Test(Stream|SSE|Anthropic|OpenAI)'`

Expected: PASS.

**Step 3: Guard event ordering**

- Keep assertions on event order and terminal events, not just substring presence.

### Task 5: Add failing handler integration tests for protocol interoperability

**Files:**

- Modify: `apps/worker/internal/handler/routes_integration_test.go`

**Step 1: Write the failing test**

- Add route-level interoperability tests for at least:
  - OpenAI inbound `/v1/chat/completions` -> Anthropic outbound
  - OpenAI inbound `/v1/responses` -> Anthropic outbound
  - Anthropic inbound `/v1/messages` -> OpenAI outbound
- Add assertions for protocol-correct success body shape and protocol-correct error envelope shape.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/handler -run 'TestRelayRoutes_(OpenAIToAnthropic|AnthropicToOpenAI)'`

Expected: FAIL on at least one newly added interoperability case.

**Step 3: Write minimal implementation**

- None yet.

**Step 4: Re-run to confirm red state**

Run: `go test ./internal/handler -run 'TestRelayRoutes_(OpenAIToAnthropic|AnthropicToOpenAI)'`

Expected: FAIL for the intended route-layer gap.

### Task 6: Implement minimal handler or relay fixes for route-level interoperability

**Files:**

- Modify: `apps/worker/internal/handler/relay.go`
- Modify: `apps/worker/internal/handler/relay_strategy.go`
- Modify: `apps/worker/internal/handler/routes_integration_test.go`
- Modify any directly necessary relay files only if the new handler tests prove a real gap

**Step 1: Fix only the failing interoperability cases**

- Preserve existing OpenAI compatibility work; do not expand into non-inference resources.

**Step 2: Run focused handler tests**

Run: `go test ./internal/handler -run 'TestRelayRoutes_(OpenAIToAnthropic|AnthropicToOpenAI)'`

Expected: PASS.

**Step 3: Re-run existing async/batch regression coverage**

Run: `go test ./internal/handler -run 'Test(AsyncRoutes_|BatchRoutes_)'`

Expected: PASS.

### Task 7: Add explicit tests for unsupported/degraded inference semantics

**Files:**

- Modify: `apps/worker/internal/relay/adapter_test.go`
- Modify: `apps/worker/internal/relay/proxy_stream_test.go`
- Modify: `apps/worker/internal/handler/routes_integration_test.go`

**Step 1: Add lock-in tests for current limitations**

- Add tests for fields or semantics that are intentionally ignored, downgraded, or partially supported.
- Name these tests explicitly so future changes can update them intentionally.

**Step 2: Run focused tests**

Run: `go test ./internal/relay ./internal/handler -run 'Test(Unsupported|Degraded|OpenAIToAnthropic|AnthropicToOpenAI|BuildAnthropicRequest|ConvertAnthropicResponse|ConvertToAnthropicResponse|Stream|SSE)'`

Expected: PASS.

### Task 8: Run final verification

**Files:**

- No code changes expected

**Step 1: Run relay tests**

Run: `go test ./internal/relay`

Expected: PASS.

**Step 2: Run handler tests**

Run: `go test ./internal/handler`

Expected: PASS.

**Step 3: Run full worker verification**

Run: `go test ./...`

Expected: PASS.

**Step 4: Manual verification**

- Exercise `/v1/chat/completions`, `/v1/responses`, and `/v1/messages` against opposite-protocol upstream fixtures.
- Confirm non-streaming and streaming response shapes match the caller protocol.

**Step 5: Commit**

Do not commit yet unless requested.
