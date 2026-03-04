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

  it("sanitizes unsupported characters and prefixes numeric aliases", () => {
    const selected: SelectedToolRef[] = [
      { clientId: 1, clientName: "123@service", toolName: "*lookup*" },
    ]
    const map = buildToolAliasMap(selected)
    expect(Object.keys(map)).toContain("tool_123_service_lookup")
  })

  it("falls back to a safe alias when names become empty after sanitization", () => {
    const selected: SelectedToolRef[] = [
      { clientId: 1, clientName: "天气", toolName: "查找" },
      { clientId: 2, clientName: "天气", toolName: "查找" },
    ]
    const map = buildToolAliasMap(selected)
    expect(Object.keys(map)).toContain("tool")
    expect(Object.keys(map)).toContain("tool_2")
  })

  it("caps alias length while keeping collision suffix", () => {
    const long = "a".repeat(200)
    const selected: SelectedToolRef[] = [
      { clientId: 1, clientName: long, toolName: long },
      { clientId: 2, clientName: long, toolName: long },
    ]
    const map = buildToolAliasMap(selected)
    const aliases = Object.keys(map)
    expect(aliases[0].length).toBeLessThanOrEqual(64)
    expect(aliases[1].length).toBeLessThanOrEqual(64)
    expect(aliases[1].endsWith("_2")).toBe(true)
  })

  it("returns empty map for empty input", () => {
    expect(buildToolAliasMap([])).toEqual({})
  })
})
