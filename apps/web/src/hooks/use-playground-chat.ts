import type {
  ChatRunnerEvent,
  ChatRunnerSession,
  ManualPendingToolCall,
} from "@/lib/playground/chat-runner"
import type { ChatMessage } from "@/lib/playground/request-builders"
import { useQuery } from "@tanstack/react-query"
import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import {
  createPlaygroundChatCompletion,
  executePlaygroundMcpTool,
  getApiBaseUrl,
  getSettings,
  listApiKeys,
  listGroups,
} from "@/lib/api-client"
import { createChatRunner } from "@/lib/playground/chat-runner"
import { readPlaygroundSettings, writePlaygroundSettings } from "@/lib/playground/persistence"
import { formatToolPayloadForDisplay } from "@/lib/playground/tool-payload"
import { usePlaygroundMcp } from "./use-playground-mcp"

export interface MessageStats {
  latencyMs: number
  firstTokenMs?: number
  inputTokens?: number
  outputTokens?: number
  totalTokens?: number
}

export interface TimelineItem {
  id: string
  callId: string
  alias: string
  title: string
  argumentsObj: Record<string, unknown>
  status: "pending" | "running" | "done" | "error"
  payload?: unknown
  resultText?: string
  error?: string
}

export interface ConversationTurn {
  id: string
  role: "user" | "assistant"
  content: string
}

export function resolveStreamForRequest(stream: boolean, mcpEnabled: boolean): boolean {
  return mcpEnabled ? false : stream
}

export function parseActiveProfileId(raw: string | undefined): number | undefined {
  if (raw === undefined || raw === "0") return undefined
  const n = Number(raw)
  return Number.isNaN(n) || n === 0 ? undefined : n
}

export function derivePlaygroundModels(groups: Array<{ name?: string }>): string[] {
  return Array.from(
    new Set(
      groups
        .map((g) => (typeof g.name === "string" ? g.name.trim() : ""))
        .filter((name) => name.length > 0),
    ),
  ).sort((a, b) => a.localeCompare(b))
}

export function normalizeConversationMessages(messages: ChatMessage[]): ChatMessage[] {
  return messages.filter((message) => message.role !== "system")
}

export function buildPlaygroundRequestMessages(
  systemPrompt: string,
  historyMessages: ChatMessage[],
): ChatMessage[] {
  const prompt = systemPrompt.trim()
  const normalized = normalizeConversationMessages(historyMessages)
  return [...(prompt ? [{ role: "system" as const, content: prompt }] : []), ...normalized]
}

export function deriveConversationTurns(messages: ChatMessage[]): ConversationTurn[] {
  return normalizeConversationMessages(messages)
    .map((message, index) => {
      if ((message.role !== "user" && message.role !== "assistant") || !message.content) return null
      const content = message.content.trim()
      if (!content) return null
      return {
        id: `${message.role}-${index}`,
        role: message.role,
        content,
      }
    })
    .filter((item): item is ConversationTurn => item !== null)
}

function getLastAssistantText(messages: ChatMessage[]): string {
  for (let i = messages.length - 1; i >= 0; i -= 1) {
    const message = messages[i]
    if (
      message.role === "assistant" &&
      typeof message.content === "string" &&
      message.content.trim()
    ) {
      return message.content
    }
  }
  return ""
}

function toErrorMessage(err: unknown): string {
  if (err instanceof Error && err.message) return err.message
  return "Unknown error"
}

function readToolErrorMessage(payload: unknown): string | null {
  if (!payload || typeof payload !== "object" || Array.isArray(payload)) return null
  const record = payload as Record<string, unknown>
  if (record.isError !== true) return null
  if (typeof record.error === "string" && record.error.trim()) {
    return record.error
  }
  return "Tool execution failed"
}

function toContinuationPayload(item: Pick<TimelineItem, "status" | "payload" | "error">): unknown {
  if (item.status === "error") {
    const existingError = readToolErrorMessage(item.payload)
    if (existingError) {
      return {
        isError: true,
        error: existingError,
      }
    }
    return {
      isError: true,
      error: item.error || "Tool execution failed",
    }
  }
  return item.payload ?? {}
}

export function canContinueManualFromTimeline(
  pendingCalls: ManualPendingToolCall[],
  timeline: TimelineItem[],
): boolean {
  return (
    pendingCalls.length > 0 &&
    pendingCalls.every((call) =>
      timeline.some(
        (x) => x.callId === call.toolCallId && (x.status === "done" || x.status === "error"),
      ),
    )
  )
}

export function buildManualToolOutputs(
  pendingCalls: ManualPendingToolCall[],
  timeline: TimelineItem[],
): Array<{ toolCallId: string; payload: unknown }> {
  return pendingCalls.flatMap((call) => {
    const item = timeline.find(
      (x) => x.callId === call.toolCallId && (x.status === "done" || x.status === "error"),
    )
    if (!item) return []
    return [{ toolCallId: call.toolCallId, payload: toContinuationPayload(item) }]
  })
}

export function mergePendingCallsIntoTimeline(
  timeline: TimelineItem[],
  pendingCalls: ManualPendingToolCall[],
): TimelineItem[] {
  const exists = new Set(timeline.map((x) => x.callId))
  const next = [...timeline]

  for (const call of pendingCalls) {
    if (exists.has(call.toolCallId)) continue
    next.push({
      id: `${call.toolCallId}-${next.length + 1}`,
      callId: call.toolCallId,
      alias: call.alias,
      title: `${call.ref.clientName}.${call.ref.toolName}`,
      argumentsObj: call.argumentsObj,
      status: "pending",
    })
  }

  return next
}

export function usePlaygroundChat() {
  const mcp = usePlaygroundMcp()

  const persistedSettings = useMemo(() => readPlaygroundSettings(), [])

  const [model, setModel] = useState(persistedSettings?.model ?? "")
  const [systemPrompt, setSystemPrompt] = useState(persistedSettings?.systemPrompt ?? "")
  const [userMessage, setUserMessage] = useState("")
  const [customApiKey, setCustomApiKey] = useState("")
  const [stream, setStream] = useState(persistedSettings?.stream ?? true)
  const [temperature, setTemperature] = useState(persistedSettings?.temperature ?? 0.7)
  const [maxTokens, setMaxTokens] = useState(persistedSettings?.maxTokens ?? 4096)
  const [topP, setTopP] = useState(persistedSettings?.topP ?? 1)

  const [response, setResponse] = useState("")
  const [isLoading, setIsLoading] = useState(false)
  const [stats, setStats] = useState<MessageStats | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [sendState, setSendState] = useState<"idle" | "sending" | "failed">("idle")
  const [lastSubmittedMessage, setLastSubmittedMessage] = useState("")
  const [messageHistory, setMessageHistory] = useState<ChatMessage[]>([])

  const [timeline, setTimeline] = useState<TimelineItem[]>([])
  const [pendingSession, setPendingSession] = useState<ChatRunnerSession | null>(null)
  const [pendingCalls, setPendingCalls] = useState<ManualPendingToolCall[]>([])

  const requestAbortRef = useRef<AbortController | null>(null)
  const toolAbortRef = useRef<Map<string, AbortController>>(new Map())

  const { data: settingsData } = useQuery({
    queryKey: ["settings"],
    queryFn: getSettings,
  })
  const activeProfileId = useMemo(
    () => parseActiveProfileId(settingsData?.data?.settings?.active_profile_id),
    [settingsData?.data?.settings?.active_profile_id],
  )

  const { data: groupData } = useQuery({
    queryKey: ["groups", activeProfileId],
    queryFn: () => listGroups(activeProfileId),
    enabled: activeProfileId !== undefined,
  })
  const models = useMemo(
    () => derivePlaygroundModels((groupData?.data?.groups ?? []) as Array<{ name?: string }>),
    [groupData?.data?.groups],
  )

  const { data: apiKeyData } = useQuery({
    queryKey: ["apikeys"],
    queryFn: listApiKeys,
  })
  const defaultApiKey = apiKeyData?.data?.apiKeys?.find((k) => k.enabled)?.apiKey ?? ""
  const authKey = customApiKey || defaultApiKey || ""

  const resolvedStream = resolveStreamForRequest(stream, mcp.enabled)
  const conversation = useMemo(() => deriveConversationTurns(messageHistory), [messageHistory])
  const requestMessagesForCurl = useMemo(() => {
    const draft = userMessage.trim()
    const nextHistory = draft
      ? [...messageHistory, { role: "user" as const, content: draft }]
      : messageHistory
    return buildPlaygroundRequestMessages(systemPrompt, nextHistory)
  }, [messageHistory, systemPrompt, userMessage])

  useEffect(() => {
    writePlaygroundSettings({
      model,
      systemPrompt,
      stream,
      temperature,
      maxTokens,
      topP,
    })
  }, [model, systemPrompt, stream, temperature, maxTokens, topP])

  useEffect(() => {
    if (models.length === 0) return
    if (!model || !models.includes(model)) {
      setModel(models[0])
    }
  }, [models, model])

  const updateTimeline = useCallback((callId: string, patch: Partial<TimelineItem>) => {
    setTimeline((prev) => prev.map((x) => (x.callId === callId ? { ...x, ...patch } : x)))
  }, [])

  const handleRunnerEvent = useCallback(
    (event: ChatRunnerEvent) => {
      if (event.type === "tool-call") {
        setTimeline((prev) => {
          if (prev.some((x) => x.callId === event.call.toolCallId)) return prev
          return [
            ...prev,
            {
              id: `${event.call.toolCallId}-${prev.length + 1}`,
              callId: event.call.toolCallId,
              alias: event.call.alias,
              title: `${event.call.ref.clientName}.${event.call.ref.toolName}`,
              argumentsObj: event.call.argumentsObj,
              status: mcp.mode === "auto" ? "running" : "pending",
            },
          ]
        })
        return
      }

      if (event.type === "tool-result") {
        const errorMessage = readToolErrorMessage(event.payload)
        updateTimeline(event.toolCallId, {
          status: errorMessage ? "error" : "done",
          payload: event.payload,
          resultText: formatToolPayloadForDisplay(event.payload),
          error: errorMessage ?? undefined,
        })
      }
    },
    [mcp.mode, updateTimeline],
  )

  const runStreaming = useCallback(
    async (messages: ChatMessage[], controller: AbortController, startTime: number) => {
      const baseUrl = getApiBaseUrl()
      const url = baseUrl ? `${baseUrl}/v1/chat/completions` : "/v1/chat/completions"
      const resp = await fetch(url, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${authKey}`,
        },
        body: JSON.stringify({
          model,
          messages,
          stream: true,
          temperature,
          max_tokens: maxTokens,
          top_p: topP,
        }),
        signal: controller.signal,
      })

      if (!resp.ok) {
        const errBody = await resp.json().catch(() => ({}))
        throw new Error(
          (errBody as { error?: { message?: string } })?.error?.message || `HTTP ${resp.status}`,
        )
      }

      if (!resp.body) throw new Error("Response body is empty")

      const reader = resp.body.getReader()
      const decoder = new TextDecoder()
      let firstTokenMs: number | undefined
      let text = ""
      let usage:
        | { prompt_tokens?: number; completion_tokens?: number; total_tokens?: number }
        | undefined

      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        const chunk = decoder.decode(value, { stream: true })
        const lines = chunk.split("\n")
        for (const line of lines) {
          if (!line.startsWith("data: ") || line === "data: [DONE]") continue
          try {
            const json = JSON.parse(line.slice(6))
            const delta = json.choices?.[0]?.delta?.content
            if (delta) {
              if (!firstTokenMs) firstTokenMs = performance.now() - startTime
              text += delta
              setResponse(text)
            }
            if (json.usage) usage = json.usage
          } catch {
            // ignore invalid sse line
          }
        }
      }

      return {
        text,
        latencyMs: performance.now() - startTime,
        firstTokenMs,
        usage,
      }
    },
    [authKey, model, temperature, maxTokens, topP],
  )

  const sendWithInput = useCallback(
    async (input: string) => {
      if (!model || !input.trim() || pendingSession) return

      const inputMessage = input.trim()
      setLastSubmittedMessage(inputMessage)

      const nextHistory: ChatMessage[] = [
        ...messageHistory,
        { role: "user", content: inputMessage },
      ]
      const messages = buildPlaygroundRequestMessages(systemPrompt, nextHistory)

      setIsLoading(true)
      setSendState("sending")
      setError(null)
      setStats(null)
      setResponse("")
      setTimeline([])
      setPendingSession(null)
      setPendingCalls([])
      setMessageHistory(nextHistory)
      setUserMessage("")

      const controller = new AbortController()
      requestAbortRef.current = controller
      const startTime = performance.now()

      try {
        if (!mcp.enabled && resolvedStream) {
          const streaming = await runStreaming(messages, controller, startTime)
          setMessageHistory([...nextHistory, { role: "assistant", content: streaming.text }])
          setStats({
            latencyMs: streaming.latencyMs,
            firstTokenMs: streaming.firstTokenMs,
            inputTokens: streaming.usage?.prompt_tokens,
            outputTokens: streaming.usage?.completion_tokens,
            totalTokens: streaming.usage?.total_tokens,
          })
          setSendState("idle")
          return
        }

        const runner = createChatRunner({
          createChatCompletion: createPlaygroundChatCompletion,
          executeTool: executePlaygroundMcpTool,
        })

        const result = await runner.run({
          apiKey: authKey,
          model,
          messages,
          mcpTools: mcp.enabled ? mcp.mcpTools : undefined,
          aliasMap: mcp.enabled ? mcp.aliasMap : undefined,
          mode: mcp.mode,
          signal: controller.signal,
          temperature,
          maxTokens,
          topP,
          onEvent: handleRunnerEvent,
        })

        if (result.status === "completed") {
          const normalized = normalizeConversationMessages(result.messages)
          setMessageHistory(normalized)
          setResponse(result.responseText || getLastAssistantText(normalized))
          setStats({
            latencyMs: performance.now() - startTime,
            inputTokens: result.response?.usage?.prompt_tokens,
            outputTokens: result.response?.usage?.completion_tokens,
            totalTokens: result.response?.usage?.total_tokens,
          })
        } else {
          const normalized = normalizeConversationMessages(result.messages)
          setMessageHistory(normalized)
          setPendingSession(result.session)
          setPendingCalls(result.pendingCalls)
          setResponse(getLastAssistantText(normalized))
          setTimeline((prev) => mergePendingCallsIntoTimeline(prev, result.pendingCalls))
        }
        setSendState("idle")
      } catch (err: unknown) {
        if ((err as Error).name !== "AbortError") {
          setError(toErrorMessage(err))
          setSendState("failed")
        } else {
          setSendState("idle")
        }
      } finally {
        setIsLoading(false)
        if (requestAbortRef.current === controller) {
          requestAbortRef.current = null
        }
      }
    },
    [
      model,
      pendingSession,
      messageHistory,
      systemPrompt,
      mcp,
      resolvedStream,
      runStreaming,
      authKey,
      temperature,
      maxTokens,
      topP,
      handleRunnerEvent,
    ],
  )

  const send = useCallback(async () => {
    if (!userMessage.trim()) return
    await sendWithInput(userMessage)
  }, [userMessage, sendWithInput])

  const retryLast = useCallback(async () => {
    if (!lastSubmittedMessage || isLoading || pendingSession) return
    await sendWithInput(lastSubmittedMessage)
  }, [lastSubmittedMessage, isLoading, pendingSession, sendWithInput])

  const stop = useCallback(() => {
    requestAbortRef.current?.abort()
    requestAbortRef.current = null
    for (const controller of toolAbortRef.current.values()) {
      controller.abort()
    }
    toolAbortRef.current.clear()
    setIsLoading(false)
    setSendState("idle")
  }, [])

  const clear = useCallback(() => {
    setResponse("")
    setError(null)
    setStats(null)
    setTimeline([])
    setPendingSession(null)
    setPendingCalls([])
    setMessageHistory([])
    setUserMessage("")
    setSendState("idle")
    setLastSubmittedMessage("")
  }, [])

  const executePendingTool = useCallback(
    async (toolCallId: string) => {
      const call = pendingCalls.find((x) => x.toolCallId === toolCallId)
      if (!call || !authKey) return

      toolAbortRef.current.get(toolCallId)?.abort()
      const controller = new AbortController()
      toolAbortRef.current.set(toolCallId, controller)

      updateTimeline(toolCallId, { status: "running", error: undefined })
      try {
        const payload = await executePlaygroundMcpTool({
          apiKey: authKey,
          clientId: call.ref.clientId,
          toolName: call.ref.toolName,
          argumentsObj: call.argumentsObj,
          signal: controller.signal,
        })
        const errorMessage = readToolErrorMessage(payload)
        updateTimeline(toolCallId, {
          status: errorMessage ? "error" : "done",
          payload,
          error: errorMessage ?? undefined,
          resultText: formatToolPayloadForDisplay(payload),
        })
      } catch (err: unknown) {
        if ((err as Error).name === "AbortError") return
        const errorMessage = toErrorMessage(err)
        const payload = { isError: true, error: errorMessage }
        updateTimeline(toolCallId, {
          status: "error",
          payload,
          error: errorMessage,
          resultText: formatToolPayloadForDisplay(payload),
        })
      } finally {
        if (toolAbortRef.current.get(toolCallId) === controller) {
          toolAbortRef.current.delete(toolCallId)
        }
      }
    },
    [pendingCalls, authKey, updateTimeline],
  )

  const continueAfterTools = useCallback(async () => {
    if (!pendingSession) return
    const outputs = buildManualToolOutputs(pendingCalls, timeline)
    if (outputs.length < pendingCalls.length) return

    setIsLoading(true)
    setSendState("sending")
    setError(null)
    const startTime = performance.now()
    const controller = new AbortController()
    requestAbortRef.current = controller
    try {
      const runner = createChatRunner({
        createChatCompletion: createPlaygroundChatCompletion,
        executeTool: executePlaygroundMcpTool,
      })
      const next = await runner.continueManual(pendingSession, outputs, handleRunnerEvent, {
        signal: controller.signal,
      })
      if (next.status === "completed") {
        const normalized = normalizeConversationMessages(next.messages)
        setMessageHistory(normalized)
        setPendingSession(null)
        setPendingCalls([])
        setResponse(next.responseText || getLastAssistantText(normalized))
        setStats({
          latencyMs: performance.now() - startTime,
          inputTokens: next.response?.usage?.prompt_tokens,
          outputTokens: next.response?.usage?.completion_tokens,
          totalTokens: next.response?.usage?.total_tokens,
        })
      } else {
        const normalized = normalizeConversationMessages(next.messages)
        setMessageHistory(normalized)
        setPendingSession(next.session)
        setPendingCalls(next.pendingCalls)
        setResponse(getLastAssistantText(normalized))
        setTimeline((prev) => mergePendingCallsIntoTimeline(prev, next.pendingCalls))
      }
      setSendState("idle")
    } catch (err: unknown) {
      if ((err as Error).name !== "AbortError") {
        setError(toErrorMessage(err))
        setSendState("failed")
      } else {
        setSendState("idle")
      }
    } finally {
      setIsLoading(false)
      if (requestAbortRef.current === controller) {
        requestAbortRef.current = null
      }
    }
  }, [pendingSession, pendingCalls, timeline, handleRunnerEvent])

  const canContinueManual = canContinueManualFromTimeline(pendingCalls, timeline)

  return {
    model,
    setModel,
    systemPrompt,
    setSystemPrompt,
    userMessage,
    setUserMessage,
    customApiKey,
    setCustomApiKey,
    stream,
    setStream,
    resolvedStream,
    temperature,
    setTemperature,
    maxTokens,
    setMaxTokens,
    topP,
    setTopP,
    response,
    isLoading,
    sendState,
    stats,
    error,
    conversation,
    models,
    defaultApiKey,
    mcp,
    requestMessagesForCurl,
    hasPendingToolCalls: pendingSession !== null,
    canRetryLast: !!lastSubmittedMessage && !isLoading && pendingSession === null,
    timeline,
    pendingCalls,
    canContinueManual,
    send,
    retryLast,
    stop,
    clear,
    executePendingTool,
    continueAfterTools,
  }
}
