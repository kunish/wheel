# OpenAI Anthropic Inference Test Matrix Design

**Goal:** Add a systematic test matrix that proves OpenAI and Anthropic inference interfaces work correctly in `wheel`, including request translation, response translation, streaming conversion, and protocol-specific edge cases.

## Current State

- `apps/worker/internal/relay/adapter_test.go` already covers parts of OpenAI -> Anthropic request conversion and some non-streaming translation helpers.
- `apps/worker/internal/relay/proxy.go` contains the core SSE conversion logic for Anthropic -> OpenAI and OpenAI -> Anthropic streaming, but test coverage is still uneven and implementation-oriented.
- `apps/worker/internal/handler/routes_integration_test.go` now covers OpenAI-compatible `/v1/chat/completions` and `/v1/responses` at the route layer, but it does not yet provide a protocol-matrix view of OpenAI and Anthropic inference interoperability.
- Existing tests are useful but distributed; they do not yet make it easy to answer "which protocol pair and edge case combinations are guaranteed to work?"

## Scope

### In Scope

- OpenAI inbound inference endpoints:
  - `/v1/chat/completions`
  - `/v1/responses`
- Anthropic inbound inference endpoint:
  - `/v1/messages`
- OpenAI -> Anthropic request translation
- Anthropic -> OpenAI response translation
- OpenAI -> Anthropic streaming conversion
- Anthropic -> OpenAI streaming conversion
- Edge-case coverage for:
  - tool calls / tool_use / tool_result
  - thinking / reasoning content
  - usage / token accounting
  - stop reason / finish reason mapping
  - protocol-correct error envelopes

### Out of Scope

- Images, audio, embeddings, moderations, and other non-inference resources
- Realtime APIs
- Assistant / file / vector store style platform resources
- Benchmarking, load testing, or network-failure chaos testing

## Recommended Approach

- Organize tests as a **protocol matrix**, but implement them at the most appropriate layer rather than forcing everything into one giant integration suite.
- Use three layers:
  - `adapter_test.go` for non-streaming request/response translation rules
  - relay streaming tests around `proxy.go` for SSE conversion semantics
  - `routes_integration_test.go` for true handler-level entrypoint behavior

This gives us one conceptual coverage matrix with low-level precision and end-to-end confidence.

## Coverage Matrix

### 1. OpenAI Inbound -> Anthropic Outbound

#### Non-streaming

- `/v1/chat/completions` with:
  - system message extraction to Anthropic `system`
  - user/assistant messages
  - tool calls
  - tool result messages
  - temperature / top_p / stop / max_tokens mapping
- `/v1/responses` with:
  - basic input translation
  - currently supported reasoning/tool semantics
  - explicit tests for any intentional degradation or ignored fields

#### Streaming

- OpenAI SSE input expectations converted to Anthropic SSE output shape:
  - `message_start`
  - `content_block_start`
  - `content_block_delta`
  - `message_delta`
  - `message_stop`
- Tool call deltas and text deltas stay ordered and distinguishable.
- Usage and thinking deltas are reflected in the Anthropic-compatible stream state.

### 2. Anthropic Inbound -> OpenAI Outbound

#### Non-streaming

- `/v1/messages` translated into OpenAI-compatible success payloads:
  - content blocks -> message content
  - tool_use / tool_result -> tool_calls / tool messages
  - stop_reason -> finish_reason
  - usage -> prompt/completion/total token fields

#### Streaming

- Anthropic SSE converted into OpenAI chunks with:
  - assistant role start
  - text deltas
  - tool call deltas
  - final `[DONE]`
- Thinking deltas are preserved according to current `wheel` OpenAI stream semantics.

### 3. Protocol-Specific Errors

- OpenAI inbound errors always use OpenAI envelope.
- Anthropic inbound errors always use Anthropic envelope.
- Cross-protocol relay failures do not leak the wrong response format.

## File Organization

### `apps/worker/internal/relay/adapter_test.go`

Add or reorganize tests for:

- OpenAI chat -> Anthropic request conversion matrix
- OpenAI responses -> Anthropic request conversion matrix
- Anthropic response -> OpenAI response conversion
- OpenAI response -> Anthropic response conversion
- Stop reason / usage / tool call mapping

### `apps/worker/internal/relay/*stream*_test.go` or existing proxy-focused test file

Add streaming matrix tests for:

- OpenAI SSE -> Anthropic SSE
- Anthropic SSE -> OpenAI SSE
- Tool delta and thinking delta behavior
- Final done-event behavior

If practical, keep SSE fixtures table-driven and assert event sequence, not only first/last line.

### `apps/worker/internal/handler/routes_integration_test.go`

Add route-level interoperability tests for:

- OpenAI inbound `/v1/chat/completions` to Anthropic outbound
- OpenAI inbound `/v1/responses` to Anthropic outbound
- Anthropic inbound `/v1/messages` to OpenAI outbound
- At least one streaming route-level case in each direction if test scaffolding can support it without excessive brittleness

## Testing Principles

- Avoid giant ad-hoc JSON blobs repeated in every test.
- Prefer protocol fixtures/builders shared across a matrix.
- Assert protocol semantics, not only `200` status:
  - exact field names
  - event order
  - usage field mapping
  - stop reason mapping
  - tool-call shape
- For unsupported or intentionally degraded fields, write tests that lock the current behavior explicitly.

## Success Criteria

- A reader can tell exactly which OpenAI/Anthropic inference combinations are covered.
- Non-streaming translation rules are locked with precise field assertions.
- Streaming event conversion is covered with sequence-aware tests.
- Route-level tests prove the public handler layer preserves protocol correctness.
- Current known limitations are explicit in tests rather than silently untested.

## Non-Goals

- No attempt to add new inference functionality in this pass.
- No protocol redesign; this effort is about validation and regression protection.
- No expansion into non-inference OpenAI resources.
