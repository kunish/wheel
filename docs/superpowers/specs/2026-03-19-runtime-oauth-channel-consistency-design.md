# Runtime OAuth Channel Consistency Design

**Date:** 2026-03-19
**Status:** Draft
**Scope:** Runtime/OAuth channels only: `Codex`, `Copilot`, `CodexCLI`, `Antigravity`

---

## 1. Problem

The runtime/OAuth channels currently behave like one family of features in the UI, but internally they still drift in several places:

- OAuth start/status/callback/import behavior is not guaranteed to be consistent across all four channels.
- Provider filtering and auth-file ownership can drift, causing “wrong file, wrong channel” or “imported but unusable” failures.
- Model discovery, default fallback models, and alias mapping are not guaranteed to line up with actual request-time model normalization.
- Some bugs are fixed in one path but not the mirrored path (`runtimecore/config` vs `runtime/corelib/config`).
- Regression coverage is uneven, so fixes for one runtime channel can silently break another.

The practical user-facing failures all look similar:

- a model appears in the UI but cannot actually be requested
- OAuth finishes but the channel still cannot be used
- a client-facing model name resolves differently in different layers
- one runtime channel gets a hardening fix while another equivalent channel does not

---

## 2. Goal

Make the runtime/OAuth channel family behave consistently across the full path from configuration to request execution.

For `Codex`, `Copilot`, `CodexCLI`, and `Antigravity`, this design aims to guarantee:

1. OAuth can start and complete correctly for that channel type.
2. Imported auth files are scoped to the correct provider/channel.
3. The model list shown to users matches the models that can actually be requested.
4. Client-facing model aliases resolve to the upstream runtime model actually used at execution time.
5. Equivalent config logic behaves the same in both config implementation paths.
6. Regression tests protect these guarantees across all four runtime channels.

### Non-Goals

- Do not expand this project to Kiro, Vertex, Qwen, iFlow, or other non-runtime/OAuth channels.
- Do not redesign the full runtime architecture.
- Do not rewrite the management UI beyond what is needed to keep runtime/OAuth channels consistent.
- Do not solve all duplicate code in one sweep; align behavior first, then consider structural cleanup separately.

---

## 3. Channel Set

This subproject covers only these four channel types:

- `types.OutboundCodex`
- `types.OutboundCopilot`
- `types.OutboundCodexCLI`
- `types.OutboundAntigravity`

These four already share a meaningful amount of logic:

- runtime auth-file management
- runtime OAuth start/status/callback flows
- runtime provider filtering
- runtime model discovery and fallback behavior

That makes them a coherent hardening target.

### 3.1 Normative channel contract

| Channel type          | Flow type   | OAuth provider | Canonical provider filter | Management start endpoint | Manual callback import  |
| --------------------- | ----------- | -------------- | ------------------------- | ------------------------- | ----------------------- |
| `OutboundCodex`       | redirect    | `codex`        | `codex`                   | `/codex-auth-url`         | required support        |
| `OutboundCopilot`     | device-code | `github`       | `copilot`                 | `/github-auth-url`        | not used in normal flow |
| `OutboundCodexCLI`    | redirect    | `codex`        | `codex-cli`               | `/codex-auth-url`         | required support        |
| `OutboundAntigravity` | redirect    | `antigravity`  | `antigravity`             | `/antigravity-auth-url`   | required support        |

This table is the source of truth for this subproject.

### 3.2 Auth-file ownership signal

The import/list/filter ownership signal for runtime/OAuth channels is:

1. channel type -> canonical runtime provider filter
2. imported auth file metadata/provider -> canonical runtime provider
3. managed filename stays channel-scoped, but provider ownership is determined by canonical provider metadata, not by OAuth provider alone

This matters most for `Codex` vs `CodexCLI`:

- both use OAuth provider `codex`
- but imported auth files must remain distinguishable by canonical provider metadata:
  - `Codex` -> `codex`
  - `CodexCLI` -> `codex-cli`

So “same OAuth provider” does **not** imply “same auth-file ownership scope.”

---

## 4. Failure Surfaces To Align

### 4.1 OAuth lifecycle

The following behaviors must be consistent where applicable:

- `start` endpoint returns channel-appropriate metadata
- `status` endpoint reports a structured, truthful phase
- `callback` handling validates provider/state/session ownership correctly
- completed OAuth imports the right auth material into the right channel scope
- missing session / superseded session / expired session degrade predictably

Device-code and redirect flows can differ in UI shape, but their state semantics should still be consistent.

### 4.1.1 Required status and error contract

Worker-facing runtime OAuth `status` polling must use this contract:

- transport `status`: `waiting` | `ok` | `error` | `expired`
- `phase`: `starting` | `awaiting_browser` | `awaiting_callback` | `callback_received` | `importing_auth_file` | `completed` | `failed` | `expired`
- `code`: structured terminal or recovery code
- `error`: human-readable terminal error only

Required failure mapping:

| Condition                           | Expected transport status | Expected phase      | Required code        |
| ----------------------------------- | ------------------------- | ------------------- | -------------------- |
| missing session on poll             | `expired`                 | `expired`           | `session_missing`    |
| superseded session                  | `expired`                 | `expired`           | `session_superseded` |
| session expired by TTL              | `expired`                 | `expired`           | `session_expired`    |
| callback state mismatch             | callback error payload    | `awaiting_callback` | `state_mismatch`     |
| callback provider mismatch          | callback error payload    | `awaiting_callback` | `provider_mismatch`  |
| import failed after successful auth | `error`                   | `failed`            | `auth_import_failed` |

Only Copilot uses device-code flow in this channel family. Codex, CodexCLI, and Antigravity are redirect flows.

Copilot device-code terminal mapping:

| Condition                          | Expected transport status | Expected phase     | Required code          |
| ---------------------------------- | ------------------------- | ------------------ | ---------------------- |
| user denied device auth            | `error`                   | `failed`           | `access_denied`        |
| device code expired                | `error`                   | `failed`           | `device_code_expired`  |
| device code rejected/invalid       | `error`                   | `failed`           | `device_code_rejected` |
| polling exhausted without success  | `expired`                 | `expired`          | `session_expired`      |
| transient provider polling failure | `waiting`                 | `awaiting_browser` | no terminal code       |

Start-time error mapping:

| Condition                                        | Expected transport status | Expected phase | Required code                 |
| ------------------------------------------------ | ------------------------- | -------------- | ----------------------------- |
| runtime management misconfigured                 | `error`                   | `failed`       | `runtime_not_configured`      |
| unsupported runtime channel                      | `error`                   | `failed`       | `unsupported_runtime_channel` |
| provider start endpoint unavailable              | `error`                   | `failed`       | `provider_unavailable`        |
| conflicting active import scope blocks new start | `error`                   | `failed`       | `runtime_session_conflict`    |

Redirect-flow `callback` responses use a **separate callback-only contract** from polling/status transport:

```json
{
  "status": "error",
  "phase": "awaiting_callback",
  "code": "state_mismatch",
  "error": "This callback belongs to a different login attempt.",
  "shouldContinuePolling": false
}
```

Rules:

- callback validation failures stay on the callback step with `phase=awaiting_callback`
- callback acceptance returns `status=accepted` or `status=duplicate` with `shouldContinuePolling=true`
- post-auth import failure returns transport `status=error`, `phase=failed`, `code=auth_import_failed`
- polling/status endpoints continue to use only `waiting | ok | error | expired`

Duplicate callback semantics:

- a callback is `duplicate` when the same active session has already accepted and persisted a callback for the same `state`
- duplicate callback handling is non-fatal
- duplicate callback handling must not trigger a second import or reassign auth material to another channel
- after a duplicate callback response, subsequent polling continues from the session's current phase

### 4.1.2 Per-endpoint behavior table

| Endpoint               | Codex                             | Copilot                                                         | CodexCLI                          | Antigravity                       |
| ---------------------- | --------------------------------- | --------------------------------------------------------------- | --------------------------------- | --------------------------------- |
| `start`                | redirect payload with `url/state` | device-code payload with `url/state/user_code/verification_uri` | redirect payload with `url/state` | redirect payload with `url/state` |
| `status` while pending | `waiting + awaiting_callback`     | `waiting + awaiting_browser`                                    | `waiting + awaiting_callback`     | `waiting + awaiting_callback`     |
| `callback` endpoint    | used                              | not part of normal flow                                         | used                              | used                              |
| post-auth import       | required                          | required                                                        | required                          | required                          |
| missing session        | `expired + session_missing`       | `expired + session_missing`                                     | `expired + session_missing`       | `expired + session_missing`       |

This table is normative for planning and tests.

### 4.1.3 Normative API payloads

These are the minimum worker-facing contracts for this subproject.

`start` response:

| Field                          | Required     | Notes                                                         |
| ------------------------------ | ------------ | ------------------------------------------------------------- |
| `url`                          | yes          | auth URL or verification URL entry point                      |
| `state`                        | yes          | authoritative runtime session identifier                      |
| `flowType`                     | yes          | `redirect` or `device_code`                                   |
| `user_code`                    | Copilot only | required for device-code flow                                 |
| `verification_uri`             | Copilot only | required for device-code flow                                 |
| `supportsManualCallbackImport` | yes          | `false` for Copilot normal flow, `true` for redirect channels |
| `expiresAt`                    | yes          | authoritative worker expiry                                   |

`status` response:

| Field                          | Required | Notes                                  |
| ------------------------------ | -------- | -------------------------------------- |
| `status`                       | yes      | `waiting`, `ok`, `error`, or `expired` |
| `phase`                        | yes      | worker phase                           |
| `code`                         | no       | structured terminal/recovery code      |
| `error`                        | no       | human-readable terminal error          |
| `expiresAt`                    | yes      | authoritative worker expiry            |
| `canRetry`                     | yes      | whether UI may offer restart           |
| `supportsManualCallbackImport` | yes      | keeps UI logic stable after refresh    |

`status` success example:

```json
{
  "status": "ok",
  "phase": "completed",
  "expiresAt": "2026-03-19T10:00:00Z",
  "canRetry": false,
  "supportsManualCallbackImport": true
}
```

On successful terminal completion:

- `status=ok`
- `phase=completed`
- `code` must be absent
- `error` must be absent

`callback` response for redirect channels:

| Field                   | Required | Notes                                                         |
| ----------------------- | -------- | ------------------------------------------------------------- |
| `status`                | yes      | `accepted`, `duplicate`, or `error`                           |
| `phase`                 | yes      | current worker phase after callback handling                  |
| `code`                  | no       | structured callback or terminal code                          |
| `error`                 | no       | human-readable error                                          |
| `shouldContinuePolling` | yes      | `true` for accepted/duplicate, `false` for validation failure |

These are worker-facing/admin-facing contracts, not raw runtime management payloads.

`canRetry` truth table:

| Transport status | Phase                  | `canRetry` |
| ---------------- | ---------------------- | ---------- |
| `waiting`        | any non-terminal phase | `false`    |
| `ok`             | `completed`            | `false`    |
| `error`          | `failed`               | `true`     |
| `expired`        | `expired`              | `true`     |

### 4.2 Provider ownership

Provider canonicalization and filtering must agree across:

- auth-file parsing
- auth-file import
- auth-file listing
- model listing
- quota lookup

If a runtime auth file belongs to `copilot`, it must never be treated as `codex`. If it belongs to `antigravity`, it must never leak into another runtime channel’s model list or import path.

Malformed, missing, or legacy provider metadata handling:

- newly imported runtime auth files must be canonicalized to a valid runtime provider before they become listable or selectable
- if canonical provider ownership cannot be determined, the file must be rejected from import into managed runtime channel state
- pre-existing malformed or legacy files may remain on disk, but they must not be listed for the wrong runtime channel and must not be selected as requestable auth for that channel

Backward-compatibility rule:

- filename-only legacy files do not gain ownership by filename alone
- if metadata is insufficient to determine canonical provider ownership, the file is treated as read-only legacy state and is non-selectable for runtime/OAuth channel use until rewritten or re-imported into canonical managed state
- this may reduce previously visible but invalid runtime entries; that is expected hardening rather than regression

Examples:

- `channel-33--codex.json` with no provider metadata -> remains on disk, not selectable, not listed as valid runtime auth for any channel
- `channel-35--codex-cli.json` with provider metadata `codex-cli` -> valid and selectable for `CodexCLI`

### 4.3 Model discovery and requestability

For each runtime channel:

- listed models must either come from actual runtime models or a clearly defined fallback set
- aliases shown to users must map to a real upstream target
- execution-time normalization must not silently diverge from listing-time names

This is the class of bug behind “model visible but unusable.”

### 4.3.1 Model resolution precedence

When model naming inputs disagree, resolve them in this order:

1. explicit user-configured `oauth-model-alias` for that provider/channel
2. built-in default alias for that provider/channel
3. upstream discovered model IDs
4. channel fallback model list
5. execution-time normalization (post-resolution routing only)

Additional rules:

- `oauth-excluded-models` applies after alias expansion and before final user-visible listing
- exclusion matching is case-insensitive and operates on the canonical visible model name after alias expansion
- execution-time upstream-only model IDs that are never exposed as visible names do not create a second exclusion namespace
- a fallback model is valid only if execution-time normalization can still route it to a real upstream target
- execution-time normalization is a post-resolution routing step; it may refine the upstream request target, but it must not act like a competing visible-name source or contradict the user-visible alias contract

Collision handling rules:

- if a client-facing alias equals a discovered upstream model ID, keep the discovered upstream model as the canonical target and do not create a second visible duplicate
- if multiple sources would produce the same visible model name, keep only one visible entry using the highest-precedence source from the resolution order
- de-duplication must happen after precedence resolution so listing-time and execution-time paths stay aligned

If a listed or discovered model cannot be executed after alias expansion and execution-time normalization, the system must treat that as a bug and stop exposing that model rather than returning a silently inconsistent list.

If upstream discovery fails or returns only invalid/unrequestable models, the required outcome is:

- use the channel fallback set only if every exposed fallback model is requestable
- otherwise expose no models for that channel from the invalid source path
- surface a structured runtime-side failure/log signal rather than listing a model that cannot execute

### 4.4 Dual config implementation drift

Both of these paths are live and must remain behaviorally aligned:

- `apps/worker/internal/runtimecore/config/*`
- `apps/worker/internal/runtime/corelib/config/*`

If one path injects defaults or normalizes aliases differently from the other, the project can appear fixed in tests or management flows while still failing in the actual runtime loader path.

### 4.4.1 Call-site inventory rule

This subproject must explicitly verify both live call paths:

- `apps/worker/internal/runtimecore/config/loader.go` -> private runtimecore sanitization path
- `apps/worker/internal/runtime/corelib/config/config.go` and management/config flows -> public corelib sanitization path

A path may only be excluded from code changes if current call sites prove it is unused for these four runtime/OAuth channels.

---

## 5. Recommended Approach

Treat runtime/OAuth consistency as a cross-channel contract, then harden each layer against that contract.

This design uses three coordinated workstreams:

1. **Availability alignment**
   - Ensure OAuth completion, provider ownership, and model requestability line up.
2. **Config consistency**
   - Keep alias/exclusion/default behavior aligned in both config paths.
3. **Regression defense**
   - Build a focused test matrix across the four runtime channels.

This is broader than a one-bug patch, but still much narrower than a platform-wide provider audit.

---

## 6. Detailed Design

### 6.1 Channel contract

For each runtime/OAuth channel, the system must answer these questions consistently:

- Which OAuth provider does this channel use?
- Which auth files belong to this channel?
- Which models are exposed to the user?
- Which upstream model is actually requested when the client sends a given model name?
- Which fallback behavior applies if upstream model discovery cannot answer?

The hardening work should explicitly document and test these answers instead of leaving them implicit across multiple helper functions.

### 6.2 OAuth/provider contract

The following mappings must be treated as the source of truth and verified together:

- channel type -> management auth endpoint
- channel type -> OAuth provider name
- channel type -> runtime provider filter
- provider string from auth file -> canonical runtime provider

If any one of these mappings differs from the others, cross-channel corruption or “imported but unavailable” behavior becomes likely.

The intended source-of-truth contract owner is a layer-neutral runtime/OAuth mapping helper set that can be consumed by handler code and mirrored config logic. At minimum, planning should treat these functions as the authoritative boundary to consolidate and test:

- channel type -> management auth endpoint
- channel type -> OAuth provider name
- channel type -> canonical runtime provider filter
- provider string -> canonical runtime provider

Endpoint handlers, model listing, quota lookup, auth-file import/filter logic, and any mirrored config behavior for these channels should consume that shared contract rather than carrying separate switch statements.

### 6.3 Model alias contract

For runtime/OAuth channels, model aliasing must satisfy two layers:

1. **Listing/discovery layer**
   - user-visible model names come from either upstream model lists or defined fallback aliases
2. **Execution layer**
   - user-visible names resolve to the actual upstream request target

The same client-facing model must not resolve one way in list/model sync and a different way in relay/executor code.

Antigravity is the current concrete example, but the contract applies to all four runtime channels.

The parity contract between listing-time and execution-time layers must be enforced through a shared canonical runtime model identity consisting of:

- visible client-facing model name
- canonical runtime provider/channel scope
- effective upstream execution target

The implementation does not need to introduce a new exported type if existing helpers can represent this consistently, but planning must treat this triplet as the conceptual source of truth.

### 6.4 Fallback model contract

Fallback model lists are acceptable, but only if they still satisfy requestability.

For each runtime channel with fallback models:

- fallback names must correspond to real runtime aliases or real upstream targets
- if a fallback model is client-facing, execution code must be able to route it correctly
- fallback behavior must be covered by tests, not left as an undocumented convenience

### 6.5 Config-path consistency contract

If a behavior exists in one config path, the corresponding runtime path must either:

- implement the same behavior, or
- be explicitly documented as unused for these channels

For this project, assume both `runtimecore/config` and `runtime/corelib/config` are in scope unless proven otherwise by current call sites.

---

## 7. Files Likely In Scope

### 7.1 Runtime/OAuth handler layer

- `apps/worker/internal/handler/codex.go`
- `apps/worker/internal/handler/codex_oauth.go`
- `apps/worker/internal/handler/codex_models.go`
- `apps/worker/internal/handler/codex_quota.go`
- `apps/worker/internal/handler/relay_antigravity_request.go`
- `apps/worker/internal/handler/relay_antigravity_test.go`
- `apps/worker/internal/handler/relay_copilot_test.go`
- `apps/worker/internal/handler/relay_codexcli_test.go`
- `apps/worker/internal/handler/codex_oauth_test.go`
- `apps/worker/internal/handler/codex_test.go`

### 7.2 Config layer

- `apps/worker/internal/runtimecore/config/sanitize.go`
- `apps/worker/internal/runtimecore/config/oauth_model_alias_defaults.go`
- `apps/worker/internal/runtime/corelib/config/config.go`
- `apps/worker/internal/runtime/corelib/config/oauth_model_alias_defaults.go`
- related config tests in both packages

### 7.3 Only if tests prove necessary

- runtime executor or translator files for Copilot, CodexCLI, or Antigravity
- management handler/config list code for `oauth-model-alias` / `oauth-excluded-models`

These should not be touched proactively unless the contract tests expose a real inconsistency.

### 7.4 Bounded implementation scope

Default in-scope work:

- runtime/OAuth handler logic
- runtime/OAuth model discovery helpers
- mirrored config alias behavior in both config packages
- regression tests

Investigate-only until a failing test proves necessity:

- executor/translator changes
- management `oauth-model-alias` / `oauth-excluded-models` API behavior

Execution-path regression tests are mandatory. Production executor/translator changes are only in scope if those mandatory tests fail against current behavior.

---

## 8. Testing Strategy

### 8.1 Contract matrix tests

Add or strengthen tests so the four runtime channels are checked against the same expectations where applicable:

- channel type -> OAuth start endpoint
- channel type -> provider filter
- channel type -> auth-file scoping
- channel type -> model discovery behavior

Minimum channel matrix:

| Area                                          | Codex | Copilot | CodexCLI | Antigravity |
| --------------------------------------------- | ----- | ------- | -------- | ----------- |
| start mapping                                 | yes   | yes     | yes      | yes         |
| provider filter                               | yes   | yes     | yes      | yes         |
| auth-file scoping                             | yes   | yes     | yes      | yes         |
| start error contract                          | yes   | yes     | yes      | yes         |
| missing/superseded/expired session behavior   | yes   | yes     | yes      | yes         |
| duplicate callback idempotency                | yes   | no      | yes      | yes         |
| wrong-provider rejection (import/filter path) | yes   | yes     | yes      | yes         |
| requestable model behavior                    | yes   | yes     | yes      | yes         |
| fallback model behavior                       | yes   | no      | yes      | yes         |

### 8.2 Config-path parity tests

Where defaults/alias behavior matters to runtime/OAuth channels, verify both config paths behave the same for:

- absent channel defaults
- explicit user overrides
- explicit nil/empty deletion markers
- alias direction (`Name` upstream, `Alias` client-facing)

### 8.3 Execution-path regression tests

For any client-facing runtime model that is normalized at execution time, add a regression test that proves:

- client-facing form reaches the correct upstream target
- already-normalized upstream form remains stable

At minimum, cover:

- Antigravity Claude alias -> `*-thinking`
- Antigravity direct `*-thinking` stability
- any additional runtime/OAuth channel where a listed client-facing model is rewritten at execution time

Per-channel execution inventory for this subproject:

- `Codex`: verify listed/requestable model parity through handler/model-sync tests unless a failing test proves execution drift
- `Copilot`: verify requestable model parity through handler/model-sync/provider-filter tests; no redirect callback execution test is required
- `CodexCLI`: verify listed/requestable model parity and any CLI-specific normalization if tests expose it
- `Antigravity`: verify explicit execution normalization via transformer/relay regression tests

### 8.4 Verification commands

At minimum, this subproject should finish with fresh evidence from:

- focused runtime/OAuth handler tests
- focused config tests in both config packages
- `make -C apps/worker test`
- `make -C apps/worker build`

The implementation plan derived from this spec must name exact focused commands for each changed layer.

---

## 9. Rollout Strategy

Break the work into narrow, test-first fixes in this order:

1. config-path parity for runtime/OAuth alias behavior
2. provider/filter/import consistency
3. model discovery and fallback consistency
4. execution-path regression coverage
5. full verification

This order reduces the chance of fixing execution symptoms while leaving config or provider ownership drift in place.

---

## 10. Acceptance Criteria

This subproject is complete when all of the following are true for `Codex`, `Copilot`, `CodexCLI`, and `Antigravity`:

- OAuth flows complete into the correct provider/channel scope
- provider filtering does not leak auth files or models across runtime channels
- user-visible runtime models correspond to actually requestable models
- client-facing aliases and execution-time targets are consistent
- mirrored config behavior is covered and aligned across both config implementations
- regression tests cover the main cross-channel failure modes

These criteria are only considered met if tests explicitly prove:

- wrong-provider imports are rejected and never imported into the wrong runtime channel
- missing/superseded/expired sessions return the expected structured outcomes
- listed fallback models for these channels are requestable or intentionally excluded
- the same model alias resolves consistently in both listing-time and execution-time paths

Rollout/remediation rule:

- the fix may tighten behavior by hiding previously visible but invalid models; this is expected hardening, not a regression
- the fix does not need to auto-migrate pre-existing bad auth files
- but once deployed, pre-existing ambiguous or wrong-provider auth state must no longer be newly imported, newly listed for the wrong channel, or newly selected as requestable
