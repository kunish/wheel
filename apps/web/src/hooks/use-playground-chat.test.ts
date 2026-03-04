import { describe, expect, it } from "vitest"
import {
  buildManualToolOutputs,
  buildPlaygroundRequestMessages,
  canContinueManualFromTimeline,
  deriveConversationTurns,
  derivePlaygroundModels,
  mergePendingCallsIntoTimeline,
  normalizeConversationMessages,
  parseActiveProfileId,
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

  it("parses active profile id from settings", () => {
    expect(parseActiveProfileId(undefined)).toBeUndefined()
    expect(parseActiveProfileId("0")).toBeUndefined()
    expect(parseActiveProfileId("abc")).toBeUndefined()
    expect(parseActiveProfileId("3")).toBe(3)
  })

  it("derives sorted unique model names from groups", () => {
    expect(
      derivePlaygroundModels([
        { name: " claude-3-5-sonnet " },
        { name: "gpt-4o" },
        { name: "gpt-4o" },
        { name: "" },
      ]),
    ).toEqual(["claude-3-5-sonnet", "gpt-4o"])
  })

  it("normalizes conversation by removing system role", () => {
    expect(
      normalizeConversationMessages([
        { role: "system", content: "you are helpful" },
        { role: "user", content: "hello" },
      ] as any),
    ).toEqual([{ role: "user", content: "hello" }])
  })

  it("builds request messages with trimmed system prompt", () => {
    expect(
      buildPlaygroundRequestMessages("  helper  ", [{ role: "user", content: "q" }] as any),
    ).toEqual([
      { role: "system", content: "helper" },
      { role: "user", content: "q" },
    ])
  })

  it("derives display turns from user and assistant messages", () => {
    expect(
      deriveConversationTurns([
        { role: "user", content: "hello" },
        { role: "assistant", content: "world" },
        { role: "tool", content: "ignored" },
      ] as any),
    ).toEqual([
      { id: "user-0", role: "user", content: "hello" },
      { id: "assistant-1", role: "assistant", content: "world" },
    ])
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
