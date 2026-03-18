# Runtime OAuth Flow Optimization Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a same-instance-reliable runtime OAuth flow that cleanly supports redirect and device-code providers, guided manual callback import, refresh recovery, and clear completion/error states.

**Architecture:** Keep the shared runtime OAuth backend in the existing worker path (`codex.go` / `codex_oauth.go`) for Codex, GitHub Copilot, and Antigravity. Add richer worker-owned session state and response contracts, then move the frontend to a hook-driven state machine plus flow-specific presentational components. Preserve the current route shape and use `start(force_restart=true)` instead of adding a separate restart endpoint.

**Tech Stack:** Go + Gin + `httptest` for worker handlers; React + TypeScript + Vitest for frontend logic; `sessionStorage` for same-instance resume; React Query invalidation for post-completion refresh.

---

## File Structure

- **Modify** `apps/worker/internal/handler/codex.go`
  - Expand the shared runtime OAuth session struct used by Codex, GitHub Copilot, and Antigravity.
- **Modify** `apps/worker/internal/handler/codex_oauth.go`
  - Add `force_restart` handling, richer `start`/`status`/`callback` contracts, phase transitions, and structured terminal codes.
- **Modify** `apps/worker/internal/handler/routes.go`
  - Keep route shapes stable; confirm no new endpoint is needed beyond current shared callback route.
- **Add** `apps/worker/internal/handler/codex_oauth_test.go`
  - Focused tests for shared runtime OAuth start/status/callback semantics.
- **Modify** `apps/web/package.json`
  - Add frontend component/hook test dependencies if missing.
- **Modify** `apps/web/vitest.config.ts`
  - Enable `jsdom` for runtime OAuth component/hook tests.
- **Modify** `apps/web/src/lib/api/codex.ts`
  - Add richer shared request/response types and `force_restart` support.
- **Add** `apps/web/src/pages/model/use-runtime-oauth-session.ts`
  - Shared hook for polling, recovery, restart, callback import, and completion callback.
- **Add** `apps/web/src/pages/model/use-runtime-oauth-session.test.tsx`
  - Hook tests for resume/restart/warning/callback flows.
- **Add** `apps/web/src/pages/model/oauth-flow-dialog.tsx`
  - Dialog shell and status badge.
- **Add** `apps/web/src/pages/model/oauth-redirect-flow.tsx`
  - Redirect-flow UI only.
- **Add** `apps/web/src/pages/model/oauth-device-code-flow.tsx`
  - Device-code UI only.
- **Add** `apps/web/src/pages/model/oauth-flow-dialog.test.tsx`
  - Flow-specific rendering tests.
- **Modify** `apps/web/src/pages/model/codex-channel-detail.tsx`
  - Replace inline OAuth state machine with the shared hook + dialog components; invalidate auth-file queries on completion.
- **Modify** `apps/web/src/i18n/locales/en/model.json`
- **Modify** `apps/web/src/i18n/locales/zh-CN/model.json`
  - Add step text, recovery copy, popup-blocked guidance, and terminal messages.

---

### Task 1: Worker Start Contract And Session Persistence

**Files:**

- Add: `apps/worker/internal/handler/codex_oauth_test.go`
- Modify: `apps/worker/internal/handler/codex.go`
- Modify: `apps/worker/internal/handler/codex_oauth.go`

- [ ] **Step 1: Write the failing backend tests for start/resume/restart semantics**

```go
func TestStartCodexOAuth_ReturnsExistingSessionWhenForceRestartIsFalse(t *testing.T) {}

func TestStartCodexOAuth_SupersedesExistingSessionWhenForceRestartIsTrue(t *testing.T) {}

func TestStartCodexOAuth_ReturnsRedirectMetadata(t *testing.T) {}

func TestStartCodexOAuth_ReturnsDeviceCodeMetadata(t *testing.T) {}
```

- [ ] **Step 2: Run the targeted worker tests to verify they fail**

Run: `go test ./internal/handler -run 'TestStartCodexOAuth_' -count=1`
Expected: FAIL because `force_restart`, richer metadata, and resumable session behavior are not implemented yet.

- [ ] **Step 3: Implement the minimal session model in `codex.go`**

```go
type codexOAuthSession struct {
	ChannelID        int
	Provider         string
	FlowType         string
	URL              string
	UserCode         string
	VerificationURI  string
	SupportsManual   bool
	State            string
	ExpiresAt        time.Time
	LastPhase        string
	LastError        string
	Existing         map[string]struct{}
	createdAt        time.Time
}
```

- [ ] **Step 4: Implement `start(force_restart)` behavior in `codex_oauth.go`**

```go
var req struct {
	ForceRestart bool `json:"force_restart"`
}

if existing, ok := loadOAuthSession(resp.State); ok && !req.ForceRestart {
	successJSON(c, serializeOAuthSession(existing))
	return
}
```

- [ ] **Step 5: Run the targeted worker tests to verify they pass**

Run: `go test ./internal/handler -run 'TestStartCodexOAuth_' -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add apps/worker/internal/handler/codex.go apps/worker/internal/handler/codex_oauth.go apps/worker/internal/handler/codex_oauth_test.go
git commit -m "feat: add resumable runtime oauth start contract"
```

### Task 2: Worker Status And Callback Contracts

**Files:**

- Modify: `apps/worker/internal/handler/codex_oauth.go`
- Modify: `apps/worker/internal/handler/codex.go`
- Add: `apps/worker/internal/handler/codex_oauth_test.go`

- [ ] **Step 1: Write the failing backend tests for status/callback phases and codes**

```go
func TestGetCodexOAuthStatus_ReturnsExpiredForMissingSession(t *testing.T) {}

func TestGetCodexOAuthStatus_ReturnsExpiredForSupersededSession(t *testing.T) {}

func TestGetCodexOAuthStatus_MapsRuntimeWaitToAwaitingCallback(t *testing.T) {}

func TestSubmitCodexOAuthCallback_AcceptsDuplicateReplay(t *testing.T) {}

func TestSubmitCodexOAuthCallback_RejectsStateMismatch(t *testing.T) {}

func TestSubmitCodexOAuthCallback_RejectsInvalidCallbackURL(t *testing.T) {}

func TestSubmitCodexOAuthCallback_ReturnsExpiredForMissingSession(t *testing.T) {}

func TestSubmitCodexOAuthCallback_RejectsProviderMismatch(t *testing.T) {}

func TestGetCodexOAuthStatus_ReturnsFailedWhenAuthImportFails(t *testing.T) {}

func TestGetCodexOAuthStatus_LogsPhaseTransitionsWithoutSensitiveCallbackData(t *testing.T) {}
```

- [ ] **Step 2: Run the targeted worker tests to verify they fail**

Run: `go test ./internal/handler -run 'Test(GetCodexOAuthStatus|SubmitCodexOAuthCallback)_' -count=1`
Expected: FAIL because phase/code mapping is still too coarse.

- [ ] **Step 3: Implement status response serialization with `phase` and terminal `code`**

```go
type oauthStatusPayload struct {
	Status                     string `json:"status"`
	Phase                      string `json:"phase"`
	Code                       string `json:"code,omitempty"`
	Error                      string `json:"error,omitempty"`
	ExpiresAt                  string `json:"expiresAt"`
	CanRetry                   bool   `json:"canRetry"`
	SupportsManualCallbackImport bool `json:"supportsManualCallbackImport"`
}
```

- [ ] **Step 4: Implement callback contract, duplicate replay semantics, and phase transition rules**

```go
if duplicate {
	successJSON(c, gin.H{
		"status": "duplicate",
		"phase":  session.LastPhase,
		"code":   "duplicate_callback",
		"shouldContinuePolling": true,
	})
	return
}
```

- [ ] **Step 4.2: Implement provider-mismatch validation before accepting callback import**

```go
if session.Provider != oauthProviderForChannelType(channel.Type) {
	errorJSON(c, http.StatusBadRequest, "provider mismatch")
	return
}
```

- [ ] **Step 4.5: Add maintainer diagnostics logging without leaking callback URLs or tokens**

```go
log.WithFields(log.Fields{
	"channel_id": channel.ID,
	"provider": provider,
	"state": state,
	"phase": phase,
	"result": result,
}).Info("runtime oauth phase transition")
```

- [ ] **Step 5: Run the targeted worker tests to verify they pass**

Run: `go test ./internal/handler -run 'Test(GetCodexOAuthStatus|SubmitCodexOAuthCallback)_' -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add apps/worker/internal/handler/codex.go apps/worker/internal/handler/codex_oauth.go apps/worker/internal/handler/codex_oauth_test.go
git commit -m "feat: add runtime oauth phase and callback contracts"
```

### Task 3: Frontend Test Harness And Shared API Types

**Files:**

- Modify: `apps/web/package.json`
- Modify: `apps/web/vitest.config.ts`
- Modify: `apps/web/src/lib/api/codex.ts`
- Modify: `apps/web/src/lib/api/codex.test.ts`

- [ ] **Step 1: Write the failing frontend tests for API helper typing and restart payloads**

```ts
it("serializes force_restart only when requested", () => {
  // assert request body shape for startCodexOAuth(..., { forceRestart: true })
})

it("exposes runtime oauth status payload types with phase and code", () => {
  // assert compile-time and runtime helper expectations
})

it("serializes callback_url requests and parses accepted/duplicate callback responses", () => {
  // assert callback request/response contract used by the hook
})
```

- [ ] **Step 2: Run the targeted frontend tests to verify they fail**

Run: `pnpm --dir apps/web test -- src/lib/api/codex.test.ts`
Expected: FAIL because the richer API contract and `force_restart` support do not exist.

- [ ] **Step 3: Add component-test support and richer API types**

```ts
// apps/web/vitest.config.ts
export default defineConfig({
  test: { globals: true, environment: "jsdom" },
})
```

```ts
export interface RuntimeOAuthStatusResponse {
  status: "waiting" | "ok" | "error" | "expired"
  phase: RuntimeOAuthPhase
  code?: string
  error?: string
  expiresAt: string
  canRetry: boolean
  supportsManualCallbackImport: boolean
}

export interface RuntimeOAuthCallbackResponse {
  status: "accepted" | "duplicate" | "error"
  phase: RuntimeOAuthPhase
  code?: string
  error?: string
  shouldContinuePolling: boolean
}
```

- [ ] **Step 4: Update `startCodexOAuth` to accept `forceRestart` and return shared metadata**

```ts
export function startCodexOAuth(channelId: number, channelType?: number, options?: { forceRestart?: boolean }) {
  return apiFetch(..., { method: "POST", body: { force_restart: options?.forceRestart ?? false } })
}
```

- [ ] **Step 4.5: Add the callback API helper contract explicitly**

```ts
export function submitCodexOAuthCallback(
  channelId: number,
  callbackUrl: string,
  channelType?: number,
) {
  return apiFetch<{ success: boolean; data: RuntimeOAuthCallbackResponse }>(
    `/api/v1/channel/${channelId}/${prefix}/oauth/callback`,
    { method: "POST", body: { callback_url: callbackUrl } },
  )
}
```

- [ ] **Step 5: Run the targeted frontend tests to verify they pass**

Run: `pnpm --dir apps/web test -- src/lib/api/codex.test.ts`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add apps/web/package.json apps/web/vitest.config.ts apps/web/src/lib/api/codex.ts apps/web/src/lib/api/codex.test.ts
git commit -m "test: prepare frontend oauth contract test harness"
```

### Task 4: Shared Runtime OAuth Hook

**Files:**

- Add: `apps/web/src/pages/model/use-runtime-oauth-session.ts`
- Add: `apps/web/src/pages/model/use-runtime-oauth-session.test.tsx`
- Modify: `apps/web/src/lib/api/codex.ts`

- [ ] **Step 1: Write the failing hook tests for resume, callback validation, and restart**

```tsx
it("restores a pending session from sessionStorage and resumes polling", async () => {})

it("clears stored session and exposes restart-only state when backend returns session_missing", async () => {})

it("keeps duplicate callback submission non-fatal and continues polling", async () => {})

it("surfaces poll_retrying after repeated polling failures and clears it on recovery", async () => {})

it("synthesizes import_stalled after 30s of post-callback waiting", async () => {})

it("calls onCompleted when phase reaches completed", async () => {})

it("keeps paste-from-clipboard failure recoverable", async () => {})
```

- [ ] **Step 2: Run the targeted hook tests to verify they fail**

Run: `pnpm --dir apps/web test -- src/pages/model/use-runtime-oauth-session.test.tsx`
Expected: FAIL because the hook does not exist yet.

- [ ] **Step 3: Implement the hook state machine and storage model**

```ts
export function useRuntimeOAuthSession(input: {
  channelId: number
  channelType?: number
  providerLabel: string
  onCompleted?: () => void
}) {
  // owns polling, sessionStorage, warning synthesis, restart, callback submit
}
```

- [ ] **Step 4: Implement local callback validation and frontend-owned warnings**

```ts
function validateCallbackUrl(raw: string, expectedState?: string) {
  const url = new URL(raw)
  const state = url.searchParams.get("state")?.trim()
  const code = url.searchParams.get("code")?.trim()
  if (!state) return { ok: false, code: "missing_state" as const }
  if (!code && !url.searchParams.get("error")) return { ok: false, code: "missing_code" as const }
  if (expectedState && state !== expectedState)
    return { ok: false, code: "state_mismatch" as const }
  return { ok: true, state, code }
}
```

- [ ] **Step 4.5: Implement polling warning synthesis and dismissal in the hook**

```ts
if (consecutivePollFailures >= 4) {
  setWarning({ code: "poll_retrying", message: t("codex.oauthPollRetrying") })
}

if (pollRecovered) {
  setWarning(null)
}
```

- [ ] **Step 5: Run the targeted hook tests to verify they pass**

Run: `pnpm --dir apps/web test -- src/pages/model/use-runtime-oauth-session.test.tsx`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/lib/api/codex.ts apps/web/src/pages/model/use-runtime-oauth-session.ts apps/web/src/pages/model/use-runtime-oauth-session.test.tsx
git commit -m "feat: add shared runtime oauth session hook"
```

### Task 5: Flow-Specific OAuth Dialog UI

**Files:**

- Add: `apps/web/src/pages/model/oauth-flow-dialog.tsx`
- Add: `apps/web/src/pages/model/oauth-redirect-flow.tsx`
- Add: `apps/web/src/pages/model/oauth-device-code-flow.tsx`
- Add: `apps/web/src/pages/model/oauth-flow-dialog.test.tsx`

- [ ] **Step 1: Write the failing dialog tests for redirect/device rendering**

```tsx
it("renders redirect flow actions and callback import card", () => {})

it("renders device-code flow without callback import UI", () => {})

it("shows popup-blocked guidance when openAuthPage reports failure", () => {})
```

- [ ] **Step 2: Run the targeted dialog tests to verify they fail**

Run: `pnpm --dir apps/web test -- src/pages/model/oauth-flow-dialog.test.tsx`
Expected: FAIL because the components do not exist yet.

- [ ] **Step 3: Implement the presentational components**

```tsx
export function OAuthFlowDialog(props: OAuthFlowDialogProps) {
  return props.flowType === "device_code" ? (
    <OAuthDeviceCodeFlow {...props} />
  ) : (
    <OAuthRedirectFlow {...props} />
  )
}
```

- [ ] **Step 4: Keep presentational boundaries strict**

```tsx
export function OAuthRedirectFlow({
  oauthUrl,
  callbackInput,
  callbackValidation,
  onCallbackInputChange,
  onSubmitCallback,
}: OAuthRedirectFlowProps) {
  // no polling/storage logic here
}
```

- [ ] **Step 5: Run the targeted dialog tests to verify they pass**

Run: `pnpm --dir apps/web test -- src/pages/model/oauth-flow-dialog.test.tsx`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/pages/model/oauth-flow-dialog.tsx apps/web/src/pages/model/oauth-redirect-flow.tsx apps/web/src/pages/model/oauth-device-code-flow.tsx apps/web/src/pages/model/oauth-flow-dialog.test.tsx
git commit -m "feat: split runtime oauth dialog by flow type"
```

### Task 6: Channel Detail Integration, Query Refresh, And Copy

**Files:**

- Modify: `apps/web/src/pages/model/codex-channel-detail.tsx`
- Modify: `apps/web/src/i18n/locales/en/model.json`
- Modify: `apps/web/src/i18n/locales/zh-CN/model.json`
- Modify: `apps/web/src/pages/model/oauth-flow-dialog.test.tsx`

- [ ] **Step 1: Write the failing integration test for completion-driven auth-file refresh**

```tsx
it("invalidates runtime auth file queries when oauth completes", async () => {})
```

- [ ] **Step 2: Run the targeted integration test to verify it fails**

Run: `pnpm --dir apps/web test -- src/pages/model/oauth-flow-dialog.test.tsx`
Expected: FAIL because `CodexChannelDetail` still owns the old inline dialog logic.

- [ ] **Step 3: Replace inline OAuth state in `codex-channel-detail.tsx` with the shared hook/dialog**

```tsx
const oauthSession = useRuntimeOAuthSession({
  channelId,
  channelType,
  providerLabel,
  onCompleted: () => {
    for (const queryKey of codexUploadRefreshQueryKeys(channelId)) {
      void queryClient.invalidateQueries({ queryKey })
    }
  },
})
```

- [ ] **Step 4: Add/refresh i18n for step labels, restart copy, popup guidance, and terminal codes**

```json
"oauthResume": "Continue pending login",
"oauthRestart": "Start over",
"oauthPopupBlocked": "Your browser blocked the login popup. Copy the link and open it manually.",
"oauthSessionMissing": "This login session is no longer available. Start a new one."
```

- [ ] **Step 5: Run the targeted frontend tests to verify they pass**

Run: `pnpm --dir apps/web test -- src/pages/model/oauth-flow-dialog.test.tsx src/pages/model/use-runtime-oauth-session.test.tsx`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/pages/model/codex-channel-detail.tsx apps/web/src/i18n/locales/en/model.json apps/web/src/i18n/locales/zh-CN/model.json apps/web/src/pages/model/oauth-flow-dialog.test.tsx
git commit -m "feat: integrate resumable runtime oauth dialog"
```

### Task 7: Full Verification

**Files:**

- Test: `apps/worker/internal/handler/codex_oauth_test.go`
- Test: `apps/web/src/pages/model/use-runtime-oauth-session.test.tsx`
- Test: `apps/web/src/pages/model/oauth-flow-dialog.test.tsx`

- [ ] **Step 1: Run the focused frontend tests**

Run: `pnpm --dir apps/web test -- src/lib/api/codex.test.ts src/pages/model/use-runtime-oauth-session.test.tsx src/pages/model/oauth-flow-dialog.test.tsx`
Expected: PASS

- [ ] **Step 1.5: Exercise the highest-risk remote redirect happy path manually**

Run:

```bash
# Precondition: run this in a single worker instance or sticky-routing environment.
# Start the app stack however this repo is normally run, then:
# 1. Start a redirect-based OAuth flow from the UI.
# 2. Finish login until the browser lands on localhost.
# 3. Paste the full callback URL back into the dialog.
# 4. Confirm the auth file list refreshes without a page reload.
```

Expected: The dialog reaches `completed` and the auth file list updates immediately.

- [ ] **Step 1.6: Exercise the device-code happy path manually**

Run:

```bash
# Precondition: run this in a single worker instance or sticky-routing environment.
# 1. Start a device-code OAuth flow from the UI.
# 2. Copy the displayed code and open the verification page.
# 3. Finish authorization and wait for the dialog to complete.
```

Expected: The dialog reaches `completed` without showing the callback import card.

- [ ] **Step 1.7: Exercise the refresh-recovery path manually**

Run:

```bash
# Precondition: run this in a single worker instance or sticky-routing environment.
# 1. Start either redirect or device-code flow.
# 2. Refresh the page while the flow is still pending.
# 3. Verify the dialog auto-reopens in resumable mode.
# 4. Either finish successfully or confirm it falls back to restart-only if the worker returns session_missing.
```

Expected: Same-instance refresh resumes cleanly; missing session degrades to a restart path with no stale UI.

- [ ] **Step 2: Run the web build**

Run: `pnpm --dir apps/web build`
Expected: PASS

- [ ] **Step 3: Run the focused worker tests**

Run: `go test ./internal/handler -run 'Test(StartCodexOAuth_|GetCodexOAuthStatus_|SubmitCodexOAuthCallback_)' -count=1`
Expected: PASS

- [ ] **Step 4: Run the worker test suite**

Run: `make -C apps/worker test`
Expected: PASS

- [ ] **Step 5: Run the worker build**

Run: `make -C apps/worker build`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add apps/web apps/worker
git commit -m "feat: optimize runtime oauth flow"
```
