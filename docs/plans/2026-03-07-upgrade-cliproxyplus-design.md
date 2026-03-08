> **Note**: This plan has been completed. The vendored module has been absorbed into the worker module. Import paths referenced below are historical.

# Upgrade CLIProxyAPIPlus Design

## Goal

Upgrade the vendored `CLIProxyAPIPlus` runtime in `apps/worker/third_party/CLIProxyAPIPlus` to the latest upstream version so Wheel can inherit newly added runtime models such as `gpt-5.4`, while preserving Wheel's existing Codex/Copilot runtime integrations.

## Current State

- `apps/worker/go.mod` depends on `github.com/router-for-me/CLIProxyAPI/v6`, but the module is redirected with a local `replace` to `./third_party/CLIProxyAPIPlus`.
- Wheel therefore runs against the vendored code, not directly against the published upstream module source.
- The current vendored model registry does not define `gpt-5.4`.
- Wheel has its own runtime-facing integration layer around the vendored runtime, especially in `apps/worker/internal/handler/codex.go` and related runtime UI/API paths.

## Recommended Approach

Upgrade the vendored `CLIProxyAPIPlus` directory to the latest upstream version, then apply only the minimum compatibility fixes needed in Wheel.

This keeps the runtime model catalog aligned with upstream instead of patching individual model definitions locally.

## Scope

### In scope

- refresh vendored `CLIProxyAPIPlus` source to the latest upstream version
- align `apps/worker/go.mod` version metadata with the vendored source version if needed
- fix any compile or integration breakage introduced by the upgrade
- verify the runtime model listing path still works for Codex/Copilot auth files and channel model sync

### Out of scope

- redesigning Wheel's runtime integration architecture
- introducing custom local model definitions for `gpt-5.4`
- changing product behavior unrelated to runtime upgrade regressions

## Design Details

### 1. Vendor-first upgrade

The important artifact is the vendored source tree, not just the version in `go.mod`.

The upgrade should:

- sync `apps/worker/third_party/CLIProxyAPIPlus` to latest upstream
- keep the local `replace` in place
- update `apps/worker/go.mod` to reflect the vendored upstream version when appropriate

This preserves Wheel's current packaging and runtime assumptions.

### 2. Minimal compatibility repair

After syncing upstream, Wheel may fail to compile or tests may fail if APIs, model metadata, or behavior changed.

Any repair work should follow these rules:

- fix only breakages caused by the upgrade
- do not opportunistically refactor unrelated runtime code
- prefer adapting Wheel wrappers rather than diverging further from upstream vendored behavior

### 3. Verification focus

The most important functional checks are:

- runtime auth-file model listing returns the upgraded model catalog
- channel model sync still persists fetched runtime models
- Codex/Copilot runtime quota and auth-file flows still compile and behave as before

## Risks

### Upstream API drift

The latest upstream may rename functions, fields, or assumptions used by Wheel.

Mitigation:

- run focused worker tests first
- patch only the impacted integration points

### Hidden runtime behavior changes

The upgraded runtime may change model filtering, auth handling, or quota payloads.

Mitigation:

- explicitly verify runtime model listing and key Codex handler tests
- inspect changed vendored files around model registry and management handlers if tests expose behavior drift

## Testing Strategy

- run targeted runtime-related worker tests first
- run broader worker verification after fixes
- confirm the vendored source actually includes the desired model definitions such as `gpt-5.4`

## Success Criteria

- vendored `CLIProxyAPIPlus` is upgraded to latest upstream
- Wheel still builds/tests for the affected worker paths
- runtime model listing now exposes `gpt-5.4` if upstream provides it
