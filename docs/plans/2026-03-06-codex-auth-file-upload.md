# Codex Auth File Upload Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Let users upload a Codex auth `.json` file from Wheel's Codex channel UI.

**Architecture:** Add a channel-scoped worker endpoint that validates the channel, accepts a multipart upload, and forwards the file to the embedded Codex runtime management endpoint. Add a matching frontend API helper and native upload controls in the two Codex entry points, then refresh the auth-file list after success.

**Tech Stack:** Go, Gin, React, TypeScript, TanStack Query, Vitest.

---

### Task 1: Backend upload endpoint

**Files:**

- Modify: `apps/worker/internal/handler/codex.go`
- Modify: `apps/worker/internal/handler/routes.go`
- Test: `apps/worker/internal/handler/codex_test.go`

1. Write a failing test for multipart upload forwarding.
2. Run the worker test to confirm failure.
3. Add a channel-scoped upload handler and route.
4. Run the worker test to confirm pass.

### Task 2: Frontend upload flow

**Files:**

- Modify: `apps/web/src/lib/api/codex.ts`
- Modify: `apps/web/src/pages/model/codex-channel-detail.tsx`
- Modify: `apps/web/src/pages/model/channel-dialog.tsx`
- Modify: `apps/web/src/i18n/locales/en/model.json`
- Modify: `apps/web/src/i18n/locales/zh-CN/model.json`

1. Add a file-upload API helper.
2. Add `.json` upload controls to both Codex UIs.
3. Show success/error feedback and refresh auth files after success.

### Task 3: Verification

**Files:**

- Verify only

1. Run `go test ./...` in `apps/worker`.
2. Run `pnpm --filter @wheel/web build` at repo root.
