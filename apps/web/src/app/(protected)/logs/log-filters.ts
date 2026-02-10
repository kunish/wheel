// Extracted log filter utilities for testability
import { formatRangeSummary } from "@/components/time-range-picker"

export interface LogFilters {
  page: number
  model: string
  status: string
  channelId: number | undefined
  keyword: string
  pageSize: number
  startTime: number | undefined
  endTime: number | undefined
}

/** Parse URL search params into a structured LogFilters object */
export function parseLogFilters(searchParams: URLSearchParams): LogFilters {
  return {
    page: Number(searchParams.get("page") ?? 1),
    model: searchParams.get("model") ?? "",
    status: searchParams.get("status") ?? "all",
    channelId: searchParams.get("channel") ? Number(searchParams.get("channel")) : undefined,
    keyword: searchParams.get("q") ?? "",
    pageSize: Number(searchParams.get("size") ?? 20),
    startTime: searchParams.get("from") ? Number(searchParams.get("from")) : undefined,
    endTime: searchParams.get("to") ? Number(searchParams.get("to")) : undefined,
  }
}

export interface FilterChip {
  key: string
  label: string
  value: string
}

/** Generate active filter chips from current filter state */
export function getActiveFilterChips(filters: LogFilters, channelName?: string): FilterChip[] {
  const chips: FilterChip[] = []
  if (filters.keyword) chips.push({ key: "q", label: "Search", value: filters.keyword })
  if (filters.model) chips.push({ key: "model", label: "Model", value: filters.model })
  if (filters.channelId)
    chips.push({
      key: "channel",
      label: "Channel",
      value: channelName ?? String(filters.channelId),
    })
  if (filters.status !== "all")
    chips.push({ key: "status", label: "Status", value: filters.status })
  if (filters.startTime || filters.endTime)
    chips.push({
      key: "time",
      label: "Time",
      value: formatRangeSummary(filters.startTime, filters.endTime),
    })
  return chips
}

/** Check if any non-default filters are active */
export function hasActiveFilters(filters: LogFilters): boolean {
  return !!(
    filters.keyword ||
    filters.model ||
    filters.channelId ||
    filters.status !== "all" ||
    filters.startTime ||
    filters.endTime
  )
}

export const TIME_PRESETS = [
  { label: "1h", seconds: 3600 },
  { label: "6h", seconds: 21600 },
  { label: "24h", seconds: 86400 },
  { label: "7d", seconds: 604800 },
  { label: "30d", seconds: 2592000 },
] as const

/** Compute the `from` timestamp for a time range preset */
export function computeTimePresetFrom(presetSeconds: number, now?: number): number {
  const currentTime = now ?? Math.floor(Date.now() / 1000)
  return currentTime - presetSeconds
}

/** Count case-insensitive occurrences of needle in text */
export function countMatches(text: string, needle: string): number {
  if (!needle || !text) return 0
  const lower = text.toLowerCase()
  const n = needle.toLowerCase()
  let count = 0
  let idx = lower.indexOf(n)
  while (idx !== -1) {
    count++
    idx = lower.indexOf(n, idx + n.length)
  }
  return count
}
export function buildFilterSearchParams(
  current: URLSearchParams,
  updates: Record<string, string | number | undefined | null>,
  resetPage = true,
): URLSearchParams {
  const params = new URLSearchParams(current.toString())
  const isPageUpdate = "page" in updates

  for (const [key, value] of Object.entries(updates)) {
    if (value === undefined || value === null || value === "") {
      params.delete(key)
    } else {
      params.set(key, String(value))
    }
  }

  // Reset page to 1 when changing filters (not when paginating)
  if (!isPageUpdate && resetPage) {
    params.delete("page")
  }

  // Clean up default values
  if (params.get("status") === "all") params.delete("status")
  if (params.get("size") === "20") params.delete("size")
  if (params.get("page") === "1") params.delete("page")

  return params
}
