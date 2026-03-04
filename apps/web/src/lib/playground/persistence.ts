import type { PlaygroundMcpMode } from "@/hooks/use-playground-mcp"

type StorageLike = Pick<Storage, "getItem" | "setItem">

const PLAYGROUND_SETTINGS_KEY = "wheel.playground.settings.v1"
const PLAYGROUND_MCP_KEY = "wheel.playground.mcp.v1"

export interface PlaygroundSettingsSnapshot {
  model: string
  systemPrompt: string
  stream: boolean
  temperature: number
  maxTokens: number
  topP: number
}

export interface PlaygroundMcpSnapshot {
  enabled: boolean
  mode: PlaygroundMcpMode
  selectedKeys: string[]
  hasUserTouchedSelection: boolean
}

function getDefaultStorage(): StorageLike | null {
  if (typeof window === "undefined") return null
  try {
    return window.localStorage
  } catch {
    return null
  }
}

function clampNumber(value: unknown, min: number, max: number, fallback: number): number {
  if (typeof value !== "number" || Number.isNaN(value)) return fallback
  if (value < min) return min
  if (value > max) return max
  return value
}

function parseRecord(raw: string | null): Record<string, unknown> | null {
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as unknown
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) return null
    return parsed as Record<string, unknown>
  } catch {
    return null
  }
}

export function readPlaygroundSettings(storage: StorageLike | null = getDefaultStorage()) {
  if (!storage) return null
  const parsed = parseRecord(storage.getItem(PLAYGROUND_SETTINGS_KEY))
  if (!parsed) return null

  return {
    model: typeof parsed.model === "string" ? parsed.model : "",
    systemPrompt: typeof parsed.systemPrompt === "string" ? parsed.systemPrompt : "",
    stream: typeof parsed.stream === "boolean" ? parsed.stream : true,
    temperature: clampNumber(parsed.temperature, 0, 2, 0.7),
    maxTokens: Math.trunc(clampNumber(parsed.maxTokens, 1, 128000, 4096)),
    topP: clampNumber(parsed.topP, 0, 1, 1),
  } satisfies PlaygroundSettingsSnapshot
}

export function writePlaygroundSettings(
  snapshot: PlaygroundSettingsSnapshot,
  storage: StorageLike | null = getDefaultStorage(),
) {
  if (!storage) return
  try {
    storage.setItem(PLAYGROUND_SETTINGS_KEY, JSON.stringify(snapshot))
  } catch {
    // ignore storage failures
  }
}

export function readPlaygroundMcpSnapshot(storage: StorageLike | null = getDefaultStorage()) {
  if (!storage) return null
  const parsed = parseRecord(storage.getItem(PLAYGROUND_MCP_KEY))
  if (!parsed) return null

  const selectedKeys = Array.isArray(parsed.selectedKeys)
    ? parsed.selectedKeys.filter((x): x is string => typeof x === "string").sort()
    : []

  return {
    enabled: typeof parsed.enabled === "boolean" ? parsed.enabled : false,
    mode: parsed.mode === "manual" ? "manual" : "auto",
    selectedKeys,
    hasUserTouchedSelection:
      typeof parsed.hasUserTouchedSelection === "boolean"
        ? parsed.hasUserTouchedSelection
        : selectedKeys.length > 0,
  } satisfies PlaygroundMcpSnapshot
}

export function writePlaygroundMcpSnapshot(
  snapshot: PlaygroundMcpSnapshot,
  storage: StorageLike | null = getDefaultStorage(),
) {
  if (!storage) return
  try {
    storage.setItem(PLAYGROUND_MCP_KEY, JSON.stringify(snapshot))
  } catch {
    // ignore storage failures
  }
}
