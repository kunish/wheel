import { describe, expect, it } from "vitest"
import { parseApiErrorMessage, unwrapMcpToolExecuteResponse } from "./playground"

describe("playground api helpers", () => {
  it("reads top-level string errors", () => {
    expect(parseApiErrorMessage({ error: "boom" }, 400)).toBe("boom")
  })

  it("reads nested error.message", () => {
    expect(parseApiErrorMessage({ error: { message: "bad" } }, 500)).toBe("bad")
  })

  it("falls back to HTTP status when message is missing", () => {
    expect(parseApiErrorMessage({}, 418)).toBe("HTTP 418")
  })

  it("unwraps MCP execute envelope payload", () => {
    const result = unwrapMcpToolExecuteResponse({
      success: true,
      data: {
        isError: true,
        content: [{ type: "text", text: "failed" }],
      },
    })
    expect(result).toEqual({
      isError: true,
      content: [{ type: "text", text: "failed" }],
    })
  })

  it("throws when MCP execute envelope has success=false", () => {
    expect(() => unwrapMcpToolExecuteResponse({ success: false, error: "no" })).toThrow("no")
  })
})
