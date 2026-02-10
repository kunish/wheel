import { describe, expect, it } from "vitest"
import {
  buildFilterSearchParams,
  computeTimePresetFrom,
  countMatches,
  getActiveFilterChips,
  hasActiveFilters,
  parseLogFilters,
  TIME_PRESETS,
} from "./log-filters"

// ── 9.1: URL Filter State Parsing ────────────────────────────────

describe("parseLogFilters", () => {
  it("returns default values for empty search params", () => {
    const params = new URLSearchParams()
    const filters = parseLogFilters(params)
    expect(filters).toEqual({
      page: 1,
      model: "",
      status: "all",
      channelId: undefined,
      keyword: "",
      pageSize: 20,
      startTime: undefined,
      endTime: undefined,
    })
  })

  it("parses page number", () => {
    const params = new URLSearchParams("page=3")
    expect(parseLogFilters(params).page).toBe(3)
  })

  it("defaults page to 1 when absent", () => {
    const params = new URLSearchParams()
    expect(parseLogFilters(params).page).toBe(1)
  })

  it("parses model filter", () => {
    const params = new URLSearchParams("model=gpt-4o")
    expect(parseLogFilters(params).model).toBe("gpt-4o")
  })

  it("handles encoded model name with special characters", () => {
    const params = new URLSearchParams(`model=${encodeURIComponent("claude-sonnet-4-5")}`)
    expect(parseLogFilters(params).model).toBe("claude-sonnet-4-5")
  })

  it("parses status filter", () => {
    const params = new URLSearchParams("status=error")
    expect(parseLogFilters(params).status).toBe("error")
  })

  it("defaults status to 'all'", () => {
    const params = new URLSearchParams()
    expect(parseLogFilters(params).status).toBe("all")
  })

  it("parses channel ID as number", () => {
    const params = new URLSearchParams("channel=42")
    expect(parseLogFilters(params).channelId).toBe(42)
  })

  it("returns undefined channelId when absent", () => {
    const params = new URLSearchParams()
    expect(parseLogFilters(params).channelId).toBeUndefined()
  })

  it("parses keyword (q param)", () => {
    const params = new URLSearchParams("q=error+timeout")
    expect(parseLogFilters(params).keyword).toBe("error timeout")
  })

  it("parses page size", () => {
    const params = new URLSearchParams("size=100")
    expect(parseLogFilters(params).pageSize).toBe(100)
  })

  it("defaults page size to 20", () => {
    const params = new URLSearchParams()
    expect(parseLogFilters(params).pageSize).toBe(20)
  })

  it("parses time range (from/to)", () => {
    const params = new URLSearchParams("from=1700000000&to=1700003600")
    const filters = parseLogFilters(params)
    expect(filters.startTime).toBe(1700000000)
    expect(filters.endTime).toBe(1700003600)
  })

  it("returns undefined time values when absent", () => {
    const params = new URLSearchParams()
    expect(parseLogFilters(params).startTime).toBeUndefined()
    expect(parseLogFilters(params).endTime).toBeUndefined()
  })

  it("parses all filters simultaneously", () => {
    const params = new URLSearchParams(
      "page=2&model=gpt-4o&status=error&channel=5&q=timeout&size=50&from=1700000000&to=1700086400",
    )
    const filters = parseLogFilters(params)
    expect(filters).toEqual({
      page: 2,
      model: "gpt-4o",
      status: "error",
      channelId: 5,
      keyword: "timeout",
      pageSize: 50,
      startTime: 1700000000,
      endTime: 1700086400,
    })
  })
})

// ── 9.4: Time Range Preset Calculation ───────────────────────────

describe("computeTimePresetFrom", () => {
  const NOW = 1700000000 // fixed timestamp for deterministic tests

  it("computes 1h preset (3600s ago)", () => {
    expect(computeTimePresetFrom(3600, NOW)).toBe(1700000000 - 3600)
  })

  it("computes 6h preset (21600s ago)", () => {
    expect(computeTimePresetFrom(21600, NOW)).toBe(1700000000 - 21600)
  })

  it("computes 24h preset (86400s ago)", () => {
    expect(computeTimePresetFrom(86400, NOW)).toBe(1700000000 - 86400)
  })

  it("computes 7d preset (604800s ago)", () => {
    expect(computeTimePresetFrom(604800, NOW)).toBe(1700000000 - 604800)
  })

  it("computes 30d preset (2592000s ago)", () => {
    expect(computeTimePresetFrom(2592000, NOW)).toBe(1700000000 - 2592000)
  })

  it("uses current time when now is not provided", () => {
    const before = Math.floor(Date.now() / 1000)
    const result = computeTimePresetFrom(3600)
    const after = Math.floor(Date.now() / 1000)
    expect(result).toBeGreaterThanOrEqual(before - 3600)
    expect(result).toBeLessThanOrEqual(after - 3600)
  })
})

describe("tIME_PRESETS constant", () => {
  it("has 5 presets", () => {
    expect(TIME_PRESETS).toHaveLength(5)
  })

  it("presets are in ascending order", () => {
    for (let i = 1; i < TIME_PRESETS.length; i++) {
      expect(TIME_PRESETS[i].seconds).toBeGreaterThan(TIME_PRESETS[i - 1].seconds)
    }
  })

  it("has correct labels and values", () => {
    expect(TIME_PRESETS).toEqual([
      { label: "1h", seconds: 3600 },
      { label: "6h", seconds: 21600 },
      { label: "24h", seconds: 86400 },
      { label: "7d", seconds: 604800 },
      { label: "30d", seconds: 2592000 },
    ])
  })
})

// ── 9.5: Filter Chip Generation ──────────────────────────────────

describe("getActiveFilterChips", () => {
  const defaultFilters = {
    page: 1,
    model: "",
    status: "all" as const,
    channelId: undefined,
    keyword: "",
    pageSize: 20,
    startTime: undefined,
    endTime: undefined,
  }

  it("returns empty array for default filters", () => {
    expect(getActiveFilterChips(defaultFilters)).toEqual([])
  })

  it("generates chip for keyword filter", () => {
    const chips = getActiveFilterChips({ ...defaultFilters, keyword: "timeout" })
    expect(chips).toEqual([{ key: "q", label: "Search", value: "timeout" }])
  })

  it("generates chip for model filter", () => {
    const chips = getActiveFilterChips({ ...defaultFilters, model: "gpt-4o" })
    expect(chips).toEqual([{ key: "model", label: "Model", value: "gpt-4o" }])
  })

  it("generates chip for channel filter with channelId", () => {
    const chips = getActiveFilterChips({ ...defaultFilters, channelId: 42 })
    expect(chips).toEqual([{ key: "channel", label: "Channel", value: "42" }])
  })

  it("uses channel name when provided", () => {
    const chips = getActiveFilterChips({ ...defaultFilters, channelId: 42 }, "OpenAI Main")
    expect(chips).toEqual([{ key: "channel", label: "Channel", value: "OpenAI Main" }])
  })

  it("generates chip for status filter", () => {
    const chips = getActiveFilterChips({ ...defaultFilters, status: "error" })
    expect(chips).toEqual([{ key: "status", label: "Status", value: "error" }])
  })

  it("does not generate chip for status='all' (default)", () => {
    const chips = getActiveFilterChips({ ...defaultFilters, status: "all" })
    expect(chips).toEqual([])
  })

  it("generates single time chip for startTime", () => {
    const chips = getActiveFilterChips({ ...defaultFilters, startTime: 1700000000 })
    expect(chips).toHaveLength(1)
    expect(chips[0].key).toBe("time")
    expect(chips[0].label).toBe("Time")
    expect(chips[0].value).toBeTruthy()
  })

  it("generates single time chip for endTime", () => {
    const chips = getActiveFilterChips({ ...defaultFilters, endTime: 1700086400 })
    expect(chips).toHaveLength(1)
    expect(chips[0].key).toBe("time")
    expect(chips[0].label).toBe("Time")
  })

  it("generates single time chip for both startTime and endTime", () => {
    const chips = getActiveFilterChips({
      ...defaultFilters,
      startTime: 1700000000,
      endTime: 1700086400,
    })
    expect(chips).toHaveLength(1)
    expect(chips[0].key).toBe("time")
  })

  it("generates multiple chips in correct order", () => {
    const chips = getActiveFilterChips({
      ...defaultFilters,
      keyword: "test",
      model: "gpt-4o",
      channelId: 5,
      status: "error",
      startTime: 1700000000,
      endTime: 1700086400,
    })
    expect(chips).toHaveLength(5)
    expect(chips.map((c) => c.key)).toEqual(["q", "model", "channel", "status", "time"])
  })
})

describe("hasActiveFilters", () => {
  const defaultFilters = {
    page: 1,
    model: "",
    status: "all",
    channelId: undefined,
    keyword: "",
    pageSize: 20,
    startTime: undefined,
    endTime: undefined,
  }

  it("returns false for default filters", () => {
    expect(hasActiveFilters(defaultFilters)).toBe(false)
  })

  it("returns true when keyword is set", () => {
    expect(hasActiveFilters({ ...defaultFilters, keyword: "test" })).toBe(true)
  })

  it("returns true when model is set", () => {
    expect(hasActiveFilters({ ...defaultFilters, model: "gpt-4o" })).toBe(true)
  })

  it("returns true when channelId is set", () => {
    expect(hasActiveFilters({ ...defaultFilters, channelId: 42 })).toBe(true)
  })

  it("returns true when status is not 'all'", () => {
    expect(hasActiveFilters({ ...defaultFilters, status: "error" })).toBe(true)
  })

  it("returns true when startTime is set", () => {
    expect(hasActiveFilters({ ...defaultFilters, startTime: 1700000000 })).toBe(true)
  })

  it("returns true when endTime is set", () => {
    expect(hasActiveFilters({ ...defaultFilters, endTime: 1700086400 })).toBe(true)
  })

  it("ignores page and pageSize", () => {
    expect(hasActiveFilters({ ...defaultFilters, page: 5, pageSize: 100 })).toBe(false)
  })
})

// ── 9.1 (continued): buildFilterSearchParams ─────────────────────

describe("buildFilterSearchParams", () => {
  it("sets a new param value", () => {
    const current = new URLSearchParams()
    const result = buildFilterSearchParams(current, { model: "gpt-4o" })
    expect(result.get("model")).toBe("gpt-4o")
  })

  it("removes param when value is undefined", () => {
    const current = new URLSearchParams("model=gpt-4o")
    const result = buildFilterSearchParams(current, { model: undefined })
    expect(result.has("model")).toBe(false)
  })

  it("removes param when value is null", () => {
    const current = new URLSearchParams("model=gpt-4o")
    const result = buildFilterSearchParams(current, { model: null })
    expect(result.has("model")).toBe(false)
  })

  it("removes param when value is empty string", () => {
    const current = new URLSearchParams("model=gpt-4o")
    const result = buildFilterSearchParams(current, { model: "" })
    expect(result.has("model")).toBe(false)
  })

  it("resets page when changing non-page filters", () => {
    const current = new URLSearchParams("page=3&model=gpt-4o")
    const result = buildFilterSearchParams(current, { status: "error" })
    expect(result.has("page")).toBe(false)
  })

  it("preserves page when updating page itself", () => {
    const current = new URLSearchParams("model=gpt-4o")
    const result = buildFilterSearchParams(current, { page: 5 })
    expect(result.get("page")).toBe("5")
  })

  it("removes default status='all'", () => {
    const current = new URLSearchParams()
    const result = buildFilterSearchParams(current, { status: "all" })
    expect(result.has("status")).toBe(false)
  })

  it("removes default size=20", () => {
    const current = new URLSearchParams()
    const result = buildFilterSearchParams(current, { size: 20 })
    expect(result.has("size")).toBe(false)
  })

  it("removes default page=1", () => {
    const current = new URLSearchParams()
    const result = buildFilterSearchParams(current, { page: 1 })
    expect(result.has("page")).toBe(false)
  })

  it("preserves existing unrelated params", () => {
    const current = new URLSearchParams("model=gpt-4o&q=test")
    const result = buildFilterSearchParams(current, { status: "error" })
    expect(result.get("model")).toBe("gpt-4o")
    expect(result.get("q")).toBe("test")
    expect(result.get("status")).toBe("error")
  })

  it("handles numeric values correctly", () => {
    const current = new URLSearchParams()
    const result = buildFilterSearchParams(current, { channel: 42, from: 1700000000 })
    expect(result.get("channel")).toBe("42")
    expect(result.get("from")).toBe("1700000000")
  })

  it("handles multiple updates atomically", () => {
    const current = new URLSearchParams("model=gpt-4o&status=error")
    const result = buildFilterSearchParams(current, {
      model: undefined,
      status: "success",
      q: "hello",
    })
    expect(result.has("model")).toBe(false)
    expect(result.get("status")).toBe("success")
    expect(result.get("q")).toBe("hello")
  })
})

// ── 9.6: In-content Search / Match Counting ─────────────────────

describe("countMatches", () => {
  it("counts single match", () => {
    expect(countMatches("hello world", "world")).toBe(1)
  })

  it("counts multiple matches", () => {
    expect(countMatches("the cat sat on the mat", "the")).toBe(2)
  })

  it("is case insensitive", () => {
    expect(countMatches("Hello HELLO hello", "hello")).toBe(3)
  })

  it("returns 0 for no matches", () => {
    expect(countMatches("hello world", "xyz")).toBe(0)
  })

  it("returns 0 for empty needle", () => {
    expect(countMatches("hello world", "")).toBe(0)
  })

  it("returns 0 for empty text", () => {
    expect(countMatches("", "hello")).toBe(0)
  })

  it("returns 0 for both empty", () => {
    expect(countMatches("", "")).toBe(0)
  })

  it("handles overlapping patterns without double-counting", () => {
    expect(countMatches("aaa", "aa")).toBe(1)
  })

  it("counts matches in JSON-like content", () => {
    const json = '{"model":"gpt-4o","content":"Hello gpt-4o user"}'
    expect(countMatches(json, "gpt-4o")).toBe(2)
  })

  it("handles special regex characters as literal text", () => {
    expect(countMatches("price is $10.00 or $20.00", "$10.00")).toBe(1)
  })

  it("handles multiline content", () => {
    const text = "line1 error\nline2 ok\nline3 error"
    expect(countMatches(text, "error")).toBe(2)
  })
})
