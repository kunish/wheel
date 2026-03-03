import { describe, expect, it } from "vitest"
import {
  buildManualToolOutputs,
  canContinueManualFromTimeline,
  resolveStreamForRequest,
} from "./use-playground-chat"

describe("use-playground-chat helpers", () => {
  it("forces non-stream when MCP is enabled", () => {
    expect(resolveStreamForRequest(true, true)).toBe(false)
    expect(resolveStreamForRequest(false, true)).toBe(false)
  })

  it("keeps stream setting in non-MCP mode", () => {
    expect(resolveStreamForRequest(true, false)).toBe(true)
    expect(resolveStreamForRequest(false, false)).toBe(false)
  })

  it("allows manual continue when each pending call is done or error", () => {
    const pendingCalls = [{ toolCallId: "call_1" }, { toolCallId: "call_2" }] as any
    const timeline = [
      { callId: "call_1", status: "done" },
      { callId: "call_2", status: "error" },
    ] as any

    expect(canContinueManualFromTimeline(pendingCalls, timeline)).toBe(true)
  })

  it("converts error states to readable error payloads for continuation", () => {
    const outputs = buildManualToolOutputs(
      [{ toolCallId: "call_1" }, { toolCallId: "call_2" }] as any,
      [
        { callId: "call_1", status: "done", payload: { ok: true } },
        { callId: "call_2", status: "error", error: "Permission denied" },
      ] as any,
    )

    expect(outputs).toEqual([
      { toolCallId: "call_1", payload: { ok: true } },
      { toolCallId: "call_2", payload: { isError: true, error: "Permission denied" } },
    ])
  })
})
