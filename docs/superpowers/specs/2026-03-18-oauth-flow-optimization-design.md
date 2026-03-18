# Runtime OAuth Flow Optimization Design

**Date:** 2026-03-18
**Status:** Draft
**Scope:** End-to-end optimization of runtime-managed OAuth flows for Codex, GitHub Copilot, and Antigravity channels in the wheel admin UI and Go worker backend.

---

## 1. Overview

The current runtime OAuth flow works, but it breaks down in remote deployments and still feels like a collection of low-level states rather than a guided task. The main problems are:

- The UI mixes redirect-based and device-code-based OAuth into one generic dialog.
- Remote deployments often land the browser on a `localhost` callback URL that cannot be reached by the deployed worker.
- Manual callback import exists only as a thin fallback; it is not yet a first-class guided recovery path.
- Polling status is too coarse (`wait` / `ok` / `error`), so users cannot tell whether the system is waiting for the browser, waiting for callback import, importing auth files, or stuck.
- The current dialog is hard to resume after close, refresh, or accidental interruption.

This design upgrades the OAuth flow into a complete, resumable, guided experience that treats remote callback import as a supported flow rather than an edge-case workaround.

Reliability target for this iteration: **same-instance reliable completion**. The design guarantees a strong flow only while the same worker instance still holds the in-memory OAuth session. Cross-instance recovery after restart, redeploy, or load-balancer hop is out of scope and should degrade to a clean restart path.

Remote deployment contract for supported behavior:

- the supported recovery path assumes `start`, `status`, and `callback` requests continue to hit the same worker instance
- if a later request lands on a different worker instance, the expected outcome is `session_missing` and a user-visible restart path
- acceptance testing for remote deployment should therefore use sticky routing or a single worker instance when verifying recovery/resume behavior

---

## 2. Goals

### 2.1 Product Goals

- Make OAuth completion reliable for remote deployments within a same-worker-instance session.
- Reduce user confusion by showing step-by-step progress instead of generic loading states.
- Distinguish redirect flows from device-code flows.
- Support best-effort recovery after dialog close, page reload, timeout, or failed callback submission when the worker session still exists.
- Improve diagnostics so users and maintainers can tell what failed and why.

### 2.2 Non-Goals

- Do not introduce a public callback domain or external callback proxy in this iteration.
- Do not change third-party OAuth provider registration or secrets.
- Do not redesign the broader channel settings page outside the OAuth dialog and its immediate state plumbing.

---

## 3. UX Design

### 3.1 Flow Types

The UI should model two explicit flow types:

1. **Redirect flow**
   - Used for Codex and Antigravity.
   - User opens an auth URL in a new page.
   - Provider eventually redirects to a callback URL.
   - If the callback reaches runtime automatically, import proceeds without extra input.
   - If the callback lands on unreachable `localhost`, user pastes the full callback URL back into the dialog.

2. **Device code flow**
   - Used for GitHub Copilot.
   - User copies a verification code and opens a verification page.
   - No manual callback URL field should be shown unless the backend explicitly marks the flow as redirect-capable.

### 3.2 Dialog Structure

Replace the current generic OAuth dialog with a step-oriented layout:

- **Header**
  - Provider name
  - Current status badge (`Starting`, `Waiting`, `Importing`, `Completed`, `Failed`, `Expired`)
- **Step panel**
  - Shows numbered steps for the active flow type
- **Primary action area**
  - Open auth page
  - Copy link / copy code
  - Paste callback URL
  - Retry polling / restart flow
- **Recovery area**
  - Session expiry notice
  - Continue pending flow after refresh
  - Restart with preserved context when safe

### 3.3 Redirect Flow Steps

For redirect flows, the UI should show:

1. **Open login page**
   - Show auth URL with `Copy link` and `Open login page`.
2. **Complete login in browser**
   - Explain that remote deployments may end on `localhost/...`.
3. **Finish callback**
   - Prefer auto-detection.
   - If auto-detection does not happen, show a dedicated callback import card:
     - Paste full callback URL
     - Paste from clipboard button
     - Local validation result before submission
     - Submit button

### 3.4 Device Code Flow Steps

For device flows, the UI should show:

1. **Copy verification code**
2. **Open verification page**
3. **Wait for account import**

The callback import module should not appear here unless the API response says manual callback import is supported.

### 3.5 Success and Failure UX

- Success should move to a stable completed state rather than only showing a toast.
- Failure should name the failed phase:
  - start failed
  - callback invalid
  - state mismatch
  - callback expired
  - auth file import failed
- The dialog should preserve the relevant inputs so users can retry without starting from zero when safe.

---

## 4. Backend Design

### 4.1 Current Relevant Endpoints

Current worker endpoints:

- `POST /api/v1/channel/:id/{prefix}/oauth/start`
- `GET /api/v1/channel/:id/{prefix}/oauth/status`
- `POST /api/v1/channel/:id/{prefix}/oauth/callback`

Current runtime management endpoints:

- `GET /v0/management/{provider}-auth-url`
- `GET /v0/management/get-auth-status`
- `POST /v0/management/oauth-callback`

### 4.1.1 Runtime vs Worker Ownership

This design intentionally keeps most orchestration in the worker layer.

Existing runtime fields/signals already available today:

- `start`: `url`, `state`, optional `user_code`, optional `verification_uri`
- `status`: coarse `wait` / `ok` / `error`
- `callback`: accepts `redirect_url` and persists callback payload

New worker-owned fields/signals introduced by this design:

- `flowType`
- `supportsManualCallbackImport`
- `expiresAt` (derived from worker session TTL)
- `phase`
- `warningCode` / `warningMessage`
- structured callback error codes

Runtime changes are **not required** for Phase 1 unless implementation reveals a missing signal that cannot be derived from current runtime responses. The worker is responsible for mapping coarse runtime responses into richer UI-facing state.

### 4.2 New Worker Status Contract

Upgrade the worker-facing OAuth status model from a simple string to a richer phase model.

Proposed response shape for `start`:

```json
{
  "url": "...",
  "state": "...",
  "flowType": "redirect",
  "supportsManualCallbackImport": true,
  "expiresAt": "2026-03-18T10:00:00Z"
}
```

Proposed response shape for device-code `start`:

```json
{
  "url": "...",
  "state": "...",
  "flowType": "device_code",
  "user_code": "ABCD-EFGH",
  "verification_uri": "...",
  "supportsManualCallbackImport": false,
  "expiresAt": "2026-03-18T10:00:00Z"
}
```

Proposed response shape for `status`:

```json
{
  "status": "waiting",
  "phase": "awaiting_callback",
  "code": "",
  "error": "",
  "warningCode": "",
  "warningMessage": "",
  "expiresAt": "2026-03-18T10:00:00Z",
  "canRetry": true,
  "supportsManualCallbackImport": true
}
```

Proposed response shape for `callback`:

```json
{
  "status": "accepted",
  "phase": "callback_received",
  "code": "",
  "error": "",
  "shouldContinuePolling": true
}
```

Worker-side phases:

- `starting`
- `awaiting_browser`
- `awaiting_callback`
- `callback_received`
- `importing_auth_file`
- `completed`
- `failed`
- `expired`

The worker may still map runtime `wait` / `ok` / `error` values internally, but the API exposed to the UI should be phase-based.

### 4.2.1 API Field Rules

| Endpoint   | Field                          | Required         | Notes                                                                  |
| ---------- | ------------------------------ | ---------------- | ---------------------------------------------------------------------- |
| `start`    | `url`                          | yes              | auth URL or verification URL entry point                               |
| `start`    | `state`                        | yes              | persisted by frontend for recovery and callback validation             |
| `start`    | `flowType`                     | yes              | `redirect` or `device_code`                                            |
| `start`    | `user_code`                    | device flow only | omitted for redirect flows                                             |
| `start`    | `verification_uri`             | device flow only | display-only helper field; `url` remains the authoritative open target |
| `start`    | `supportsManualCallbackImport` | yes              | `true` for redirect flows in this project                              |
| `start`    | `expiresAt`                    | yes              | authoritative expiry used by frontend recovery                         |
| `status`   | `status`                       | yes              | transport-level summary: `waiting`, `ok`, `error`, `expired`           |
| `status`   | `phase`                        | yes              | UI state source of truth                                               |
| `status`   | `code`                         | no               | structured terminal or recovery code                                   |
| `status`   | `error`                        | no               | terminal failure message only                                          |
| `status`   | `warningCode`                  | no               | structured non-terminal warning code                                   |
| `status`   | `warningMessage`               | no               | inline warning copy while polling continues                            |
| `status`   | `expiresAt`                    | yes              | mirrors worker session expiry                                          |
| `status`   | `canRetry`                     | yes              | indicates restart CTA should be shown                                  |
| `status`   | `supportsManualCallbackImport` | yes              | keeps UI consistent after reload                                       |
| `callback` | `status`                       | yes              | `accepted`, `duplicate`, or `error`                                    |
| `callback` | `phase`                        | yes              | current worker phase after processing callback                         |
| `callback` | `code`                         | no               | structured error code on failure                                       |
| `callback` | `error`                        | no               | human-readable message for inline display                              |
| `callback` | `shouldContinuePolling`        | yes              | always `true` for accepted or duplicate success                        |

`state` is the authoritative session identifier for both redirect and device-code flows. No separate `sessionId` is introduced in this iteration.

Session identity source:

- redirect flow: the runtime-provided OAuth `state` returned by `start`
- device-code flow: the runtime-provided session `state` returned by `start`, even though the user primarily sees `user_code`
- if runtime fails to return `state` for any flow, `start` must fail rather than letting the worker mint a different identifier

Request contracts:

- `POST /api/v1/channel/:id/{prefix}/oauth/start`
  - content type: `application/json`
  - optional body field: `force_restart` (default `false`)
  - `force_restart=false` with an active session returns the existing live session metadata instead of superseding it
  - `force_restart=true` always creates a new session and supersedes the previous one for the same channel/provider
- `GET /api/v1/channel/:id/{prefix}/oauth/status?state=<state>`
  - `state` is required and must match the worker session being resumed or actively polled.
  - missing `state` -> `400` with code `missing_state`
  - unknown, expired, or superseded `state` -> payload with phase `expired` and an appropriate error code such as `session_expired` or `session_superseded`
- `POST /api/v1/channel/:id/{prefix}/oauth/callback`
  - content type: `application/json`
  - required body field: `callback_url`
  - no separate `state` or `provider` fields in this iteration; the worker extracts `state` from `callback_url` and derives provider from channel type

`status` semantics:

- `waiting` = non-terminal, including non-terminal warning cases
- `ok` = terminal success, phase must be `completed`
- `error` = terminal failure, phase must be `failed`
- `expired` = terminal expiry / superseded / missing-session outcome, phase must be `expired`

Field population rules:

- `code` is used for structured terminal/recovery outcomes such as `session_missing`, `session_superseded`, `access_denied`, `device_code_expired`
- `error` is the human-readable terminal message paired with `code`
- `warningCode` / `warningMessage` are frontend-hook-owned fields synthesized while `status=waiting`; the backend does not need to emit them in this iteration

### 4.2.2 UI Mapping Rules

| Worker phase          | Badge       | Primary UI                                            |
| --------------------- | ----------- | ----------------------------------------------------- |
| `starting`            | `Starting`  | spinner, disable actions                              |
| `awaiting_browser`    | `Waiting`   | device-code instructions and verification page action |
| `awaiting_callback`   | `Waiting`   | redirect auth link plus callback import card          |
| `callback_received`   | `Importing` | callback accepted, keep polling                       |
| `importing_auth_file` | `Importing` | progress text, no callback input                      |
| `completed`           | `Completed` | success summary and close CTA                         |
| `failed`              | `Failed`    | inline error and retry/restart CTA                    |
| `expired`             | `Expired`   | restart CTA only                                      |

### 4.3 Session State Enrichment

Extend `codexOAuthSession` in `apps/worker/internal/handler/codex.go` to track more than `ChannelID` and auth-file snapshot:

- `Provider`
- `FlowType`
- `SupportsManualCallbackImport`
- `ExpiresAt`
- `LastPhase`
- `LastError`
- `CallbackImportedAt`

This state will let the worker represent meaningful progress even when runtime only exposes coarse polling information.

Session durability expectation:

- worker session TTL remains the existing `codexOAuthSessionTTL` value unless implementation reveals a reason to change it
- worker OAuth session state is in-memory only for this iteration
- worker session persists all resume-critical fields: `state`, `provider`, `flowType`, `url`, optional `user_code`, optional `verification_uri`, `expiresAt`, `lastPhase`, `lastError`
- resumability covers dialog close, page refresh, and accidental navigation while the same worker process remains alive
- remote deployments are **best-effort** only unless requests are routed back to the same worker instance; lack of instance affinity may legitimately produce `session_missing`
- worker restart or redeploy may legitimately surface `session_missing`
- `session_missing` after worker restart is acceptable and must show a restart CTA rather than pretending recovery is possible

### 4.3.1 Session Concurrency Rules

The worker should use a **single active OAuth session per channel + provider** model.

Rules:

- `start()` with no active local session creates a new session.
- `start()` while an active local session exists should resume that session instead of silently superseding it.
- explicit `restart()` always creates a new session and invalidates the prior one for the same channel/provider.
- `start(force_restart=true)` is the backend form of `restart()` and invalidates the previous worker session for the same channel/provider.
- A replaced session should surface as `expired` with error code `session_superseded` to the old UI instance.
- Multiple tabs may observe the same active session, but only the latest started session is authoritative.
- `sessionStorage` keys should be scoped by `channelId` + `channelType` + provider.
- A pasted callback whose `state` belongs to an invalidated session must return `state_mismatch` or `session_expired`, never silently attach to the new flow.

This keeps the model simple and avoids ambiguous recovery behavior.

Example sequence:

1. Tab A calls `start()` and receives `state=A1`.
2. User refreshes Tab A; UI auto-reopens and resumes `state=A1`.
3. Tab B opens the same channel and chooses `restart()`; worker creates `state=B1` and supersedes `A1`.
4. Tab A polls `status?state=A1` and receives phase `expired` with code `session_superseded`.
5. Tab A clears local storage and shows restart guidance.

### 4.4 Callback Import Hardening

`SubmitCodexOAuthCallback` in `apps/worker/internal/handler/codex_oauth.go` should be expanded into a proper guided import endpoint.

Requirements:

- Validate callback URL format before forwarding.
- Parse the URL locally to extract:
  - `state`
  - `code`
  - `error`
  - `error_description`
- Confirm the incoming `state` belongs to the channel session.
- Confirm the stored worker session provider matches the provider implied by the channel type.
- Return structured error categories instead of only a gateway error string.
- Treat repeated submissions for the same callback as idempotent where possible.
- Mark session phase as `callback_received` immediately after successful write.

Proposed error codes:

- `invalid_callback_url`
- `missing_state`
- `missing_code`
- `state_mismatch`
- `session_expired`
- `session_missing`
- `session_superseded`
- `provider_mismatch`
- `runtime_callback_rejected`

Provider-originated failures should be normalized as:

- user canceled / denied consent -> `access_denied`
- provider returned generic OAuth error -> `provider_error`
- device code expired -> `device_code_expired`
- device code declined / invalid -> `device_code_rejected`

`provider_mismatch` applies only when the worker's stored session provider and the channel-derived provider disagree. It is not inferred from the pasted callback URL.

Non-terminal warning codes:

- `poll_retrying`
- `import_stalled`
- `runtime_temporarily_unreachable`

`session_missing` is the single recovery code for unknown state, cross-instance hop, worker restart, or any other situation where the current worker instance cannot find the referenced session.

Success semantics:

- `accepted`
  - callback file was persisted for the active session
  - worker phase becomes `callback_received`
  - frontend clears local validation errors and keeps polling
- `duplicate`
  - callback was already imported for the same active session
  - treated as successful replay, not a blocking error
  - response `phase` returns the worker's current post-callback phase, which may already be `callback_received` or `importing_auth_file`
  - optional informational code: `duplicate_callback`
  - frontend keeps polling
- `error`
  - frontend stays on callback import step and shows inline error

### 4.5 Import Completion Tracking

Today the worker imports auth files when `get-auth-status` returns `ok`.

That behavior should remain, but the worker should explicitly transition its own phase:

- `awaiting_callback` -> `callback_received`
- `callback_received` -> `importing_auth_file`
- `importing_auth_file` -> `completed`

If auth file import fails after callback receipt, the state should move to `failed` with a clear reason. This avoids telling the UI only that OAuth “failed” when the real issue happened in post-auth import.

### 4.5.1 Runtime-to-Worker Phase Mapping

| Runtime signal                                        | Worker phase                              | Terminal | Notes                                                                                    |
| ----------------------------------------------------- | ----------------------------------------- | -------- | ---------------------------------------------------------------------------------------- |
| start request in flight                               | `starting`                                | no       | before runtime response is received                                                      |
| start succeeded, redirect flow                        | `awaiting_callback`                       | no       | UI shows auth link and callback guidance immediately after `start`                       |
| start succeeded, device flow                          | `awaiting_browser`                        | no       | UI shows code + verification page                                                        |
| runtime status `wait` before callback                 | `awaiting_browser` or `awaiting_callback` | no       | redirect flows stay in `awaiting_callback`; device-code flows stay in `awaiting_browser` |
| callback accepted                                     | `callback_received`                       | no       | worker has persisted callback                                                            |
| runtime still preparing imported auth file            | `importing_auth_file`                     | no       | post-callback polling state                                                              |
| runtime status `ok` and import succeeds               | `completed`                               | yes      | success path                                                                             |
| runtime status `error` with `access_denied`           | `failed`                                  | yes      | provider/user declined consent                                                           |
| runtime status `error` with `device_code_expired`     | `failed`                                  | yes      | restart required                                                                         |
| runtime status `error` with `device_code_rejected`    | `failed`                                  | yes      | restart required                                                                         |
| runtime status `error` with retryable transport issue | keep current phase + warning              | no       | UI shows recoverable warning, polling continues                                          |
| worker session timeout                                | `expired`                                 | yes      | restart required                                                                         |

---

## 5. Frontend Design

### 5.1 State Model

The existing dialog logic in `apps/web/src/pages/model/codex-channel-detail.tsx` has accumulated too many unrelated responsibilities. The OAuth UI should be split into focused components.

Provider abstraction rule:

- do not introduce a new cross-provider interface layer in this iteration
- keep provider ownership in the existing worker mapping helpers (`channel type` -> `provider`, `flow type`)
- keep the shared frontend behavior in `useRuntimeOAuthSession`, with provider-specific rendering differences handled by `flowType` and response metadata

This keeps the change set focused and avoids speculative abstraction while still supporting Codex, GitHub Copilot, and Antigravity cleanly.

Recommended structure:

- `CodexChannelDetail`
  - Launches flow and owns high-level dialog visibility
- `OAuthFlowDialog`
  - Receives active session state and renders the overall dialog shell
- `OAuthRedirectFlow`
  - Renders auth link, callback import card, and retry guidance
- `OAuthDeviceCodeFlow`
  - Renders device code and verification page guidance
- `useRuntimeOAuthSession`
  - Owns polling, persistence, expiry, recovery, and submit-callback actions

This can be kept under `apps/web/src/pages/model/` unless a shared runtime-auth area becomes necessary.

### 5.1.1 Hook / Component Boundary

`useRuntimeOAuthSession` should own:

- starting the flow
- polling status
- persisting and clearing `sessionStorage`
- callback URL validation orchestration and backend submission
- clipboard read helper for callback import
- `window.open` result handling for auth-page launch
- mapping backend error codes into UI-friendly inline state
- invoking completion callbacks so the parent can invalidate/refetch the auth file list when phase becomes `completed`

`OAuthFlowDialog` should own:

- dialog shell and close behavior
- status badge rendering
- wiring active flow type to the correct child component

`OAuthRedirectFlow` and `OAuthDeviceCodeFlow` should remain presentational:

- render props in
- event callbacks out
- no direct storage or polling logic

Popup-blocked rule:

- if `window.open` returns `null` or is blocked, keep the flow in the same non-terminal phase
- show inline guidance to use the visible `Copy link` action instead
- do not clear callback input or restart the session because popup blocking is not a session failure

Minimum hook interface:

- state returned:
  - `flowType`
  - `phase`
  - `status`
  - `error`
  - `warningCode`
  - `warningMessage`
  - `oauthUrl`
  - `userCode`
  - `verificationUri`
  - `callbackInput`
  - `callbackValidation`
  - `isSubmittingCallback`
  - `canRetry`
- actions returned:
  - `start()`
  - `openAuthPage()`
  - `copyAuthLink()`
  - `copyUserCode()`
  - `pasteCallbackFromClipboard()`
  - `setCallbackInput(value)`
  - `submitCallback()`
  - `restart()`
  - `dismissWarning()`

Presentational component props:

- `OAuthRedirectFlow`
  - receives rendered strings/state plus `onOpenAuthPage`, `onCopyAuthLink`, `onPasteCallback`, `onCallbackInputChange`, `onSubmitCallback`, `onRestart`
- `OAuthDeviceCodeFlow`
  - receives rendered strings/state plus `onOpenAuthPage`, `onCopyUserCode`, `onRestart`

### 5.2 Local Session Persistence

Store the active session in `sessionStorage` keyed by channel and provider so users can recover after refresh.

Persist only what is needed:

- `channelId`
- `channelType`
- `state`
- `flowType`
- `oauthUrl`
- `userCode`
- `verificationUri`
- `expiresAt`

On dialog reopen or page mount:

- If a pending session exists and has not expired, auto-reopen the dialog in resumable mode and resume polling.
- If expired, show a restart path and clear the stored session.

Recovery contract:

- If frontend storage exists but worker returns `expired`, clear storage and show restart.
- If frontend storage exists but worker returns `session_missing`, treat it the same as expired.
- If worker reports a different active state for the same channel/provider, replace local storage only after a fresh `start`; do not guess or merge sessions.

Resume handshake:

- frontend restores a pending session only by calling `status` with the stored `state`
- if `status` confirms that exact `state` is active, continue rendering from returned `phase`
- if `status` reports the state as expired, missing, or superseded, clear local storage and show restart
- frontend must never attach to a different active session unless the user explicitly starts a new flow

### 5.3 Callback Import UX

The callback import card should support:

- manual paste
- paste-from-clipboard convenience
- immediate local validation
- disabled submit until validation passes
- visible parsed state summary when useful

Validation rules in the browser:

- must parse as URL
- must contain `state`
- must contain `code` or `error`
- if local session state exists, pasted state must match

The frontend should only call the backend after local validation passes.

### 5.4 Polling Behavior

Polling should move from a blind interval into a small state machine:

- start polling after successful `start`
- poll every 3 seconds while waiting for browser interaction or callback import
- after `callback_received`, continue polling every 2 seconds until terminal state or 30 seconds of import wait
- stop polling on `completed`, `failed`, or `expired`

The UI should not silently swallow all polling failures.

Minimum retry rules:

- up to 3 consecutive network failures remain silent inline and keep the previous phase
- on the 4th consecutive polling failure, show a warning banner with retry guidance
- on the next successful poll, clear the warning and reset the failure counter
- polling still stops at the existing overall session timeout / expiry

Import stall rule:

- if the flow remains in `callback_received` or `importing_auth_file` for more than 30 seconds, keep background polling every 5 seconds
- also show a non-terminal warning banner with code `import_stalled`
- the user may either keep waiting or restart the flow
- do not convert this condition into terminal `failed` unless the backend explicitly returns `failed`

### 5.5 Toast vs Inline Status

Toasts should become secondary confirmation only.

Primary feedback should be inline inside the dialog so that:

- users understand what happened even if a toast is missed
- flow state is visible after refresh
- error resolution steps stay attached to the active flow

---

## 6. File-Level Design

### 6.1 Backend Files

- **Modify** `apps/worker/internal/handler/codex.go`
  - enrich the shared runtime OAuth session struct and helpers used by Codex, GitHub Copilot, and Antigravity
- **Modify** `apps/worker/internal/handler/codex_oauth.go`
  - enrich the shared runtime OAuth start/status/callback logic used by Codex, GitHub Copilot, and Antigravity
- **Modify** `apps/worker/internal/handler/routes.go`
  - keep current callback route registration for all three runtime providers; no route shape change needed
- **Add** `apps/worker/internal/handler/codex_oauth_test.go`
  - cover shared start/status/callback/restart flows using mocked `CodexManagementClient` responses and in-memory session state across Codex, GitHub Copilot, and Antigravity channel types

### 6.2 Frontend Files

- **Modify** `apps/web/src/lib/api/codex.ts`
  - richer shared TypeScript response types for start/status/callback across all runtime OAuth providers
- **Refactor** `apps/web/src/pages/model/codex-channel-detail.tsx`
  - reduce inline OAuth dialog complexity while remaining the shared entry point for Codex, GitHub Copilot, and Antigravity channel details
- **Add** focused OAuth UI pieces near `apps/web/src/pages/model/`
  - `oauth-flow-dialog.tsx`
  - `oauth-redirect-flow.tsx`
  - `oauth-device-code-flow.tsx`
  - `use-runtime-oauth-session.ts`
- **Modify** `apps/web/src/i18n/locales/en/model.json`
- **Modify** `apps/web/src/i18n/locales/zh-CN/model.json`

This decomposition keeps the existing page intact while isolating the runtime OAuth behavior into smaller, testable units.

---

## 7. Error Handling

### 7.1 User-Facing Errors

Every failure should be mapped to an actionable message.

Examples:

- Invalid pasted callback URL -> “The pasted address is not a valid callback URL.”
- Missing state -> “This callback is incomplete. Copy the full browser address and try again.”
- State mismatch -> “This callback belongs to a different login attempt. Restart OAuth and try again.”
- Session expired -> “This login attempt expired. Start a new OAuth session.”
- Access denied -> “Login was canceled or permission was denied. Try again if you want to continue.”
- Device code expired -> “The verification code expired. Start a new login attempt.”
- Import failed after callback -> “Login succeeded, but importing the auth file failed.”

### 7.2 Maintainer Diagnostics

Backend logs should include:

- channel ID
- provider
- state
- worker phase transition
- callback import result
- import failure stage

Sensitive tokens and full callback URLs must not be logged.

---

## 8. Testing Strategy

### 8.1 Backend Tests

Add handler tests covering:

- start returns flow metadata for each provider type
- start with `force_restart=false` resumes existing live session metadata
- start with `force_restart=true` supersedes the prior session
- status returns structured phases
- callback URL submission with valid redirect URL
- invalid callback URL rejection
- state mismatch rejection
- expired session rejection
- duplicate callback submission behavior
- auth import failure after callback receipt

### 8.2 Frontend Tests

Add React tests covering:

- redirect flow renders callback import section
- device flow renders code/verification guidance only
- invalid callback URL blocks submission
- pasted state mismatch surfaces inline error
- successful callback import updates inline phase state
- pending session recovery from storage
- expired session recovery path
- worker session missing while local storage exists
- callback endpoint returns structured error code and message
- clipboard read permission failure
- `window.open` returns blocked / `null` and the copy path remains usable

### 8.3 Verification Commands

- Frontend unit tests: `pnpm --dir apps/web test`
- TypeScript typecheck and web build: `pnpm --dir apps/web build`
- Worker tests: `make -C apps/worker test`
- Worker build: `make -C apps/worker build`

### 8.4 Integration Acceptance Scenario

Add one high-priority integration scenario that exercises the main product risk:

1. Start a redirect-based OAuth flow from the remote UI.
2. Open the auth page and complete login until the browser lands on an unreachable `localhost/...` callback URL.
3. Copy the full callback URL into the admin UI callback import card.
4. Verify the callback is accepted, polling resumes, and the worker reaches `completed`.
5. Verify the imported auth file appears in the auth file list without restarting the page.

Add two more acceptance scenarios:

1. **Device-code happy path**
   - start a device-code flow
   - copy the code, open the verification page, complete authorization
   - verify the UI reaches `completed` without showing callback import UI
2. **Refresh recovery path**
   - start either flow type and refresh while still pending
   - verify the UI restores from `sessionStorage`, resumes polling with the stored `state`, and either completes or shows restart if the worker session is missing/expired

---

## 9. Rollout Strategy

Implement in two small phases:

1. **Phase 1: Contract and state model**
   - richer backend responses
   - callback import validation
   - tests
2. **Phase 2: UI restructuring**
   - split dialog into flow-specific components
   - persistence and recovery
   - inline status and error messaging

Compatibility rule:

- Phase 1 and Phase 2 should ship together unless the frontend first lands a compatibility layer that accepts both old and new status contracts.
- If split across releases, the backend must preserve old `status` behavior alongside new `phase` fields until the new UI is deployed.

Compatibility matrix:

- old frontend + new backend -> supported only if backend preserves old `status` semantics
- new frontend + old backend -> not supported; deploy backend contract first or deploy together
- new frontend + new backend -> target steady state

This keeps the system shippable after each stage and avoids a large one-shot rewrite.

---

## 10. Recommendation

Proceed with the guided OAuth session model and split the UI by flow type. This directly addresses the remote `localhost` callback problem while also cleaning up the broader user journey. It keeps the current backend architecture and runtime integration intact, but makes the flow legible, resumable, and much easier to support.
