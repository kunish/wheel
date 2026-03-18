import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import * as React from "react"
import { act } from "react"
import { createRoot } from "react-dom/client"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { listCodexAuthFiles, listCodexQuota } from "@/lib/api"
import { getCodexOAuthStatus, startCodexOAuth, submitCodexOAuthCallback } from "@/lib/api/codex"
import { CodexChannelDetail } from "./codex-channel-detail"
import { codexUploadRefreshQueryKeys } from "./codex-query-keys"
import { OAuthFlowDialog } from "./oauth-flow-dialog"
import { getRuntimeOAuthSessionStorageKey } from "./use-runtime-oauth-session"

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api")
  return {
    ...actual,
    listCodexAuthFiles: vi.fn(),
    listCodexQuota: vi.fn(),
  }
})

vi.mock("@/lib/api/codex", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api/codex")>("@/lib/api/codex")
  return {
    ...actual,
    getCodexOAuthStatus: vi.fn(),
    startCodexOAuth: vi.fn(),
    submitCodexOAuthCallback: vi.fn(),
  }
})

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, values?: Record<string, unknown>) => {
      const messages: Record<string, string> = {
        "actions.loading": "Loading",
        "codex.importOAuth": "Import OAuth",
        "codex.oauthCallbackPlaceholder": "Paste the localhost callback URL",
        "codex.oauthCallbackSubmit": "Import callback",
        "codex.oauthCopyCode": "Copy verification code",
        "codex.oauthCopyLink": "Copy authorization link",
        "codex.oauthOpenLink": "Launch browser",
        "runtime.searchAuthFiles": "Search auth files",
        "runtime.noAuthFiles": `No auth files found yet. Import ${String(values?.provider ?? "provider")} accounts with OAuth or upload auth files to continue.`,
        "runtime.authFiles": `${String(values?.provider ?? "Provider")} Auth Files`,
        "runtime.oauth.dialog.deviceCodeTitle": "Verification step",
        "runtime.oauth.dialog.deviceCodeDescription": "Use this code on the verification page.",
        "runtime.oauth.dialog.redirectTitle": "Browser sign-in step",
        "runtime.oauth.dialog.redirectDescription":
          "Finish login in the browser, then paste the callback URL.",
        "runtime.oauth.dialog.redirectHelp": "Retry from a fresh link if the browser flow stalls.",
        "runtime.oauth.dialog.callbackTitle": "Callback import step",
        "runtime.oauth.dialog.callbackDescription": "Paste the full localhost callback here.",
        "runtime.oauth.dialog.pasteFromClipboard": "Paste callback from clipboard",
        "runtime.oauth.dialog.importing": "Importing callback...",
        "runtime.oauth.dialog.popupBlockedTitle": "Popup help",
        "runtime.oauth.dialog.retry": "Retry browser launch",
        "typeLabels.35": "Codex CLI",
      }
      return messages[key] ?? key
    },
  }),
}))

interface RenderResult {
  container: HTMLDivElement
  rerender: (element: React.ReactNode) => Promise<void>
  unmount: () => Promise<void>
}

async function flushMicrotasks() {
  await Promise.resolve()
  await Promise.resolve()
}

async function waitFor(predicate: () => boolean, attempts = 10) {
  for (let index = 0; index < attempts; index += 1) {
    if (predicate()) {
      return true
    }
    await act(async () => {
      await flushMicrotasks()
    })
  }
  return false
}

async function waitForElement(getElement: () => Element | null, attempts = 10) {
  for (let index = 0; index < attempts; index += 1) {
    const element = getElement()
    if (element) {
      return element
    }
    await act(async () => {
      await flushMicrotasks()
    })
  }
  return null
}

async function render(element: React.ReactNode): Promise<RenderResult> {
  const container = document.createElement("div")
  document.body.appendChild(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(element)
    await flushMicrotasks()
  })

  return {
    container,
    async rerender(nextElement) {
      await act(async () => {
        root.render(nextElement)
        await flushMicrotasks()
      })
    },
    async unmount() {
      await act(async () => {
        root.unmount()
        await flushMicrotasks()
      })
      container.remove()
    },
  }
}

async function click(element: Element | null) {
  if (!(element instanceof HTMLElement)) {
    throw new TypeError("Expected HTMLElement")
  }

  await act(async () => {
    element.click()
    await flushMicrotasks()
  })
}

function getButtonsByText(label: string) {
  return Array.from(document.querySelectorAll("button")).filter((button) =>
    button.textContent?.includes(label),
  )
}

function getButtonByText(label: string) {
  return getButtonsByText(label)[0] ?? null
}

function getButtonByLabels(labels: string[]) {
  return (
    Array.from(document.querySelectorAll("button")).find((button) =>
      labels.some((label) => button.textContent?.includes(label)),
    ) ?? null
  )
}

function getInputByPlaceholder(placeholder: string) {
  return document.querySelector(`input[placeholder="${placeholder}"]`)
}

function getCallbackInput() {
  return (
    Array.from(document.querySelectorAll("input")).find((input) => {
      const placeholder = input.getAttribute("placeholder") ?? ""
      return placeholder.includes("callback") || placeholder.includes("localhost:1455")
    }) ?? null
  )
}

function getLinkByText(text: string) {
  return (
    Array.from(document.querySelectorAll("a")).find((link) => link.textContent === text) ?? null
  )
}

async function change(input: Element | null, value: string) {
  if (!(input instanceof HTMLInputElement)) {
    throw new TypeError("Expected HTMLInputElement")
  }

  await act(async () => {
    const descriptor = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")
    descriptor?.set?.call(input, value)
    input.dispatchEvent(new Event("input", { bubbles: true }))
    input.dispatchEvent(new Event("change", { bubbles: true }))
    await flushMicrotasks()
  })
}

function createProps(overrides?: Partial<React.ComponentProps<typeof OAuthFlowDialog>>) {
  return {
    open: true,
    onOpenChange: vi.fn(),
    title: "Import OAuth",
    description: "Finish OAuth in your browser and return here.",
    flowType: "redirect" as const,
    oauthUrl: "https://auth.example.com/oauth/start",
    userCode: "",
    verificationUri: "",
    callbackInput: "",
    callbackValidation: { code: "empty" as const },
    warningCode: undefined,
    warningMessage: undefined,
    canRetry: true,
    isSubmittingCallback: false,
    onOpenAuthPage: vi.fn(() => true),
    onCopyAuthLink: vi.fn(),
    onCopyUserCode: vi.fn(),
    onPasteCallback: vi.fn(),
    onCallbackInputChange: vi.fn(),
    onSubmitCallback: vi.fn(),
    onRetry: vi.fn(),
    ...overrides,
  }
}

describe("oAuthFlowDialog", () => {
  beforeEach(() => {
    ;(
      globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }
    ).IS_REACT_ACT_ENVIRONMENT = true
    vi.clearAllMocks()
    vi.stubGlobal(
      "open",
      vi.fn(() => ({})),
    )
    sessionStorage.clear()
  })

  afterEach(() => {
    document.body.innerHTML = ""
    vi.unstubAllGlobals()
  })

  it("renders redirect flow actions and callback import card", async () => {
    const props = createProps()

    function Harness() {
      const [callbackInput, setCallbackInput] = React.useState("")

      return (
        <OAuthFlowDialog
          {...props}
          callbackInput={callbackInput}
          onCallbackInputChange={(value) => {
            setCallbackInput(value)
            props.onCallbackInputChange(value)
          }}
        />
      )
    }

    const view = await render(<Harness />)

    expect(getButtonByText("Launch browser")).not.toBeNull()
    expect(getButtonByText("Copy authorization link")).not.toBeNull()
    expect(getLinkByText("https://auth.example.com/oauth/start")).not.toBeNull()
    expect(document.body.textContent).toContain("Browser sign-in step")
    expect(document.body.textContent).toContain("Callback import step")
    expect(getButtonByText("Paste callback from clipboard")).not.toBeNull()

    const callbackInput = getInputByPlaceholder("Paste the localhost callback URL")
    await change(callbackInput, "http://localhost:1455/callback?code=abc&state=state-1")
    await click(getButtonByText("Launch browser"))
    await click(getButtonByText("Import callback"))

    expect(props.onCallbackInputChange).toHaveBeenCalledWith(
      "http://localhost:1455/callback?code=abc&state=state-1",
    )
    expect(props.onOpenAuthPage).toHaveBeenCalledTimes(1)
    expect(props.onSubmitCallback).toHaveBeenCalledTimes(1)

    await click(getButtonsByText("Copy authorization link")[0] ?? null)

    expect(props.onCopyAuthLink).toHaveBeenCalledWith("https://auth.example.com/oauth/start")

    await view.unmount()
  })

  it("renders device-code flow without callback import UI", async () => {
    const props = createProps({
      flowType: "device_code",
      userCode: "ABCD-EFGH",
      verificationUri: "https://auth.example.com/verify",
    })

    const view = await render(<OAuthFlowDialog {...props} />)

    expect(document.body.textContent).toContain("Verification step")
    expect(document.body.textContent).toContain("ABCD-EFGH")
    expect(getLinkByText("https://auth.example.com/verify")).not.toBeNull()
    expect(document.body.textContent).not.toContain("Callback import step")
    expect(document.body.textContent).not.toContain("Paste callback from clipboard")

    await click(getButtonsByText("Copy authorization link")[0] ?? null)

    expect(props.onCopyAuthLink).toHaveBeenCalledWith("https://auth.example.com/verify")

    await view.unmount()
  })

  it("shows popup-blocked guidance when opening the auth page fails", async () => {
    function Harness() {
      const [warning, setWarning] = React.useState<{ code?: string; message?: string }>({})

      return (
        <OAuthFlowDialog
          {...createProps({
            warningCode: warning.code,
            warningMessage: warning.message,
            onOpenAuthPage: () => {
              setWarning({
                code: "popup_blocked",
                message: "Popup blocked. Open the link manually or allow popups and try again.",
              })
              return false
            },
          })}
        />
      )
    }

    const view = await render(<Harness />)

    await click(getButtonByText("Launch browser"))

    expect(document.body.textContent).toContain(
      "Popup blocked. Open the link manually or allow popups and try again.",
    )
    expect(document.body.textContent).toContain("Popup help")
    expect(document.body.textContent).toContain("Retry browser launch")

    await view.unmount()
  })

  it("invalidates auth-file refresh queries when callback completion finishes the oauth flow", async () => {
    const channelId = 7
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    })
    const invalidateQueriesSpy = vi
      .spyOn(queryClient, "invalidateQueries")
      .mockResolvedValue(undefined)

    vi.mocked(listCodexAuthFiles).mockResolvedValue({
      success: true,
      data: {
        files: [],
        total: 0,
        page: 1,
        pageSize: 8,
        capabilities: {
          localEnabled: true,
          managementEnabled: true,
          modelsEnabled: true,
          oauthEnabled: true,
        },
      },
    } as Awaited<ReturnType<typeof listCodexAuthFiles>>)
    vi.mocked(listCodexQuota).mockResolvedValue({
      success: true,
      data: {
        items: [],
        total: 0,
        page: 1,
        pageSize: 8,
      },
    } as Awaited<ReturnType<typeof listCodexQuota>>)
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

    const view = await render(
      <QueryClientProvider client={queryClient}>
        <CodexChannelDetail channelId={channelId} channelType={35} />
      </QueryClientProvider>,
    )

    invalidateQueriesSpy.mockClear()

    await click(getButtonByText("Import OAuth"))
    const callbackInput = await waitForElement(() => getCallbackInput())
    await change(callbackInput, "http://localhost:1455/callback?code=abc&state=state-complete")
    await click(getButtonByLabels(["Submit", "Import callback"]))

    expect(submitCodexOAuthCallback).toHaveBeenCalledWith(
      channelId,
      "http://localhost:1455/callback?code=abc&state=state-complete",
      35,
    )

    for (const queryKey of codexUploadRefreshQueryKeys(channelId)) {
      expect(invalidateQueriesSpy).toHaveBeenCalledWith({ queryKey })
    }

    await view.unmount()
    queryClient.clear()
  })

  it("auto-opens the oauth dialog when an active session is restored after refresh", async () => {
    const channelId = 7
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    })

    sessionStorage.setItem(
      getRuntimeOAuthSessionStorageKey(channelId, 35),
      JSON.stringify({
        channelId,
        channelType: 35,
        state: "resume-state",
        flowType: "redirect",
        oauthUrl: "https://auth.example.com/oauth/start",
        supportsManualCallbackImport: true,
        expiresAt: "2099-03-18T10:10:00Z",
      }),
    )
    vi.mocked(listCodexAuthFiles).mockResolvedValue({
      success: true,
      data: {
        files: [],
        total: 0,
        page: 1,
        pageSize: 8,
        capabilities: {
          localEnabled: true,
          managementEnabled: true,
          modelsEnabled: true,
          oauthEnabled: true,
        },
      },
    } as Awaited<ReturnType<typeof listCodexAuthFiles>>)
    vi.mocked(listCodexQuota).mockResolvedValue({
      success: true,
      data: {
        items: [],
        total: 0,
        page: 1,
        pageSize: 8,
      },
    } as Awaited<ReturnType<typeof listCodexQuota>>)
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

    const view = await render(
      <QueryClientProvider client={queryClient}>
        <CodexChannelDetail channelId={channelId} channelType={35} />
      </QueryClientProvider>,
    )

    expect(
      await waitFor(() => document.body.textContent?.includes("Browser sign-in step") === true),
    ).toBe(true)
    expect(document.body.textContent).toContain("Callback import step")

    await view.unmount()
    queryClient.clear()
  })

  it("retries popup-blocked launch without restarting the oauth session", async () => {
    const channelId = 7
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    })
    const openSpy = vi.mocked(window.open)

    vi.mocked(listCodexAuthFiles).mockResolvedValue({
      success: true,
      data: {
        files: [],
        total: 0,
        page: 1,
        pageSize: 8,
        capabilities: {
          localEnabled: true,
          managementEnabled: true,
          modelsEnabled: true,
          oauthEnabled: true,
        },
      },
    } as Awaited<ReturnType<typeof listCodexAuthFiles>>)
    vi.mocked(listCodexQuota).mockResolvedValue({
      success: true,
      data: {
        items: [],
        total: 0,
        page: 1,
        pageSize: 8,
      },
    } as Awaited<ReturnType<typeof listCodexQuota>>)
    vi.mocked(startCodexOAuth).mockResolvedValue({
      success: true,
      data: {
        url: "https://auth.example.com/oauth/start",
        state: "state-popup",
        flowType: "redirect",
        supportsManualCallbackImport: true,
        expiresAt: "2099-03-18T10:10:00Z",
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

    openSpy.mockReturnValueOnce(null).mockReturnValueOnce({} as Window)

    const view = await render(
      <QueryClientProvider client={queryClient}>
        <CodexChannelDetail channelId={channelId} channelType={35} />
      </QueryClientProvider>,
    )

    await click(getButtonByText("Import OAuth"))
    await click(await waitForElement(() => getButtonByText("Launch browser")))
    expect(document.body.textContent).toContain("Popup help")

    await click(getButtonByText("Retry browser launch"))

    expect(startCodexOAuth).toHaveBeenCalledTimes(1)
    expect(openSpy).toHaveBeenCalledTimes(2)
    expect(openSpy).toHaveBeenNthCalledWith(
      2,
      "https://auth.example.com/oauth/start",
      "_blank",
      "noopener,noreferrer",
    )

    await view.unmount()
    queryClient.clear()
  })

  it("refetches quota data when auth search changes", async () => {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    })

    vi.mocked(listCodexAuthFiles).mockResolvedValue({
      success: true,
      data: {
        files: [],
        total: 0,
        page: 1,
        pageSize: 8,
        capabilities: {
          localEnabled: true,
          managementEnabled: true,
          modelsEnabled: true,
          oauthEnabled: true,
        },
      },
    } as Awaited<ReturnType<typeof listCodexAuthFiles>>)
    vi.mocked(listCodexQuota).mockResolvedValue({
      success: true,
      data: {
        items: [],
        total: 0,
        page: 1,
        pageSize: 8,
      },
    } as Awaited<ReturnType<typeof listCodexQuota>>)

    const view = await render(
      <QueryClientProvider client={queryClient}>
        <CodexChannelDetail channelId={7} channelType={35} />
      </QueryClientProvider>,
    )

    expect(listCodexQuota).toHaveBeenCalledTimes(1)

    await change(getInputByPlaceholder("Search auth files"), "alice@example.com")
    await waitFor(() => vi.mocked(listCodexQuota).mock.calls.length >= 2)

    expect(listCodexQuota).toHaveBeenLastCalledWith(
      7,
      expect.objectContaining({ search: "alice@example.com" }),
    )

    await view.unmount()
    queryClient.clear()
  })

  it("invalidates auth-file refresh queries when polling completes a device-code oauth flow", async () => {
    const channelId = 7
    vi.useFakeTimers()

    const queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    })
    const invalidateQueriesSpy = vi
      .spyOn(queryClient, "invalidateQueries")
      .mockResolvedValue(undefined)

    vi.mocked(listCodexAuthFiles).mockResolvedValue({
      success: true,
      data: {
        files: [],
        total: 0,
        page: 1,
        pageSize: 8,
        capabilities: {
          localEnabled: true,
          managementEnabled: true,
          modelsEnabled: true,
          oauthEnabled: true,
        },
      },
    } as Awaited<ReturnType<typeof listCodexAuthFiles>>)
    vi.mocked(listCodexQuota).mockResolvedValue({
      success: true,
      data: {
        items: [],
        total: 0,
        page: 1,
        pageSize: 8,
      },
    } as Awaited<ReturnType<typeof listCodexQuota>>)
    vi.mocked(startCodexOAuth).mockResolvedValue({
      success: true,
      data: {
        url: "https://auth.example.com/oauth/start",
        state: "device-complete",
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

    const view = await render(
      <QueryClientProvider client={queryClient}>
        <CodexChannelDetail channelId={channelId} channelType={35} />
      </QueryClientProvider>,
    )

    invalidateQueriesSpy.mockClear()

    await click(getButtonByText("Import OAuth"))

    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000)
      await flushMicrotasks()
    })

    for (const queryKey of codexUploadRefreshQueryKeys(channelId)) {
      expect(invalidateQueriesSpy).toHaveBeenCalledWith({ queryKey })
    }

    await view.unmount()
    queryClient.clear()
    vi.useRealTimers()
  })
})
