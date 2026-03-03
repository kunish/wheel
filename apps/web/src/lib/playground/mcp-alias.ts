export interface SelectedToolRef {
  clientId: number
  clientName: string
  toolName: string
}

export interface ToolAliasRef extends SelectedToolRef {
  alias: string
}

export function buildToolAliasMap(selected: SelectedToolRef[]): Record<string, ToolAliasRef> {
  const out: Record<string, ToolAliasRef> = {}
  const seen = new Map<string, number>()
  for (const item of selected) {
    const base = `${item.clientName}_${item.toolName}`.replace(/[\s-]+/g, "_")
    const n = (seen.get(base) ?? 0) + 1
    seen.set(base, n)
    const alias = n === 1 ? base : `${base}_${n}`
    out[alias] = { ...item, alias }
  }
  return out
}
