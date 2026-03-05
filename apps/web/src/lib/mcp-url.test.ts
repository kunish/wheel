import { describe, expect, it } from "vitest"
import { resolveMcpServerUrl, resolveMcpToolExecuteUrl } from "./mcp-url"

describe("resolveMcpServerUrl", () => {
  it("prefers backend-provided endpoint", () => {
    expect(
      resolveMcpServerUrl({
        backendServerUrl: "https://api.example.com/mcp/sse",
        apiBaseUrl: "http://localhost:8787",
        windowOrigin: "http://localhost:5173",
      }),
    ).toBe("https://api.example.com/mcp/sse")
  })

  it("upgrades legacy /mcp endpoint to /mcp/sse", () => {
    expect(
      resolveMcpServerUrl({
        backendServerUrl: "https://api.example.com/mcp/",
        apiBaseUrl: "",
        windowOrigin: "http://localhost:5173",
      }),
    ).toBe("https://api.example.com/mcp/sse")
  })

  it("uses API base url when backend endpoint is missing", () => {
    expect(
      resolveMcpServerUrl({
        apiBaseUrl: "http://localhost:8787",
        windowOrigin: "http://localhost:5173",
      }),
    ).toBe("http://localhost:8787/mcp/sse")
  })

  it("falls back to page origin", () => {
    expect(
      resolveMcpServerUrl({
        apiBaseUrl: "",
        windowOrigin: "http://localhost:3000",
      }),
    ).toBe("http://localhost:3000/mcp/sse")
  })
})

describe("resolveMcpToolExecuteUrl", () => {
  it("builds tool execute endpoint from mcp server url", () => {
    expect(resolveMcpToolExecuteUrl("http://localhost:3000/mcp/sse")).toBe(
      "http://localhost:3000/v1/mcp/tool/execute",
    )
  })

  it("builds tool execute endpoint from legacy /mcp url", () => {
    expect(resolveMcpToolExecuteUrl("http://localhost:3000/mcp/")).toBe(
      "http://localhost:3000/v1/mcp/tool/execute",
    )
  })
})
