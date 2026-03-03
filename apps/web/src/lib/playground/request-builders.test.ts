import { describe, expect, it } from "vitest"
import { buildChatPayload, buildMcpToolExecutePayload } from "./request-builders"

describe("request-builders", () => {
  it("builds chat payload without tools when MCP disabled", () => {
    const body = buildChatPayload({
      model: "gpt-4o",
      messages: [{ role: "user", content: "hi" }],
    })
    expect(body.model).toBe("gpt-4o")
    expect(body.tools).toBeUndefined()
    expect(body.stream).toBe(true)
  })

  it("builds chat payload with tools when MCP enabled", () => {
    const body = buildChatPayload({
      model: "gpt-4o",
      messages: [],
      mcpTools: [
        { type: "function", function: { name: "x", description: "do x", parameters: {} } },
      ],
    })
    expect(body.tools).toHaveLength(1)
    expect(body.stream).toBe(false) // MCP mode defaults to non-streaming
  })

  it("respects explicit stream override", () => {
    const body = buildChatPayload({
      model: "gpt-4o",
      messages: [],
      stream: false,
    })
    expect(body.stream).toBe(false)
  })

  it("builds mcp execute payload", () => {
    const body = buildMcpToolExecutePayload(1, "weather_search", { q: "Tokyo" })
    expect(body.clientId).toBe(1)
    expect(body.toolName).toBe("weather_search")
    expect(body.arguments).toEqual({ q: "Tokyo" })
  })
})
