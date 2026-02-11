// ───────────── formatting helpers ─────────────

function safeNum(n: number): number {
  return Number.isFinite(n) ? n : 0
}

export function formatCount(n: number): { value: string; unit: string; raw: number } {
  n = safeNum(n)
  if (n >= 1_000_000_000) return { value: (n / 1_000_000_000).toFixed(1), unit: "B", raw: n }
  if (n >= 1_000_000) return { value: (n / 1_000_000).toFixed(1), unit: "M", raw: n }
  if (n >= 1_000) return { value: (n / 1_000).toFixed(1), unit: "K", raw: n }
  return { value: String(Math.round(n)), unit: "", raw: n }
}

export function formatMoney(n: number): { value: string; unit: string; raw: number } {
  n = safeNum(n)
  if (n >= 1_000_000) return { value: `$${(n / 1_000_000).toFixed(2)}`, unit: "M", raw: n }
  if (n >= 1_000) return { value: `$${(n / 1_000).toFixed(2)}`, unit: "K", raw: n }
  if (n >= 1) return { value: `$${n.toFixed(2)}`, unit: "", raw: n }
  return { value: `$${n.toFixed(4)}`, unit: "", raw: n }
}

export function formatTime(ms: number): { value: string; unit: string; raw: number } {
  ms = safeNum(ms)
  if (ms >= 3600000) return { value: (ms / 3600000).toFixed(1), unit: "h", raw: ms }
  if (ms >= 60000) return { value: (ms / 60000).toFixed(1), unit: "m", raw: ms }
  if (ms >= 1000) return { value: (ms / 1000).toFixed(1), unit: "s", raw: ms }
  return { value: String(Math.round(ms)), unit: "ms", raw: ms }
}
