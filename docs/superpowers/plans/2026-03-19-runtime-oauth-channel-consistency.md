# Runtime OAuth Channel Consistency Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `Codex`, `Copilot`, `CodexCLI`, and `Antigravity` behave consistently across OAuth lifecycle, auth-file ownership, model discovery, alias resolution, and requestability.

**Architecture:** Treat runtime/OAuth consistency as a shared contract with one mapping source for channel type -> OAuth provider -> canonical runtime provider filter -> management endpoint, then harden the surrounding layers to obey that contract. Keep the fix bounded to handler/config/model-discovery/regression-test layers unless failing tests prove an execution-path inconsistency that must be corrected.

**Tech Stack:** Go, Gin handler tests, config sanitization tests in both `runtimecore` and `runtime/corelib`, relay/request regression tests, `make`-driven worker verification.

---

## File Structure

- **Modify** `apps/worker/internal/handler/codex.go`
  - Consolidate and document the shared runtime/OAuth contract helpers for provider filtering and canonicalization.
- **Modify** `apps/worker/internal/handler/codex_oauth.go`
  - Align `start`, `status`, and `callback` behavior to the normative worker-facing contract.
- **Modify** `apps/worker/internal/handler/codex_models.go`
  - Align model discovery, fallback exposure, and requestability behavior.
- **Modify** `apps/worker/internal/handler/codex_test.go`
  - Lock the channel mapping contract and model discovery/provider-filter behavior.
- **Modify** `apps/worker/internal/handler/codex_oauth_test.go`
  - Lock the runtime/OAuth lifecycle contract, including session, callback, and wrong-provider cases.
- **Modify** `apps/worker/internal/runtime/corelib/config/config.go`
- **Modify** `apps/worker/internal/runtime/corelib/config/oauth_model_alias_defaults.go`
- **Modify** `apps/worker/internal/runtime/corelib/config/oauth_model_alias_test.go`
  - Keep corelib alias/default behavior aligned with runtime/OAuth expectations.
- **Modify** `apps/worker/internal/runtimecore/config/sanitize.go`
- **Modify** `apps/worker/internal/runtimecore/config/oauth_model_alias_defaults.go`
- **Add** `apps/worker/internal/runtimecore/config/oauth_model_alias_runtime_oauth_test.go`
  - Mirror runtime/OAuth alias/default behavior in the runtime loader path.
- **Modify** `apps/worker/internal/handler/relay_antigravity_test.go`
- **Modify only if tests prove necessary** `apps/worker/internal/handler/relay_antigravity_request.go`
  - Preserve execution-path parity for Antigravity aliases.

Do not proactively expand into management API modules or unrelated executors unless a failing test shows the bounded scope cannot satisfy the contract.

---

### Task 1: Lock The Shared Runtime/OAuth Channel Contract

**Files:**

- Modify: `apps/worker/internal/handler/codex.go`
- Modify: `apps/worker/internal/handler/codex_oauth.go`
- Modify: `apps/worker/internal/handler/codex_test.go`

- [ ] **Step 1: Write the failing contract tests**

```go
func TestRuntimeOAuthChannelContractMappings(t *testing.T) {}

func TestCanonicalRuntimeProvider_MapsKnownAliases(t *testing.T) {}

func TestRuntimeProviderMatches_DistinguishesCodexAndCodexCLI(t *testing.T) {}
```

Cover at minimum:

- channel type -> management auth endpoint
- channel type -> OAuth provider name
- channel type -> canonical runtime provider filter
- provider canonicalization for `github-copilot/github/copilot`, `codex-cli/openai-codex-cli`, `antigravity/google-antigravity`
- `Codex` vs `CodexCLI` ownership separation even though both use OAuth provider `codex`

- [ ] **Step 2: Run the focused handler tests to verify they fail**

Run: `go test ./apps/worker/internal/handler -run 'Test(RuntimeOAuthChannelContractMappings|CanonicalRuntimeProvider_MapsKnownAliases|RuntimeProviderMatches_DistinguishesCodexAndCodexCLI|ManagementAuthEndpoint)' -count=1`
Expected: FAIL because the contract is not yet fully locked by tests and/or current helpers will need tightening.

- [ ] **Step 3: Implement the minimal shared contract helpers in `codex.go`**

```go
func runtimeProviderFilter(t types.OutboundType) string
func canonicalRuntimeProvider(provider string) string
func runtimeProviderMatches(channelType types.OutboundType, provider string) bool
func oauthProviderForChannelType(t types.OutboundType) string
func managementAuthEndpoint(t types.OutboundType) string
```

If any mapping logic is duplicated across `codex.go` and `codex_oauth.go`, move toward these helpers rather than adding new switch statements.

- [ ] **Step 4: Re-run the focused handler tests to verify they pass**

Run: `go test ./apps/worker/internal/handler -run 'Test(RuntimeOAuthChannelContractMappings|CanonicalRuntimeProvider_MapsKnownAliases|RuntimeProviderMatches_DistinguishesCodexAndCodexCLI|ManagementAuthEndpoint)' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add apps/worker/internal/handler/codex.go apps/worker/internal/handler/codex_oauth.go apps/worker/internal/handler/codex_test.go
git commit -m "test: lock runtime oauth channel mappings"
```

### Task 2: Align OAuth Lifecycle Semantics Across The Four Channels

**Files:**

- Modify: `apps/worker/internal/handler/codex.go`
- Modify: `apps/worker/internal/handler/codex_oauth.go`
- Modify: `apps/worker/internal/handler/codex_oauth_test.go`

- [ ] **Step 1: Write the failing lifecycle tests**

```go
func TestStartCodexOAuth_ReturnsNormativePayloadByChannel(t *testing.T) {}

func TestStartCodexOAuth_UsesStartErrorContract(t *testing.T) {}

func TestGetCodexOAuthStatus_SetsCanRetryByTerminalState(t *testing.T) {}

func TestSubmitCodexOAuthCallback_RedirectValidationFailuresStayAwaitingCallback(t *testing.T) {}

func TestSubmitCodexOAuthCallback_DuplicateIsIdempotent(t *testing.T) {}

func TestGetCodexOAuthStatus_WrongProviderImportsAreRejected(t *testing.T) {}
```

Cover at minimum:

- `Codex`, `Copilot`, `CodexCLI`, `Antigravity` `start` payload differences
- start-time error contract: `runtime_not_configured`, `unsupported_runtime_channel`, `provider_unavailable`, `runtime_session_conflict`
- `canRetry` truth table
- `session_missing`, `session_superseded`, `session_expired`, `auth_import_failed`
- `state_mismatch` / `provider_mismatch`
- duplicate callback idempotency
- Copilot device-code terminal codes

- [ ] **Step 2: Run the focused OAuth lifecycle tests to verify they fail**

Run: `go test ./apps/worker/internal/handler -run 'Test(StartCodexOAuth_|GetCodexOAuthStatus_|SubmitCodexOAuthCallback_)' -count=1`
Expected: FAIL because at least some normative lifecycle semantics are not fully enforced yet.

- [ ] **Step 3: Implement the minimal lifecycle fixes in `codex_oauth.go`**

```go
// keep one worker-facing contract
type codexOAuthSession struct { /* existing fields + required status/code/error state */ }

func serializeCodexOAuthSession(session codexOAuthSession) gin.H
func serializeCodexOAuthTransport(session codexOAuthSession) gin.H
func serializeCodexOAuthCallbackAccepted(session codexOAuthSession) gin.H
func serializeCodexOAuthCallbackError(session codexOAuthSession, continuePolling bool) gin.H
```

Do not redesign the architecture. Fix only the contract gaps exposed by the tests.

- [ ] **Step 4: Re-run the focused OAuth lifecycle tests to verify they pass**

Run: `go test ./apps/worker/internal/handler -run 'Test(StartCodexOAuth_|GetCodexOAuthStatus_|SubmitCodexOAuthCallback_)' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add apps/worker/internal/handler/codex.go apps/worker/internal/handler/codex_oauth.go apps/worker/internal/handler/codex_oauth_test.go
git commit -m "fix: align runtime oauth lifecycle contract"
```

### Task 3: Fix Auth-File Ownership, Legacy Handling, And Model Discovery Consistency

**Files:**

- Modify: `apps/worker/internal/handler/codex_models.go`
- Modify: `apps/worker/internal/handler/codex.go`
- Modify: `apps/worker/internal/handler/codex_auth_store.go`
- Modify: `apps/worker/internal/handler/codex_test.go`
- Modify if needed: `apps/worker/internal/handler/codex_quota.go`

- [ ] **Step 1: Write the failing model/ownership tests**

```go
func TestCollectCodexChannelModels_RespectsCanonicalProviderOwnership(t *testing.T) {}

func TestCollectCodexChannelModels_HidesUnrequestableFallbacks(t *testing.T) {}

func TestSyncCodexChannelModels_FallbacksRemainRequestableByChannel(t *testing.T) {}

func TestListManagedCodexAuthFiles_LegacyAmbiguousProviderIsNonSelectable(t *testing.T) {}

func TestListCodexQuota_UsesCanonicalRuntimeProviderByChannel(t *testing.T) {}
```

Cover at minimum:

- `Codex` vs `CodexCLI` auth-file separation
- malformed or missing provider metadata becomes non-selectable, not newly owned by filename
- fallback models are only exposed when requestable for that channel, covering `Codex`, `CodexCLI`, and `Antigravity`
- quota lookup uses the same canonical provider/filter contract as import and listing for all four runtime/OAuth channels
- Copilot has no fallback model behavior in scope

- [ ] **Step 2: Run the focused model/ownership tests to verify they fail**

Run: `go test ./apps/worker/internal/handler -run 'Test(CollectCodexChannelModels_|SyncCodexChannelModels_|ListManagedCodexAuthFiles_|ListCodexQuota_)' -count=1`
Expected: FAIL because at least some ownership/requestability edge cases are not yet locked down.

- [ ] **Step 3: Implement the minimal ownership and model-discovery fixes**

```go
func canonicalRuntimeProvider(provider string) string
func (h *Handler) collectCodexChannelModels(...)
func defaultAntigravityModels() []string
func parseCodexAuthContent([]byte) (...)
func (h *Handler) importOAuthAuthFilesToDB(...)
```

Rules to implement:

- wrong-provider files are rejected/ignored, never cross-listed
- legacy ambiguous files remain read-only legacy state, not selectable
- fallback models that cannot execute are not exposed

`oauth-excluded-models` is intentionally out of scope for this task unless a failing test proves it contributes to listing/requestability drift for these four runtime/OAuth channels.

- [ ] **Step 4: Re-run the focused model/ownership tests to verify they pass**

Run: `go test ./apps/worker/internal/handler -run 'Test(CollectCodexChannelModels_|SyncCodexChannelModels_|ListManagedCodexAuthFiles_|ListCodexQuota_)' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add apps/worker/internal/handler/codex_models.go apps/worker/internal/handler/codex.go apps/worker/internal/handler/codex_auth_store.go apps/worker/internal/handler/codex_test.go apps/worker/internal/handler/codex_quota.go
git commit -m "fix: enforce runtime oauth auth ownership"
```

### Task 4: Keep Config Alias Behavior Aligned In Both Config Paths

**Files:**

- Modify: `apps/worker/internal/runtime/corelib/config/config.go`
- Modify: `apps/worker/internal/runtime/corelib/config/oauth_model_alias_defaults.go`
- Modify: `apps/worker/internal/runtime/corelib/config/oauth_model_alias_test.go`
- Modify: `apps/worker/internal/runtimecore/config/sanitize.go`
- Modify: `apps/worker/internal/runtimecore/config/oauth_model_alias_defaults.go`
- Add: `apps/worker/internal/runtimecore/config/oauth_model_alias_runtime_oauth_test.go`

- [ ] **Step 1: Write the failing config parity tests**

```go
func TestSanitizeOAuthModelAlias_RuntimeOAuthDefaultsStayAligned(t *testing.T) {}

func TestSanitizeOAuthModelAlias_RuntimeOAuthExplicitDeletionStaysDeleted(t *testing.T) {}

func TestLoadConfig_RuntimeOAuthAliasParity(t *testing.T) {}
```

Cover at minimum:

- absent defaults for the runtime/OAuth channels that have them
- explicit nil/empty markers remain deleted
- alias direction is always `Name` upstream, `Alias` client-facing
- both config packages produce equivalent runtime/OAuth alias behavior

Both live config call paths are intentionally in scope here per the spec’s call-site inventory rule.

- [ ] **Step 2: Run the focused config tests to verify they fail**

Run: `go test ./apps/worker/internal/runtime/corelib/config -run 'TestSanitizeOAuthModelAlias_' -count=1 && go test ./apps/worker/internal/runtimecore/config -run 'Test(LoadConfig_|SanitizeOAuthModelAlias_)' -count=1`
Expected: FAIL because at least one parity gap should appear before the fixes.

- [ ] **Step 3: Implement the minimal parity fixes in both config paths**

```go
func defaultKiroAliases() []OAuthModelAlias
func defaultGitHubCopilotAliases() []OAuthModelAlias
func defaultAntigravityAliases() []OAuthModelAlias
```

Do not expand alias inventory beyond what the tests require for runtime/OAuth consistency.

- [ ] **Step 4: Re-run the focused config tests to verify they pass**

Run: `go test ./apps/worker/internal/runtime/corelib/config -run 'TestSanitizeOAuthModelAlias_' -count=1 && go test ./apps/worker/internal/runtimecore/config -run 'Test(LoadConfig_|SanitizeOAuthModelAlias_)' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add apps/worker/internal/runtime/corelib/config/config.go apps/worker/internal/runtime/corelib/config/oauth_model_alias_defaults.go apps/worker/internal/runtime/corelib/config/oauth_model_alias_test.go apps/worker/internal/runtimecore/config/sanitize.go apps/worker/internal/runtimecore/config/oauth_model_alias_defaults.go apps/worker/internal/runtimecore/config/oauth_model_alias_runtime_oauth_test.go
git commit -m "fix: align runtime oauth alias config behavior"
```

### Task 5: Lock Execution-Path Parity Where Tests Prove It Matters

**Files:**

- Modify: `apps/worker/internal/handler/relay_antigravity_test.go`
- Modify only if the tests prove necessary: `apps/worker/internal/handler/relay_antigravity_request.go`
- Modify only if the tests prove necessary: `apps/worker/internal/handler/relay_copilot_test.go`
- Modify only if the tests prove necessary: `apps/worker/internal/handler/relay_codexcli_test.go`

- [ ] **Step 1: Write the failing execution-path regression tests**

```go
func TestTransformClaudeToGemini_ClientFacingClaudeAliasResolvesToThinkingModel(t *testing.T) {}

func TestTransformClaudeToGemini_UpstreamThinkingModelRemainsStable(t *testing.T) {}
```

If additional runtime/OAuth channels prove to rewrite listed client-facing models at execution time, add one focused regression per affected channel.

- [ ] **Step 2: Run the focused execution-path tests to verify the current boundary**

Run: `go test ./apps/worker/internal/handler -run 'TestTransformClaudeToGemini_|TestAntigravity|TestCopilot|TestCodexCLI' -count=1`
Expected: Either PASS immediately (coverage-only task) or FAIL and expose the exact execution drift.

- [ ] **Step 3: Implement the smallest execution-path fix only if Step 2 fails**

```go
func transformClaudeToGemini(body map[string]any, model string, projectID string) geminiGenerateContentRequestEnvelope
```

Do not broaden into unrelated executor work.

- [ ] **Step 4: Re-run the focused execution-path tests to verify they pass**

Run: `go test ./apps/worker/internal/handler -run 'TestTransformClaudeToGemini_|TestAntigravity|TestCopilot|TestCodexCLI' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add apps/worker/internal/handler/relay_antigravity_test.go apps/worker/internal/handler/relay_antigravity_request.go apps/worker/internal/handler/relay_copilot_test.go apps/worker/internal/handler/relay_codexcli_test.go
git commit -m "test: cover runtime oauth execution parity"
```

### Task 6: Full Verification

**Files:**

- Test: `apps/worker/internal/handler/codex_test.go`
- Test: `apps/worker/internal/handler/codex_oauth_test.go`
- Test: `apps/worker/internal/runtime/corelib/config/oauth_model_alias_test.go`
- Test: `apps/worker/internal/runtimecore/config/oauth_model_alias_runtime_oauth_test.go`
- Test: `apps/worker/internal/handler/relay_antigravity_test.go`

- [ ] **Step 1: Run focused runtime/OAuth handler tests**

Run: `go test ./apps/worker/internal/handler -run 'Test(RuntimeOAuthChannelContractMappings|CanonicalRuntimeProvider_MapsKnownAliases|RuntimeProviderMatches_DistinguishesCodexAndCodexCLI|ManagementAuthEndpoint|StartCodexOAuth_|GetCodexOAuthStatus_|SubmitCodexOAuthCallback_|CollectCodexChannelModels_|SyncCodexChannelModels_|ListManagedCodexAuthFiles_|ListCodexQuota_|TransformClaudeToGemini_|TestAntigravity|TestCopilot|TestCodexCLI)' -count=1`
Expected: PASS

- [ ] **Step 2: Run focused config tests in both config packages**

Run: `go test ./apps/worker/internal/runtime/corelib/config -run 'TestSanitizeOAuthModelAlias_' -count=1 && go test ./apps/worker/internal/runtimecore/config -run 'Test(LoadConfig_|SanitizeOAuthModelAlias_)' -count=1`
Expected: PASS

- [ ] **Step 3: Run the worker test suite**

Run: `make -C apps/worker test`
Expected: PASS

- [ ] **Step 4: Run the worker build**

Run: `make -C apps/worker build`
Expected: PASS

- [ ] **Step 5: Record verification result and stop**

Do not create an extra no-op commit. Completion for this task is fresh verification evidence.
