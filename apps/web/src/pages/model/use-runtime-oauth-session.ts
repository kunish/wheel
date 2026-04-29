import type {
  RuntimeOAuthCallbackResponse,
  RuntimeOAuthFlowType,
  RuntimeOAuthPhase,
  RuntimeOAuthStatusCode,
  RuntimeOAuthStatusResponse,
} from "@/lib/api/codex"
import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from "react"
import {
  getCodexOAuthStatus,
  runtimeProviderFilter,
  startCodexOAuth,
  submitCodexOAuthCallback,
} from "@/lib/api/codex"

const INITIAL_POLL_INTERVAL_MS = 3000
const POST_CALLBACK_POLL_INTERVAL_MS = 2000
const STALLED_POLL_INTERVAL_MS = 5000
const IMPORT_STALLED_AFTER_MS = 30000

type LocalCallbackValidationCode =
  | "empty"
  | "ok"
  | "invalid_url"
  | "missing_local_session"
  | "missing_state"
  | "missing_code"
  | "state_mismatch"

type CallbackValidationCode = LocalCallbackValidationCode | RuntimeOAuthStatusCode

interface CallbackValidation {
  code: CallbackValidationCode
  message?: string
}

interface RuntimeOAuthWarning {
  code: "poll_retrying" | "import_stalled" | "clipboard_read_failed" | "popup_blocked"
  message: string
}

interface StoredRuntimeOAuthSession {
  channelId: number
  channelType?: number
  state: string
  flowType: RuntimeOAuthFlowType
  oauthUrl: string
  supportsManualCallbackImport: boolean
  userCode?: string
  verificationUri?: string
  expiresAt: string
}

interface RuntimeOAuthSessionState {
  status: RuntimeOAuthStatusResponse["status"] | "idle"
  phase: RuntimeOAuthPhase | "idle"
  flowType?: RuntimeOAuthFlowType
  oauthUrl: string
  userCode: string
  verificationUri: string
  state: string
  expiresAt: string
  supportsManualCallbackImport: boolean
  canRetry: boolean
  error?: string
  errorCode?: string
}

interface RuntimeOAuthState {
  session: RuntimeOAuthSessionState
  restoredFromStorage: boolean
}

type RuntimeOAuthStateAction =
  | { type: "replaceSession"; session: RuntimeOAuthSessionState }
  | { type: "patchSession"; patch: Partial<RuntimeOAuthSessionState> }
  | { type: "setRestoredFromStorage"; restoredFromStorage: boolean }
  | { type: "restoreStoredSession"; stored: StoredRuntimeOAuthSession }

function getInitialState(): RuntimeOAuthSessionState {
  return {
    status: "idle",
    phase: "idle",
    oauthUrl: "",
    userCode: "",
    verificationUri: "",
    state: "",
    expiresAt: "",
    supportsManualCallbackImport: false,
    canRetry: false,
  }
}

function getPhaseForFlow(flowType: RuntimeOAuthFlowType) {
  return flowType === "device_code" ? "awaiting_browser" : "awaiting_callback"
}

function getSessionFromStoredSession(stored: StoredRuntimeOAuthSession): RuntimeOAuthSessionState {
  return {
    status: "waiting",
    phase: getPhaseForFlow(stored.flowType),
    flowType: stored.flowType,
    oauthUrl: stored.oauthUrl,
    userCode: stored.userCode || "",
    verificationUri: stored.verificationUri || "",
    state: stored.state,
    expiresAt: stored.expiresAt,
    supportsManualCallbackImport: stored.supportsManualCallbackImport,
    canRetry: true,
  }
}

function getInitialOAuthState(storageKey: string): RuntimeOAuthState {
  const stored = readStoredSession(storageKey)
  if (!stored || Date.parse(stored.expiresAt) <= Date.now()) {
    return { session: getInitialState(), restoredFromStorage: false }
  }

  return { session: getSessionFromStoredSession(stored), restoredFromStorage: true }
}

function runtimeOAuthStateReducer(
  state: RuntimeOAuthState,
  action: RuntimeOAuthStateAction,
): RuntimeOAuthState {
  switch (action.type) {
    case "replaceSession":
      return { ...state, session: action.session }
    case "patchSession":
      return { ...state, session: { ...state.session, ...action.patch } }
    case "setRestoredFromStorage":
      return { ...state, restoredFromStorage: action.restoredFromStorage }
    case "restoreStoredSession":
      return {
        session: getSessionFromStoredSession(action.stored),
        restoredFromStorage: true,
      }
  }
}

function getValidationMessage(code: Exclude<LocalCallbackValidationCode, "empty" | "ok">) {
  switch (code) {
    case "invalid_url":
      return "The pasted address is not a valid callback URL."
    case "missing_local_session":
      return "This login attempt is no longer active. Start a new OAuth session."
    case "missing_state":
      return "This callback is incomplete. Copy the full browser address and try again."
    case "missing_code":
      return "This callback is incomplete. Copy the full browser address and try again."
    case "state_mismatch":
      return "This callback belongs to a different login attempt. Restart OAuth and try again."
  }
}

function getErrorMessage(code?: string, fallback?: string) {
  switch (code) {
    case "session_missing":
      return "This login attempt is no longer available. Start a new OAuth session."
    case "session_expired":
      return "This login attempt expired. Start a new OAuth session."
    case "access_denied":
      return "Login was canceled or permission was denied. Try again if you want to continue."
    case "auth_import_failed":
      return (
        fallback ||
        "OAuth succeeded, but importing the auth file failed. Start a new OAuth session."
      )
    case "device_code_expired":
      return "The verification code expired. Start a new login attempt."
    case "device_code_rejected":
      return "The verification code was rejected. Start a new login attempt."
    case "state_mismatch":
      return "This callback belongs to a different login attempt. Restart OAuth and try again."
    case "provider_mismatch":
      return "This callback belongs to a different provider. Restart OAuth and try again."
    case "invalid_callback_url":
      return "The pasted callback URL is invalid."
    case "missing_state":
    case "missing_code":
      return "This callback is incomplete. Copy the full browser address and try again."
    default:
      return fallback || "OAuth failed."
  }
}

function getWarningMessage(code: RuntimeOAuthWarning["code"]) {
  switch (code) {
    case "poll_retrying":
      return "Connection is unstable. Retrying OAuth status checks."
    case "import_stalled":
      return "OAuth finished, but auth file import is taking longer than expected."
    case "clipboard_read_failed":
      return "Clipboard access failed. Paste the callback URL manually instead."
    case "popup_blocked":
      return "The auth page was blocked from opening. Use the copied link instead."
  }
}

function validateCallbackUrl(raw: string, expectedState?: string): CallbackValidation {
  const trimmed = raw.trim()
  if (!trimmed) {
    return { code: "empty" }
  }

  let url: URL
  try {
    url = new URL(trimmed)
  } catch {
    return { code: "invalid_url", message: getValidationMessage("invalid_url") }
  }

  const state = url.searchParams.get("state")?.trim()
  if (!state) {
    return { code: "missing_state", message: getValidationMessage("missing_state") }
  }

  const code = url.searchParams.get("code")?.trim()
  const error = url.searchParams.get("error")?.trim()
  if (!code && !error) {
    return { code: "missing_code", message: getValidationMessage("missing_code") }
  }

  if (expectedState && state !== expectedState) {
    return { code: "state_mismatch", message: getValidationMessage("state_mismatch") }
  }

  return { code: "ok" }
}

function readStoredSession(storageKey: string): StoredRuntimeOAuthSession | null {
  const raw = sessionStorage.getItem(storageKey)
  if (!raw) {
    return null
  }

  try {
    const parsed = JSON.parse(raw) as StoredRuntimeOAuthSession
    if (
      !parsed?.state ||
      !parsed?.flowType ||
      !parsed?.oauthUrl ||
      !parsed?.expiresAt ||
      typeof parsed.supportsManualCallbackImport !== "boolean"
    ) {
      return null
    }
    return parsed
  } catch {
    return null
  }
}

export function getRuntimeOAuthSessionStorageKey(channelId: number, channelType?: number) {
  return `runtime-oauth-session:${channelId}:${channelType ?? "default"}:${runtimeProviderFilter(channelType)}`
}

export function useRuntimeOAuthSession(input: {
  channelId: number
  channelType?: number
  providerLabel: string
  onCompleted?: () => void
}) {
  const storageKey = useMemo(
    () => getRuntimeOAuthSessionStorageKey(input.channelId, input.channelType),
    [input.channelId, input.channelType],
  )
  const [{ session, restoredFromStorage }, dispatchOAuthState] = useReducer(
    runtimeOAuthStateReducer,
    storageKey,
    getInitialOAuthState,
  )
  const [callbackInputValue, setCallbackInputValue] = useState("")
  const [callbackValidation, setCallbackValidation] = useState<CallbackValidation>({
    code: "empty",
  })
  const [warning, setWarning] = useState<RuntimeOAuthWarning | null>(null)
  const [isSubmittingCallback, setIsSubmittingCallback] = useState(false)
  const pollTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const pollingActiveRef = useRef(false)
  const consecutivePollFailuresRef = useRef(0)
  const completedRef = useRef(false)
  const postCallbackStartedAtRef = useRef<number | null>(null)

  const clearPollTimeout = useCallback(() => {
    if (pollTimeoutRef.current) {
      clearTimeout(pollTimeoutRef.current)
      pollTimeoutRef.current = null
    }
  }, [])

  const stopPolling = useCallback(() => {
    pollingActiveRef.current = false
    clearPollTimeout()
  }, [clearPollTimeout])

  const clearStoredSession = useCallback(() => {
    sessionStorage.removeItem(storageKey)
  }, [storageKey])

  const persistSession = useCallback(
    (next: StoredRuntimeOAuthSession) => {
      sessionStorage.setItem(storageKey, JSON.stringify(next))
    },
    [storageKey],
  )

  const setWarningCode = useCallback((code: RuntimeOAuthWarning["code"]) => {
    setWarning({ code, message: getWarningMessage(code) })
  }, [])

  const clearWarningCode = useCallback((code?: RuntimeOAuthWarning["code"]) => {
    setWarning((current) => {
      if (!current) {
        return current
      }
      if (!code || current.code === code) {
        return null
      }
      return current
    })
  }, [])

  const setSessionFields = useCallback((patch: Partial<RuntimeOAuthSessionState>) => {
    dispatchOAuthState({ type: "patchSession", patch })
  }, [])

  const setRestoredFromStorage = useCallback((restoredFromStorage: boolean) => {
    dispatchOAuthState({ type: "setRestoredFromStorage", restoredFromStorage })
  }, [])

  const markCompleted = useCallback(() => {
    stopPolling()
    clearStoredSession()
    setRestoredFromStorage(false)
    setSessionFields({
      status: "ok",
      phase: "completed",
      error: undefined,
      errorCode: undefined,
    })
    clearWarningCode()
    if (!completedRef.current) {
      completedRef.current = true
      input.onCompleted?.()
    }
  }, [
    clearStoredSession,
    clearWarningCode,
    input,
    setRestoredFromStorage,
    setSessionFields,
    stopPolling,
  ])

  const expireToRestartOnly = useCallback(
    (errorCode?: string, error?: string, patch?: Partial<RuntimeOAuthSessionState>) => {
      stopPolling()
      clearStoredSession()
      setRestoredFromStorage(false)
      setSessionFields({
        status: "expired",
        phase: "expired",
        oauthUrl: "",
        userCode: "",
        verificationUri: "",
        state: "",
        supportsManualCallbackImport: false,
        canRetry: true,
        errorCode,
        error: getErrorMessage(errorCode, error),
        ...patch,
      })
      clearWarningCode()
    },
    [clearStoredSession, clearWarningCode, setRestoredFromStorage, setSessionFields, stopPolling],
  )

  const setTerminalCallbackError = useCallback(
    (data: Pick<RuntimeOAuthCallbackResponse, "phase" | "code" | "error">) => {
      stopPolling()
      clearStoredSession()
      setRestoredFromStorage(false)
      setSessionFields({
        status: data.phase === "expired" ? "expired" : "error",
        phase: data.phase,
        errorCode: data.code,
        error: getErrorMessage(data.code, data.error),
      })
      clearWarningCode()
    },
    [clearStoredSession, clearWarningCode, setRestoredFromStorage, setSessionFields, stopPolling],
  )

  const setTerminalPollError = useCallback(
    (
      data: Pick<
        RuntimeOAuthStatusResponse,
        | "status"
        | "phase"
        | "code"
        | "error"
        | "expiresAt"
        | "canRetry"
        | "supportsManualCallbackImport"
      >,
    ) => {
      stopPolling()
      clearStoredSession()
      setSessionFields({
        status: data.status,
        phase: data.phase,
        expiresAt: data.expiresAt,
        canRetry: data.canRetry,
        supportsManualCallbackImport: data.supportsManualCallbackImport,
        errorCode: data.code,
        error: getErrorMessage(data.code, data.error),
      })
      clearWarningCode()
    },
    [clearStoredSession, clearWarningCode, setSessionFields, stopPolling],
  )

  const applyTerminalState = useCallback(
    (data: RuntimeOAuthStatusResponse) => {
      if (data.phase === "completed") {
        markCompleted()
        setSessionFields({
          expiresAt: data.expiresAt,
          canRetry: data.canRetry,
          supportsManualCallbackImport: data.supportsManualCallbackImport,
        })
        return true
      }

      if (
        data.status === "expired" ||
        data.phase === "expired" ||
        data.code === "session_missing"
      ) {
        expireToRestartOnly(data.code, data.error, {
          status: data.status,
          phase: data.phase,
          expiresAt: data.expiresAt,
          canRetry: data.canRetry,
        })
        return true
      }

      if (data.status === "error" || data.phase === "failed") {
        setTerminalPollError(data)
        return true
      }

      return false
    },
    [expireToRestartOnly, markCompleted, setSessionFields, setTerminalPollError],
  )

  const applyTerminalCallbackState = useCallback(
    (data: RuntimeOAuthCallbackResponse) => {
      if (data.phase === "completed") {
        markCompleted()
        return true
      }

      if (data.code === "session_missing") {
        expireToRestartOnly(data.code, data.error)
        return true
      }

      if (data.phase === "failed" || data.phase === "expired") {
        setTerminalCallbackError(data)
        return true
      }

      if (data.status === "error") {
        clearPollTimeout()
        postCallbackStartedAtRef.current = null
        setCallbackValidation({
          code: data.code || "invalid_callback_url",
          message: getErrorMessage(data.code, data.error),
        })
        setSessionFields({
          status: "waiting",
          phase: data.phase,
          error: undefined,
          errorCode: undefined,
        })
        return true
      }

      return false
    },
    [
      clearPollTimeout,
      expireToRestartOnly,
      markCompleted,
      setSessionFields,
      setTerminalCallbackError,
    ],
  )

  const pollStatus = useCallback(async () => {
    if (!pollingActiveRef.current || !input.channelId || !session.state) {
      return false
    }

    try {
      const response = await getCodexOAuthStatus(input.channelId, session.state, input.channelType)
      consecutivePollFailuresRef.current = 0
      clearWarningCode("poll_retrying")

      if (applyTerminalState(response.data)) {
        return false
      }

      const nextPhase = response.data.phase
      const isPostCallback =
        nextPhase === "callback_received" || nextPhase === "importing_auth_file"
      if (isPostCallback && postCallbackStartedAtRef.current === null) {
        postCallbackStartedAtRef.current = Date.now()
      }
      if (!isPostCallback) {
        postCallbackStartedAtRef.current = null
        clearWarningCode("import_stalled")
      }

      if (
        isPostCallback &&
        postCallbackStartedAtRef.current !== null &&
        Date.now() - postCallbackStartedAtRef.current >= IMPORT_STALLED_AFTER_MS
      ) {
        setWarningCode("import_stalled")
      }

      setSessionFields({
        status: response.data.status,
        phase: response.data.phase,
        expiresAt: response.data.expiresAt,
        canRetry: response.data.canRetry,
        supportsManualCallbackImport: response.data.supportsManualCallbackImport,
        error: undefined,
        errorCode: undefined,
      })
      return true
    } catch {
      consecutivePollFailuresRef.current += 1
      if (consecutivePollFailuresRef.current >= 4) {
        setWarningCode("poll_retrying")
      }
      return true
    }
  }, [applyTerminalState, clearWarningCode, input, session.state, setSessionFields, setWarningCode])

  const scheduleNextPoll = useCallback(() => {
    if (!pollingActiveRef.current) {
      return
    }

    clearPollTimeout()

    if (!session.state) {
      return
    }

    const postCallbackPhase =
      session.phase === "callback_received" || session.phase === "importing_auth_file"
    const postCallbackAge =
      postCallbackPhase && postCallbackStartedAtRef.current !== null
        ? Date.now() - postCallbackStartedAtRef.current
        : 0
    const delay = postCallbackPhase
      ? postCallbackAge >= IMPORT_STALLED_AFTER_MS
        ? STALLED_POLL_INTERVAL_MS
        : POST_CALLBACK_POLL_INTERVAL_MS
      : INITIAL_POLL_INTERVAL_MS

    pollTimeoutRef.current = setTimeout(() => {
      if (!pollingActiveRef.current) {
        return
      }
      void pollStatus().then((shouldContinue) => {
        if (shouldContinue) {
          scheduleNextPoll()
        }
      })
    }, delay)
  }, [clearPollTimeout, pollStatus, session.phase, session.state])

  const beginPolling = useCallback(() => {
    if (!pollingActiveRef.current || !session.state) {
      return
    }
    scheduleNextPoll()
  }, [scheduleNextPoll, session.state])

  const hydrateFromStart = useCallback(
    (data: {
      url: string
      state: string
      flowType: RuntimeOAuthFlowType
      supportsManualCallbackImport: boolean
      expiresAt: string
      user_code?: string
      verification_uri?: string
    }) => {
      completedRef.current = false
      setRestoredFromStorage(false)
      pollingActiveRef.current = true
      postCallbackStartedAtRef.current = null
      consecutivePollFailuresRef.current = 0
      clearWarningCode()
      const nextSession: RuntimeOAuthSessionState = {
        status: "waiting",
        phase: getPhaseForFlow(data.flowType),
        flowType: data.flowType,
        oauthUrl: data.url,
        userCode: data.user_code || "",
        verificationUri: data.verification_uri || "",
        state: data.state,
        expiresAt: data.expiresAt,
        supportsManualCallbackImport: data.supportsManualCallbackImport,
        canRetry: true,
      }
      dispatchOAuthState({ type: "replaceSession", session: nextSession })
      persistSession({
        channelId: input.channelId,
        channelType: input.channelType,
        state: data.state,
        flowType: data.flowType,
        oauthUrl: data.url,
        supportsManualCallbackImport: data.supportsManualCallbackImport,
        userCode: data.user_code,
        verificationUri: data.verification_uri,
        expiresAt: data.expiresAt,
      })
      setCallbackInputValue("")
      setCallbackValidation({ code: "empty" })
    },
    [clearWarningCode, input.channelId, input.channelType, persistSession, setRestoredFromStorage],
  )

  const startFlow = useCallback(
    async (forceRestart?: boolean) => {
      stopPolling()
      setRestoredFromStorage(false)
      const response = await startCodexOAuth(input.channelId, input.channelType, {
        ...(forceRestart ? { forceRestart: true } : {}),
      })
      hydrateFromStart(response.data)
    },
    [hydrateFromStart, input.channelId, input.channelType, setRestoredFromStorage, stopPolling],
  )

  const setCallbackInput = useCallback(
    (value: string) => {
      setCallbackInputValue(value)
      setCallbackValidation(validateCallbackUrl(value, session.state || undefined))
      clearWarningCode("clipboard_read_failed")
    },
    [clearWarningCode, session.state],
  )

  const submitCallback = useCallback(async () => {
    if (!session.state) {
      setCallbackValidation({
        code: "missing_local_session",
        message: getValidationMessage("missing_local_session"),
      })
      return false
    }

    const validation = validateCallbackUrl(callbackInputValue, session.state || undefined)
    setCallbackValidation(validation)
    if (validation.code !== "ok") {
      return false
    }

    setIsSubmittingCallback(true)
    try {
      const response = await submitCodexOAuthCallback(
        input.channelId,
        callbackInputValue.trim(),
        input.channelType,
      )
      if (applyTerminalCallbackState(response.data)) {
        return true
      }

      if (
        response.data.phase === "callback_received" ||
        response.data.phase === "importing_auth_file"
      ) {
        postCallbackStartedAtRef.current = Date.now()
      }

      setSessionFields({
        phase: response.data.phase,
        status: "waiting",
        error: undefined,
        errorCode: undefined,
      })
      setCallbackValidation({ code: "ok" })
      if (response.data.shouldContinuePolling) {
        scheduleNextPoll()
      }
      return true
    } finally {
      setIsSubmittingCallback(false)
    }
  }, [
    applyTerminalCallbackState,
    callbackInputValue,
    input.channelId,
    input.channelType,
    scheduleNextPoll,
    session.state,
    setSessionFields,
  ])

  const pasteCallbackFromClipboard = useCallback(async () => {
    try {
      const value = await navigator.clipboard.readText()
      setCallbackInput(value)
      return true
    } catch {
      setWarningCode("clipboard_read_failed")
      return false
    }
  }, [setCallbackInput, setWarningCode])

  const openAuthPage = useCallback(() => {
    if (!session.oauthUrl) {
      return false
    }
    const popup = window.open(session.oauthUrl, "_blank", "noopener,noreferrer")
    if (popup === null) {
      setWarningCode("popup_blocked")
      return false
    }
    clearWarningCode("popup_blocked")
    return true
  }, [clearWarningCode, session.oauthUrl, setWarningCode])

  const copyToClipboard = useCallback(async (value: string) => {
    await navigator.clipboard.writeText(value)
  }, [])

  useEffect(() => {
    if (session.state) {
      beginPolling()
      return clearPollTimeout
    }

    return undefined
  }, [beginPolling, clearPollTimeout, session.state])

  useEffect(() => {
    const stored = readStoredSession(storageKey)
    if (!stored) {
      return
    }

    if (Date.parse(stored.expiresAt) <= Date.now()) {
      clearStoredSession()
      return
    }

    completedRef.current = false
    pollingActiveRef.current = true
    dispatchOAuthState({ type: "restoreStoredSession", stored })
  }, [clearStoredSession, storageKey])

  useEffect(() => () => stopPolling(), [stopPolling])

  return {
    flowType: session.flowType,
    phase: session.phase,
    status: session.status,
    error: session.error,
    errorCode: session.errorCode,
    warningCode: warning?.code,
    warningMessage: warning?.message,
    restoredFromStorage,
    oauthUrl: session.oauthUrl,
    userCode: session.userCode,
    verificationUri: session.verificationUri,
    callbackInput: callbackInputValue,
    callbackValidation,
    isSubmittingCallback,
    canRetry: session.canRetry,
    supportsManualCallbackImport: session.supportsManualCallbackImport,
    start: () => startFlow(false),
    restart: () => startFlow(true),
    openAuthPage,
    copyAuthLink: () => copyToClipboard(session.oauthUrl),
    copyUserCode: () => copyToClipboard(session.userCode),
    pasteCallbackFromClipboard,
    setCallbackInput,
    submitCallback,
    dismissWarning: () => clearWarningCode(),
  }
}
