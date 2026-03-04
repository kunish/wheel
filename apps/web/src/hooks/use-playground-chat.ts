import type {
  ChatRunnerEvent,
  ChatRunnerSession,
  ManualPendingToolCall,
} from "@/lib/playground/chat-runner"
import type { ChatMessage } from "@/lib/playground/request-builders"
import { useQuery } from "@tanstack/react-query"
import { useCallback, useMemo, useRef, useState } from "react"
import {
  createPlaygroundChatCompletion,
  executePlaygroundMcpTool,
  getApiBaseUrl,
  listApiKeys,
  listGroups,
} from "@/lib/api-client"
import { createChatRunner } from "@/lib/playground/chat-runner"
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

export function resolveStreamForRequest(stream: boolean, mcpEnabled: boolean): boolean {
  return mcpEnabled ? false : stream
}

function toErrorMessage(err: unknown): string {
  if (err instanceof Error && err.message) return err.message
  return "Unknown error"
}

function safeStringify(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
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

  const [model, setModel] = useState("")
  const [systemPrompt, setSystemPrompt] = useState("")
  const [userMessage, setUserMessage] = useState("")
  const [customApiKey, setCustomApiKey] = useState("")
  const [stream, setStream] = useState(true)
  const [temperature, setTemperature] = useState(0.7)
  const [maxTokens, setMaxTokens] = useState(4096)
  const [topP, setTopP] = useState(1)

  const [response, setResponse] = useState("")
  const [isLoading, setIsLoading] = useState(false)
  const [stats, setStats] = useState<MessageStats | null>(null)
  const [error, setError] = useState<string | null>(null)

  const [timeline, setTimeline] = useState<TimelineItem[]>([])
  const [pendingSession, setPendingSession] = useState<ChatRunnerSession | null>(null)
  const [pendingCalls, setPendingCalls] = useState<ManualPendingToolCall[]>([])

  const requestAbortRef = useRef<AbortController | null>(null)
  const toolAbortRef = useRef<Map<string, AbortController>>(new Map())

  const { data: groupData } = useQuery({
    queryKey: ["groups"],
    queryFn: () => listGroups(),
  })
  const models = useMemo(
    () => ((groupData?.data?.groups ?? []) as { name: string }[]).map((g) => g.name).sort(),
    [groupData?.data?.groups],
  )

  const { data: apiKeyData } = useQuery({
    queryKey: ["apikeys"],
    queryFn: listApiKeys,
  })
  const defaultApiKey = apiKeyData?.data?.apiKeys?.find((k) => k.enabled)?.apiKey ?? ""
  const authKey = customApiKey || defaultApiKey || ""

  const resolvedStream = resolveStreamForRequest(stream, mcp.enabled)

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
          resultText: safeStringify(event.payload),
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

      setStats({
        latencyMs: performance.now() - startTime,
        firstTokenMs,
        inputTokens: usage?.prompt_tokens,
        outputTokens: usage?.completion_tokens,
        totalTokens: usage?.total_tokens,
      })
    },
    [authKey, model, temperature, maxTokens, topP],
  )

  const send = useCallback(async () => {
    if (!model || !userMessage.trim()) return

    setIsLoading(true)
    setError(null)
    setStats(null)
    setResponse("")
    setTimeline([])
    setPendingSession(null)
    setPendingCalls([])

    const controller = new AbortController()
    requestAbortRef.current = controller
    const startTime = performance.now()

    const messages: ChatMessage[] = [
      ...(systemPrompt ? [{ role: "system" as const, content: systemPrompt }] : []),
      { role: "user" as const, content: userMessage },
    ]

    try {
      if (!mcp.enabled && resolvedStream) {
        await runStreaming(messages, controller, startTime)
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
        setResponse(result.responseText)
        setStats({
          latencyMs: performance.now() - startTime,
          inputTokens: result.response?.usage?.prompt_tokens,
          outputTokens: result.response?.usage?.completion_tokens,
          totalTokens: result.response?.usage?.total_tokens,
        })
      } else {
        setPendingSession(result.session)
        setPendingCalls(result.pendingCalls)
        setTimeline((prev) => mergePendingCallsIntoTimeline(prev, result.pendingCalls))
      }
    } catch (err: unknown) {
      if ((err as Error).name !== "AbortError") {
        setError(toErrorMessage(err))
      }
    } finally {
      setIsLoading(false)
      if (requestAbortRef.current === controller) {
        requestAbortRef.current = null
      }
    }
  }, [
    model,
    userMessage,
    systemPrompt,
    mcp,
    resolvedStream,
    runStreaming,
    authKey,
    temperature,
    maxTokens,
    topP,
    handleRunnerEvent,
  ])

  const stop = useCallback(() => {
    requestAbortRef.current?.abort()
    requestAbortRef.current = null
    for (const controller of toolAbortRef.current.values()) {
      controller.abort()
    }
    toolAbortRef.current.clear()
    setIsLoading(false)
  }, [])

  const clear = useCallback(() => {
    setResponse("")
    setError(null)
    setStats(null)
    setTimeline([])
    setPendingSession(null)
    setPendingCalls([])
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
          resultText: safeStringify(payload),
        })
      } catch (err: unknown) {
        if ((err as Error).name === "AbortError") return
        const errorMessage = toErrorMessage(err)
        const payload = { isError: true, error: errorMessage }
        updateTimeline(toolCallId, {
          status: "error",
          payload,
          error: errorMessage,
          resultText: safeStringify(payload),
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
        setPendingSession(null)
        setPendingCalls([])
        setResponse(next.responseText)
        setStats({
          latencyMs: performance.now() - startTime,
          inputTokens: next.response?.usage?.prompt_tokens,
          outputTokens: next.response?.usage?.completion_tokens,
          totalTokens: next.response?.usage?.total_tokens,
        })
      } else {
        setPendingSession(next.session)
        setPendingCalls(next.pendingCalls)
        setTimeline((prev) => mergePendingCallsIntoTimeline(prev, next.pendingCalls))
      }
    } catch (err: unknown) {
      if ((err as Error).name !== "AbortError") {
        setError(toErrorMessage(err))
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
    stats,
    error,
    models,
    defaultApiKey,
    mcp,
    timeline,
    pendingCalls,
    canContinueManual,
    send,
    stop,
    clear,
    executePendingTool,
    continueAfterTools,
  }
}
