# Codex Runtime Cutover Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove all legacy compatibility terminology so Codex auth/runtime management is expressed purely as Wheel's embedded Codex runtime.

**Architecture:** Collapse the remaining compatibility layer into a single internal Codex runtime model. Keep the existing `/api/v1/channel/:id/codex/...` API surface and `codex_auth_files` persistence model, but delete legacy env/config fallbacks, remove unused runtime toggles, stop exposing runtime config/management settings as external setup, and rewrite docs/examples around Wheel-managed Codex runtime behavior.

**Tech Stack:** Go worker, Bun/TiDB DAL, React web app, Docker Compose, Markdown docs

---

### Task 1: Config Cutover

**Files:**

- Modify: `apps/worker/internal/config/config.go`
- Modify: `apps/worker/internal/config/config_codexruntime_test.go`

**Step 1: Write the failing test**

- Assert `Config` no longer exposes runtime enable/config-path/management URL env compatibility.
- Assert runtime startup policy is always fail-fast.
- Assert removed legacy env names no longer affect config loading.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config`
Expected: FAIL because config still exposes compatibility behavior.

**Step 3: Write minimal implementation**

- Remove compatibility env reads and dead config fields.
- Keep only the strict-startup runtime control if still needed.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config`
Expected: PASS.

### Task 2: Worker Runtime Wiring

**Files:**

- Modify: `apps/worker/cmd/worker/codex_startup.go`
- Modify: `apps/worker/cmd/worker/codex_startup_test.go`
- Modify: `apps/worker/cmd/worker/main.go`
- Modify: `apps/worker/internal/codexruntime/runtime.go`
- Modify: `apps/worker/internal/codexruntime/service.go`
- Modify: `apps/worker/internal/codexruntime/service_test.go`

**Step 1: Update tests for internal-only runtime wiring**

- Remove expectations around external config path / management URL settings.
- Keep tests for strict startup and internal key generation.

**Step 2: Run focused tests**

Run: `go test ./cmd/worker ./internal/codexruntime`
Expected: FAIL before implementation if old fields remain required.

**Step 3: Write minimal implementation**

- Generate and use managed runtime config path internally.
- Fix base directories and filenames to `codex-runtime` naming.
- Remove remaining legacy runtime wording from logs/errors where possible.

**Step 4: Run focused tests to verify it passes**

Run: `go test ./cmd/worker ./internal/codexruntime`
Expected: PASS.

### Task 3: Handler and Supporting Types Cleanup

**Files:**

- Modify: `apps/worker/internal/handler/codex.go`
- Modify: `apps/worker/internal/handler/codex_test.go`
- Modify: `apps/worker/internal/handler/routes.go`
- Modify: `apps/worker/internal/types/models.go`
- Modify: `apps/worker/internal/types/enums.go`

**Step 1: Update tests / assertions for no-compat semantics**

- Remove legacy env wording expectations.
- Keep auth upload, local auth dir, quota parsing, and route tests green.

**Step 2: Run focused tests**

Run: `go test ./internal/handler ./internal/types`
Expected: PASS or fail only for renamed runtime/config references.

**Step 3: Write minimal implementation**

- Replace legacy runtime naming in comments/errors/helpers.
- Keep third-party SDK import paths only where unavoidable.

**Step 4: Run focused tests**

Run: `go test ./internal/handler ./internal/types`
Expected: PASS.

### Task 4: Docs and Example Config Cleanup

**Files:**

- Modify: `.env.example`
- Modify: `docker-compose.yml`
- Modify: `README.md`
- Rename/Modify: `docs/codex-runtime.md`
- Modify: `docs/plans/2026-03-06-codex-auth-file-upload.md`

**Step 1: Rewrite examples**

- Remove obsolete env examples.
- Describe Wheel-managed embedded Codex runtime as the only supported setup.

**Step 2: Verify references**

Run: `grep`/search for removed legacy runtime naming across Wheel-owned docs/config.
Expected: only unavoidable third-party SDK import paths remain in Go code.

### Task 5: Final Verification

**Files:**

- Verify all touched files above

**Step 1: Run worker test suite**

Run: `go test ./...`
Workdir: `apps/worker`
Expected: PASS.

**Step 2: Run web build**

Run: `pnpm --filter @wheel/web build`
Expected: PASS.

**Step 3: Run residue search**

Search for removed legacy runtime naming.
Expected: only third-party SDK import paths remain, not Wheel-owned naming.
