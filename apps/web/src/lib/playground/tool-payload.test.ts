import { describe, expect, it } from "vitest"
import { formatToolPayloadForDisplay, normalizeToolPayloadForModel } from "./tool-payload"

describe("tool-payload helpers", () => {
  it("uses MCP text content for model payload", () => {
    const content = normalizeToolPayloadForModel({
      isError: false,
      content: [{ type: "text", text: "hello world" }],
    })
    expect(content).toBe("hello world")
  })

  it("adds tool_error prefix when payload indicates error", () => {
    const content = normalizeToolPayloadForModel({ isError: true, error: "permission denied" })
    expect(content).toBe("[tool_error]\npermission denied")
  })

  it("pretty prints JSON text for timeline display", () => {
    const text = formatToolPayloadForDisplay({
      isError: false,
      content: [{ type: "text", text: '{"total":2,"items":[{"id":1}]}' }],
    })
    expect(text).toContain('"total": 2')
    expect(text).toContain('"items"')
  })

  it("formats plain error payload for timeline display", () => {
    const text = formatToolPayloadForDisplay({ isError: true, error: "tool failed" })
    expect(text).toBe("Error: tool failed")
  })
})
