# OpenAI API Compatibility Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make `wheel`'s `/v1/*` routes behave like an OpenAI-compatible API for core inference and multimedia endpoints, including stable error shapes, multipart audio uploads, and binary speech responses.

**Architecture:** Keep `apps/worker/internal/handler/relay.go` as the only public `/v1/*` gateway. Add endpoint-aware request parsing and execution paths in `wheel`, while continuing to reuse the existing relay routing, channel selection, retry, plugin, and observability infrastructure.

**Tech Stack:** Go, Gin, Bun, net/http, multipart form handling, existing `wheel` relay infrastructure

---

### Task 1: Add failing route and contract tests for OpenAI compatibility

**Files:**

- Create: `apps/worker/internal/handler/relay_openai_contract_test.go`
- Modify: `apps/worker/internal/handler/relay_routes_test.go`

**Step 1: Write the failing test**

- Add route assertions for the existing OpenAI-style endpoints if any are missing from the contract test coverage.
- Add contract tests that hit lightweight relay endpoints and assert:
  - OpenAI error envelope shape for invalid requests
  - `/v1/models` still returns `object: "list"` and `data[]`

**Step 2: Run test to verify it fails**

Run: `go test ./internal/handler -run 'TestRegisterRelayRoutes_NoWildcardConflicts|TestOpenAIContract'`

Expected: FAIL because the current handler does not yet fully enforce the desired OpenAI response contract.

**Step 3: Write minimal implementation**

- None yet.

**Step 4: Run test again to confirm the red state**

Run: `go test ./internal/handler -run 'TestRegisterRelayRoutes_NoWildcardConflicts|TestOpenAIContract'`

Expected: FAIL for the expected reason.

**Step 5: Commit**

Do not commit yet unless requested.

### Task 2: Add failing parser tests for JSON and multipart OpenAI requests

**Files:**

- Modify: `apps/worker/internal/relay/parser_test.go`
- Create: `apps/worker/internal/relay/multimodal_test.go`
- Create: `apps/worker/internal/handler/relay_request_test.go`

**Step 1: Write the failing test**

- Extend request-type and model extraction coverage in `parser_test.go`.
- Add focused tests for multimodal defaults and request classification in `multimodal_test.go`.
- Add handler-level parsing tests in `relay_request_test.go` for:
  - JSON `audio/speech`
  - multipart `audio/transcriptions`
  - multipart `audio/translations`
  - missing model/file validation

**Step 2: Run test to verify it fails**

Run: `go test ./internal/relay ./internal/handler -run 'TestDetectRequestType|TestExtractModel|TestExtractMultimodalModel|TestParseRelayRequest'`

Expected: FAIL because request parsing is currently JSON-only and does not support multipart audio uploads.

**Step 3: Write minimal implementation**

- None yet.

**Step 4: Re-run focused tests**

Run: `go test ./internal/relay ./internal/handler -run 'TestDetectRequestType|TestExtractModel|TestExtractMultimodalModel|TestParseRelayRequest'`

Expected: FAIL for the intended parser gap.

**Step 5: Commit**

Do not commit yet unless requested.

### Task 3: Implement endpoint-aware request parsing in the handler layer

**Files:**

- Create: `apps/worker/internal/handler/relay_request.go`
- Modify: `apps/worker/internal/handler/relay.go`
- Modify: `apps/worker/internal/relay/parser.go`
- Modify: `apps/worker/internal/relay/multimodal.go`

**Step 1: Extract request parsing out of `relay.go`**

- Move parsing logic into a dedicated helper file so JSON and multipart branches are easier to maintain.
- Keep the public control flow in `handleRelay` stable.

**Step 2: Implement JSON request normalization**

- Preserve current JSON handling for `chat/completions`, `responses`, `embeddings`, `moderations`, and image generation.
- Normalize stream and model extraction into one shared path.

**Step 3: Implement multipart request normalization**

- Support multipart file parsing for `audio/transcriptions` and `audio/translations`.
- Preserve enough metadata for downstream upstream request building and logging.

**Step 4: Enforce OpenAI-style validation failures**

- Return OpenAI error envelopes for unsupported content type, missing file, missing model, and malformed JSON/form data.

**Step 5: Run focused tests**

Run: `go test ./internal/relay ./internal/handler -run 'TestDetectRequestType|TestExtractModel|TestExtractMultimodalModel|TestParseRelayRequest'`

Expected: PASS.

### Task 4: Implement endpoint-aware execution for JSON, multipart, and binary responses

**Files:**

- Modify: `apps/worker/internal/handler/relay_strategy.go`
- Modify: `apps/worker/internal/handler/relay_retry.go`
- Modify: `apps/worker/internal/relay/multimodal.go`
- Modify: `apps/worker/internal/handler/relay_background.go`

**Step 1: Add multimodal execution branching**

- Route image/moderation JSON requests through the multimodal helper path where appropriate.
- Route `audio/speech` through a binary response path.

**Step 2: Add multipart upstream request support**

- Extend multimodal request building so transcription/translation requests can be proxied as multipart, not only JSON.

**Step 3: Preserve response-writing semantics**

- Keep JSON endpoints writing OpenAI-compatible JSON.
- Keep binary speech responses streaming bytes plus relevant headers (`Content-Type`, `Content-Length`, `Content-Disposition`).

**Step 4: Make background execution consistent**

- Ensure async/batch helpers explicitly reject unsupported multimedia paths or correctly reuse the new execution path if they are meant to support them.

**Step 5: Run focused tests**

Run: `go test ./internal/handler -run 'TestOpenAIContract|TestRoutes'`

Expected: PASS for the covered contract/integration cases.

### Task 5: Tighten OpenAI response and error formatting

**Files:**

- Modify: `apps/worker/internal/handler/relay_log.go`
- Modify: `apps/worker/internal/handler/relay.go`
- Modify: `apps/worker/internal/handler/relay_openai_contract_test.go`

**Step 1: Expand the error envelope helper**

- Add stable OpenAI-style fields such as `param` and `code` when available or explicitly set them to `null`/omitted in a consistent way.

**Step 2: Preserve useful OpenAI-compatible headers**

- Surface request IDs and safe rate-limit headers where available.

**Step 3: Stabilize `/v1/models` response shape**

- Keep the current aggregation approach but ensure the payload remains SDK-friendly and deterministic.

**Step 4: Run focused tests**

Run: `go test ./internal/handler -run TestOpenAIContract`

Expected: PASS.

### Task 6: Add end-to-end integration coverage for multimedia and inference endpoints

**Files:**

- Modify: `apps/worker/internal/handler/routes_integration_test.go`
- Modify: `apps/worker/internal/handler/async_batch_flow_test.go`

**Step 1: Add inference integration cases**

- Cover `chat/completions`, `responses`, and `embeddings` against test upstream servers.

**Step 2: Add multimedia integration cases**

- Cover `images/generations`, `audio/transcriptions`, `audio/translations`, and `audio/speech`.
- Assert multipart upload behavior and binary passthrough behavior.

**Step 3: Guard custom wheel APIs**

- Confirm `/v1/async/chat/completions` and `/v1/batch` still behave as before after the parser and strategy refactor.

**Step 4: Run package tests**

Run: `go test ./internal/handler`

Expected: PASS.

### Task 7: Run final verification

**Files:**

- No code changes expected

**Step 1: Run relay package tests**

Run: `go test ./internal/relay`

Expected: PASS.

**Step 2: Run handler package tests**

Run: `go test ./internal/handler`

Expected: PASS.

**Step 3: Run full worker verification**

Run: `go test ./...`

Expected: PASS.

**Step 4: Manual verification**

- Use an OpenAI-compatible client or curl against `wheel` for:
  - `GET /v1/models`
  - `POST /v1/chat/completions`
  - `POST /v1/responses`
  - `POST /v1/embeddings`
  - `POST /v1/images/generations`
  - `POST /v1/audio/transcriptions`
  - `POST /v1/audio/speech`
- Confirm errors are OpenAI-shaped and audio speech returns binary content.

**Step 5: Commit**

Do not commit yet unless requested.
