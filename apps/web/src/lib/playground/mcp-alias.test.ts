import type { SelectedToolRef } from "./mcp-alias"
import { describe, expect, it } from "vitest"
import { buildToolAliasMap } from "./mcp-alias"

describe("buildToolAliasMap", () => {
  it("builds alias -> tool ref map and deduplicates collisions", () => {
    const selected: SelectedToolRef[] = [
      { clientId: 1, clientName: "weather", toolName: "search" },
      { clientId: 2, clientName: "weather", toolName: "search" },
    ]
    const map = buildToolAliasMap(selected)
    expect(Object.keys(map)).toContain("weather_search")
    expect(Object.keys(map)).toContain("weather_search_2")
    expect(map.weather_search.clientId).toBe(1)
    expect(map.weather_search_2.clientId).toBe(2)
  })

  it("normalises whitespace and hyphens in alias", () => {
    const selected: SelectedToolRef[] = [
      { clientId: 1, clientName: "my-service", toolName: "get data" },
    ]
    const map = buildToolAliasMap(selected)
    expect(Object.keys(map)).toContain("my_service_get_data")
  })

  it("returns empty map for empty input", () => {
    expect(buildToolAliasMap([])).toEqual({})
  })
})
