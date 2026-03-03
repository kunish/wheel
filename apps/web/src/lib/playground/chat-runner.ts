import type { ToolAliasRef } from "./mcp-alias"
import type { ChatMessage, McpToolDef } from "./request-builders"
import type { PlaygroundMcpToolExecuteInit } from "@/lib/api"
import { buildChatPayload } from "./request-builders"
import { extractToolCalls, makeToolMessage } from "./tool-loop"

export interface ChatRunnerDeps {
  createChatCompletion: (input: {
    apiKey: string
    body: Record<string, unknown>
    signal?: AbortSignal
  }) => Promise<any>
  executeTool: (input: PlaygroundMcpToolExecuteInit) => Promise<any>
}

export type ChatRunnerMode = "auto" | "manual"

export interface ChatRunnerRunInput {
  apiKey: string
  model: string
  messages: ChatMessage[]
  mcpTools?: McpToolDef[]
  aliasMap?: Record<string, ToolAliasRef>
  mode: ChatRunnerMode
  maxRounds?: number
  signal?: AbortSignal
  temperature?: number
  maxTokens?: number
  topP?: number
  onEvent?: (event: ChatRunnerEvent) => void
}

export interface ManualPendingToolCall {
  toolCallId: string
  alias: string
  argumentsObj: Record<string, unknown>
  ref: ToolAliasRef
}

export interface ChatRunnerSession {
  input: Omit<ChatRunnerRunInput, "messages" | "onEvent">
  messages: ChatMessage[]
  round: number
}

export interface ManualToolOutput {
  toolCallId: string
  payload: unknown
}

export type ChatRunnerRunResult =
  | {
      status: "completed"
      messages: ChatMessage[]
      responseText: string
      response: any
    }
  | {
      status: "paused"
      session: ChatRunnerSession
      pendingCalls: ManualPendingToolCall[]
      messages: ChatMessage[]
    }

export type ChatRunnerEvent =
  | { type: "assistant"; message: ChatMessage }
  | { type: "tool-call"; call: ManualPendingToolCall }
  | { type: "tool-result"; toolCallId: string; payload: unknown }

const DEFAULT_MAX_ROUNDS = 6

function toErrorMessage(err: unknown): string {
  if (err instanceof Error && err.message) return err.message
  return "Unknown tool execution error"
}

function buildToolErrorPayload(err: unknown): { isError: true; error: string } {
  return {
    isError: true,
    error: toErrorMessage(err),
  }
}

function asRecord(value: string): Record<string, unknown> {
  if (!value) return {}
  try {
    const parsed = JSON.parse(value) as unknown
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) return {}
    return parsed as Record<string, unknown>
  } catch {
    return {}
  }
}

function getAssistantMessage(resp: any): ChatMessage {
  const raw = resp?.choices?.[0]?.message
  return {
    role: "assistant",
    content: typeof raw?.content === "string" ? raw.content : "",
    tool_calls: Array.isArray(raw?.tool_calls) ? raw.tool_calls : undefined,
  }
}

function toPendingCalls(
  resp: any,
  aliasMap: Record<string, ToolAliasRef>,
): ManualPendingToolCall[] {
  const calls = extractToolCalls(resp)
  return calls.map((call) => {
    const ref = aliasMap[call.name]
    if (!ref) throw new Error(`Missing tool alias mapping for: ${call.name}`)
    return {
      toolCallId: call.id,
      alias: call.name,
      argumentsObj: asRecord(call.argumentsText),
      ref,
    }
  })
}

async function executeAutoTools(
  deps: ChatRunnerDeps,
  apiKey: string,
  pendingCalls: ManualPendingToolCall[],
  signal: AbortSignal | undefined,
  onEvent?: (event: ChatRunnerEvent) => void,
): Promise<ChatMessage[]> {
  const toolMessages: ChatMessage[] = []
  for (const pending of pendingCalls) {
    onEvent?.({ type: "tool-call", call: pending })
    let payload: unknown
    try {
      payload = await deps.executeTool({
        apiKey,
        clientId: pending.ref.clientId,
        toolName: pending.ref.toolName,
        argumentsObj: pending.argumentsObj,
        signal,
      })
    } catch (err: unknown) {
      payload = buildToolErrorPayload(err)
    }
    onEvent?.({ type: "tool-result", toolCallId: pending.toolCallId, payload })
    toolMessages.push(makeToolMessage(pending.toolCallId, payload))
  }
  return toolMessages
}

async function runLoop(
  deps: ChatRunnerDeps,
  input: ChatRunnerRunInput,
  startMessages: ChatMessage[],
  startRound = 1,
): Promise<ChatRunnerRunResult> {
  const maxRounds = input.maxRounds ?? DEFAULT_MAX_ROUNDS
  const aliasMap = input.aliasMap ?? {}
  const allMessages: ChatMessage[] = [...startMessages]

  for (let round = startRound; round <= maxRounds; round += 1) {
    const body = buildChatPayload({
      model: input.model,
      messages: allMessages,
      mcpTools: input.mcpTools,
      stream: false,
      temperature: input.temperature,
      maxTokens: input.maxTokens,
      topP: input.topP,
    })

    const resp = await deps.createChatCompletion({
      apiKey: input.apiKey,
      body,
      signal: input.signal,
    })
    const assistant = getAssistantMessage(resp)
    allMessages.push(assistant)
    input.onEvent?.({ type: "assistant", message: assistant })

    const pendingCalls = toPendingCalls(resp, aliasMap)
    if (pendingCalls.length === 0) {
      return {
        status: "completed",
        response: resp,
        responseText: assistant.content ?? "",
        messages: allMessages,
      }
    }

    if (input.mode === "manual") {
      return {
        status: "paused",
        pendingCalls,
        messages: allMessages,
        session: {
          input: {
            apiKey: input.apiKey,
            model: input.model,
            mcpTools: input.mcpTools,
            aliasMap,
            mode: input.mode,
            maxRounds,
            signal: input.signal,
            temperature: input.temperature,
            maxTokens: input.maxTokens,
            topP: input.topP,
          },
          messages: allMessages,
          round,
        },
      }
    }

    const toolMessages = await executeAutoTools(
      deps,
      input.apiKey,
      pendingCalls,
      input.signal,
      input.onEvent,
    )
    allMessages.push(...toolMessages)
  }

  throw new Error(`Tool loop exceeded max rounds: ${maxRounds}`)
}

export function createChatRunner(deps: ChatRunnerDeps) {
  return {
    run(input: ChatRunnerRunInput) {
      return runLoop(deps, input, input.messages)
    },
    continueManual(
      session: ChatRunnerSession,
      toolOutputs: ManualToolOutput[],
      onEvent?: (event: ChatRunnerEvent) => void,
    ) {
      const toolMessages = toolOutputs.map((x) => {
        onEvent?.({ type: "tool-result", toolCallId: x.toolCallId, payload: x.payload })
        return makeToolMessage(x.toolCallId, x.payload)
      })
      return runLoop(
        deps,
        {
          ...session.input,
          onEvent,
          mode: "manual",
          messages: [],
        },
        [...session.messages, ...toolMessages],
        session.round + 1,
      )
    },
  }
}
