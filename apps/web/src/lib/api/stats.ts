import type {
  ChannelStatsRow,
  ModelStatsItem,
  StatsDaily,
  StatsHourly,
  StatsMetrics,
} from "../types/stats"
import { apiFetch } from "./client"

// Re-export stats types for convenience
export type {
  ChannelStatsRow,
  ModelMeta,
  ModelStatsItem,
  StatsDaily,
  StatsHourly,
  StatsMetrics,
} from "../types/stats"

// ── Stats ──

export function getChannelStats() {
  return apiFetch<{ success: boolean; data: ChannelStatsRow[] }>("/api/v1/stats/channel")
}

export function getTotalStats() {
  return apiFetch<{ success: boolean; data: StatsMetrics }>("/api/v1/stats/total")
}

function getBrowserTz(): string {
  const offset = -new Date().getTimezoneOffset()
  const sign = offset >= 0 ? "+" : "-"
  const abs = Math.abs(offset)
  const h = String(Math.floor(abs / 60)).padStart(2, "0")
  const m = String(abs % 60).padStart(2, "0")
  return `${sign}${h}:${m}`
}

export function getDailyStats() {
  return apiFetch<{ success: boolean; data: StatsDaily[] }>(
    `/api/v1/stats/daily?tz=${encodeURIComponent(getBrowserTz())}`,
  )
}

export function getHourlyStats(start?: string, end?: string) {
  const params = new URLSearchParams()
  if (start) params.set("start", start)
  if (end) params.set("end", end)
  params.set("tz", getBrowserTz())
  return apiFetch<{ success: boolean; data: StatsHourly[] }>(
    `/api/v1/stats/hourly?${params.toString()}`,
  )
}

export function getModelStats() {
  return apiFetch<{ success: boolean; data: ModelStatsItem[] }>("/api/v1/stats/model")
}
