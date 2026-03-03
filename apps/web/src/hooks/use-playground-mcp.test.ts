import type { SelectableToolRef } from "@/lib/playground/tool-selection"
import { describe, expect, it } from "vitest"
import { resolveSelectedKeysForMcpTools } from "./use-playground-mcp"

const AVAILABLE_TOOLS: SelectableToolRef[] = [
  {
    key: "1:forecast",
    clientId: 1,
    clientName: "weather",
    toolName: "forecast",
  },
  {
    key: "1:search",
    clientId: 1,
    clientName: "weather",
    toolName: "search",
  },
]

describe("use-playground-mcp helpers", () => {
  it("defaults to select-all before user edits selection", () => {
    const selected = resolveSelectedKeysForMcpTools(AVAILABLE_TOOLS, [], false)

    expect(selected).toEqual(["1:forecast", "1:search"])
  })

  it("keeps empty selection after user clears all", () => {
    const selected = resolveSelectedKeysForMcpTools(AVAILABLE_TOOLS, [], true)

    expect(selected).toEqual([])
  })
})
