import type {
  CodexAuthUploadBatchResult,
  RuntimeOAuthCallbackResponse,
  RuntimeOAuthStatusCode,
} from "./codex"
import { afterEach, describe, expect, expectTypeOf, it, vi } from "vitest"
import {
  buildCodexAuthUploadFormData,
  getCodexAuthUploadToastState,
  getCodexOAuthStatus,
  startCodexOAuth,
  submitCodexOAuthCallback,
} from "./codex"

function mockJsonResponse(body: unknown, init?: ResponseInit) {
  return new Response(JSON.stringify(body), {
    headers: { "Content-Type": "application/json" },
    ...init,
  })
}

function expectFetchCall(index = 0) {
  const fetchMock = vi.mocked(fetch)
  const call = fetchMock.mock.calls[index]
  expect(call).toBeDefined()
  return {
    input: call[0],
    init: (call[1] ?? {}) as RequestInit,
  }
}

function expectJsonBody(init: RequestInit | undefined, body: unknown) {
  expect(init?.body).toBe(JSON.stringify(body))
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe("codex api helpers", () => {
  it("appends every selected file under repeated files fields", () => {
    const files = [
      new File(["a"], "first.json", { type: "application/json" }),
      new File(["b"], "second.json", { type: "application/json" }),
    ]

    const formData = buildCodexAuthUploadFormData(files)
    const appended = formData.getAll("files")

    expect(appended).toHaveLength(2)
    expect(appended[0]).toBe(files[0])
    expect(appended[1]).toBe(files[1])
    expect(formData.getAll("file")).toEqual([])
  })

  it("returns success toast state for fully successful batches", () => {
    const result: CodexAuthUploadBatchResult = {
      total: 2,
      successCount: 2,
      failedCount: 0,
      results: [
        { name: "a.json", status: "ok" },
        { name: "b.json", status: "ok" },
      ],
    }

    expect(getCodexAuthUploadToastState(result)).toEqual({
      level: "success",
      key: "codex.uploadSummarySuccess",
      values: { total: 2, successCount: 2, failedCount: 0 },
    })
  })

  it("returns partial toast state for mixed batch results", () => {
    const result: CodexAuthUploadBatchResult = {
      total: 3,
      successCount: 2,
      failedCount: 1,
      results: [
        { name: "a.json", status: "ok" },
        { name: "b.json", status: "error", error: "invalid auth file json" },
        { name: "c.json", status: "ok" },
      ],
    }

    expect(getCodexAuthUploadToastState(result)).toEqual({
      level: "info",
      key: "codex.uploadSummaryPartial",
      values: { total: 3, successCount: 2, failedCount: 1 },
    })
  })

  it("returns error toast state when the whole batch fails", () => {
    const result: CodexAuthUploadBatchResult = {
      total: 2,
      successCount: 0,
      failedCount: 2,
      results: [
        { name: "a.json", status: "error", error: "invalid auth file json" },
        { name: "b.json", status: "error", error: "duplicate auth file" },
      ],
    }

    expect(getCodexAuthUploadToastState(result)).toEqual({
      level: "error",
      key: "codex.uploadSummaryError",
      values: { total: 2, successCount: 0, failedCount: 2 },
    })
  })

  it("serializes force_restart only when requested", async () => {
    vi.stubGlobal("fetch", vi.fn())
    vi.mocked(fetch)
      .mockResolvedValueOnce(
        mockJsonResponse({
          success: true,
          data: {
            url: "https://auth.example.com/oauth/start",
            state: "state-1",
            flowType: "redirect",
            supportsManualCallbackImport: true,
            expiresAt: "2026-03-18T10:00:00Z",
          },
        }),
      )
      .mockResolvedValueOnce(
        mockJsonResponse({
          success: true,
          data: {
            url: "https://auth.example.com/oauth/start",
            state: "state-2",
            flowType: "redirect",
            supportsManualCallbackImport: true,
            expiresAt: "2026-03-18T10:05:00Z",
          },
        }),
      )

    await startCodexOAuth(42, 35)
    await startCodexOAuth(42, 35, { forceRestart: true })

    const first = expectFetchCall(0)
    expect(first.input).toBe("/api/v1/channel/42/codex/oauth/start")
    expect(first.init.method).toBe("POST")
    expect(first.init.body).toBeUndefined()

    const second = expectFetchCall(1)
    expect(second.input).toBe("/api/v1/channel/42/codex/oauth/start")
    expect(second.init.method).toBe("POST")
    expectJsonBody(second.init, { force_restart: true })
  })

  it("routes oauth helpers through provider-specific prefixes for channel types 34 35 and 36", async () => {
    vi.stubGlobal("fetch", vi.fn())
    vi.mocked(fetch).mockImplementation(() =>
      Promise.resolve(
        mockJsonResponse({
          success: true,
          data: {
            status: "waiting",
            phase: "awaiting_callback",
            expiresAt: "2026-03-18T10:10:00Z",
            canRetry: true,
            supportsManualCallbackImport: true,
            shouldContinuePolling: true,
            url: "https://auth.example.com/oauth/start",
            state: "state-1",
            flowType: "redirect",
          },
        }),
      ),
    )

    await startCodexOAuth(1, 34, { forceRestart: true })
    await getCodexOAuthStatus(2, "state-2", 35)
    await submitCodexOAuthCallback(3, "http://localhost:1455/callback?code=abc&state=state-3", 36)

    const copilot = expectFetchCall(0)
    expect(copilot.input).toBe("/api/v1/channel/1/copilot/oauth/start")
    expectJsonBody(copilot.init, { force_restart: true })

    const codex = expectFetchCall(1)
    expect(codex.input).toBe("/api/v1/channel/2/codex/oauth/status?state=state-2")
    expect(codex.init.method).toBe("GET")

    const antigravity = expectFetchCall(2)
    expect(antigravity.input).toBe("/api/v1/channel/3/antigravity/oauth/callback")
    expectJsonBody(antigravity.init, {
      callback_url: "http://localhost:1455/callback?code=abc&state=state-3",
    })
  })

  it("exposes runtime oauth status payload types with phase and code", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        mockJsonResponse({
          success: true,
          data: {
            status: "expired",
            phase: "expired",
            code: "session_missing",
            error: "OAuth session expired or is no longer available on this worker",
            expiresAt: "2026-03-18T10:10:00Z",
            canRetry: true,
            supportsManualCallbackImport: true,
          },
        }),
      ),
    )

    await expect(getCodexOAuthStatus(7, "missing-state", 35)).resolves.toEqual({
      success: true,
      data: {
        status: "expired",
        phase: "expired",
        code: "session_missing",
        error: "OAuth session expired or is no longer available on this worker",
        expiresAt: "2026-03-18T10:10:00Z",
        canRetry: true,
        supportsManualCallbackImport: true,
      },
    })
  })

  it("serializes callback_url requests and parses callback-specific responses", async () => {
    vi.stubGlobal("fetch", vi.fn())
    vi.mocked(fetch)
      .mockResolvedValueOnce(
        mockJsonResponse({
          success: true,
          data: {
            status: "accepted",
            phase: "callback_received",
            shouldContinuePolling: true,
          },
        }),
      )
      .mockResolvedValueOnce(
        mockJsonResponse({
          success: true,
          data: {
            status: "duplicate",
            phase: "callback_received",
            code: "duplicate_callback",
            shouldContinuePolling: true,
          },
        }),
      )
      .mockResolvedValueOnce(
        mockJsonResponse({
          success: true,
          data: {
            status: "error",
            phase: "expired",
            code: "session_missing",
            error: "OAuth session expired or is no longer available on this worker",
            shouldContinuePolling: false,
          },
        }),
      )
      .mockResolvedValueOnce(
        mockJsonResponse({
          success: true,
          data: {
            status: "error",
            phase: "awaiting_callback",
            code: "state_mismatch",
            error:
              "This callback belongs to a different login attempt. Restart OAuth and try again.",
            shouldContinuePolling: false,
          },
        }),
      )

    await expect(
      submitCodexOAuthCallback(99, "http://localhost:1455/callback?code=abc&state=state-1", 35),
    ).resolves.toEqual({
      success: true,
      data: {
        status: "accepted",
        phase: "callback_received",
        shouldContinuePolling: true,
      },
    })

    await expect(
      submitCodexOAuthCallback(99, "http://localhost:1455/callback?code=abc&state=state-1", 35),
    ).resolves.toEqual({
      success: true,
      data: {
        status: "duplicate",
        phase: "callback_received",
        code: "duplicate_callback",
        shouldContinuePolling: true,
      },
    })

    await expect(
      submitCodexOAuthCallback(99, "http://localhost:1455/callback?code=abc&state=missing", 35),
    ).resolves.toEqual({
      success: true,
      data: {
        status: "error",
        phase: "expired",
        code: "session_missing",
        error: "OAuth session expired or is no longer available on this worker",
        shouldContinuePolling: false,
      },
    })

    await expect(
      submitCodexOAuthCallback(99, "http://localhost:1455/callback?code=abc&state=wrong-state", 35),
    ).resolves.toEqual({
      success: true,
      data: {
        status: "error",
        phase: "awaiting_callback",
        code: "state_mismatch",
        error: "This callback belongs to a different login attempt. Restart OAuth and try again.",
        shouldContinuePolling: false,
      },
    })

    const first = expectFetchCall(0)
    expect(first.input).toBe("/api/v1/channel/99/codex/oauth/callback")
    expect(first.init.method).toBe("POST")
    expectJsonBody(first.init, {
      callback_url: "http://localhost:1455/callback?code=abc&state=state-1",
    })
  })

  it("narrows callback response codes by status", () => {
    const duplicateResponse: RuntimeOAuthCallbackResponse = {
      status: "duplicate",
      phase: "callback_received",
      code: "duplicate_callback",
      shouldContinuePolling: true,
    }
    const errorResponse: RuntimeOAuthCallbackResponse = {
      status: "error",
      phase: "awaiting_callback",
      code: "state_mismatch",
      error: "This callback belongs to a different login attempt.",
      shouldContinuePolling: false,
    }

    if (duplicateResponse.status === "duplicate") {
      expectTypeOf(duplicateResponse.code).toEqualTypeOf<"duplicate_callback">()
    }

    if (errorResponse.status === "error") {
      expectTypeOf(errorResponse.code).toEqualTypeOf<RuntimeOAuthStatusCode | undefined>()
    }
  })

  it("includes auth_import_failed in runtime oauth status codes", () => {
    const code: RuntimeOAuthStatusCode = "auth_import_failed"

    expect(code).toBe("auth_import_failed")
  })
})
