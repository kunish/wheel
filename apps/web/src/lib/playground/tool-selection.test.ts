import type { MCPClientRecord } from "@/lib/api"
import { describe, expect, it } from "vitest"
import {
  buildMcpToolsForChat,
  buildSelectableTools,
  pickSelectedToolKeys,
  toSelectedToolRefs,
} from "./tool-selection"

function makeClient(overrides: Partial<MCPClientRecord>): MCPClientRecord {
  return {
    id: 1,
    name: "weather",
    connectionType: "http",
    connectionString: "https://example.com/mcp",
    authType: "none",
    toolsToExecute: [],
    toolsToAutoExec: [],
    enabled: true,
    state: "connected",
    tools: [],
    ...overrides,
  }
}

describe("tool-selection", () => {
  it("builds selectable tools from connected and enabled clients only", () => {
    const clients: MCPClientRecord[] = [
      makeClient({ id: 1, name: "weather", tools: [{ name: "search", description: "Search" }] }),
      makeClient({ id: 2, name: "disabled", enabled: false, tools: [{ name: "x" }] }),
      makeClient({ id: 3, name: "offline", state: "disconnected", tools: [{ name: "y" }] }),
    ]

    const tools = buildSelectableTools(clients)

    expect(tools).toHaveLength(1)
    expect(tools[0]).toMatchObject({ clientId: 1, clientName: "weather", toolName: "search" })
  })

  it("keeps valid previously selected keys", () => {
    const available = buildSelectableTools([
      makeClient({ id: 1, name: "weather", tools: [{ name: "search" }, { name: "forecast" }] }),
    ])

    const selected = pickSelectedToolKeys(available, ["1:search", "999:missing"])

    expect(selected).toEqual(["1:search"])
  })

  it("defaults to select all when previous selection has no valid keys", () => {
    const available = buildSelectableTools([
      makeClient({ id: 1, name: "weather", tools: [{ name: "search" }, { name: "forecast" }] }),
    ])

    const selected = pickSelectedToolKeys(available, ["999:missing"])

    expect(selected).toEqual(["1:forecast", "1:search"])
  })

  it("keeps empty selection when fallback-to-all is disabled", () => {
    const available = buildSelectableTools([
      makeClient({ id: 1, name: "weather", tools: [{ name: "search" }, { name: "forecast" }] }),
    ])

    const selected = pickSelectedToolKeys(available, ["999:missing"], { fallbackToAll: false })

    expect(selected).toEqual([])
  })

  it("maps selected keys back to refs for execution", () => {
    const available = buildSelectableTools([
      makeClient({ id: 1, name: "weather", tools: [{ name: "search" }, { name: "forecast" }] }),
    ])

    const refs = toSelectedToolRefs(available, ["1:search"])

    expect(refs).toEqual([{ clientId: 1, clientName: "weather", toolName: "search" }])
  })

  it("builds chat tools and alias map from selected refs", () => {
    const selected = [
      { clientId: 1, clientName: "weather", toolName: "search", description: "Search weather" },
    ]

    const result = buildMcpToolsForChat(selected)

    expect(result.tools).toEqual([
      {
        type: "function",
        function: {
          name: "weather_search",
          description: "[weather] Search weather",
          parameters: { type: "object", additionalProperties: true },
        },
      },
    ])
    expect(result.aliasMap.weather_search).toMatchObject({ clientId: 1, toolName: "search" })
  })
})
