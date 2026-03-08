# Runtime Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Move the embedded runtime from a vendored third-party module into Wheel-owned packages without breaking Codex/Copilot management, embedded runtime startup, or OpenAI-compatible APIs.

**Architecture:** Migrate the runtime in layers. First inventory and fence the dependency surface, then move base packages, then bring the embedded runtime boot seam under a Wheel-owned `internal/runtimeapi/cliproxy` package, then continue auth/watcher/executor core, then API and management layers, and only then remove the vendored module boundary. Preserve behavior first; defer cleanup and simplification until after ownership is fully internal.

**Tech Stack:** Go, Gin, Bun, embedded runtime service boot, OpenAI-compatible HTTP handlers, auth manager, watcher, executor pipeline

---

### Task 1: Inventory the current runtime dependency surface

**Files:**

- Modify: `docs/plans/2026-03-08-runtime-integration.md`
- Inspect: `apps/worker/go.mod`
- Inspect: `apps/worker/internal/codexruntime/service.go`
- Inspect: `apps/worker/internal/codexruntime/runtime.go`
- Inspect: `apps/worker/internal/handler/routes.go`
- Inspect: `apps/worker/internal/handler/codex.go`

**Step 1: Write the inventory notes**

Add a short checked list inside this plan covering:

- direct imports of `github.com/router-for-me/CLIProxyAPI/v6/...`
- embedded runtime boot path
- management routes that depend on the runtime
- protocol handlers that must not regress

Inventory checklist:

- [x] Current direct worker-owned imports are limited to:
  - `apps/worker/internal/codexruntime/service.go` importing `github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy`
  - `apps/worker/internal/codexruntime/service.go` importing `github.com/router-for-me/CLIProxyAPI/v6/sdk/config`
  - `apps/worker/internal/handler/codex.go` importing `github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth`
- [x] The current module boundary is still the vendored runtime:
  - `apps/worker/go.mod` requires `github.com/router-for-me/CLIProxyAPI/v6 v6.8.47`
  - `apps/worker/go.mod` replaces it with `./third_party/CLIProxyAPIPlus`
- [x] Current embedded runtime boot path runs through `apps/worker/internal/codexruntime/service.go` and `apps/worker/internal/codexruntime/runtime.go`:
  - `EnsureManagedConfig` writes the managed runtime config and auth directory
  - `ManagedConfigPath` points the generated config at port `8317`
  - `sdkconfig.LoadConfig` reads that config
  - `cliproxy.NewBuilder().WithConfig(...).WithConfigPath(...).WithLocalManagementPassword(...).Build().Run` starts the embedded runtime
- [x] Current runtime-dependent management surface in `apps/worker/internal/handler/routes.go` and `apps/worker/internal/handler/codex.go` includes:
  - auth file list/upload/status/delete and batch variants
  - auth-file model listing, quota listing, and key sync
  - OAuth start/status flows
  - management bridge calls to `/v0/management/auth-files`, `/auth-files/models`, `/api-call`, `/copilot-quota`, `/codex-auth-url`, and `/get-auth-status`
- [x] OpenAI-compatible API behavior that must not regress remains in the vendored runtime for now:
  - routing for `/v1/models`, `/v1/chat/completions`, `/v1/completions`, and `/v1/responses` in `apps/worker/third_party/CLIProxyAPIPlus/internal/api/server.go`
  - handler implementations in `apps/worker/third_party/CLIProxyAPIPlus/sdk/api/handlers/openai/openai_handlers.go` and `apps/worker/third_party/CLIProxyAPIPlus/sdk/api/handlers/openai/openai_responses_handlers.go`
  - regression coverage in `apps/worker/third_party/CLIProxyAPIPlus/sdk/api/handlers/openai/openai_model_validation_test.go`, `apps/worker/third_party/CLIProxyAPIPlus/sdk/api/handlers/openai/openai_stream_sse_test.go`, and `apps/worker/third_party/CLIProxyAPIPlus/sdk/api/handlers/openai/openai_responses_handlers_stream_error_test.go`
- [x] Inventory finding from the verification command: worker-owned direct imports are narrow, while most remaining dependency surface is still inside `apps/worker/third_party/CLIProxyAPIPlus`, including runtime boot, management internals, watcher/auth/executor code, and OpenAI-compatible API handlers/tests targeted by later migration tasks.

**Step 2: Verify the dependency surface**

Run: `rg 'github.com/router-for-me/CLIProxyAPI/v6|third_party/CLIProxyAPIPlus' apps/worker -g'*.go'`

Expected: a concrete list of imports and vendored touchpoints to migrate.

**Step 3: Commit dependency inventory updates**

Do not commit yet unless requested.

### Task 2: Create first-party runtime package skeletons

**Files:**

- Create: `apps/worker/internal/runtimecore/.gitkeep` or first real package files
- Create: `apps/worker/internal/runtimeauth/.gitkeep` or first real package files
- Create: `apps/worker/internal/runtimeapi/.gitkeep` or first real package files
- Create: `apps/worker/internal/runtimectrl/.gitkeep` or first real package files
- Modify: `docs/plans/2026-03-08-runtime-integration.md`

**Step 1: Write the failing check**

Add a plan checklist item asserting the target package layout exists and is referenced by the migration notes.

- [x] Target first-party runtime package roots now exist at `apps/worker/internal/runtimecore`, `apps/worker/internal/runtimeauth`, `apps/worker/internal/runtimeapi`, and `apps/worker/internal/runtimectrl` as minimal Go package entry points for later migration tasks.

**Step 2: Verify the directories do not exist yet**

Run: `ls apps/worker/internal`

Expected: the new runtime ownership package families are absent or incomplete.

**Step 3: Create minimal package roots**

For each package family, add the smallest valid Go file, for example:

```go
package runtimecore
```

Repeat analogously for `runtimeauth`, `runtimeapi`, and `runtimectrl`.

**Step 4: Verify the package roots compile**

Run: `go test ./internal/...`

Expected: PASS with the new empty packages in place.

### Task 3: Migrate shared low-risk packages first

**Files:**

- Modify/Create: packages under `apps/worker/internal/runtimecore/...`
- Modify/Create: packages under `apps/worker/internal/runtimeauth/...`
- Modify/Create: packages under `apps/worker/internal/runtimeapi/...`
- Modify: imports in `apps/worker/internal/codexruntime/service.go`
- Modify: imports in runtime-adjacent worker packages using shared vendored utilities

**Step 1: Write the failing test or compile target**

Choose one low-risk package group first, such as shared interfaces or config helpers, and write/update a test proving the Wheel-owned copy is used.

- [x] First migrated low-risk package group: config helpers, via a Wheel-owned wrapper at `apps/worker/internal/runtimecore/config` wired into `apps/worker/internal/codexruntime/service.go`.

Example scaffold:

```go
func TestRuntimeOwnedConfigLoaderCompiles(t *testing.T) {
	_, err := LoadConfig("testdata/config.yaml")
	if err != nil && !strings.Contains(err.Error(), "no such file") {
		t.Fatal(err)
	}
}
```

**Step 2: Run the narrow test to verify the old path is still the one in use**

Run: `go test ./internal/codexruntime ./internal/handler -run 'TestRuntimeOwned'`

Expected: FAIL or no coverage because the new owned package is not wired yet.

**Step 3: Copy code with minimal edits**

- move shared interfaces/config/access/logging/utility code into the new owned packages
- keep names and behavior stable
- avoid cleanup refactors in this step

**Step 4: Re-run targeted tests**

Run: `go test ./internal/...`

Expected: PASS.

### Task 4: Migrate auth manager, watcher, translator, and executors as one batch

- [x] Reordered seam-first note: embedded runtime startup now moves behind a Wheel-owned `internal/runtimeapi/cliproxy` builder/service seam before continuing deeper auth, watcher, and executor migration.

- Task 4 concrete owned package set: `internal/runtimeauth` now owns auth store/manager primitives; `internal/runtimecore/watcher` owns the auth refresh + OAuth alias-triggered reload loop; `internal/runtimecore/translator` owns the translator registry/pipeline; `internal/runtimecore/executor` owns the GitHub Copilot non-stream execution path used by Wheel runtime integration tests.

**Files:**

- Modify/Create: `apps/worker/internal/runtimeauth/...`
- Modify/Create: `apps/worker/internal/runtimecore/...`
- Modify: `apps/worker/internal/codexruntime/service.go`
- Modify: worker integration files that depend on runtime auth behavior
- Test: migrated auth/watcher/executor tests

**Step 1: Write the failing regression tests**

Add or migrate tests for:

- auth registration/load
- model list refresh
- Copilot/Codex execution path
- OAuth model alias propagation

Example shape:

```go
func TestAuthManagerRefreshesOAuthAliases(t *testing.T) {
	manager := NewManager(nil, nil, nil)
	manager.SetOAuthModelAlias(testAliasConfig())
	if got := manager.ResolveAlias("github-copilot", "claude-opus-4-6"); got != "claude-opus-4.6" {
		t.Fatalf("got %q", got)
	}
}
```

**Step 2: Run only the migrated core tests**

Run: `go test ./internal/runtimeauth/... ./internal/runtimecore/...`

Expected: FAIL before wiring is complete.

**Step 3: Move the core packages without redesigning behavior**

- migrate auth manager and watcher together
- migrate translators and executors in the same batch
- keep behavioral fixes already added by Wheel

**Step 4: Verify core migration**

Run: `go test ./internal/runtimeauth/... ./internal/runtimecore/...`

Expected: PASS.

### Task 5: Migrate API server and protocol handlers

**Files:**

- Modify/Create: `apps/worker/internal/runtimeapi/...`
- Modify/Create: `apps/worker/internal/runtimectrl/...`
- Modify: `apps/worker/internal/codexruntime/service.go`
- Modify: any worker wiring that currently imports vendored API packages

**Step 1: Write the failing protocol tests**

Port or add tests covering:

- chat completions
- completions
- responses
- streaming SSE framing
- missing-model validation

Example test skeleton:

```go
func TestRuntimeOpenAIRejectsMissingModel(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"messages":[{"role":"user","content":"hello"}]}`))
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.Code)
	}
}
```

**Step 2: Run protocol tests to confirm the migration gap**

Run: `go test ./internal/runtimeapi/... -run 'Test.*OpenAI|Test.*Server'`

Expected: FAIL until handlers and server wiring are migrated.

**Step 3: Move API server and handlers**

- migrate OpenAI-compatible handlers first
- migrate management routes and wsrelay after the server core compiles
- update the embedded runtime boot path to import only Wheel-owned runtime packages

**Step 4: Verify protocol behavior**

Run: `go test ./internal/runtimeapi/... ./internal/runtimectrl/... ./internal/codexruntime/...`

Expected: PASS.

### Task 6: Remove the vendored module boundary

**Files:**

- Modify: `apps/worker/go.mod`
- Modify: `apps/worker/go.sum`
- Delete: `apps/worker/third_party/CLIProxyAPIPlus/...`
- Modify: any remaining imports under `apps/worker/...`

**Step 1: Write the failing verification**

Add a verification checklist item asserting that:

- no worker Go file imports `github.com/router-for-me/CLIProxyAPI/v6/...`
- `go.mod` no longer contains the runtime replace line

**Step 2: Verify remaining vendored imports exist before removal**

Run: `rg 'github.com/router-for-me/CLIProxyAPI/v6|third_party/CLIProxyAPIPlus' apps/worker -g'*.go'`

Expected: remaining hits before final cleanup.

**Step 3: Remove the replace and rewrite imports**

- delete the `replace github.com/router-for-me/CLIProxyAPI/v6 => ./third_party/CLIProxyAPIPlus` line from `apps/worker/go.mod`
- update all remaining imports to Wheel-owned packages
- only then delete the vendored tree

**Step 4: Re-run worker-wide tests**

Run: `go test ./...`

Expected: PASS.

### Task 7: End-to-end runtime verification

**Files:**

- No code changes expected

**Step 1: Verify embedded runtime boots**

Run the worker in the normal local development path used by Wheel.

Expected: embedded runtime starts and management endpoints respond.

**Step 2: Verify Wheel entrypoint protocol behavior**

Run:

- `curl http://127.0.0.1:8787/v1/models`
- `curl http://127.0.0.1:8787/v1/chat/completions ...`
- `curl http://127.0.0.1:8787/v1/completions ...`
- `curl http://127.0.0.1:8787/v1/responses ...`

Expected: non-streaming, streaming, tools, and error envelopes remain correct.

**Step 3: Verify runtime direct entrypoint behavior**

Run the same coverage set against `http://127.0.0.1:8317`.

Expected: endpoint behavior matches migrated runtime expectations.

**Step 4: Verify management flows**

Check:

- auth file list/upload/update/delete
- batch auth operations
- quota listing
- sync keys
- OAuth start/status

Expected: PASS.

**Step 5: Commit**

Do not commit yet unless requested.
