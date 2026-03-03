export interface ChatMessage {
  role: "system" | "user" | "assistant" | "tool"
  content?: string
  tool_calls?: any[]
  tool_call_id?: string
}

export interface McpToolDef {
  type: "function"
  function: {
    name: string
    description?: string
    parameters?: Record<string, unknown>
  }
}

export interface BuildChatPayloadInput {
  model: string
  messages: ChatMessage[]
  mcpTools?: McpToolDef[]
  stream?: boolean
  temperature?: number
  maxTokens?: number
  topP?: number
}

export function buildChatPayload(input: BuildChatPayloadInput) {
  const hasMcpTools = input.mcpTools && input.mcpTools.length > 0

  // When MCP tools are present, default to non-streaming (v1 tool loop is non-streaming)
  const stream = input.stream ?? !hasMcpTools

  const body: Record<string, unknown> = {
    model: input.model,
    messages: input.messages,
    stream,
  }

  if (input.temperature !== undefined) body.temperature = input.temperature
  if (input.maxTokens !== undefined) body.max_tokens = input.maxTokens
  if (input.topP !== undefined) body.top_p = input.topP

  if (hasMcpTools) {
    body.tools = input.mcpTools
  }

  return body
}

export interface McpToolExecutePayload {
  clientId: number
  toolName: string
  arguments: Record<string, unknown>
}

export function buildMcpToolExecutePayload(
  clientId: number,
  toolName: string,
  args: Record<string, unknown>,
): McpToolExecutePayload {
  return { clientId, toolName, arguments: args }
}

export function canSendPlaygroundRequest(state: {
  isLoading: boolean
  isPaused?: boolean
}): boolean {
  if (state.isLoading) return false
  if (state.isPaused) return false
  return true
}
