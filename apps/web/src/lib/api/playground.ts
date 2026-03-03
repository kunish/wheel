import { getApiBaseUrl } from "./client"

// ── Playground-specific API helpers ──

export interface PlaygroundChatCompletionInit {
  apiKey: string
  body: Record<string, unknown>
  signal?: AbortSignal
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
    throw new Error(
      (errBody as { error?: { message?: string } })?.error?.message || `HTTP ${resp.status}`,
    )
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
    throw new Error(
      (errBody as { error?: { message?: string } })?.error?.message || `HTTP ${resp.status}`,
    )
  }

  return resp.json()
}
