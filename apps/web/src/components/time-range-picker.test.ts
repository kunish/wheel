import { describe, expect, it } from "vitest"
import {
  detectPreset,
  formatCompactDate,
  formatRangeSummary,
  TIME_RANGE_PRESETS,
} from "./time-range-picker"

// ── 5.1: Range Summary Formatting ──────────────────────────────────

describe("formatCompactDate", () => {
  it("formats a timestamp as MM/DD HH:mm", () => {
    // 2023-11-15 10:30:00 local time
    const d = new Date(2023, 10, 15, 10, 30, 0)
    const ts = Math.floor(d.getTime() / 1000)
    expect(formatCompactDate(ts)).toBe("11/15 10:30")
  })

  it("pads single-digit month and day", () => {
    const d = new Date(2023, 0, 5, 8, 5, 0)
    const ts = Math.floor(d.getTime() / 1000)
    expect(formatCompactDate(ts)).toBe("01/05 08:05")
  })

  it("handles midnight", () => {
    const d = new Date(2023, 5, 20, 0, 0, 0)
    const ts = Math.floor(d.getTime() / 1000)
    expect(formatCompactDate(ts)).toBe("06/20 00:00")
  })
})

describe("detectPreset", () => {
  const NOW = 1700000000

  it("returns null when from is undefined", () => {
    expect(detectPreset(undefined, NOW, NOW)).toBeNull()
  })

  it("returns null when to is undefined", () => {
    expect(detectPreset(NOW - 3600, undefined, NOW)).toBeNull()
  })

  it("returns null when both are undefined", () => {
    expect(detectPreset(undefined, undefined, NOW)).toBeNull()
  })

  it("detects 1h preset", () => {
    const result = detectPreset(NOW - 3600, NOW, NOW)
    expect(result).toEqual({ label: "1h", seconds: 3600 })
  })

  it("detects 6h preset", () => {
    const result = detectPreset(NOW - 21600, NOW, NOW)
    expect(result).toEqual({ label: "6h", seconds: 21600 })
  })

  it("detects 24h preset", () => {
    const result = detectPreset(NOW - 86400, NOW, NOW)
    expect(result).toEqual({ label: "24h", seconds: 86400 })
  })

  it("detects 7d preset", () => {
    const result = detectPreset(NOW - 604800, NOW, NOW)
    expect(result).toEqual({ label: "7d", seconds: 604800 })
  })

  it("detects 30d preset", () => {
    const result = detectPreset(NOW - 2592000, NOW, NOW)
    expect(result).toEqual({ label: "30d", seconds: 2592000 })
  })

  it("detects preset when to is within 60s of now", () => {
    const result = detectPreset(NOW - 3600 + 30, NOW + 30, NOW)
    expect(result).toEqual({ label: "1h", seconds: 3600 })
  })

  it("returns null when to is more than 60s from now", () => {
    const result = detectPreset(NOW - 3600 - 120, NOW - 120, NOW)
    expect(result).toBeNull()
  })

  it("returns null for non-preset durations", () => {
    const result = detectPreset(NOW - 5000, NOW, NOW)
    expect(result).toBeNull()
  })
})

describe("formatRangeSummary", () => {
  const NOW = 1700000000

  it("returns placeholder when no range is set", () => {
    expect(formatRangeSummary(undefined, undefined)).toBe("Time Range")
  })

  it("shows 'Last 1h' for 1h preset", () => {
    expect(formatRangeSummary(NOW - 3600, NOW, NOW)).toBe("Last 1h")
  })

  it("shows 'Last 6h' for 6h preset", () => {
    expect(formatRangeSummary(NOW - 21600, NOW, NOW)).toBe("Last 6h")
  })

  it("shows 'Last 24h' for 24h preset", () => {
    expect(formatRangeSummary(NOW - 86400, NOW, NOW)).toBe("Last 24h")
  })

  it("shows 'Last 7d' for 7d preset", () => {
    expect(formatRangeSummary(NOW - 604800, NOW, NOW)).toBe("Last 7d")
  })

  it("shows 'Last 30d' for 30d preset", () => {
    expect(formatRangeSummary(NOW - 2592000, NOW, NOW)).toBe("Last 30d")
  })

  it("shows compact range for custom from+to", () => {
    // A range that doesn't match any preset
    const from = NOW - 5000
    const to = NOW - 2000
    const result = formatRangeSummary(from, to, NOW)
    expect(result).toMatch(/^\d{2}\/\d{2} \d{2}:\d{2} – \d{2}\/\d{2} \d{2}:\d{2}$/)
  })

  it("shows 'After MM/DD HH:mm' for from-only", () => {
    const result = formatRangeSummary(NOW - 3600, undefined, NOW)
    expect(result).toMatch(/^After \d{2}\/\d{2} \d{2}:\d{2}$/)
  })

  it("shows 'Before MM/DD HH:mm' for to-only", () => {
    const result = formatRangeSummary(undefined, NOW, NOW)
    expect(result).toMatch(/^Before \d{2}\/\d{2} \d{2}:\d{2}$/)
  })

  it("shows custom range when duration matches preset but to is old", () => {
    // Duration matches 1h but `to` is far in the past
    const to = NOW - 7200
    const from = to - 3600
    const result = formatRangeSummary(from, to, NOW)
    expect(result).toMatch(/^\d{2}\/\d{2} \d{2}:\d{2} – \d{2}\/\d{2} \d{2}:\d{2}$/)
  })
})

// ── 5.2: Preset Timestamp Computation ────────────────────────────

describe("tIME_RANGE_PRESETS", () => {
  it("has 5 presets", () => {
    expect(TIME_RANGE_PRESETS).toHaveLength(5)
  })

  it("presets are in ascending order by seconds", () => {
    for (let i = 1; i < TIME_RANGE_PRESETS.length; i++) {
      expect(TIME_RANGE_PRESETS[i].seconds).toBeGreaterThan(TIME_RANGE_PRESETS[i - 1].seconds)
    }
  })

  it("1h preset = 3600 seconds", () => {
    expect(TIME_RANGE_PRESETS[0]).toEqual({ label: "1h", seconds: 3600 })
  })

  it("6h preset = 21600 seconds", () => {
    expect(TIME_RANGE_PRESETS[1]).toEqual({ label: "6h", seconds: 21600 })
  })

  it("24h preset = 86400 seconds", () => {
    expect(TIME_RANGE_PRESETS[2]).toEqual({ label: "24h", seconds: 86400 })
  })

  it("7d preset = 604800 seconds", () => {
    expect(TIME_RANGE_PRESETS[3]).toEqual({ label: "7d", seconds: 604800 })
  })

  it("30d preset = 2592000 seconds", () => {
    expect(TIME_RANGE_PRESETS[4]).toEqual({ label: "30d", seconds: 2592000 })
  })

  it("each preset produces valid from/to timestamps", () => {
    const now = Math.floor(Date.now() / 1000)
    for (const preset of TIME_RANGE_PRESETS) {
      const from = now - preset.seconds
      const to = now
      expect(from).toBeLessThan(to)
      expect(to - from).toBe(preset.seconds)
    }
  })
})
