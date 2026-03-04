import { describe, expect, it } from "vitest"
import {
  buildManualToolOutputs,
  canContinueManualFromTimeline,
  mergePendingCallsIntoTimeline,
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

  it("preserves pending call order when building continuation payloads", () => {
    const outputs = buildManualToolOutputs(
      [{ toolCallId: "call_1" }, { toolCallId: "call_2" }] as any,
      [
        { callId: "call_2", status: "done", payload: { second: true } },
        { callId: "call_1", status: "done", payload: { first: true } },
      ] as any,
    )

    expect(outputs).toEqual([
      { toolCallId: "call_1", payload: { first: true } },
      { toolCallId: "call_2", payload: { second: true } },
    ])
  })

  it("merges newly paused calls into timeline without duplicates", () => {
    const timeline = [
      {
        id: "existing",
        callId: "call_1",
        alias: "a",
        title: "t",
        argumentsObj: {},
        status: "done",
      },
    ] as any

    const merged = mergePendingCallsIntoTimeline(timeline, [
      {
        toolCallId: "call_1",
        alias: "a",
        argumentsObj: {},
        ref: { clientName: "c", toolName: "t" },
      },
      {
        toolCallId: "call_2",
        alias: "b",
        argumentsObj: { q: 1 },
        ref: { clientName: "c2", toolName: "t2" },
      },
    ] as any)

    expect(merged).toHaveLength(2)
    expect(merged[1]).toMatchObject({
      callId: "call_2",
      alias: "b",
      title: "c2.t2",
      status: "pending",
    })
  })
})
