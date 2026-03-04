export interface SelectedToolRef {
  clientId: number
  clientName: string
  toolName: string
}

export interface ToolAliasRef extends SelectedToolRef {
  alias: string
}

const MAX_ALIAS_LENGTH = 64

function sanitizeAliasPart(value: string): string {
  return value
    .normalize("NFKD")
    .replace(/[^\w\s-]/g, "_")
    .replace(/[\s-]+/g, "_")
    .replace(/_+/g, "_")
    .replace(/^_+|_+$/g, "")
}

function buildBaseAlias(clientName: string, toolName: string): string {
  const left = sanitizeAliasPart(clientName)
  const right = sanitizeAliasPart(toolName)
  const joined = sanitizeAliasPart(`${left}_${right}`)
  const withPrefix = /^\d/.test(joined) ? `tool_${joined}` : joined
  const safe = withPrefix || "tool"
  const truncated = safe.slice(0, MAX_ALIAS_LENGTH).replace(/_+$/g, "")
  return truncated || "tool"
}

function withCollisionSuffix(base: string, occurrence: number): string {
  if (occurrence <= 1) return base
  const suffix = `_${occurrence}`
  const head = base.slice(0, Math.max(1, MAX_ALIAS_LENGTH - suffix.length)).replace(/_+$/g, "")
  return `${head || "tool"}${suffix}`
}

export function buildToolAliasMap(selected: SelectedToolRef[]): Record<string, ToolAliasRef> {
  const out: Record<string, ToolAliasRef> = {}
  const seen = new Map<string, number>()
  for (const item of selected) {
    const base = buildBaseAlias(item.clientName, item.toolName)
    const n = (seen.get(base) ?? 0) + 1
    seen.set(base, n)
    const alias = withCollisionSuffix(base, n)
    out[alias] = { ...item, alias }
  }
  return out
}
