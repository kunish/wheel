import type { ModelMeta } from "@/lib/api"
import { describe, expect, it } from "vitest"
import { fuzzyLookup } from "./use-model-meta"

// ── Helpers ──────────────────────────────────────────────────────────

/** Shorthand to create a ModelMeta entry */
function meta(name: string, provider = "anthropic"): ModelMeta {
  return { name, provider, providerName: provider, logoUrl: "" }
}

// ── Fixtures ─────────────────────────────────────────────────────────

/**
 * Simulated models.dev metadata map — mirrors the real data shape
 * where dated variants often have unbeautified names.
 */
const MODEL_MAP: Record<string, ModelMeta> = {
  // Anthropic — canonical (beautified)
  "claude-sonnet-4-5": meta("Claude Sonnet 4.5"),
  "claude-opus-4-6": meta("Claude Opus 4.6"),
  "claude-haiku-4-5": meta("Claude Haiku 4.5"),
  "claude-opus-4-5": meta("Claude Opus 4.5"),
  "claude-sonnet-4": meta("Claude Sonnet 4"),

  // Anthropic — dated variants (unbeautified — models.dev stores raw ID as name)
  "claude-sonnet-4-5-20250929": meta("claude-sonnet-4-5-20250929"),
  "claude-haiku-4-5-20251001": meta("claude-haiku-4-5-20251001"),
  "claude-opus-4-5-20251101": meta("claude-opus-4-5-20251101"),

  // Anthropic — thinking variants (unbeautified)
  "claude-sonnet-4-5-20250929-thinking": meta("claude-sonnet-4-5-20250929-thinking"),
  "claude-opus-4-5-20251101-thinking": meta("claude-opus-4-5-20251101-thinking"),

  // OpenAI — beautified
  "gpt-4o": meta("GPT-4o", "openai"),
  "gpt-4o-2024-11-20": meta("GPT-4o (2024-11-20)", "openai"),
  "gpt-4o-mini": meta("GPT-4o Mini", "openai"),
  o1: meta("OpenAI o1", "openai"),
  "o1-preview": meta("OpenAI o1 Preview", "openai"),

  // Google — beautified
  "gemini-2.0-flash": meta("Gemini 2.0 Flash", "google"),

  // DeepSeek — beautified
  "deepseek-chat": meta("DeepSeek V3", "deepseek"),
  "deepseek-reasoner": meta("DeepSeek R1", "deepseek"),
}

// ── Tests ────────────────────────────────────────────────────────────

describe("fuzzyLookup", () => {
  // ── Exact match ──────────────────────────────────────────────────

  describe("exact match", () => {
    it("returns beautified name for exact match", () => {
      const result = fuzzyLookup(MODEL_MAP, "claude-opus-4-6")
      expect(result?.name).toBe("Claude Opus 4.6")
    })

    it("returns beautified name for OpenAI model", () => {
      const result = fuzzyLookup(MODEL_MAP, "gpt-4o")
      expect(result?.name).toBe("GPT-4o")
    })

    it("returns beautified name for dated OpenAI model (already beautified in source)", () => {
      const result = fuzzyLookup(MODEL_MAP, "gpt-4o-2024-11-20")
      expect(result?.name).toBe("GPT-4o (2024-11-20)")
    })

    it("returns null for unknown model", () => {
      expect(fuzzyLookup(MODEL_MAP, "unknown-model-xyz")).toBeNull()
    })
  })

  // ── Date suffix stripping (YYYYMMDD) ─────────────────────────────

  describe("date suffix — YYYYMMDD format", () => {
    it("beautifies claude-sonnet-4-5-20250929 → Claude Sonnet 4.5", () => {
      const result = fuzzyLookup(MODEL_MAP, "claude-sonnet-4-5-20250929")
      expect(result?.name).toBe("Claude Sonnet 4.5")
    })

    it("beautifies claude-haiku-4-5-20251001 → Claude Haiku 4.5", () => {
      const result = fuzzyLookup(MODEL_MAP, "claude-haiku-4-5-20251001")
      expect(result?.name).toBe("Claude Haiku 4.5")
    })

    it("beautifies claude-opus-4-5-20251101 → Claude Opus 4.5", () => {
      const result = fuzzyLookup(MODEL_MAP, "claude-opus-4-5-20251101")
      expect(result?.name).toBe("Claude Opus 4.5")
    })

    it("preserves provider/logo from the exact-matched entry", () => {
      const result = fuzzyLookup(MODEL_MAP, "claude-sonnet-4-5-20250929")
      expect(result?.provider).toBe("anthropic")
      expect(result?.logoUrl).toBe("")
    })
  })

  // ── Date suffix stripping (YYYY-MM-DD) ───────────────────────────

  describe("date suffix — YYYY-MM-DD format", () => {
    it("falls back via date stripping when not in map", () => {
      // gpt-4o-mini-2024-07-18 is not in the map, but gpt-4o-mini is
      const result = fuzzyLookup(MODEL_MAP, "gpt-4o-mini-2024-07-18")
      expect(result?.name).toBe("GPT-4o Mini")
    })
  })

  // ── Suffix stripping ─────────────────────────────────────────────

  describe("suffix stripping", () => {
    it("strips -thinking suffix to find base model", () => {
      // claude-opus-4-6-thinking → claude-opus-4-6
      const result = fuzzyLookup(MODEL_MAP, "claude-opus-4-6-thinking")
      expect(result?.name).toBe("Claude Opus 4.6")
    })

    it("strips -latest suffix", () => {
      const result = fuzzyLookup(MODEL_MAP, "claude-sonnet-4-5-latest")
      expect(result?.name).toBe("Claude Sonnet 4.5")
    })

    it("strips -preview suffix", () => {
      const result = fuzzyLookup(MODEL_MAP, "claude-sonnet-4-preview")
      expect(result?.name).toBe("Claude Sonnet 4")
    })

    it("strips -online suffix", () => {
      const result = fuzzyLookup(MODEL_MAP, "deepseek-chat-online")
      expect(result?.name).toBe("DeepSeek V3")
    })
  })

  // ── Combined: suffix + date ──────────────────────────────────────

  describe("suffix + date combined stripping", () => {
    it("strips -thinking then date: claude-sonnet-4-5-20250929-thinking → Claude Sonnet 4.5", () => {
      const result = fuzzyLookup(MODEL_MAP, "claude-sonnet-4-5-20250929-thinking")
      expect(result?.name).toBe("Claude Sonnet 4.5")
    })

    it("strips -thinking then date: claude-opus-4-5-20251101-thinking → Claude Opus 4.5", () => {
      const result = fuzzyLookup(MODEL_MAP, "claude-opus-4-5-20251101-thinking")
      expect(result?.name).toBe("Claude Opus 4.5")
    })
  })

  // ── Prefix stripping ─────────────────────────────────────────────

  describe("prefix stripping", () => {
    it("strips kiro- prefix", () => {
      const result = fuzzyLookup(MODEL_MAP, "kiro-claude-opus-4-6")
      expect(result?.name).toBe("Claude Opus 4.6")
    })

    it("strips kiro- prefix + -thinking suffix", () => {
      const result = fuzzyLookup(MODEL_MAP, "kiro-claude-opus-4-6-thinking")
      expect(result?.name).toBe("Claude Opus 4.6")
    })
  })

  // ── Beautification logic ─────────────────────────────────────────

  describe("name beautification", () => {
    it("does NOT alter an already-beautified name", () => {
      const result = fuzzyLookup(MODEL_MAP, "gpt-4o-2024-11-20")
      // The source entry already has a proper display name
      expect(result?.name).toBe("GPT-4o (2024-11-20)")
    })

    it("replaces raw name with base model's beautified name", () => {
      // claude-sonnet-4-5-20250929 has raw name "claude-sonnet-4-5-20250929"
      // should be replaced with "Claude Sonnet 4.5" from claude-sonnet-4-5
      const result = fuzzyLookup(MODEL_MAP, "claude-sonnet-4-5-20250929")
      expect(result?.name).not.toBe("claude-sonnet-4-5-20250929")
      expect(result?.name).toBe("Claude Sonnet 4.5")
    })

    it("returns raw name when no beautified base model exists", () => {
      const mapWithNoBase: Record<string, ModelMeta> = {
        "custom-model-20250101": meta("custom-model-20250101"),
      }
      const result = fuzzyLookup(mapWithNoBase, "custom-model-20250101")
      expect(result?.name).toBe("custom-model-20250101")
    })
  })

  // ── Edge cases ───────────────────────────────────────────────────

  describe("edge cases", () => {
    it("returns null for empty string", () => {
      expect(fuzzyLookup(MODEL_MAP, "")).toBeNull()
    })

    it("returns null for empty map", () => {
      expect(fuzzyLookup({}, "claude-opus-4-6")).toBeNull()
    })

    it("handles model ID that looks like a date but isn't", () => {
      const mapWithNumericSuffix: Record<string, ModelMeta> = {
        "model-12345678": meta("Model 12345678"),
      }
      const result = fuzzyLookup(mapWithNumericSuffix, "model-12345678")
      expect(result?.name).toBe("Model 12345678")
    })

    it("does not strip date from middle of model ID", () => {
      // Only trailing dates should be stripped
      const result = fuzzyLookup(MODEL_MAP, "20250929-claude-sonnet")
      expect(result).toBeNull()
    })

    it("handles model with only date suffix (no base in map)", () => {
      const result = fuzzyLookup(MODEL_MAP, "nonexistent-20250101")
      expect(result).toBeNull()
    })

    it("prefers exact match over suffix-stripped match", () => {
      const mapWithBoth: Record<string, ModelMeta> = {
        "model-preview": meta("Model Preview Edition"),
        model: meta("Model Base"),
      }
      const result = fuzzyLookup(mapWithBoth, "model-preview")
      expect(result?.name).toBe("Model Preview Edition")
    })

    it("handles multiple applicable suffixes — first match wins", () => {
      // -thinking is before -latest in STRIP_SUFFIXES
      const mapWithBase: Record<string, ModelMeta> = {
        model: meta("Model Base"),
      }
      const result = fuzzyLookup(mapWithBase, "model-thinking")
      expect(result?.name).toBe("Model Base")
    })
  })

  // ── Remaining STRIP_SUFFIXES ─────────────────────────────────────

  describe("all STRIP_SUFFIXES coverage", () => {
    const baseMap: Record<string, ModelMeta> = {
      "some-model": meta("Some Model", "test"),
    }

    it("strips -high suffix", () => {
      const result = fuzzyLookup(baseMap, "some-model-high")
      expect(result?.name).toBe("Some Model")
    })

    it("strips -low suffix", () => {
      const result = fuzzyLookup(baseMap, "some-model-low")
      expect(result?.name).toBe("Some Model")
    })

    it("strips -medium suffix", () => {
      const result = fuzzyLookup(baseMap, "some-model-medium")
      expect(result?.name).toBe("Some Model")
    })

    it("does not strip unknown suffix", () => {
      const result = fuzzyLookup(baseMap, "some-model-turbo")
      expect(result).toBeNull()
    })
  })

  // ── findMeta step 3: prefix strip with inner suffix loop ────────

  describe("prefix + suffix inner loop (findMeta step 3)", () => {
    it("strips prefix then suffix when prefix-only is not in map", () => {
      // Map has "base-model" but not "kiro-base-model"
      // kiro-base-model-latest → strip "kiro-" → "base-model-latest" (not in map)
      //   → inner loop: strip "-latest" → "base-model" (in map!)
      const m: Record<string, ModelMeta> = {
        "base-model": meta("Base Model"),
      }
      const result = fuzzyLookup(m, "kiro-base-model-latest")
      expect(result?.name).toBe("Base Model")
    })

    it("strips prefix then suffix for -online", () => {
      const m: Record<string, ModelMeta> = {
        "deep-model": meta("Deep Model", "deepseek"),
      }
      const result = fuzzyLookup(m, "kiro-deep-model-online")
      expect(result?.name).toBe("Deep Model")
    })

    it("strips prefix then suffix for -high", () => {
      const m: Record<string, ModelMeta> = {
        "deep-model": meta("Deep Model", "deepseek"),
      }
      const result = fuzzyLookup(m, "kiro-deep-model-high")
      expect(result?.name).toBe("Deep Model")
    })

    it("returns null when prefix stripped but neither base nor base-suffix in map", () => {
      const m: Record<string, ModelMeta> = {
        "other-model": meta("Other"),
      }
      const result = fuzzyLookup(m, "kiro-nonexistent-thinking")
      expect(result).toBeNull()
    })
  })

  // ── findMeta step 4: date-only stripping (not via exact match) ──

  describe("date-only stripping (findMeta step 4)", () => {
    it("strips YYYYMMDD date when exact match not in map", () => {
      // Model ID with date is NOT in the map, but base is
      const m: Record<string, ModelMeta> = {
        "new-model": meta("New Model"),
      }
      const result = fuzzyLookup(m, "new-model-20260101")
      expect(result?.name).toBe("New Model")
    })

    it("strips YYYY-MM-DD date when exact match not in map", () => {
      const m: Record<string, ModelMeta> = {
        "new-model": meta("New Model", "openai"),
      }
      const result = fuzzyLookup(m, "new-model-2026-01-15")
      expect(result?.name).toBe("New Model")
    })

    it("returns null when date-stripped base not in map either", () => {
      const m: Record<string, ModelMeta> = {
        "other-model": meta("Other"),
      }
      const result = fuzzyLookup(m, "missing-model-20260101")
      expect(result).toBeNull()
    })
  })

  // ── findMeta step 5: suffix + date (not exact) ─────────────────

  describe("suffix + date combined (findMeta step 5)", () => {
    it("strips -latest then YYYYMMDD date", () => {
      const m: Record<string, ModelMeta> = {
        "model-x": meta("Model X"),
      }
      const result = fuzzyLookup(m, "model-x-20260201-latest")
      expect(result?.name).toBe("Model X")
    })

    it("strips -online then YYYY-MM-DD date", () => {
      const m: Record<string, ModelMeta> = {
        "model-y": meta("Model Y", "openai"),
      }
      const result = fuzzyLookup(m, "model-y-2026-03-15-online")
      expect(result?.name).toBe("Model Y")
    })

    it("strips -high then YYYYMMDD date", () => {
      const m: Record<string, ModelMeta> = {
        "model-z": meta("Model Z"),
      }
      const result = fuzzyLookup(m, "model-z-20260101-high")
      expect(result?.name).toBe("Model Z")
    })

    it("returns null when suffix+date stripped base not in map", () => {
      const result = fuzzyLookup(MODEL_MAP, "nonexistent-20260101-thinking")
      expect(result).toBeNull()
    })
  })

  // ── Beautification: suffix-only raw names ──────────────────────

  describe("beautification with suffix-only (no date)", () => {
    it("beautifies raw name via suffix stripping when base has display name", () => {
      const m: Record<string, ModelMeta> = {
        "my-model": meta("My Model"),
        "my-model-thinking": meta("my-model-thinking"), // raw
      }
      const result = fuzzyLookup(m, "my-model-thinking")
      expect(result?.name).toBe("My Model")
    })

    it("keeps raw name when base model also has raw name", () => {
      const m: Record<string, ModelMeta> = {
        "my-model": meta("my-model"), // also raw
        "my-model-thinking": meta("my-model-thinking"), // raw
      }
      const result = fuzzyLookup(m, "my-model-thinking")
      expect(result?.name).toBe("my-model-thinking")
    })
  })

  // ── Beautification: YYYY-MM-DD date format ─────────────────────

  describe("beautification with YYYY-MM-DD date", () => {
    it("beautifies raw name with YYYY-MM-DD date", () => {
      const m: Record<string, ModelMeta> = {
        "gpt-4o-mini": meta("GPT-4o Mini", "openai"),
        "gpt-4o-mini-2024-07-18": meta("gpt-4o-mini-2024-07-18", "openai"), // raw
      }
      const result = fuzzyLookup(m, "gpt-4o-mini-2024-07-18")
      expect(result?.name).toBe("GPT-4o Mini")
    })
  })

  // ── Beautification: suffix + YYYY-MM-DD date combined ──────────

  describe("beautification with suffix + YYYY-MM-DD date", () => {
    it("beautifies raw name with both suffix and YYYY-MM-DD date", () => {
      const m: Record<string, ModelMeta> = {
        "gpt-4o": meta("GPT-4o", "openai"),
        "gpt-4o-2024-11-20-thinking": meta("gpt-4o-2024-11-20-thinking", "openai"), // raw
      }
      const result = fuzzyLookup(m, "gpt-4o-2024-11-20-thinking")
      expect(result?.name).toBe("GPT-4o")
    })
  })

  // ── Return value completeness ──────────────────────────────────

  describe("return value completeness", () => {
    it("preserves all ModelMeta fields on suffix-stripped match", () => {
      const result = fuzzyLookup(MODEL_MAP, "deepseek-chat-online")
      expect(result).toEqual({
        name: "DeepSeek V3",
        provider: "deepseek",
        providerName: "deepseek",
        logoUrl: "",
      })
    })

    it("preserves provider from original entry when beautifying name", () => {
      // claude-sonnet-4-5-20250929 exact entry has provider "anthropic"
      // beautification replaces name but keeps rest from findMeta result
      const result = fuzzyLookup(MODEL_MAP, "claude-sonnet-4-5-20250929")
      expect(result?.provider).toBe("anthropic")
      expect(result?.providerName).toBe("anthropic")
      expect(result?.logoUrl).toBe("")
    })

    it("preserves provider from original entry when beautifying via suffix+date", () => {
      const result = fuzzyLookup(MODEL_MAP, "claude-sonnet-4-5-20250929-thinking")
      expect(result?.provider).toBe("anthropic")
      expect(result?.providerName).toBe("anthropic")
    })

    it("returns full ModelMeta object on exact match", () => {
      const result = fuzzyLookup(MODEL_MAP, "gemini-2.0-flash")
      expect(result).toEqual({
        name: "Gemini 2.0 Flash",
        provider: "google",
        providerName: "google",
        logoUrl: "",
      })
    })
  })

  // ── isRawName boundary cases ───────────────────────────────────

  describe("isRawName detection boundary cases", () => {
    it("treats name with uppercase as beautified (no replacement)", () => {
      const m: Record<string, ModelMeta> = {
        "Model-V2": meta("Model-V2"), // has uppercase → not raw
      }
      const result = fuzzyLookup(m, "Model-V2")
      expect(result?.name).toBe("Model-V2")
    })

    it("treats name with spaces as beautified (no replacement)", () => {
      const m: Record<string, ModelMeta> = {
        "model v2": meta("model v2"), // has space → not raw (even lowercase)
      }
      const result = fuzzyLookup(m, "model v2")
      expect(result?.name).toBe("model v2")
    })

    it("treats all-lowercase hyphenated name as raw", () => {
      const m: Record<string, ModelMeta> = {
        "model-v2": meta("model-v2"), // all lowercase, no spaces → raw
      }
      const result = fuzzyLookup(m, "model-v2")
      expect(result?.name).toBe("model-v2")
    })

    it("treats name with dots but no uppercase/spaces as raw", () => {
      const m: Record<string, ModelMeta> = {
        "gemini-2.0-flash-lite": meta("gemini-2.0-flash-lite"),
      }
      const result = fuzzyLookup(m, "gemini-2.0-flash-lite")
      expect(result?.name).toBe("gemini-2.0-flash-lite")
    })
  })

  // ── stripDate edge cases ───────────────────────────────────────

  describe("stripDate edge cases", () => {
    it("does not strip 7-digit suffix (not a date)", () => {
      const m: Record<string, ModelMeta> = {
        model: meta("Model"),
      }
      // "model-1234567" — 7 digits, should NOT be stripped as YYYYMMDD
      const result = fuzzyLookup(m, "model-1234567")
      expect(result).toBeNull()
    })

    it("does not strip 9-digit suffix", () => {
      const m: Record<string, ModelMeta> = {
        model: meta("Model"),
      }
      const result = fuzzyLookup(m, "model-123456789")
      expect(result).toBeNull()
    })

    it("does not strip partial YYYY-MM date (missing day)", () => {
      const m: Record<string, ModelMeta> = {
        model: meta("Model"),
      }
      const result = fuzzyLookup(m, "model-2024-01")
      expect(result).toBeNull()
    })

    it("strips date even when model name contains dots", () => {
      const m: Record<string, ModelMeta> = {
        "gemini-2.0-flash": meta("Gemini 2.0 Flash", "google"),
      }
      const result = fuzzyLookup(m, "gemini-2.0-flash-20260101")
      expect(result?.name).toBe("Gemini 2.0 Flash")
    })
  })

  // ── Fallback chain priority ────────────────────────────────────

  describe("fallback chain priority", () => {
    it("prefers suffix match (step 2) over date match (step 4)", () => {
      // If both suffix and date could match, suffix comes first
      const m: Record<string, ModelMeta> = {
        model: meta("Model Base"),
        "model-20260101": meta("Model Dated"),
      }
      // "model-20260101-thinking" → step 2: strip -thinking → "model-20260101" (in map!) → stops
      const result = fuzzyLookup(m, "model-20260101-thinking")
      expect(result?.name).toBe("Model Dated")
    })

    it("prefers prefix match (step 3) over date match (step 4)", () => {
      const m: Record<string, ModelMeta> = {
        "claude-model": meta("Claude Model"),
      }
      // "kiro-claude-model" → step 3: strip "kiro-" → "claude-model" (in map!)
      const result = fuzzyLookup(m, "kiro-claude-model")
      expect(result?.name).toBe("Claude Model")
    })

    it("falls through to step 5 when steps 1-4 all miss", () => {
      const m: Record<string, ModelMeta> = {
        "base-model": meta("Base Model"),
      }
      // "base-model-20260101-thinking" →
      //   step 1: not exact → step 2: strip -thinking → "base-model-20260101" (not in map)
      //   step 3: no prefix → step 4: strip date "base-model-20260101" → no YYYYMMDD at end of full ID with -thinking
      //   step 5: strip -thinking → "base-model-20260101" → strip date → "base-model" (in map!)
      const result = fuzzyLookup(m, "base-model-20260101-thinking")
      expect(result?.name).toBe("Base Model")
    })
  })

  // ── Multiple providers ─────────────────────────────────────────

  describe("cross-provider scenarios", () => {
    it("handles same model name from different providers independently", () => {
      const m: Record<string, ModelMeta> = {
        "model-v1": meta("Anthropic V1", "anthropic"),
      }
      const result = fuzzyLookup(m, "model-v1")
      expect(result?.provider).toBe("anthropic")
    })

    it("preserves correct provider through date stripping", () => {
      const m: Record<string, ModelMeta> = {
        "o3-mini": meta("OpenAI o3 Mini", "openai"),
      }
      const result = fuzzyLookup(m, "o3-mini-2025-01-31")
      expect(result?.name).toBe("OpenAI o3 Mini")
      expect(result?.provider).toBe("openai")
    })

    it("preserves correct provider through suffix stripping", () => {
      const m: Record<string, ModelMeta> = {
        "gemini-2.0-flash": meta("Gemini 2.0 Flash", "google"),
      }
      const result = fuzzyLookup(m, "gemini-2.0-flash-latest")
      expect(result?.name).toBe("Gemini 2.0 Flash")
      expect(result?.provider).toBe("google")
    })
  })

  // ── Special characters and unusual IDs ─────────────────────────

  describe("unusual model IDs", () => {
    it("handles model ID with only hyphens", () => {
      expect(fuzzyLookup(MODEL_MAP, "---")).toBeNull()
    })

    it("handles very long model ID", () => {
      const longId = "a".repeat(200)
      expect(fuzzyLookup(MODEL_MAP, longId)).toBeNull()
    })

    it("handles model ID that is just a date", () => {
      expect(fuzzyLookup(MODEL_MAP, "20250929")).toBeNull()
    })

    it("handles model ID that is a prefix only", () => {
      expect(fuzzyLookup(MODEL_MAP, "kiro-")).toBeNull()
    })

    it("handles model ID that is a suffix only", () => {
      expect(fuzzyLookup(MODEL_MAP, "-thinking")).toBeNull()
    })

    it("handles model ID with colons (provider:model format)", () => {
      const m: Record<string, ModelMeta> = {
        "anthropic:claude-3": meta("Claude 3", "anthropic"),
      }
      const result = fuzzyLookup(m, "anthropic:claude-3")
      expect(result?.name).toBe("Claude 3")
    })

    it("handles model ID with slashes (org/model format)", () => {
      const m: Record<string, ModelMeta> = {
        "meta-llama/llama-3.1-70b": meta("Llama 3.1 70B", "meta"),
      }
      const result = fuzzyLookup(m, "meta-llama/llama-3.1-70b")
      expect(result?.name).toBe("Llama 3.1 70B")
    })
  })

  // ── Beautification: baseId === modelId guard ───────────────────

  describe("beautification baseId === modelId guard", () => {
    it("does not beautify when stripping produces same ID (no suffix or date)", () => {
      // A raw name that has no strippable suffix or date → baseId stays equal to modelId
      const m: Record<string, ModelMeta> = {
        "raw-model-name": meta("raw-model-name"),
      }
      const result = fuzzyLookup(m, "raw-model-name")
      expect(result?.name).toBe("raw-model-name")
    })

    it("beautifies when stripping suffix changes baseId", () => {
      const m: Record<string, ModelMeta> = {
        "raw-model": meta("Raw Model Display"),
        "raw-model-latest": meta("raw-model-latest"), // raw
      }
      const result = fuzzyLookup(m, "raw-model-latest")
      expect(result?.name).toBe("Raw Model Display")
    })

    it("beautifies when stripping date changes baseId", () => {
      const m: Record<string, ModelMeta> = {
        "raw-model": meta("Raw Model Display"),
        "raw-model-20260201": meta("raw-model-20260201"), // raw
      }
      const result = fuzzyLookup(m, "raw-model-20260201")
      expect(result?.name).toBe("Raw Model Display")
    })
  })

  // ── Real-world models.dev scenarios ──────────────────────────────

  describe("real-world scenarios", () => {
    it("claude Code request model: claude-sonnet-4-5-20250929", () => {
      expect(fuzzyLookup(MODEL_MAP, "claude-sonnet-4-5-20250929")?.name).toBe("Claude Sonnet 4.5")
    })

    it("claude Code request model: claude-opus-4-6", () => {
      expect(fuzzyLookup(MODEL_MAP, "claude-opus-4-6")?.name).toBe("Claude Opus 4.6")
    })

    it("relay actual model: claude-sonnet-4-5-20250929-thinking", () => {
      expect(fuzzyLookup(MODEL_MAP, "claude-sonnet-4-5-20250929-thinking")?.name).toBe(
        "Claude Sonnet 4.5",
      )
    })

    it("openAI model with date: gpt-4o-2024-11-20", () => {
      expect(fuzzyLookup(MODEL_MAP, "gpt-4o-2024-11-20")?.name).toBe("GPT-4o (2024-11-20)")
    })

    it("google model: gemini-2.0-flash", () => {
      expect(fuzzyLookup(MODEL_MAP, "gemini-2.0-flash")?.name).toBe("Gemini 2.0 Flash")
    })

    it("deepSeek model: deepseek-reasoner", () => {
      expect(fuzzyLookup(MODEL_MAP, "deepseek-reasoner")?.name).toBe("DeepSeek R1")
    })
  })
})
