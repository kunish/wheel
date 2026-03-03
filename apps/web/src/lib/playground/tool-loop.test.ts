import { describe, expect, it } from "vitest"
import { extractToolCalls, makeToolMessage, shouldStopLoop } from "./tool-loop"

describe("tool-loop", () => {
  it("extracts tool calls from assistant message", () => {
    const calls = extractToolCalls({
      choices: [
        { message: { tool_calls: [{ id: "c1", function: { name: "a", arguments: "{}" } }] } },
      ],
    })
    expect(calls).toHaveLength(1)
    expect(calls[0].id).toBe("c1")
    expect(calls[0].name).toBe("a")
    expect(calls[0].argumentsText).toBe("{}")
  })

  it("returns empty array when no tool_calls", () => {
    expect(extractToolCalls({ choices: [{ message: { content: "hello" } }] })).toEqual([])
    expect(extractToolCalls({})).toEqual([])
  })

  it("builds tool role message with matching tool_call_id", () => {
    const msg = makeToolMessage("c1", { ok: true })
    expect(msg.role).toBe("tool")
    expect(msg.tool_call_id).toBe("c1")
    expect(msg.content).toBe(JSON.stringify({ ok: true }))
  })

  it("stops when rounds exceed max", () => {
    expect(shouldStopLoop({ round: 7, maxRounds: 6, pendingCalls: 1 })).toBe(true)
  })

  it("stops when no pending calls", () => {
    expect(shouldStopLoop({ round: 1, maxRounds: 6, pendingCalls: 0 })).toBe(true)
  })

  it("continues when within limits and has pending calls", () => {
    expect(shouldStopLoop({ round: 3, maxRounds: 6, pendingCalls: 2 })).toBe(false)
  })
})
