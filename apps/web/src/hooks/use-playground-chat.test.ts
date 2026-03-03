import { describe, expect, it } from "vitest"
import { resolveStreamForRequest } from "./use-playground-chat"

describe("use-playground-chat helpers", () => {
  it("forces non-stream when MCP is enabled", () => {
    expect(resolveStreamForRequest(true, true)).toBe(false)
    expect(resolveStreamForRequest(false, true)).toBe(false)
  })

  it("keeps stream setting in non-MCP mode", () => {
    expect(resolveStreamForRequest(true, false)).toBe(true)
    expect(resolveStreamForRequest(false, false)).toBe(false)
  })
})
