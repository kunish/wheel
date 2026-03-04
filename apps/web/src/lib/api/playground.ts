import { getApiBaseUrl } from "./client"

// ── Playground-specific API helpers ──

export interface PlaygroundChatCompletionInit {
  apiKey: string
  body: Record<string, unknown>
  signal?: AbortSignal
}

interface ApiEnvelope<T> {
  success?: boolean
  data?: T
  error?: unknown
}

export interface PlaygroundMcpToolExecuteResult {
  content?: Array<{ type: string; text?: string }>
  isError?: boolean
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null
  return value as Record<string, unknown>
}

export function parseApiErrorMessage(errBody: unknown, status: number): string {
  const record = asRecord(errBody)
  if (!record) return `HTTP ${status}`

  const topError = record.error
  if (typeof topError === "string" && topError.trim()) return topError

  const nested = asRecord(topError)
  if (nested && typeof nested.message === "string" && nested.message.trim()) {
    return nested.message
  }

  return `HTTP ${status}`
}

function isApiEnvelope(value: unknown): value is ApiEnvelope<unknown> {
  const record = asRecord(value)
  return !!record && ("success" in record || "data" in record || "error" in record)
}

export function unwrapMcpToolExecuteResponse(json: unknown): PlaygroundMcpToolExecuteResult {
  if (isApiEnvelope(json)) {
    if (json.success === false) {
      throw new Error(parseApiErrorMessage(json, 200))
    }
    const data = asRecord(json.data)
    if (!data) return {}
    return {
      content: Array.isArray(data.content)
        ? (data.content as Array<{ type: string; text?: string }>)
        : undefined,
      isError: data.isError === true,
    }
  }

  const raw = asRecord(json)
  if (!raw) return {}
  return {
    content: Array.isArray(raw.content)
      ? (raw.content as Array<{ type: string; text?: string }>)
      : undefined,
    isError: raw.isError === true,
  }
}

export async function createPlaygroundChatCompletion(init: PlaygroundChatCompletionInit) {
  const baseUrl = getApiBaseUrl()
  const url = baseUrl ? `${baseUrl}/v1/chat/completions` : "/v1/chat/completions"

  const resp = await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${init.apiKey}`,
    },
    body: JSON.stringify(init.body),
    signal: init.signal,
  })

  if (!resp.ok) {
    const errBody = await resp.json().catch(() => ({}))
    throw new Error(parseApiErrorMessage(errBody, resp.status))
  }

  return resp.json()
}

export interface PlaygroundMcpToolExecuteInit {
  apiKey: string
  clientId: number
  toolName: string
  argumentsObj: Record<string, unknown>
  signal?: AbortSignal
}

export async function executePlaygroundMcpTool(init: PlaygroundMcpToolExecuteInit) {
  const baseUrl = getApiBaseUrl()
  const url = baseUrl ? `${baseUrl}/v1/mcp/tool/execute` : "/v1/mcp/tool/execute"

  const resp = await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${init.apiKey}`,
    },
    body: JSON.stringify({
      clientId: init.clientId,
      toolName: init.toolName,
      arguments: init.argumentsObj,
    }),
    signal: init.signal,
  })

  if (!resp.ok) {
    const errBody = await resp.json().catch(() => ({}))
    throw new Error(parseApiErrorMessage(errBody, resp.status))
  }

  const json = (await resp.json().catch(() => ({}))) as unknown
  return unwrapMcpToolExecuteResponse(json)
}
