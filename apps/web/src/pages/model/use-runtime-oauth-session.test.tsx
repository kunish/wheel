import { act } from "react"
import { createRoot } from "react-dom/client"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { getCodexOAuthStatus, startCodexOAuth, submitCodexOAuthCallback } from "@/lib/api/codex"
import {
  getRuntimeOAuthSessionStorageKey,
  useRuntimeOAuthSession,
} from "./use-runtime-oauth-session"

vi.mock("@/lib/api/codex", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api/codex")>("@/lib/api/codex")
  return {
    ...actual,
    startCodexOAuth: vi.fn(),
    getCodexOAuthStatus: vi.fn(),
    submitCodexOAuthCallback: vi.fn(),
  }
})

const channelId = 7
const channelType = 35
const providerLabel = "Codex"

type HookResult = ReturnType<typeof useRuntimeOAuthSession>

function createSession(overrides?: Partial<Record<string, string | undefined>>) {
  return {
    channelId,
    channelType,
    state: "resume-state",
    flowType: "redirect" as const,
    oauthUrl: "https://auth.example.com/oauth/start",
    supportsManualCallbackImport: overrides?.supportsManualCallbackImport !== "false",
    userCode: overrides?.userCode ?? undefined,
    verificationUri: overrides?.verificationUri ?? undefined,
    expiresAt: overrides?.expiresAt ?? "2099-03-18T10:10:00Z",
  }
}

async function flushMicrotasks() {
  await Promise.resolve()
  await Promise.resolve()
}

async function advance(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms)
    await flushMicrotasks()
  })
}

async function renderOAuthHook(options?: { onCompleted?: () => void }) {
  const container = document.createElement("div")
  const root = createRoot(container)
  let current: HookResult | undefined

  function Harness() {
    current = useRuntimeOAuthSession({
      channelId,
      channelType,
      providerLabel,
      onCompleted: options?.onCompleted,
    })
    return null
  }

  await act(async () => {
    root.render(<Harness />)
    await flushMicrotasks()
  })

  return {
    get result() {
      if (!current) {
        throw new Error("Hook did not render")
      }
      return current
    },
    async unmount() {
      await act(async () => {
        root.unmount()
        await flushMicrotasks()
      })
    },
  }
}

describe("useRuntimeOAuthSession", () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
    sessionStorage.clear()
    ;(
      globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }
    ).IS_REACT_ACT_ENVIRONMENT = true
    vi.stubGlobal(
      "open",
      vi.fn(() => ({})),
    )
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        readText: vi.fn(),
        writeText: vi.fn(),
      },
    })
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it("restores a pending session from sessionStorage and resumes polling", async () => {
    sessionStorage.setItem(
      getRuntimeOAuthSessionStorageKey(channelId, channelType),
      JSON.stringify(createSession()),
    )
    vi.mocked(getCodexOAuthStatus).mockResolvedValue({
      success: true,
      data: {
        status: "waiting",
        phase: "awaiting_callback",
        expiresAt: "2099-03-18T10:10:00Z",
        canRetry: true,
        supportsManualCallbackImport: true,
      },
    })

    const hook = await renderOAuthHook()

    await advance(3000)

    expect(getCodexOAuthStatus).toHaveBeenCalledWith(channelId, "resume-state", channelType)
    expect(hook.result.status).toBe("waiting")
    expect(hook.result.phase).toBe("awaiting_callback")
    expect(hook.result.oauthUrl).toBe("https://auth.example.com/oauth/start")
  })

  it("clears stored session and exposes restart-only state when backend returns session_missing", async () => {
    sessionStorage.setItem(
      getRuntimeOAuthSessionStorageKey(channelId, channelType),
      JSON.stringify(createSession()),
    )
    vi.mocked(getCodexOAuthStatus).mockResolvedValue({
      success: true,
      data: {
        status: "expired",
        phase: "expired",
        code: "session_missing",
        error: "OAuth session expired or is no longer available on this worker",
        expiresAt: "2099-03-18T10:10:00Z",
        canRetry: true,
        supportsManualCallbackImport: true,
      },
    })

    const hook = await renderOAuthHook()
    await advance(3000)

    expect(
      sessionStorage.getItem(getRuntimeOAuthSessionStorageKey(channelId, channelType)),
    ).toBeNull()
    expect(hook.result.status).toBe("expired")
    expect(hook.result.phase).toBe("expired")
    expect(hook.result.errorCode).toBe("session_missing")
    expect(hook.result.canRetry).toBe(true)
    expect(hook.result.oauthUrl).toBe("")
  })

  it("keeps duplicate callback submission non-fatal and continues polling", async () => {
    vi.mocked(startCodexOAuth).mockResolvedValue({
      success: true,
      data: {
        url: "https://auth.example.com/oauth/start",
        state: "state-1",
        flowType: "redirect",
        supportsManualCallbackImport: true,
        expiresAt: "2099-03-18T10:10:00Z",
      },
    })
    vi.mocked(submitCodexOAuthCallback).mockResolvedValue({
      success: true,
      data: {
        status: "duplicate",
        phase: "callback_received",
        code: "duplicate_callback",
        shouldContinuePolling: true,
      },
    })
    vi.mocked(getCodexOAuthStatus).mockResolvedValue({
      success: true,
      data: {
        status: "waiting",
        phase: "importing_auth_file",
        expiresAt: "2099-03-18T10:10:00Z",
        canRetry: true,
        supportsManualCallbackImport: true,
      },
    })

    const hook = await renderOAuthHook()

    await act(async () => {
      await hook.result.start()
    })
    await act(async () => {
      hook.result.setCallbackInput("http://localhost:1455/callback?code=abc&state=state-1")
    })
    await act(async () => {
      await hook.result.submitCallback()
    })
    await advance(2000)

    expect(submitCodexOAuthCallback).toHaveBeenCalledWith(
      channelId,
      "http://localhost:1455/callback?code=abc&state=state-1",
      channelType,
    )
    expect(hook.result.errorCode).toBeUndefined()
    expect(hook.result.phase).toBe("importing_auth_file")
    expect(getCodexOAuthStatus).toHaveBeenCalledWith(channelId, "state-1", channelType)
  })

  it("clears stored session and degrades to restart-only state when callback submit returns session_missing", async () => {
    vi.mocked(startCodexOAuth).mockResolvedValue({
      success: true,
      data: {
        url: "https://auth.example.com/oauth/start",
        state: "state-missing",
        flowType: "redirect",
        supportsManualCallbackImport: true,
        expiresAt: "2099-03-18T10:10:00Z",
      },
    })
    vi.mocked(submitCodexOAuthCallback).mockResolvedValue({
      success: true,
      data: {
        status: "error",
        phase: "expired",
        code: "session_missing",
        error: "OAuth session expired or is no longer available on this worker",
        shouldContinuePolling: false,
      },
    })
    vi.mocked(getCodexOAuthStatus).mockResolvedValue({
      success: true,
      data: {
        status: "waiting",
        phase: "awaiting_callback",
        expiresAt: "2099-03-18T10:10:00Z",
        canRetry: true,
        supportsManualCallbackImport: true,
      },
    })

    const hook = await renderOAuthHook()

    await act(async () => {
      await hook.result.start()
    })
    await act(async () => {
      hook.result.setCallbackInput("http://localhost:1455/callback?code=abc&state=state-missing")
    })
    await act(async () => {
      await hook.result.submitCallback()
    })

    expect(
      sessionStorage.getItem(getRuntimeOAuthSessionStorageKey(channelId, channelType)),
    ).toBeNull()
    expect(hook.result.status).toBe("expired")
    expect(hook.result.phase).toBe("expired")
    expect(hook.result.errorCode).toBe("session_missing")
    expect(hook.result.canRetry).toBe(true)
    expect(hook.result.oauthUrl).toBe("")
  })

  it("keeps redirect flow active and surfaces callback validation when backend returns a structured callback error", async () => {
    vi.mocked(startCodexOAuth).mockResolvedValue({
      success: true,
      data: {
        url: "https://auth.example.com/oauth/start",
        state: "state-inline-error",
        flowType: "redirect",
        supportsManualCallbackImport: true,
        expiresAt: "2099-03-18T10:10:00Z",
      },
    })
    vi.mocked(submitCodexOAuthCallback).mockResolvedValue({
      success: true,
      data: {
        status: "error",
        phase: "awaiting_callback",
        code: "state_mismatch",
        error: "This callback belongs to a different login attempt. Restart OAuth and try again.",
        shouldContinuePolling: false,
      },
    })

    const hook = await renderOAuthHook()

    await act(async () => {
      await hook.result.start()
    })
    await act(async () => {
      hook.result.setCallbackInput(
        "http://localhost:1455/callback?code=abc&state=state-inline-error",
      )
    })
    await act(async () => {
      await hook.result.submitCallback()
    })
    await advance(4000)

    expect(
      sessionStorage.getItem(getRuntimeOAuthSessionStorageKey(channelId, channelType)),
    ).not.toBeNull()
    expect(hook.result.status).toBe("waiting")
    expect(hook.result.phase).toBe("awaiting_callback")
    expect(hook.result.errorCode).toBeUndefined()
    expect(hook.result.callbackValidation.code).toBe("state_mismatch")
    expect(hook.result.callbackValidation.message).toBe(
      "This callback belongs to a different login attempt. Restart OAuth and try again.",
    )
    expect(getCodexOAuthStatus).toHaveBeenCalledTimes(1)
  })

  it("calls onCompleted and stores completed terminal state when callback submit returns completed", async () => {
    const onCompleted = vi.fn()
    vi.mocked(startCodexOAuth).mockResolvedValue({
      success: true,
      data: {
        url: "https://auth.example.com/oauth/start",
        state: "state-complete",
        flowType: "redirect",
        supportsManualCallbackImport: true,
        expiresAt: "2099-03-18T10:10:00Z",
      },
    })
    vi.mocked(submitCodexOAuthCallback).mockResolvedValue({
      success: true,
      data: {
        status: "accepted",
        phase: "completed",
        shouldContinuePolling: false,
      },
    })

    const hook = await renderOAuthHook({ onCompleted })

    await act(async () => {
      await hook.result.start()
    })
    await act(async () => {
      hook.result.setCallbackInput("http://localhost:1455/callback?code=abc&state=state-complete")
    })
    await act(async () => {
      await hook.result.submitCallback()
    })

    expect(
      sessionStorage.getItem(getRuntimeOAuthSessionStorageKey(channelId, channelType)),
    ).toBeNull()
    expect(hook.result.status).toBe("ok")
    expect(hook.result.phase).toBe("completed")
    expect(hook.result.errorCode).toBeUndefined()
    expect(onCompleted).toHaveBeenCalledTimes(1)
  })

  it("blocks callback submission when there is no active local session state", async () => {
    const hook = await renderOAuthHook()

    await act(async () => {
      hook.result.setCallbackInput("http://localhost:1455/callback?code=abc&state=orphan-state")
    })
    await act(async () => {
      await hook.result.submitCallback()
    })

    expect(submitCodexOAuthCallback).not.toHaveBeenCalled()
    expect(hook.result.callbackValidation.code).toBe("missing_local_session")
  })

  it("stops polling and transitions to callback error state when callback submit returns a non-session terminal error", async () => {
    vi.mocked(startCodexOAuth).mockResolvedValue({
      success: true,
      data: {
        url: "https://auth.example.com/oauth/start",
        state: "state-denied",
        flowType: "redirect",
        supportsManualCallbackImport: true,
        expiresAt: "2099-03-18T10:10:00Z",
      },
    })
    vi.mocked(submitCodexOAuthCallback).mockResolvedValue({
      success: true,
      data: {
        status: "error",
        phase: "failed",
        code: "access_denied",
        error: "Login was canceled",
        shouldContinuePolling: false,
      },
    })

    const hook = await renderOAuthHook()

    await act(async () => {
      await hook.result.start()
    })
    await act(async () => {
      hook.result.setCallbackInput(
        "http://localhost:1455/callback?error=access_denied&state=state-denied",
      )
    })
    await act(async () => {
      await hook.result.submitCallback()
    })
    await advance(4000)

    expect(
      sessionStorage.getItem(getRuntimeOAuthSessionStorageKey(channelId, channelType)),
    ).toBeNull()
    expect(hook.result.status).toBe("error")
    expect(hook.result.phase).toBe("failed")
    expect(hook.result.errorCode).toBe("access_denied")
    expect(hook.result.error).toBe(
      "Login was canceled or permission was denied. Try again if you want to continue.",
    )
    expect(getCodexOAuthStatus).not.toHaveBeenCalled()
  })

  it("clears stored session when polling returns a non-session terminal error", async () => {
    sessionStorage.setItem(
      getRuntimeOAuthSessionStorageKey(channelId, channelType),
      JSON.stringify(createSession()),
    )
    vi.mocked(getCodexOAuthStatus).mockResolvedValue({
      success: true,
      data: {
        status: "error",
        phase: "failed",
        code: "access_denied",
        error: "Login was canceled",
        expiresAt: "2099-03-18T10:10:00Z",
        canRetry: true,
        supportsManualCallbackImport: true,
      },
    })

    const hook = await renderOAuthHook()
    await advance(3000)

    expect(
      sessionStorage.getItem(getRuntimeOAuthSessionStorageKey(channelId, channelType)),
    ).toBeNull()
    expect(hook.result.status).toBe("error")
    expect(hook.result.phase).toBe("failed")
    expect(hook.result.errorCode).toBe("access_denied")
  })

  it("clears stale warnings when polling returns a terminal error", async () => {
    sessionStorage.setItem(
      getRuntimeOAuthSessionStorageKey(channelId, channelType),
      JSON.stringify(createSession()),
    )
    vi.mocked(getCodexOAuthStatus)
      .mockRejectedValueOnce(new Error("network-1"))
      .mockRejectedValueOnce(new Error("network-2"))
      .mockRejectedValueOnce(new Error("network-3"))
      .mockRejectedValueOnce(new Error("network-4"))
      .mockResolvedValue({
        success: true,
        data: {
          status: "error",
          phase: "failed",
          code: "access_denied",
          error: "Login was canceled",
          expiresAt: "2099-03-18T10:10:00Z",
          canRetry: true,
          supportsManualCallbackImport: true,
        },
      })

    const hook = await renderOAuthHook()

    await advance(12000)
    expect(hook.result.warningCode).toBe("poll_retrying")

    await advance(3000)
    expect(hook.result.status).toBe("error")
    expect(hook.result.warningCode).toBeUndefined()
  })

  it("restores supportsManualCallbackImport from stored session explicitly", async () => {
    sessionStorage.setItem(
      getRuntimeOAuthSessionStorageKey(channelId, channelType),
      JSON.stringify(createSession({ supportsManualCallbackImport: "false" })),
    )
    vi.mocked(getCodexOAuthStatus).mockResolvedValue({
      success: true,
      data: {
        status: "waiting",
        phase: "awaiting_callback",
        expiresAt: "2099-03-18T10:10:00Z",
        canRetry: true,
        supportsManualCallbackImport: false,
      },
    })

    const hook = await renderOAuthHook()

    expect(hook.result.supportsManualCallbackImport).toBe(false)
  })

  it("surfaces poll_retrying after repeated polling failures and clears it on recovery", async () => {
    sessionStorage.setItem(
      getRuntimeOAuthSessionStorageKey(channelId, channelType),
      JSON.stringify(createSession()),
    )
    vi.mocked(getCodexOAuthStatus)
      .mockRejectedValueOnce(new Error("network-1"))
      .mockRejectedValueOnce(new Error("network-2"))
      .mockRejectedValueOnce(new Error("network-3"))
      .mockRejectedValueOnce(new Error("network-4"))
      .mockResolvedValue({
        success: true,
        data: {
          status: "waiting",
          phase: "awaiting_callback",
          expiresAt: "2099-03-18T10:10:00Z",
          canRetry: true,
          supportsManualCallbackImport: true,
        },
      })

    const hook = await renderOAuthHook()

    await advance(12000)
    expect(hook.result.warningCode).toBe("poll_retrying")

    await advance(3000)
    expect(hook.result.warningCode).toBeUndefined()
  })

  it("synthesizes import_stalled after 30s of post-callback waiting", async () => {
    vi.mocked(startCodexOAuth).mockResolvedValue({
      success: true,
      data: {
        url: "https://auth.example.com/oauth/start",
        state: "state-2",
        flowType: "redirect",
        supportsManualCallbackImport: true,
        expiresAt: "2099-03-18T10:10:00Z",
      },
    })
    vi.mocked(submitCodexOAuthCallback).mockResolvedValue({
      success: true,
      data: {
        status: "accepted",
        phase: "callback_received",
        shouldContinuePolling: true,
      },
    })
    vi.mocked(getCodexOAuthStatus).mockResolvedValue({
      success: true,
      data: {
        status: "waiting",
        phase: "importing_auth_file",
        expiresAt: "2099-03-18T10:10:00Z",
        canRetry: true,
        supportsManualCallbackImport: true,
      },
    })

    const hook = await renderOAuthHook()

    await act(async () => {
      await hook.result.start()
    })
    await act(async () => {
      hook.result.setCallbackInput("http://localhost:1455/callback?code=abc&state=state-2")
    })
    await act(async () => {
      await hook.result.submitCallback()
    })
    await advance(31000)

    expect(hook.result.phase).toBe("importing_auth_file")
    expect(hook.result.warningCode).toBe("import_stalled")
  })

  it("calls onCompleted when phase reaches completed", async () => {
    const onCompleted = vi.fn()
    vi.mocked(startCodexOAuth).mockResolvedValue({
      success: true,
      data: {
        url: "https://auth.example.com/oauth/start",
        state: "state-3",
        flowType: "device_code",
        supportsManualCallbackImport: false,
        expiresAt: "2099-03-18T10:10:00Z",
        user_code: "ABCD-EFGH",
        verification_uri: "https://auth.example.com/verify",
      },
    })
    vi.mocked(getCodexOAuthStatus).mockResolvedValue({
      success: true,
      data: {
        status: "ok",
        phase: "completed",
        expiresAt: "2099-03-18T10:10:00Z",
        canRetry: false,
        supportsManualCallbackImport: false,
      },
    })

    const hook = await renderOAuthHook({ onCompleted })

    await act(async () => {
      await hook.result.start()
    })
    await advance(3000)

    expect(hook.result.phase).toBe("completed")
    expect(onCompleted).toHaveBeenCalledTimes(1)
  })

  it("keeps paste-from-clipboard failure recoverable during an active oauth session", async () => {
    vi.mocked(startCodexOAuth).mockResolvedValue({
      success: true,
      data: {
        url: "https://auth.example.com/oauth/start",
        state: "clipboard-state",
        flowType: "redirect",
        supportsManualCallbackImport: true,
        expiresAt: "2099-03-18T10:10:00Z",
      },
    })
    vi.mocked(navigator.clipboard.readText).mockRejectedValue(new Error("denied"))

    const hook = await renderOAuthHook()

    await act(async () => {
      await hook.result.start()
    })

    await act(async () => {
      await hook.result.pasteCallbackFromClipboard()
    })

    expect(hook.result.status).not.toBe("failed")
    expect(hook.result.warningCode).toBe("clipboard_read_failed")

    await act(async () => {
      hook.result.setCallbackInput("http://localhost:1455/callback?code=abc&state=clipboard-state")
    })

    expect(hook.result.phase).toBe("awaiting_callback")
    expect(hook.result.callbackInput).toBe(
      "http://localhost:1455/callback?code=abc&state=clipboard-state",
    )
    expect(hook.result.callbackValidation.code).toBe("ok")
  })
})
