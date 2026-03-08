> **Note**: This plan has been completed. The vendored module has been absorbed into the worker module. Import paths referenced below are historical.

# Runtime Integration Design

## Goal

Turn the embedded runtime from a vendored third-party tree into an officially owned part of the Wheel codebase, while preserving nearly all existing runtime capabilities and keeping Codex/Copilot, OpenAI-compatible APIs, auth management, and management endpoints stable during migration.

## Current State

- Wheel embeds the runtime through `apps/worker/internal/codexruntime/service.go`.
- The worker module depends on `github.com/router-for-me/CLIProxyAPI/v6` and rewires it locally through `apps/worker/go.mod` with:
  - `replace github.com/router-for-me/CLIProxyAPI/v6 => ./third_party/CLIProxyAPIPlus`
- Wheel-specific fixes currently live in two places:
  - worker-owned relay/runtime glue under `apps/worker/internal/...`
  - vendored runtime patches under `apps/worker/third_party/CLIProxyAPIPlus/...`
- This makes ownership unclear and increases the chance that runtime behavior drifts across duplicate logic paths.

## Problems To Solve

### 1. Ownership is split across Wheel and vendored runtime code

Important runtime behavior is already effectively maintained by Wheel, but the source of truth still sits in a vendored module layout.

### 2. Wheel-specific behavior is hard to consolidate

Recent fixes such as:

- runtime model alias propagation
- Copilot model normalization
- SSE framing correctness
- missing-model validation
- runtime localhost/base-URL safety

show that the practical ownership boundary already belongs to Wheel.

### 3. The replace-based dependency model hides integration risk

As long as the worker still imports the vendored module through `replace`, it is harder to reason about:

- which code is authoritative
- which packages are safe to change
- when a behavior change is a Wheel change versus an upstream sync

## Constraints

- Keep nearly all existing runtime capabilities for now.
- Do not treat this as a broad feature-pruning exercise.
- Keep embedded runtime behavior working during every migration phase.
- Avoid a single massive cutover.
- Keep OpenAI-compatible behavior stable at both:
  - Wheel entrypoint `:8787`
  - embedded runtime entrypoint `:8317`

## Recommended Approach

Use a gradual ownership migration.

Move the runtime into Wheel-owned packages in layers, while preserving behavior and delaying cleanup until imports and startup paths fully depend on Wheel-owned code.

This gives Wheel official ownership without forcing an all-at-once rewrite.

## Design Details

### 1. Ownership model

After migration, the runtime is no longer treated as a third-party vendored dependency. It becomes a Wheel-maintained subsystem with clear internal package boundaries.

Wheel remains the product root. The runtime becomes an implementation subsystem under `apps/worker/internal/...`, not an embedded external project.

### 2. Target package layout

The approved target layout for the migration is:

- `apps/worker/internal/runtimecore/`
  - registry
  - translators
  - executors
  - watcher lifecycle
  - shared interfaces and low-level runtime flow
- `apps/worker/internal/runtimeauth/`
  - auth manager
  - auth storage helpers
  - provider auth flows
  - oauth alias resolution
- `apps/worker/internal/runtimeapi/`
  - API server wiring
  - OpenAI/Claude/Gemini handlers
  - transport formatting and stream helpers
- `apps/worker/internal/runtimectrl/`
  - management APIs
  - runtime administration helpers
  - Wheel-local runtime control extensions
- `apps/worker/internal/codexruntime/`
  - remains the thin embedding layer used by Wheel startup
  - owns config materialization, managed auth file layout, runtime lifecycle glue

This is the target end-state package split, not a claim about the current worker layout. Task 1 inventories the current dependency surface so later tasks can migrate toward this structure deliberately.

### 3. Migration phases

#### Phase 1: Dependency-surface inventory

- document all direct imports of `github.com/router-for-me/CLIProxyAPI/v6/...`
- identify which runtime packages are directly used by Wheel boot, auth management, and relay flows
- freeze the dependency surface so new worker code does not add fresh direct imports into the vendored tree

#### Phase 2: Base layer migration

Move low-risk, low-behavior packages first:

- shared interfaces
- config loading helpers
- registry/model listing primitives
- access helpers
- logging helpers
- pure utility packages

At this point, logic should remain functionally unchanged.

#### Phase 3: Runtime core migration

Move the tightly coupled runtime engine together:

- auth manager
- watcher
- translator pipeline
- executor layer

These components should migrate as one batch because they share state and lifecycle assumptions.

#### Phase 4: API and management migration

Move:

- API server assembly
- OpenAI-compatible handlers
- management API surface
- websocket relay and supporting runtime control routes

Then update `apps/worker/internal/codexruntime/service.go` to depend only on Wheel-owned packages.

#### Phase 5: Remove vendored module boundary

Only after all startup and runtime imports are first-party:

- remove `replace github.com/router-for-me/CLIProxyAPI/v6 => ./third_party/CLIProxyAPIPlus`
- delete `apps/worker/third_party/CLIProxyAPIPlus`
- remove compatibility shims and temporary forwarding code

### 4. Behavior-preservation strategy

This migration should not start as a logic refactor.

The order of priorities is:

1. preserve behavior
2. centralize ownership
3. remove duplication
4. simplify internal design later

That means the first migration steps should prefer:

- code moves
- package renames
- import rewrites
- startup rewiring

and should avoid opportunistic redesigns.

## Risks And Mitigations

### 1. Hidden behavior changes from package moves

Risk:

- registration order
- package-global state
- watcher initialization timing
- default config or header behavior

Mitigation:

- move code with minimal edits first
- keep each phase small and independently verifiable
- avoid mixed package move + logic rewrite in the same step

### 2. Duplicate Wheel-specific logic continues to drift

Risk:

- alias normalization and runtime compatibility fixes remain duplicated across worker and runtime layers

Mitigation:

- after packages are first-party, consolidate Wheel-owned behavior into single shared implementation points

### 3. OpenAI compatibility regressions slip through unit tests

Risk:

- SSE framing
- error envelopes
- tool-call chunk translation
- endpoint-specific request conversion

Mitigation:

- require unit, integration, and real HTTP verification for every migration phase that touches API behavior

### 4. Auth manager / watcher / executor migrations break refresh flows

Risk:

- auths load but model listings do not refresh
- streaming works but token refresh breaks
- management APIs appear healthy while runtime state is stale

Mitigation:

- migrate these components together in one dedicated phase
- verify auth load, model list, and request execution as one acceptance group

### 5. Removing the module replace too early breaks embedded runtime boot

Mitigation:

- do not remove the replace until `apps/worker/internal/codexruntime/service.go` no longer imports the vendored module path at all

## Testing Strategy

### Automated coverage

Keep and migrate existing tests for:

- `apps/worker/internal/handler/...`
- `apps/worker/internal/relay/...`
- `apps/worker/internal/codexruntime/...`
- runtime OpenAI handlers
- server config reload
- auth manager and watcher
- Copilot executor behavior

### End-to-end protocol verification

For every major runtime API migration phase, verify both:

- `http://127.0.0.1:8787`
- `http://127.0.0.1:8317`

Required endpoint coverage:

- `/v1/models`
- `/v1/chat/completions`
- `/v1/completions`
- `/v1/responses`

Required behavior coverage:

- non-streaming
- streaming
- tools
- error envelopes
- SSE correctness

### Management verification

Verify:

- auth file upload/list/update/delete
- bulk auth operations
- quota listing
- sync keys
- OAuth start/status
- config reload effects on aliases and model lists

## Success Criteria

- Wheel embeds and starts the runtime using only Wheel-owned runtime packages.
- `apps/worker/go.mod` no longer uses the vendored `replace` for runtime ownership.
- `apps/worker/third_party/CLIProxyAPIPlus` is removed.
- OpenAI-compatible behavior remains stable at both `:8787` and `:8317`.
- Codex/Copilot management flows continue to work.
- The runtime is clearly maintained as a first-party subsystem of Wheel.

## Out Of Scope

- broad provider pruning in this migration
- redesigning runtime APIs for new product behavior
- changing the public OpenAI-compatible contract beyond compatibility fixes required during migration
- long-term support-policy cleanup for kept-but-noncritical runtime features
