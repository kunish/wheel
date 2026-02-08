import type { ModelMeta } from "@/lib/api"
import { useQuery } from "@tanstack/react-query"
import { getModelMetadata } from "@/lib/api"

export function useModelMetadataQuery() {
  return useQuery({
    queryKey: ["model-metadata"],
    queryFn: getModelMetadata,
    staleTime: 60 * 60 * 1000, // 1 hour
  })
}

// Suffixes that models.dev doesn't track — strip them for fallback matching
const STRIP_SUFFIXES = ["-thinking", "-latest", "-online", "-preview", "-high", "-low", "-medium"]

// Prefixes added by resellers / wrappers — strip to find canonical model
const STRIP_PREFIXES = ["kiro-"]

// Date patterns appended to model IDs:
//   YYYY-MM-DD (e.g. gpt-4o-2024-11-20)
//   YYYYMMDD   (e.g. claude-sonnet-4-5-20250929)
const DATE_SUFFIXES = [
  /-\d{4}-\d{2}-\d{2}$/, // YYYY-MM-DD
  /-\d{8}$/, // YYYYMMDD
]

/**
 * Check if a ModelMeta entry has an "unbeautified" name
 * (i.e. the name field is just the raw model ID, not a human-friendly display name).
 */
function isRawName(name: string): boolean {
  // Human-friendly names contain uppercase or spaces (e.g. "Claude Sonnet 4.5")
  // Raw IDs are all lowercase with hyphens/dots (e.g. "claude-sonnet-4-5-20250929")
  return name === name.toLowerCase() && !name.includes(" ")
}

/**
 * Strip trailing date segments from a model ID.
 * Supports both YYYY-MM-DD and YYYYMMDD formats.
 */
function stripDate(modelId: string): string | null {
  for (const re of DATE_SUFFIXES) {
    const stripped = modelId.replace(re, "")
    if (stripped !== modelId) return stripped
  }
  return null
}

/**
 * Try progressively more aggressive normalization to find a match in the map.
 * Returns [meta, matchedKey] or null.
 */
function findMeta(map: Record<string, ModelMeta>, modelId: string): ModelMeta | null {
  // 1. Exact match
  if (map[modelId]) return map[modelId]

  // 2. Strip known suffixes (e.g. claude-opus-4-5-thinking → claude-opus-4-5)
  for (const suffix of STRIP_SUFFIXES) {
    if (modelId.endsWith(suffix)) {
      const base = modelId.slice(0, -suffix.length)
      if (map[base]) return map[base]
    }
  }

  // 3. Strip known prefixes (e.g. kiro-claude-opus-4-5 → claude-opus-4-5)
  for (const prefix of STRIP_PREFIXES) {
    if (modelId.startsWith(prefix)) {
      const base = modelId.slice(prefix.length)
      if (map[base]) return map[base]
      for (const suffix of STRIP_SUFFIXES) {
        if (base.endsWith(suffix)) {
          const inner = base.slice(0, -suffix.length)
          if (map[inner]) return map[inner]
        }
      }
    }
  }

  // 4. Strip trailing date (e.g. claude-sonnet-4-5-20250929 → claude-sonnet-4-5)
  const dateStripped = stripDate(modelId)
  if (dateStripped && map[dateStripped]) return map[dateStripped]

  // 5. Strip suffix then date (e.g. claude-sonnet-4-5-20250929-thinking → claude-sonnet-4-5)
  for (const suffix of STRIP_SUFFIXES) {
    if (modelId.endsWith(suffix)) {
      const base = modelId.slice(0, -suffix.length)
      const ds = stripDate(base)
      if (ds && map[ds]) return map[ds]
    }
  }

  return null
}

/**
 * Fuzzy lookup with display name beautification.
 *
 * models.dev sometimes stores the raw model ID as the name for dated variants
 * (e.g. claude-sonnet-4-5-20250929 → name: "claude-sonnet-4-5-20250929").
 * When this happens, we fall back to the undated base model's display name
 * (e.g. claude-sonnet-4-5 → name: "Claude Sonnet 4.5").
 */
export function fuzzyLookup(map: Record<string, ModelMeta>, modelId: string): ModelMeta | null {
  const meta = findMeta(map, modelId)
  if (!meta) return null

  // If the matched entry has a proper display name, use it directly
  if (!isRawName(meta.name)) return meta

  // The name is raw (e.g. "claude-sonnet-4-5-20250929").
  // Try to find a better display name from the base model (without date/suffix).
  let baseId = modelId

  // Strip suffixes first
  for (const suffix of STRIP_SUFFIXES) {
    if (baseId.endsWith(suffix)) {
      baseId = baseId.slice(0, -suffix.length)
      break
    }
  }

  // Strip date
  const ds = stripDate(baseId)
  if (ds) baseId = ds

  // Look up the base model
  if (baseId !== modelId && map[baseId] && !isRawName(map[baseId].name)) {
    return { ...meta, name: map[baseId].name }
  }

  return meta
}

export function useModelMeta(modelId: string): ModelMeta | null {
  const { data } = useModelMetadataQuery()
  if (!data?.data) return null
  return fuzzyLookup(data.data, modelId)
}
