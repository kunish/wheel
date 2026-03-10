import { normalizeToolPayloadForModel } from "./tool-payload"

interface ToolCall {
  id: string
  name: string
  argumentsText: string
}

export function extractToolCalls(resp: any): ToolCall[] {
  const raw = resp?.choices?.[0]?.message?.tool_calls ?? []
  return raw.map((x: any) => ({
    id: x.id,
    name: x.function?.name ?? "",
    argumentsText: x.function?.arguments ?? "{}",
  }))
}

export function makeToolMessage(toolCallId: string, payload: unknown) {
  return {
    role: "tool" as const,
    tool_call_id: toolCallId,
    content: normalizeToolPayloadForModel(payload),
  }
}

export function shouldStopLoop(input: { round: number; maxRounds: number; pendingCalls: number }) {
  return input.round > input.maxRounds || input.pendingCalls === 0
}
