import { describe, expect, it } from "vitest"
import { detectTruncation } from "./log"

// ── 3.3: Truncation Detection ────────────────────────────────────

describe("detectTruncation", () => {
  // ── Standard truncation markers ──

  describe("[truncated, N chars total] pattern", () => {
    it("detects standard truncation marker", () => {
      expect(detectTruncation("[truncated, 5000 chars total]")).toBe(true)
    })

    it("detects truncation marker without comma", () => {
      expect(detectTruncation("[truncated 5000 chars total]")).toBe(true)
    })

    it("detects truncation marker with large char counts", () => {
      expect(detectTruncation("[truncated, 123456 chars total]")).toBe(true)
    })

    it("detects truncation marker with extra whitespace", () => {
      expect(detectTruncation("[truncated,  5000  chars  total]")).toBe(true)
    })

    it("detects truncation marker embedded in content", () => {
      const content =
        '{"messages":[{"role":"user","content":"Hello [truncated, 2000 chars total]"}]}'
      expect(detectTruncation(content)).toBe(true)
    })
  })

  // ── Messages omitted markers ──

  describe("[N messages omitted] pattern", () => {
    it("detects single message omitted", () => {
      expect(detectTruncation("[1 message omitted")).toBe(true)
    })

    it("detects plural messages omitted", () => {
      expect(detectTruncation("[5 messages omitted")).toBe(true)
    })

    it("detects large message count omitted", () => {
      expect(detectTruncation("[100 messages omitted")).toBe(true)
    })

    it("detects with closing bracket and context", () => {
      expect(detectTruncation("[3 messages omitted, showing first and last]")).toBe(true)
    })
  })

  // ── Image data omitted ──

  describe("[image data omitted] pattern", () => {
    it("detects image data omitted marker", () => {
      expect(detectTruncation("[image data omitted]")).toBe(true)
    })

    it("detects image data omitted embedded in JSON", () => {
      const content =
        '{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"[image data omitted]"}}]}]}'
      expect(detectTruncation(content)).toBe(true)
    })
  })

  // ── No truncation ──

  describe("non-truncated content", () => {
    it("returns false for regular content", () => {
      expect(detectTruncation('{"messages":[{"role":"user","content":"Hello"}]}')).toBe(false)
    })

    it("returns false for empty string", () => {
      expect(detectTruncation("")).toBe(false)
    })

    it("returns false for content mentioning truncation in text", () => {
      expect(detectTruncation("The text was truncated by the user")).toBe(false)
    })

    it("returns false for content with brackets but no marker", () => {
      expect(detectTruncation("[some other content]")).toBe(false)
    })

    it("returns false for content with 'omitted' but no matching pattern", () => {
      expect(detectTruncation("This section was omitted")).toBe(false)
    })
  })

  // ── Multiple markers ──

  describe("multiple truncation markers", () => {
    it("detects when multiple markers are present", () => {
      const content = "[truncated, 5000 chars total] and [image data omitted]"
      expect(detectTruncation(content)).toBe(true)
    })
  })
})
