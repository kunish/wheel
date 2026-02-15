import type { StatsDaily, StatsHourly, StatsMetrics } from "@/lib/api-client"
import { useEffect, useState } from "react"
import { subscribe } from "@/hooks/use-stats-ws"

// ───────────── Types ─────────────

export interface DayData {
  dateStr: string
  displayDate: string
  isFuture: boolean
  daily: StatsDaily | null
}

export interface HeatmapTooltip {
  label: string
  metrics: StatsMetrics | null
}

export interface StreamingDelta {
  inputTokens: number
  outputTokens: number
  inputCost: number
  outputCost: number
}

export interface StreamIncrement {
  estimatedInputTokens: number
  outputTokens: number
  cost: number
  inputPrice: number
  outputPrice: number
}

export type HeatmapView = "day" | "week" | "month" | "year"

export type RankSortKey = "requests" | "tokens" | "cost" | "latency"

export type ChannelSortKey = "requests" | "cost" | "latency"

export interface HeroGearClockProps {
  dayHourlyMap: Map<number, StatsHourly>
  isToday: boolean
  nowHour: number
  now: Date
  selectedDayDateStr: string
  selectedDisplayDate: string
  gearAngle: number
  navigateToHour: (dateStr: string, hour: number) => void
  handleMouseEnter: (e: React.MouseEvent, tooltip: HeatmapTooltip) => void
  handleMouseLeave: () => void
  children?: React.ReactNode
}

// ───────────── Constants ─────────────

export const ACTIVITY_LEVELS = [
  { min: 100, level: 4 },
  { min: 30, level: 3 },
  { min: 10, level: 2 },
  { min: 1, level: 1 },
]

export const LEVEL_COLORS = [
  "var(--muted)",
  "color-mix(in srgb, var(--primary) 25%, transparent)",
  "color-mix(in srgb, var(--primary) 50%, transparent)",
  "color-mix(in srgb, var(--primary) 75%, transparent)",
  "var(--primary)",
]

export const HEATMAP_VIEWS: HeatmapView[] = ["day", "week", "month", "year"]
export const HEATMAP_VIEW_KEY = "activity-heatmap-view"

// Gear geometry constants
export const GEAR_CX = 200
export const GEAR_CY = 200
export const GEAR_GAP = 1.5
export const GEAR_ARC_SPAN = 30 - GEAR_GAP * 2
export const GEAR_BASE = -90
export const GEAR_PM_OUTER = 172
export const GEAR_PM_INNER = 134
export const GEAR_AM_OUTER = 128
export const GEAR_AM_INNER = 90
export const GEAR_HUB_R = 83
export const GEAR_RING_R = 176
export const GEAR_TOOTH_H = 10
export const GEAR_OUTER_R = GEAR_RING_R + GEAR_TOOTH_H

// ───────────── Utility functions ─────────────

export function getActivityLevel(value: number): number {
  if (value === 0) return 0
  return ACTIVITY_LEVELS.find((l) => value >= l.min)?.level ?? 1
}

/** Detect the locale's first day of week (0=Sun, 1=Mon, … 6=Sat).
 *  Uses Intl.Locale weekInfo when available, falls back to Sunday. */
export function getFirstDayOfWeek(lang: string): number {
  try {
    const locale = new Intl.Locale(lang) as Intl.Locale & {
      weekInfo?: { firstDay: number }
      getWeekInfo?: () => { firstDay: number }
    }
    // weekInfo is a getter property (Chrome 99+, Safari 17.4+)
    const info = locale.weekInfo ?? locale.getWeekInfo?.()
    if (info) {
      // Intl returns 1=Mon … 7=Sun; convert to JS getDay() convention (0=Sun … 6=Sat)
      return info.firstDay === 7 ? 0 : info.firstDay
    }
  } catch {}
  return 0 // fallback: Sunday
}

export function buildDayData(d: Date, map: Map<string, StatsDaily>, today: Date): DayData {
  const dateStr =
    d.getFullYear().toString() +
    (d.getMonth() + 1).toString().padStart(2, "0") +
    d.getDate().toString().padStart(2, "0")
  return {
    dateStr,
    displayDate: `${d.getFullYear()}-${(d.getMonth() + 1).toString().padStart(2, "0")}-${d.getDate().toString().padStart(2, "0")}`,
    isFuture: d > today,
    daily: map.get(dateStr) ?? null,
  }
}

/** Format a Date to YYYYMMDD string */
export function toDateStr(d: Date): string {
  return (
    d.getFullYear().toString() +
    (d.getMonth() + 1).toString().padStart(2, "0") +
    d.getDate().toString().padStart(2, "0")
  )
}

// ───────────── Power Pipeline + Reactor Grid ─────────────

export interface PowerPipelineProps {
  weekDays: DayData[]
  weekdayLabels: string[]
  todayStr: string
  gearAngle: number
  drillIntoDay: (dateStr: string) => void
  handleMouseEnter: (e: React.MouseEvent, tooltip: HeatmapTooltip) => void
  handleMouseLeave: () => void
}

export interface ReactorGridProps {
  monthDays: (DayData | null)[]
  weekdayLabels: string[]
  todayStr: string
  gearAngle: number
  drillIntoDay: (dateStr: string) => void
  handleMouseEnter: (e: React.MouseEvent, tooltip: HeatmapTooltip) => void
  handleMouseLeave: () => void
}

// Mini reactor geometry (viewBox 0 0 80 80)
export const MINI_REACTOR_CX = 40
export const MINI_REACTOR_CY = 40
export const MINI_REACTOR_OUTER_R = 36
export const MINI_REACTOR_ACTIVITY_R = 30
export const MINI_REACTOR_INNER_R = 22
export const MINI_REACTOR_CORE_R = 6

/** Map activity level (0-4) to an intensity fraction (0-1) */
export function levelToIntensity(level: number): number {
  return level / 4
}

// Pipeline geometry
export const PIPE_MIN_H = 20
export const PIPE_MAX_H = 120
export const PIPE_SVG_W = 700
export const PIPE_SVG_H = 200

export function gearArcPath(startDeg: number, spanDeg: number, rIn: number, rOut: number) {
  const toRad = (deg: number) => (deg * Math.PI) / 180
  const a1 = toRad(startDeg)
  const a2 = toRad(startDeg + spanDeg)
  const x1 = GEAR_CX + rOut * Math.cos(a1)
  const y1 = GEAR_CY + rOut * Math.sin(a1)
  const x2 = GEAR_CX + rOut * Math.cos(a2)
  const y2 = GEAR_CY + rOut * Math.sin(a2)
  const x3 = GEAR_CX + rIn * Math.cos(a2)
  const y3 = GEAR_CY + rIn * Math.sin(a2)
  const x4 = GEAR_CX + rIn * Math.cos(a1)
  const y4 = GEAR_CY + rIn * Math.sin(a1)
  return `M ${x1} ${y1} A ${rOut} ${rOut} 0 0 1 ${x2} ${y2} L ${x3} ${y3} A ${rIn} ${rIn} 0 0 0 ${x4} ${y4} Z`
}

export function getStoredView(): HeatmapView {
  try {
    const v = localStorage.getItem(HEATMAP_VIEW_KEY)
    if (v && HEATMAP_VIEWS.includes(v as HeatmapView)) return v as HeatmapView
  } catch {}
  return "day"
}

// ───────────── Period totals ─────────────

export interface PeriodTotals {
  req: number
  inTok: number
  outTok: number
  cost: number
}

/** Compute aggregate totals from an array of stats-like objects (StatsDaily, StatsHourly, or DayData.daily). */
export function computePeriodTotals(
  items: Array<
    | {
        request_success?: number
        request_failed?: number
        input_token?: number
        output_token?: number
        input_cost?: number
        output_cost?: number
      }
    | null
    | undefined
  >,
): PeriodTotals {
  let req = 0
  let inTok = 0
  let outTok = 0
  let cost = 0
  for (const s of items) {
    if (!s) continue
    req += (s.request_success ?? 0) + (s.request_failed ?? 0)
    inTok += s.input_token ?? 0
    outTok += s.output_token ?? 0
    cost += (s.input_cost ?? 0) + (s.output_cost ?? 0)
  }
  return { req, inTok, outTok, cost }
}

// ───────────── Hooks ─────────────

/** Returns a cumulative rotation angle (deg) that ticks forward on every WS data event. */
export function useGearRotation(): number {
  const [angle, setAngle] = useState(0)

  useEffect(() => {
    const unsub = subscribe(() => {
      setAngle((a) => a + 15)
    })
    return () => {
      unsub()
    }
  }, [])

  return angle
}

// ───────────── Shared components ─────────────

/** Render a formatted value with its unit (if any) */
export function Fmt({ fmt }: { fmt: { value: string; unit: string } }) {
  return (
    <span className="font-medium">
      {fmt.value}
      {fmt.unit && <span className="text-muted-foreground ml-0.5 text-[10px]">{fmt.unit}</span>}
    </span>
  )
}
