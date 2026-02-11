import type {
  ChannelStatsRow,
  ModelStatsItem,
  StatsDaily,
  StatsHourly,
  StatsMetrics,
} from "@/lib/api"
import { autoUpdate, flip, FloatingPortal, offset, shift, useFloating } from "@floating-ui/react"
import { useQuery } from "@tanstack/react-query"
import {
  Activity,
  AlertCircle,
  ArrowDownToLine,
  ArrowUpFromLine,
  Bot,
  ChartColumnBig,
  Clock,
  DollarSign,
  MessageSquare,
  RefreshCw,
} from "lucide-react"
import { motion } from "motion/react"
import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { AnimatedNumber } from "@/components/animated-number"
import { ModelBadge } from "@/components/model-badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { subscribe, useWsEvent } from "@/hooks/use-stats-ws"
import {
  getChannelStats,
  getDailyStats,
  getHourlyStats,
  getModelStats,
  getTotalStats,
} from "@/lib/api"
import { formatCount, formatMoney, formatTime } from "@/lib/format"

const LazyChartSection = lazy(() => import("@/components/chart-section"))

// ───────────── Gear rotation on data events ─────────────

/** Returns a cumulative rotation angle (deg) that ticks forward on every WS data event. */
function useGearRotation(): number {
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

// ───────────── Inline Error ─────────────

function InlineError({ message, onRetry }: { message: string; onRetry: () => void }) {
  const { t } = useTranslation("common")
  return (
    <Card className="flex flex-col items-center justify-center gap-3 py-10">
      <AlertCircle className="text-destructive h-8 w-8" />
      <p className="text-muted-foreground text-sm">{message}</p>
      <Button variant="outline" size="sm" className="gap-1.5" onClick={onRetry}>
        <RefreshCw className="h-3.5 w-3.5" />
        {t("actions.retry")}
      </Button>
    </Card>
  )
}

// ───────────── Total (4 stat cards) ─────────────

interface StreamingDelta {
  inputTokens: number
  outputTokens: number
  inputCost: number
  outputCost: number
}

function TotalSection({
  data,
  isLoading,
  streamingDelta,
}: {
  data?: StatsMetrics
  isLoading?: boolean
  streamingDelta?: StreamingDelta
}) {
  const { t } = useTranslation("dashboard")

  const d = streamingDelta ?? { inputTokens: 0, outputTokens: 0, inputCost: 0, outputCost: 0 }

  const cards = useMemo(
    () => [
      {
        title: t("stats.requestStats"),
        headerIcon: Activity,
        items: [
          {
            label: t("stats.requests"),
            raw: (data?.request_success ?? 0) + (data?.request_failed ?? 0),
            format: formatCount,
            icon: MessageSquare,
            bg: "bg-blue-500/10",
          },
          {
            label: t("stats.timeUsed"),
            raw: data?.wait_time ?? 0,
            format: formatTime,
            icon: Clock,
            bg: "bg-blue-500/10",
          },
        ],
      },
      {
        title: t("stats.overview"),
        headerIcon: ChartColumnBig,
        items: [
          {
            label: t("stats.totalTokens"),
            raw:
              (data?.input_token ?? 0) + (data?.output_token ?? 0) + d.inputTokens + d.outputTokens,
            format: formatCount,
            icon: Bot,
            bg: "bg-emerald-500/10",
          },
          {
            label: t("stats.totalCost"),
            raw: (data?.input_cost ?? 0) + (data?.output_cost ?? 0) + d.inputCost + d.outputCost,
            format: formatMoney,
            icon: DollarSign,
            bg: "bg-emerald-500/10",
          },
        ],
      },
      {
        title: t("stats.input"),
        headerIcon: ArrowDownToLine,
        items: [
          {
            label: t("stats.inputTokens"),
            raw: (data?.input_token ?? 0) + d.inputTokens,
            format: formatCount,
            icon: Bot,
            bg: "bg-orange-500/10",
          },
          {
            label: t("stats.inputCost"),
            raw: (data?.input_cost ?? 0) + d.inputCost,
            format: formatMoney,
            icon: DollarSign,
            bg: "bg-orange-500/10",
          },
        ],
      },
      {
        title: t("stats.output"),
        headerIcon: ArrowUpFromLine,
        items: [
          {
            label: t("stats.outputTokens"),
            raw: (data?.output_token ?? 0) + d.outputTokens,
            format: formatCount,
            icon: Bot,
            bg: "bg-violet-500/10",
          },
          {
            label: t("stats.outputCost"),
            raw: (data?.output_cost ?? 0) + d.outputCost,
            format: formatMoney,
            icon: DollarSign,
            bg: "bg-violet-500/10",
          },
        ],
      },
    ],
    [t, data, d.inputTokens, d.outputTokens, d.inputCost, d.outputCost],
  )

  return (
    <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
      {cards.map((card, index) => (
        <motion.div
          key={card.title}
          initial={{ opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3, delay: index * 0.08, ease: [0.33, 1, 0.68, 1] }}
        >
          <Card className="gap-3 p-4">
            <div className="flex items-center gap-2 px-0">
              <card.headerIcon className="text-muted-foreground h-4 w-4 shrink-0" />
              <span className="text-sm font-bold">{card.title}</span>
            </div>
            <div className="flex flex-col gap-2 px-0">
              {card.items.map((item) => {
                const formatted = item.format(item.raw)
                return (
                  <div key={item.label} className="flex items-center gap-2">
                    <div
                      className={`flex h-7 w-7 shrink-0 items-center justify-center rounded-md ${item.bg}`}
                    >
                      <item.icon className="text-primary h-3.5 w-3.5" />
                    </div>
                    <div className="flex flex-col">
                      <span className="text-muted-foreground text-xs leading-tight">
                        {item.label}
                      </span>
                      {isLoading ? (
                        <Skeleton className="mt-1 h-5 w-16" />
                      ) : (
                        <span className="text-lg leading-tight font-bold tabular-nums">
                          <AnimatedNumber
                            value={item.raw}
                            formatter={(n) => item.format(n).value}
                          />
                          {formatted.unit && (
                            <span className="text-muted-foreground ml-0.5 text-xs font-medium">
                              {formatted.unit}
                            </span>
                          )}
                        </span>
                      )}
                    </div>
                  </div>
                )
              })}
            </div>
          </Card>
        </motion.div>
      ))}
    </div>
  )
}

// ───────────── Activity Heatmap ─────────────

interface DayData {
  dateStr: string
  displayDate: string
  isFuture: boolean
  daily: StatsDaily | null
}

interface HeatmapTooltip {
  label: string
  metrics: StatsMetrics | null
}

const ACTIVITY_LEVELS = [
  { min: 100, level: 4 },
  { min: 30, level: 3 },
  { min: 10, level: 2 },
  { min: 1, level: 1 },
]

function getActivityLevel(value: number): number {
  if (value === 0) return 0
  return ACTIVITY_LEVELS.find((l) => value >= l.min)?.level ?? 1
}

/** Detect the locale's first day of week (0=Sun, 1=Mon, … 6=Sat).
 *  Uses Intl.Locale weekInfo when available, falls back to Sunday. */
function getFirstDayOfWeek(lang: string): number {
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

type HeatmapView = "day" | "week" | "month" | "year"

const HEATMAP_VIEWS: HeatmapView[] = ["day", "week", "month", "year"]
const HEATMAP_VIEW_KEY = "activity-heatmap-view"

function getStoredView(): HeatmapView {
  try {
    const v = localStorage.getItem(HEATMAP_VIEW_KEY)
    if (v && HEATMAP_VIEWS.includes(v as HeatmapView)) return v as HeatmapView
  } catch {}
  return "day"
}

const LEVEL_COLORS = [
  "var(--muted)",
  "color-mix(in srgb, var(--primary) 25%, transparent)",
  "color-mix(in srgb, var(--primary) 50%, transparent)",
  "color-mix(in srgb, var(--primary) 75%, transparent)",
  "var(--primary)",
]

function buildDayData(d: Date, map: Map<string, StatsDaily>, today: Date): DayData {
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
function toDateStr(d: Date): string {
  return (
    d.getFullYear().toString() +
    (d.getMonth() + 1).toString().padStart(2, "0") +
    d.getDate().toString().padStart(2, "0")
  )
}

// ───────────── Hero Gear Clock ─────────────

const GEAR_CX = 200
const GEAR_CY = 200
const GEAR_GAP = 1.5
const GEAR_ARC_SPAN = 30 - GEAR_GAP * 2
const GEAR_BASE = -90 - 15
const GEAR_PM_OUTER = 172
const GEAR_PM_INNER = 134
const GEAR_AM_OUTER = 128
const GEAR_AM_INNER = 90
const GEAR_HUB_R = 83
const GEAR_RING_R = 176
const GEAR_TOOTH_H = 10
const GEAR_OUTER_R = GEAR_RING_R + GEAR_TOOTH_H

function gearArcPath(startDeg: number, spanDeg: number, rIn: number, rOut: number) {
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

interface HeroGearClockProps {
  dayHourlyMap: Map<number, StatsHourly>
  isToday: boolean
  nowHour: number
  now: Date
  reqCount: number
  inTokens: number
  outTokens: number
  totalCost: number
  selectedDayDateStr: string
  selectedDisplayDate: string
  gearAngle: number
  t: (key: string) => string
  navigateToHour: (dateStr: string, hour: number) => void
  handleMouseEnter: (e: React.MouseEvent, tooltip: HeatmapTooltip) => void
  handleMouseLeave: () => void
}

function HeroGearClock({
  dayHourlyMap,
  isToday,
  nowHour,
  now,
  reqCount,
  inTokens,
  outTokens,
  totalCost,
  selectedDayDateStr,
  selectedDisplayDate,
  gearAngle,
  t,
  navigateToHour,
  handleMouseEnter,
  handleMouseLeave,
}: HeroGearClockProps) {
  const toRad = (deg: number) => (deg * Math.PI) / 180

  // Count active hours for reactor energy intensity
  const activeHours = Array.from({ length: 24 }, (_, h) => {
    const hourly = dayHourlyMap.get(h)
    return hourly ? (hourly.request_success ?? 0) + (hourly.request_failed ?? 0) : 0
  }).filter((c) => c > 0).length

  const energyIntensity = Math.min(1, activeHours / 12)

  return (
    <motion.div
      className="relative flex items-center justify-center py-4"
      initial={{ opacity: 0, scale: 0.8 }}
      animate={{ opacity: 1, scale: 1 }}
      transition={{ duration: 0.6, ease: [0.16, 1, 0.3, 1] }}
    >
      {/* ── Arc Reactor background glow ── */}
      <div
        className="pointer-events-none absolute rounded-full"
        style={{
          width: "420px",
          height: "420px",
          background: `radial-gradient(circle, color-mix(in srgb, var(--nb-lime) ${Math.round(8 + energyIntensity * 10)}%, transparent) 0%, transparent 70%)`,
          animation: "reactor-core-pulse 3s ease-in-out infinite",
        }}
      />

      {/* ── Decorative outer reactor rings — slow rotating, breathing ── */}
      <svg
        viewBox="0 0 400 400"
        className="pointer-events-none absolute w-full max-w-[520px]"
        style={{ animation: "reactor-ring-breathe 4s ease-in-out infinite" }}
      >
        {/* Outermost energy ring */}
        <circle
          cx="200"
          cy="200"
          r="198"
          fill="none"
          stroke="var(--nb-lime)"
          strokeWidth="0.5"
          opacity={0.3}
        />
        {/* Outer dashed containment ring */}
        <circle
          cx="200"
          cy="200"
          r="194"
          fill="none"
          stroke="var(--nb-lime)"
          strokeWidth="0.3"
          strokeDasharray="2 6"
          opacity={0.2}
          style={{ animation: "reactor-spin-slow 60s linear infinite" }}
        />
      </svg>

      {/* ── Counter-rotating decorative gear (behind main) ── */}
      <svg
        viewBox="0 0 400 400"
        className="pointer-events-none absolute w-full max-w-[520px] opacity-[0.04]"
        style={{
          transform: `rotate(${-gearAngle * 0.5}deg)`,
          transition: "transform 0.6s cubic-bezier(0.34, 1.56, 0.64, 1)",
        }}
      >
        {Array.from({ length: 32 }, (_, i) => {
          const angle = i * (360 / 32)
          const rad = toRad(angle)
          const perp = rad + Math.PI / 2
          const hw = 5
          const iR = 185
          const oR = 198
          return (
            <path
              key={i}
              d={`M ${200 + iR * Math.cos(rad) + hw * Math.cos(perp)} ${200 + iR * Math.sin(rad) + hw * Math.sin(perp)} L ${200 + oR * Math.cos(rad) + (hw - 1) * Math.cos(perp)} ${200 + oR * Math.sin(rad) + (hw - 1) * Math.sin(perp)} L ${200 + oR * Math.cos(rad) - (hw - 1) * Math.cos(perp)} ${200 + oR * Math.sin(rad) - (hw - 1) * Math.sin(perp)} L ${200 + iR * Math.cos(rad) - hw * Math.cos(perp)} ${200 + iR * Math.sin(rad) - hw * Math.sin(perp)} Z`}
              fill="var(--foreground)"
            />
          )
        })}
        <circle cx="200" cy="200" r="185" fill="none" stroke="var(--foreground)" strokeWidth="3" />
      </svg>

      {/* ── Main gear SVG ── */}
      <svg viewBox="-15 -15 430 430" className="relative w-full max-w-[520px]">
        <defs>
          {/* Reactor core gradient for hub */}
          <radialGradient id="reactor-core-grad" cx="50%" cy="50%" r="50%">
            <stop
              offset="0%"
              stopColor="var(--nb-lime)"
              stopOpacity={0.12 + energyIntensity * 0.08}
            />
            <stop offset="60%" stopColor="var(--nb-lime)" stopOpacity={0.04} />
            <stop offset="100%" stopColor="var(--nb-lime)" stopOpacity="0" />
          </radialGradient>
          {/* Energy channel gradient for spokes */}
          <linearGradient id="spoke-energy" x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" stopColor="var(--nb-lime)" stopOpacity="0.3" />
            <stop offset="50%" stopColor="var(--nb-lime)" stopOpacity="0.08" />
            <stop offset="100%" stopColor="var(--nb-lime)" stopOpacity="0.3" />
          </linearGradient>
        </defs>

        {/* ── Rotating gear teeth ring ── */}
        <g
          style={{
            transformOrigin: `${GEAR_CX}px ${GEAR_CY}px`,
            filter: "drop-shadow(2px 2px 0 color-mix(in srgb, var(--nb-shadow) 25%, transparent))",
            transform: `rotate(${gearAngle}deg)`,
            transition: "transform 0.6s cubic-bezier(0.34, 1.56, 0.64, 1)",
          }}
        >
          {Array.from({ length: 24 }, (_, i) => {
            const seg = Math.floor(i / 2)
            const sub = i % 2
            const segCenter = seg * 30 - 90
            const toothAngle = segCenter + (sub === 0 ? -7 : 7)
            const hw = 4.5
            const mid = toRad(toothAngle)
            const perp = mid + Math.PI / 2
            const ix1 = GEAR_CX + GEAR_RING_R * Math.cos(mid) + hw * Math.cos(perp)
            const iy1 = GEAR_CY + GEAR_RING_R * Math.sin(mid) + hw * Math.sin(perp)
            const ix2 = GEAR_CX + GEAR_RING_R * Math.cos(mid) - hw * Math.cos(perp)
            const iy2 = GEAR_CY + GEAR_RING_R * Math.sin(mid) - hw * Math.sin(perp)
            const ox1 = GEAR_CX + GEAR_OUTER_R * Math.cos(mid) + (hw - 1) * Math.cos(perp)
            const oy1 = GEAR_CY + GEAR_OUTER_R * Math.sin(mid) + (hw - 1) * Math.sin(perp)
            const ox2 = GEAR_CX + GEAR_OUTER_R * Math.cos(mid) - (hw - 1) * Math.cos(perp)
            const oy2 = GEAR_CY + GEAR_OUTER_R * Math.sin(mid) - (hw - 1) * Math.sin(perp)
            return (
              <path
                key={`tooth-${i}`}
                d={`M ${ix1} ${iy1} L ${ox1} ${oy1} L ${ox2} ${oy2} L ${ix2} ${iy2} Z`}
                fill="var(--primary)"
                opacity={0.25}
              />
            )
          })}
          {/* Outer gear ring */}
          <circle
            cx={GEAR_CX}
            cy={GEAR_CY}
            r={GEAR_RING_R}
            fill="none"
            stroke="var(--border)"
            strokeWidth="2.5"
          />
        </g>

        {/* ── PM ring (outer): hours 12-23 ── */}
        {Array.from({ length: 12 }, (_, i) => {
          const h = i + 12
          const isFutureHour = isToday && h > nowHour
          const hourly = dayHourlyMap.get(h)
          const hCount = isFutureHour
            ? 0
            : (hourly?.request_success ?? 0) + (hourly?.request_failed ?? 0)
          const hLevel = isFutureHour ? -1 : getActivityLevel(hCount)
          const startDeg = i * 30 + GEAR_BASE + GEAR_GAP
          const isCurrentHour = isToday && h === nowHour

          return (
            <path
              key={`pm-${h}`}
              d={gearArcPath(startDeg, GEAR_ARC_SPAN, GEAR_PM_INNER, GEAR_PM_OUTER)}
              fill={hLevel === -1 ? "none" : LEVEL_COLORS[hLevel]}
              stroke={isCurrentHour ? "var(--nb-lime)" : hLevel === -1 ? "var(--border)" : "none"}
              strokeWidth={isCurrentHour ? "2" : hLevel === -1 ? "0.5" : "0"}
              strokeDasharray={hLevel === -1 && !isCurrentHour ? "3 2" : "none"}
              opacity={hLevel === -1 ? 0.3 : 1}
              className="cursor-pointer transition-all hover:opacity-80"
              style={isCurrentHour ? { filter: "drop-shadow(0 0 6px var(--nb-lime))" } : undefined}
              onClick={() => navigateToHour(selectedDayDateStr, h)}
              onMouseEnter={(e) =>
                handleMouseEnter(e, {
                  label: `${selectedDisplayDate} ${h.toString().padStart(2, "0")}:00`,
                  metrics: hourly ?? null,
                })
              }
              onMouseLeave={handleMouseLeave}
            />
          )
        })}

        {/* ── Divider ring between AM / PM — reactor containment ring ── */}
        <circle
          cx={GEAR_CX}
          cy={GEAR_CY}
          r={131}
          fill="none"
          stroke="var(--border)"
          strokeWidth="1.5"
        />
        {/* Energy trace on divider */}
        <circle
          cx={GEAR_CX}
          cy={GEAR_CY}
          r={131}
          fill="none"
          stroke="var(--nb-lime)"
          strokeWidth="0.5"
          opacity={0.15}
          strokeDasharray="8 20"
          style={{
            transformOrigin: `${GEAR_CX}px ${GEAR_CY}px`,
            animation: "reactor-spin-slow 30s linear infinite",
          }}
        />

        {/* ── AM ring (inner): hours 0-11 ── */}
        {Array.from({ length: 12 }, (_, i) => {
          const h = i
          const isFutureHour = isToday && h > nowHour
          const hourly = dayHourlyMap.get(h)
          const hCount = isFutureHour
            ? 0
            : (hourly?.request_success ?? 0) + (hourly?.request_failed ?? 0)
          const hLevel = isFutureHour ? -1 : getActivityLevel(hCount)
          const startDeg = i * 30 + GEAR_BASE + GEAR_GAP
          const isCurrentHour = isToday && h === nowHour

          return (
            <path
              key={`am-${h}`}
              d={gearArcPath(startDeg, GEAR_ARC_SPAN, GEAR_AM_INNER, GEAR_AM_OUTER)}
              fill={hLevel === -1 ? "none" : LEVEL_COLORS[hLevel]}
              stroke={isCurrentHour ? "var(--nb-lime)" : hLevel === -1 ? "var(--border)" : "none"}
              strokeWidth={isCurrentHour ? "2" : hLevel === -1 ? "0.5" : "0"}
              strokeDasharray={hLevel === -1 && !isCurrentHour ? "3 2" : "none"}
              opacity={hLevel === -1 ? 0.3 : 1}
              className="cursor-pointer transition-all hover:opacity-80"
              style={isCurrentHour ? { filter: "drop-shadow(0 0 6px var(--nb-lime))" } : undefined}
              onClick={() => navigateToHour(selectedDayDateStr, h)}
              onMouseEnter={(e) =>
                handleMouseEnter(e, {
                  label: `${selectedDisplayDate} ${h.toString().padStart(2, "0")}:00`,
                  metrics: hourly ?? null,
                })
              }
              onMouseLeave={handleMouseLeave}
            />
          )
        })}

        {/* ── Reactor core hub ── */}
        {/* Outer glow ring around hub */}
        <circle
          cx={GEAR_CX}
          cy={GEAR_CY}
          r={GEAR_HUB_R + 2}
          fill="none"
          stroke="var(--nb-lime)"
          strokeWidth="0.8"
          opacity={0.2}
          style={{ animation: "reactor-core-pulse 3s ease-in-out infinite" }}
        />
        {/* Hub fill */}
        <circle
          cx={GEAR_CX}
          cy={GEAR_CY}
          r={GEAR_HUB_R}
          fill="var(--card)"
          stroke="var(--border)"
          strokeWidth="2.5"
        />
        {/* Reactor energy fill inside hub */}
        <circle
          cx={GEAR_CX}
          cy={GEAR_CY}
          r={GEAR_HUB_R - 1}
          fill="url(#reactor-core-grad)"
          style={{ animation: "reactor-core-pulse 3s ease-in-out infinite" }}
        />

        {/* ── Energy channel spokes ── */}
        {Array.from({ length: 12 }, (_, i) => {
          const angle = toRad(i * 30 + GEAR_BASE + 15)
          const x1 = GEAR_CX + GEAR_HUB_R * Math.cos(angle)
          const y1 = GEAR_CY + GEAR_HUB_R * Math.sin(angle)
          const x2 = GEAR_CX + GEAR_RING_R * Math.cos(angle)
          const y2 = GEAR_CY + GEAR_RING_R * Math.sin(angle)
          return (
            <g key={`spoke-${i}`}>
              {/* Energy glow line */}
              <line
                x1={x1}
                y1={y1}
                x2={x2}
                y2={y2}
                stroke="var(--nb-lime)"
                strokeWidth="2.5"
                opacity={0.06}
                strokeLinecap="round"
              />
              {/* Structural spoke */}
              <line
                x1={x1}
                y1={y1}
                x2={x2}
                y2={y2}
                stroke="var(--border)"
                strokeWidth="1.2"
                opacity={0.1}
              />
            </g>
          )
        })}

        {/* ── Inner reactor rings (decorative concentric circles) ── */}
        <circle
          cx={GEAR_CX}
          cy={GEAR_CY}
          r={GEAR_HUB_R - 6}
          fill="none"
          stroke="var(--nb-lime)"
          strokeWidth="0.4"
          opacity={0.12}
        />
        <circle
          cx={GEAR_CX}
          cy={GEAR_CY}
          r="12"
          fill="none"
          stroke="var(--nb-lime)"
          strokeWidth="0.6"
          opacity={0.2}
          style={{ animation: "reactor-core-pulse 2.5s ease-in-out infinite" }}
        />
        <circle cx={GEAR_CX} cy={GEAR_CY} r="8" fill="var(--border)" opacity={0.1} />
        {/* Core dot — energy center */}
        <circle
          cx={GEAR_CX}
          cy={GEAR_CY}
          r="3"
          fill="var(--nb-lime)"
          opacity={0.3}
          style={{ animation: "reactor-core-pulse 2s ease-in-out infinite" }}
        />

        {/* ── Clock hour labels ── */}
        {Array.from({ length: 12 }, (_, i) => {
          const displayHour = i === 0 ? 12 : i
          const midAngle = i * 30 - 90
          const labelR = GEAR_RING_R + GEAR_TOOTH_H / 2 + 12
          const x = GEAR_CX + labelR * Math.cos(toRad(midAngle))
          const y = GEAR_CY + labelR * Math.sin(toRad(midAngle))
          const isCardinal = i % 3 === 0
          return (
            <text
              key={`label-${i}`}
              x={x}
              y={y}
              textAnchor="middle"
              dominantBaseline="central"
              fill="var(--foreground)"
              fontSize={isCardinal ? "13" : "9"}
              fontWeight={isCardinal ? "900" : "700"}
              fontFamily="inherit"
              opacity={isCardinal ? 1 : 0.5}
            >
              {displayHour}
            </text>
          )
        })}

        {/* ── AM / PM labels ── */}
        <text
          x={GEAR_CX}
          y={GEAR_CY - (GEAR_PM_INNER + GEAR_PM_OUTER) / 2}
          textAnchor="middle"
          dominantBaseline="central"
          fill="var(--muted-foreground)"
          fontSize="7"
          fontWeight="800"
          fontFamily="inherit"
          letterSpacing="2"
          opacity={0.45}
        >
          PM
        </text>
        <text
          x={GEAR_CX}
          y={GEAR_CY - (GEAR_AM_INNER + GEAR_AM_OUTER) / 2}
          textAnchor="middle"
          dominantBaseline="central"
          fill="var(--muted-foreground)"
          fontSize="7"
          fontWeight="800"
          fontFamily="inherit"
          letterSpacing="2"
          opacity={0.45}
        >
          AM
        </text>

        {/* ── Current hour hand (today only) ── */}
        {isToday &&
          (() => {
            const minuteFraction = now.getMinutes() / 60
            const clockPos = (nowHour % 12) + minuteFraction
            const handAngle = clockPos * 30 - 90
            const isPM = nowHour >= 12
            const handLen = isPM ? GEAR_PM_OUTER : GEAR_AM_OUTER
            const hx = GEAR_CX + handLen * Math.cos(toRad(handAngle))
            const hy = GEAR_CY + handLen * Math.sin(toRad(handAngle))
            return (
              <>
                {/* Hand glow */}
                <line
                  x1={GEAR_CX}
                  y1={GEAR_CY}
                  x2={hx}
                  y2={hy}
                  stroke="var(--nb-lime)"
                  strokeWidth="6"
                  strokeLinecap="round"
                  opacity={0.15}
                />
                {/* Hand line */}
                <line
                  x1={GEAR_CX}
                  y1={GEAR_CY}
                  x2={hx}
                  y2={hy}
                  stroke="var(--destructive)"
                  strokeWidth="3"
                  strokeLinecap="round"
                />
                <circle cx={GEAR_CX} cy={GEAR_CY} r="5" fill="var(--destructive)" />
                <circle cx={hx} cy={hy} r="3.5" fill="var(--destructive)" />
              </>
            )
          })()}

        {/* ── Center stats — big bold request count ── */}
        <text
          x={GEAR_CX}
          y={GEAR_CY - 22}
          textAnchor="middle"
          fill="var(--foreground)"
          fontSize="28"
          fontWeight="900"
          fontFamily="inherit"
        >
          {formatCount(reqCount).value}
        </text>
        <text
          x={GEAR_CX}
          y={GEAR_CY - 6}
          textAnchor="middle"
          fill="var(--muted-foreground)"
          fontSize="8"
          fontWeight="700"
          fontFamily="inherit"
          letterSpacing="1.5"
        >
          {t("stats.requests").toUpperCase()}
        </text>

        {/* ── Bottom stats row inside hub ── */}
        {[
          { label: "IN", value: formatCount(inTokens) },
          { label: "OUT", value: formatCount(outTokens) },
          { label: "$", value: formatMoney(totalCost) },
        ].map((s, si) => {
          const xPos = GEAR_CX - 36 + si * 36
          return (
            <g key={s.label}>
              <text
                x={xPos}
                y={GEAR_CY + 14}
                textAnchor="middle"
                fill="var(--muted-foreground)"
                fontSize="6"
                fontWeight="700"
                fontFamily="inherit"
                letterSpacing="0.5"
              >
                {s.label}
              </text>
              <text
                x={xPos}
                y={GEAR_CY + 25}
                textAnchor="middle"
                fill="var(--foreground)"
                fontSize="10"
                fontWeight="900"
                fontFamily="inherit"
              >
                {s.value.value}
                {s.value.unit && s.value.unit}
              </text>
            </g>
          )
        })}
      </svg>
    </motion.div>
  )
}

function ActivitySection({ data }: { data?: StatsDaily[] }) {
  const { t, i18n } = useTranslation("dashboard")
  const { t: tc } = useTranslation("common")
  const gearAngle = useGearRotation()
  const firstDay = useMemo(() => getFirstDayOfWeek(i18n.language), [i18n.language])
  const [view, setViewRaw] = useState<HeatmapView>(getStoredView)
  const setView = useCallback((v: HeatmapView) => {
    setViewRaw(v)
    try {
      localStorage.setItem(HEATMAP_VIEW_KEY, v)
    } catch {}
  }, [])
  const [activeTooltip, setActiveTooltip] = useState<HeatmapTooltip | null>(null)
  const [selectedDateStr, setSelectedDateStr] = useState<string | null>(null)
  const [weekOffset, setWeekOffset] = useState(0)
  const [monthOffset, setMonthOffset] = useState(0)
  const [yearOffset, setYearOffset] = useState(0)
  const navigate = useNavigate()

  const [today] = useState(() => new Date())

  const { refs, floatingStyles } = useFloating({
    placement: "top",
    open: !!activeTooltip,
    middleware: [offset(8), flip(), shift({ padding: 8 })],
    whileElementsMounted: autoUpdate,
  })

  const dataMap = useMemo(() => new Map((data ?? []).map((d) => [d.date, d])), [data])

  const weekdayLabelsRaw = useMemo(
    () => [
      tc("weekdays.sun"),
      tc("weekdays.mon"),
      tc("weekdays.tue"),
      tc("weekdays.wed"),
      tc("weekdays.thu"),
      tc("weekdays.fri"),
      tc("weekdays.sat"),
    ],
    [tc],
  )

  // Reorder weekday labels so the locale's first day comes first
  const weekdayLabels = useMemo(
    () => [...weekdayLabelsRaw.slice(firstDay), ...weekdayLabelsRaw.slice(0, firstDay)],
    [weekdayLabelsRaw, firstDay],
  )

  const weekdaysFull = useMemo(
    () => [
      t("weekdaysFull.sunday"),
      t("weekdaysFull.monday"),
      t("weekdaysFull.tuesday"),
      t("weekdaysFull.wednesday"),
      t("weekdaysFull.thursday"),
      t("weekdaysFull.friday"),
      t("weekdaysFull.saturday"),
    ],
    [t],
  )

  const viewLabels = useMemo(
    () =>
      ({
        day: t("activity.day"),
        week: t("activity.week"),
        month: t("activity.month"),
        year: t("activity.year"),
      }) as Record<HeatmapView, string>,
    [t],
  )

  // ── Year view: 53 weeks × 7 days, column-first grid ──
  const yearAnchor = useMemo(() => {
    const d = new Date(today.getFullYear() + yearOffset, 11, 31)
    // If offset is 0, use today as the anchor so future days are dashed
    if (yearOffset === 0) return today
    return d
  }, [today, yearOffset])

  const yearLabel = useMemo(() => {
    return `${today.getFullYear() + yearOffset}`
  }, [today, yearOffset])

  const yearDays = useMemo(() => {
    const anchor = yearAnchor
    const anchorDay = (anchor.getDay() - firstDay + 7) % 7
    const start = new Date(anchor)
    start.setDate(start.getDate() - anchorDay - 52 * 7)
    const result: DayData[] = []
    for (let i = 0; i < 53 * 7; i++) {
      const d = new Date(start)
      d.setDate(d.getDate() + i)
      result.push(buildDayData(d, dataMap, today))
    }
    return result
  }, [dataMap, today, yearAnchor, firstDay])

  // ── Month view: offset-based month ──
  const monthAnchor = useMemo(() => {
    return new Date(today.getFullYear(), today.getMonth() + monthOffset, 1)
  }, [today, monthOffset])

  const monthLabel = useMemo(() => {
    return `${monthAnchor.getFullYear()}-${(monthAnchor.getMonth() + 1).toString().padStart(2, "0")}`
  }, [monthAnchor])

  const monthDays = useMemo(() => {
    const year = monthAnchor.getFullYear()
    const month = monthAnchor.getMonth()
    const monthFirstDate = new Date(year, month, 1)
    const lastDay = new Date(year, month + 1, 0)
    const startPad = (monthFirstDate.getDay() - firstDay + 7) % 7
    const result: (DayData | null)[] = []
    for (let i = 0; i < startPad; i++) result.push(null)
    for (let d = 1; d <= lastDay.getDate(); d++) {
      const date = new Date(year, month, d)
      result.push(buildDayData(date, dataMap, today))
    }
    return result
  }, [dataMap, today, monthAnchor, firstDay])

  // ── Week view: offset-based week, starting from locale first day ──
  const weekStart = useMemo(() => {
    const todayDay = today.getDay()
    const diff = (todayDay - firstDay + 7) % 7
    const start = new Date(today)
    start.setDate(start.getDate() - diff + weekOffset * 7)
    return start
  }, [today, weekOffset, firstDay])

  const weekLabel = useMemo(() => {
    const end = new Date(weekStart)
    end.setDate(end.getDate() + 6)
    const fmt = (d: Date) =>
      `${d.getFullYear()}-${(d.getMonth() + 1).toString().padStart(2, "0")}-${d.getDate().toString().padStart(2, "0")}`
    return `${fmt(weekStart)} ~ ${fmt(end)}`
  }, [weekStart])

  const weekDays = useMemo(() => {
    const result: DayData[] = []
    for (let i = 0; i < 7; i++) {
      const d = new Date(weekStart)
      d.setDate(d.getDate() + i)
      result.push(buildDayData(d, dataMap, today))
    }
    return result
  }, [dataMap, today, weekStart])

  // ── Day view: hourly data for the selected date ──
  const selectedDayDateStr = selectedDateStr ?? toDateStr(today)

  const { data: dayHourlyData } = useQuery({
    queryKey: ["stats", "hourly", selectedDayDateStr, selectedDayDateStr],
    queryFn: () => getHourlyStats(selectedDayDateStr, selectedDayDateStr),
    enabled: view === "day",
  })

  const dayHourlyMap = useMemo(() => {
    const raw = dayHourlyData?.data
    if (!raw) return new Map<number, StatsHourly>()
    const map = new Map<number, StatsHourly>()
    for (const s of raw) {
      if (s.date === selectedDayDateStr) {
        map.set(s.hour, s)
      }
    }
    return map
  }, [dayHourlyData, selectedDayDateStr])

  const selectedDayData = useMemo(() => {
    return dataMap.get(selectedDayDateStr) ?? null
  }, [dataMap, selectedDayDateStr])

  const selectedDisplayDate = useMemo(() => {
    const ds = selectedDayDateStr
    return `${ds.slice(0, 4)}-${ds.slice(4, 6)}-${ds.slice(6, 8)}`
  }, [selectedDayDateStr])

  const selectedDayWeekday = useMemo(() => {
    const ds = selectedDayDateStr
    const d = new Date(
      Number.parseInt(ds.slice(0, 4)),
      Number.parseInt(ds.slice(4, 6)) - 1,
      Number.parseInt(ds.slice(6, 8)),
    )
    return weekdaysFull[d.getDay()]
  }, [selectedDayDateStr, weekdaysFull])

  const handleMouseEnter = useCallback(
    (e: React.MouseEvent, tooltip: HeatmapTooltip) => {
      refs.setReference(e.currentTarget)
      setActiveTooltip(tooltip)
    },
    [refs],
  )

  const handleMouseLeave = useCallback(() => {
    setActiveTooltip(null)
  }, [])

  /** Switch to Day view for a specific date */
  const drillIntoDay = useCallback((dateStr: string) => {
    setSelectedDateStr(dateStr)
    setView("day")
  }, [])

  /** Navigate to logs filtered by a day's time range */
  const navigateToDay = useCallback(
    (dateStr: string) => {
      const y = Number.parseInt(dateStr.slice(0, 4))
      const m = Number.parseInt(dateStr.slice(4, 6)) - 1
      const d = Number.parseInt(dateStr.slice(6, 8))
      const from = Math.floor(new Date(y, m, d).getTime() / 1000)
      const to = from + 86400 - 1
      navigate(`/logs?from=${from}&to=${to}`)
    },
    [navigate],
  )

  /** Navigate to logs filtered by an hour's time range */
  const navigateToHour = useCallback(
    (dateStr: string, hour: number) => {
      const y = Number.parseInt(dateStr.slice(0, 4))
      const m = Number.parseInt(dateStr.slice(4, 6)) - 1
      const d = Number.parseInt(dateStr.slice(6, 8))
      const from = Math.floor(new Date(y, m, d, hour).getTime() / 1000)
      const to = from + 3600 - 1
      navigate(`/logs?from=${from}&to=${to}`)
    },
    [navigate],
  )

  /** Navigate to logs filtered by the current week's time range */
  const navigateToWeek = useCallback(() => {
    const from = Math.floor(weekStart.getTime() / 1000)
    const end = new Date(weekStart)
    end.setDate(end.getDate() + 7)
    const to = Math.floor(end.getTime() / 1000) - 1
    navigate(`/logs?from=${from}&to=${to}`)
  }, [weekStart, navigate])

  /** Navigate to logs filtered by the current month's time range */
  const navigateToMonth = useCallback(() => {
    const from = Math.floor(monthAnchor.getTime() / 1000)
    const end = new Date(monthAnchor.getFullYear(), monthAnchor.getMonth() + 1, 1)
    const to = Math.floor(end.getTime() / 1000) - 1
    navigate(`/logs?from=${from}&to=${to}`)
  }, [monthAnchor, navigate])

  /** Navigate to logs filtered by the current year's time range */
  const navigateToYear = useCallback(() => {
    const y = today.getFullYear() + yearOffset
    const from = Math.floor(new Date(y, 0, 1).getTime() / 1000)
    const to = Math.floor(new Date(y + 1, 0, 1).getTime() / 1000) - 1
    navigate(`/logs?from=${from}&to=${to}`)
  }, [today, yearOffset, navigate])

  /** Navigate to the previous or next day in Day view */
  const shiftDay = useCallback(
    (delta: -1 | 1) => {
      const ds = selectedDayDateStr
      const d = new Date(
        Number.parseInt(ds.slice(0, 4)),
        Number.parseInt(ds.slice(4, 6)) - 1,
        Number.parseInt(ds.slice(6, 8)),
      )
      d.setDate(d.getDate() + delta)
      if (d > today) return
      setSelectedDateStr(toDateStr(d))
    },
    [selectedDayDateStr, today],
  )

  function renderCell(day: DayData | null, key: string) {
    if (!day) return <div key={key} />
    if (day.isFuture)
      return (
        <div key={key} className="border-border/30 aspect-square rounded-sm border border-dashed" />
      )
    const count = (day.daily?.request_success ?? 0) + (day.daily?.request_failed ?? 0)
    const level = getActivityLevel(count)
    return (
      <div
        key={key}
        className="aspect-square cursor-pointer rounded-sm transition-transform hover:scale-125"
        style={{ backgroundColor: LEVEL_COLORS[level] }}
        onClick={() => drillIntoDay(day.dateStr)}
        onMouseEnter={(e) => handleMouseEnter(e, { label: day.displayDate, metrics: day.daily })}
        onMouseLeave={handleMouseLeave}
      />
    )
  }

  return (
    <Card className="gap-0">
      <div className="flex items-center justify-between px-4 pt-4 pb-2">
        <span className="text-sm font-bold">{t("activity.title")}</span>
        <div className="flex gap-1">
          {(["day", "week", "month", "year"] as const).map((v) => (
            <button
              key={v}
              onClick={() => setView(v)}
              className={`rounded-md border-2 px-2.5 py-1 text-xs font-bold transition-all ${
                view === v
                  ? "border-border bg-primary text-primary-foreground shadow-[2px_2px_0_var(--nb-shadow)]"
                  : "text-muted-foreground hover:text-foreground border-transparent"
              }`}
            >
              {viewLabels[v]}
            </button>
          ))}
        </div>
      </div>

      <div className="px-4 pb-2">
        {/* ── Day view: single date hourly breakdown with stats ── */}
        {view === "day" &&
          (() => {
            const now = new Date()
            const nowStr = toDateStr(now)
            const nowHour = now.getHours()
            const isToday = selectedDayDateStr === nowStr
            const isFutureDate = (() => {
              const d = new Date(
                Number.parseInt(selectedDayDateStr.slice(0, 4)),
                Number.parseInt(selectedDayDateStr.slice(4, 6)) - 1,
                Number.parseInt(selectedDayDateStr.slice(6, 8)),
              )
              return d > today
            })()

            // Aggregate stats for this day
            const dayMetrics = selectedDayData
            const reqCount = dayMetrics
              ? (dayMetrics.request_success ?? 0) + (dayMetrics.request_failed ?? 0)
              : 0
            const inTokens = dayMetrics?.input_token ?? 0
            const outTokens = dayMetrics?.output_token ?? 0
            const totalCost = dayMetrics
              ? (dayMetrics.input_cost ?? 0) + (dayMetrics.output_cost ?? 0)
              : 0

            return (
              <div className="flex flex-col gap-3">
                {/* Date header with nav arrows */}
                <div className="flex items-center gap-3">
                  <button
                    onClick={() => shiftDay(-1)}
                    className="text-muted-foreground hover:text-foreground hover:bg-muted hover:border-border rounded-md border-2 border-transparent p-1 transition-colors"
                  >
                    <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                      <path
                        d="M10 4L6 8L10 12"
                        stroke="currentColor"
                        strokeWidth="2"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      />
                    </svg>
                  </button>
                  <div className="flex items-baseline gap-2">
                    <span className="text-base font-bold">{selectedDisplayDate}</span>
                    <span className="text-muted-foreground text-xs font-medium">
                      {selectedDayWeekday}
                    </span>
                  </div>
                  <button
                    onClick={() => shiftDay(1)}
                    disabled={isFutureDate || isToday}
                    className="text-muted-foreground hover:text-foreground hover:bg-muted hover:border-border rounded-md border-2 border-transparent p-1 transition-colors disabled:cursor-not-allowed disabled:opacity-30 disabled:hover:border-transparent disabled:hover:bg-transparent"
                  >
                    <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                      <path
                        d="M6 4L10 8L6 12"
                        stroke="currentColor"
                        strokeWidth="2"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      />
                    </svg>
                  </button>
                  <div className="ml-auto flex items-center gap-3">
                    {!isToday && (
                      <button
                        onClick={() => setSelectedDateStr(null)}
                        className="text-muted-foreground hover:text-foreground text-xs font-medium underline-offset-2 hover:underline"
                      >
                        {t("activity.today")}
                      </button>
                    )}
                    <button
                      onClick={() => navigateToDay(selectedDayDateStr)}
                      className="text-muted-foreground hover:text-foreground text-xs font-medium underline-offset-2 hover:underline"
                    >
                      {t("activity.viewLogs")}
                    </button>
                  </div>
                </div>

                {/* Stats summary cards */}
                <div className="grid grid-cols-4 gap-2">
                  {[
                    {
                      label: t("stats.requests"),
                      value: formatCount(reqCount),
                      icon: MessageSquare,
                      bg: "bg-blue-500/10",
                    },
                    {
                      label: t("stats.inputTokens"),
                      value: formatCount(inTokens),
                      icon: ArrowDownToLine,
                      bg: "bg-orange-500/10",
                    },
                    {
                      label: t("stats.outputTokens"),
                      value: formatCount(outTokens),
                      icon: ArrowUpFromLine,
                      bg: "bg-violet-500/10",
                    },
                    {
                      label: t("stats.cost"),
                      value: formatMoney(totalCost),
                      icon: DollarSign,
                      bg: "bg-emerald-500/10",
                    },
                  ].map((stat) => (
                    <div
                      key={stat.label}
                      className="bg-muted/50 flex items-center gap-2 rounded-md px-2.5 py-2"
                    >
                      <div
                        className={`flex h-6 w-6 shrink-0 items-center justify-center rounded-md ${stat.bg}`}
                      >
                        <stat.icon className="text-primary h-3 w-3" />
                      </div>
                      <div className="min-w-0">
                        <div className="text-muted-foreground text-[10px] leading-tight">
                          {stat.label}
                        </div>
                        <div className="text-sm leading-tight font-bold tabular-nums">
                          {stat.value.value}
                          {stat.value.unit && (
                            <span className="text-muted-foreground ml-0.5 text-[10px] font-medium">
                              {stat.value.unit}
                            </span>
                          )}
                        </div>
                      </div>
                    </div>
                  ))}
                </div>

                {/* ══ HERO GEAR CLOCK — Bold dual-ring with data-driven rotation ══ */}
                <HeroGearClock
                  dayHourlyMap={dayHourlyMap}
                  isToday={isToday}
                  nowHour={nowHour}
                  now={now}
                  reqCount={reqCount}
                  inTokens={inTokens}
                  outTokens={outTokens}
                  totalCost={totalCost}
                  selectedDayDateStr={selectedDayDateStr}
                  selectedDisplayDate={selectedDisplayDate}
                  gearAngle={gearAngle}
                  t={t}
                  navigateToHour={navigateToHour}
                  handleMouseEnter={handleMouseEnter}
                  handleMouseLeave={handleMouseLeave}
                />
              </div>
            )
          })()}

        {view === "year" && (
          <div className="flex flex-col gap-2">
            <div className="flex items-center gap-3">
              <button
                onClick={() => setYearOffset((o) => o - 1)}
                className="text-muted-foreground hover:text-foreground hover:bg-muted hover:border-border rounded-md border-2 border-transparent p-1 transition-colors"
              >
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                  <path
                    d="M10 4L6 8L10 12"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                </svg>
              </button>
              <span className="text-base font-bold">{yearLabel}</span>
              <button
                onClick={() => setYearOffset((o) => o + 1)}
                disabled={yearOffset >= 0}
                className="text-muted-foreground hover:text-foreground hover:bg-muted hover:border-border rounded-md border-2 border-transparent p-1 transition-colors disabled:cursor-not-allowed disabled:opacity-30 disabled:hover:border-transparent disabled:hover:bg-transparent"
              >
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                  <path
                    d="M6 4L10 8L6 12"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                </svg>
              </button>
              <div className="ml-auto flex items-center gap-3">
                {yearOffset !== 0 && (
                  <button
                    onClick={() => setYearOffset(0)}
                    className="text-muted-foreground hover:text-foreground text-xs font-medium underline-offset-2 hover:underline"
                  >
                    {t("activity.thisYear")}
                  </button>
                )}
                <button
                  onClick={navigateToYear}
                  className="text-muted-foreground hover:text-foreground text-xs font-medium underline-offset-2 hover:underline"
                >
                  {t("activity.viewLogs")}
                </button>
              </div>
            </div>
            <div
              className="grid gap-[3px]"
              style={{
                gridTemplateColumns: "repeat(53, 1fr)",
                gridTemplateRows: "repeat(7, 1fr)",
                gridAutoFlow: "column",
              }}
            >
              {yearDays.map((day) => renderCell(day, day.dateStr))}
            </div>
          </div>
        )}

        {view === "month" &&
          (() => {
            const todayStr = toDateStr(today)
            const monthMaxCount = Math.max(
              1,
              ...monthDays
                .filter(Boolean)
                .map((d) =>
                  d && d.daily ? (d.daily.request_success ?? 0) + (d.daily.request_failed ?? 0) : 0,
                ),
            )
            // Aggregate monthly stats
            const monthTotalReq = monthDays.reduce(
              (s, d) =>
                s + (d?.daily ? (d.daily.request_success ?? 0) + (d.daily.request_failed ?? 0) : 0),
              0,
            )
            const monthTotalIn = monthDays.reduce((s, d) => s + (d?.daily?.input_token ?? 0), 0)
            const monthTotalOut = monthDays.reduce((s, d) => s + (d?.daily?.output_token ?? 0), 0)
            const monthTotalCost = monthDays.reduce(
              (s, d) => s + (d?.daily ? (d.daily.input_cost ?? 0) + (d.daily.output_cost ?? 0) : 0),
              0,
            )

            return (
              <div className="flex flex-col gap-3">
                {/* Month header with nav */}
                <div className="flex items-center gap-3">
                  <button
                    onClick={() => setMonthOffset((o) => o - 1)}
                    className="text-muted-foreground hover:text-foreground hover:bg-muted hover:border-border rounded-md border-2 border-transparent p-1 transition-colors"
                  >
                    <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                      <path
                        d="M10 4L6 8L10 12"
                        stroke="currentColor"
                        strokeWidth="2"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      />
                    </svg>
                  </button>
                  <span className="text-base font-bold">{monthLabel}</span>
                  <button
                    onClick={() => setMonthOffset((o) => o + 1)}
                    disabled={monthOffset >= 0}
                    className="text-muted-foreground hover:text-foreground hover:bg-muted hover:border-border rounded-md border-2 border-transparent p-1 transition-colors disabled:cursor-not-allowed disabled:opacity-30 disabled:hover:border-transparent disabled:hover:bg-transparent"
                  >
                    <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                      <path
                        d="M6 4L10 8L6 12"
                        stroke="currentColor"
                        strokeWidth="2"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      />
                    </svg>
                  </button>
                  <div className="ml-auto flex items-center gap-3">
                    {monthOffset !== 0 && (
                      <button
                        onClick={() => setMonthOffset(0)}
                        className="text-muted-foreground hover:text-foreground text-xs font-medium underline-offset-2 hover:underline"
                      >
                        {t("activity.thisMonth")}
                      </button>
                    )}
                    <button
                      onClick={navigateToMonth}
                      className="text-muted-foreground hover:text-foreground text-xs font-medium underline-offset-2 hover:underline"
                    >
                      {t("activity.viewLogs")}
                    </button>
                  </div>
                </div>

                {/* Monthly stats summary */}
                <div className="grid grid-cols-4 gap-2">
                  {[
                    {
                      label: t("stats.requests"),
                      value: formatCount(monthTotalReq),
                      icon: MessageSquare,
                      bg: "bg-blue-500/10",
                    },
                    {
                      label: t("stats.inputTokens"),
                      value: formatCount(monthTotalIn),
                      icon: ArrowDownToLine,
                      bg: "bg-orange-500/10",
                    },
                    {
                      label: t("stats.outputTokens"),
                      value: formatCount(monthTotalOut),
                      icon: ArrowUpFromLine,
                      bg: "bg-violet-500/10",
                    },
                    {
                      label: t("stats.cost"),
                      value: formatMoney(monthTotalCost),
                      icon: DollarSign,
                      bg: "bg-emerald-500/10",
                    },
                  ].map((stat) => (
                    <div
                      key={stat.label}
                      className="bg-muted/50 flex items-center gap-2 rounded-md px-2.5 py-2"
                    >
                      <div
                        className={`flex h-6 w-6 shrink-0 items-center justify-center rounded-md ${stat.bg}`}
                      >
                        <stat.icon className="text-primary h-3 w-3" />
                      </div>
                      <div className="min-w-0">
                        <div className="text-muted-foreground text-[10px] leading-tight">
                          {stat.label}
                        </div>
                        <div className="text-sm leading-tight font-bold tabular-nums">
                          {stat.value.value}
                          {stat.value.unit && (
                            <span className="text-muted-foreground ml-0.5 text-[10px] font-medium">
                              {stat.value.unit}
                            </span>
                          )}
                        </div>
                      </div>
                    </div>
                  ))}
                </div>

                {/* Calendar grid */}
                <div>
                  {/* Weekday header */}
                  <div className="mb-1.5 grid grid-cols-7 gap-1.5">
                    {weekdayLabels.map((label) => (
                      <div
                        key={label}
                        className="text-muted-foreground py-0.5 text-center text-xs font-bold"
                      >
                        {label}
                      </div>
                    ))}
                  </div>
                  {/* Day cells */}
                  <div className="grid grid-cols-7 gap-1.5">
                    {monthDays.map((day, i) => {
                      // eslint-disable-next-line react/no-array-index-key -- padding cells have no stable identity
                      if (!day) return <div key={`pad-${i}`} />
                      const isToday = day.dateStr === todayStr
                      const dayNum = Number.parseInt(day.dateStr.slice(6, 8))

                      if (day.isFuture) {
                        return (
                          <div
                            key={day.dateStr}
                            className="border-border/20 flex aspect-square flex-col items-center justify-center rounded-md border border-dashed"
                          >
                            <span className="text-muted-foreground/30 text-xs font-bold tabular-nums">
                              {dayNum}
                            </span>
                          </div>
                        )
                      }

                      const count =
                        (day.daily?.request_success ?? 0) + (day.daily?.request_failed ?? 0)
                      const level = getActivityLevel(count)
                      const barWidthPct =
                        monthMaxCount > 0 ? Math.max(8, (count / monthMaxCount) * 100) : 8

                      return (
                        <div
                          key={day.dateStr}
                          className={`flex aspect-square cursor-pointer flex-col items-center justify-between rounded-md border-2 p-1 transition-all hover:scale-105 ${
                            isToday
                              ? "border-foreground bg-card shadow-[2px_2px_0_var(--nb-shadow)]"
                              : "border-border/40 hover:border-border bg-card/50"
                          }`}
                          onClick={() => drillIntoDay(day.dateStr)}
                          onMouseEnter={(e) =>
                            handleMouseEnter(e, { label: day.displayDate, metrics: day.daily })
                          }
                          onMouseLeave={handleMouseLeave}
                        >
                          {/* Day number */}
                          <span
                            className={`text-xs leading-none font-bold tabular-nums ${isToday ? "text-foreground" : "text-muted-foreground"}`}
                          >
                            {dayNum}
                          </span>
                          {/* Request count */}
                          {count > 0 && (
                            <span className="text-foreground text-[9px] leading-none font-bold tabular-nums">
                              {formatCount(count).value}
                            </span>
                          )}
                          {/* Activity bar at bottom */}
                          <div className="bg-muted/60 h-[3px] w-full overflow-hidden rounded-full">
                            <div
                              className="h-full rounded-full transition-all duration-500"
                              style={{
                                width: count > 0 ? `${barWidthPct}%` : "0%",
                                backgroundColor: LEVEL_COLORS[level],
                              }}
                            />
                          </div>
                        </div>
                      )
                    })}
                  </div>
                </div>
              </div>
            )
          })()}

        {view === "week" &&
          (() => {
            const weekMaxCount = Math.max(
              1,
              ...weekDays.map((d) =>
                d.daily ? (d.daily.request_success ?? 0) + (d.daily.request_failed ?? 0) : 0,
              ),
            )
            const todayStr = toDateStr(today)
            // Aggregate weekly stats
            const weekTotalReq = weekDays.reduce(
              (s, d) =>
                s + (d.daily ? (d.daily.request_success ?? 0) + (d.daily.request_failed ?? 0) : 0),
              0,
            )
            const weekTotalIn = weekDays.reduce((s, d) => s + (d.daily?.input_token ?? 0), 0)
            const weekTotalOut = weekDays.reduce((s, d) => s + (d.daily?.output_token ?? 0), 0)
            const weekTotalCost = weekDays.reduce(
              (s, d) => s + (d.daily ? (d.daily.input_cost ?? 0) + (d.daily.output_cost ?? 0) : 0),
              0,
            )

            return (
              <div className="flex flex-col gap-3">
                {/* Week header with nav */}
                <div className="flex items-center gap-3">
                  <button
                    onClick={() => setWeekOffset((o) => o - 1)}
                    className="text-muted-foreground hover:text-foreground hover:bg-muted hover:border-border rounded-md border-2 border-transparent p-1 transition-colors"
                  >
                    <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                      <path
                        d="M10 4L6 8L10 12"
                        stroke="currentColor"
                        strokeWidth="2"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      />
                    </svg>
                  </button>
                  <span className="text-base font-bold">{weekLabel}</span>
                  <button
                    onClick={() => setWeekOffset((o) => o + 1)}
                    disabled={weekOffset >= 0}
                    className="text-muted-foreground hover:text-foreground hover:bg-muted hover:border-border rounded-md border-2 border-transparent p-1 transition-colors disabled:cursor-not-allowed disabled:opacity-30 disabled:hover:border-transparent disabled:hover:bg-transparent"
                  >
                    <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                      <path
                        d="M6 4L10 8L6 12"
                        stroke="currentColor"
                        strokeWidth="2"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      />
                    </svg>
                  </button>
                  <div className="ml-auto flex items-center gap-3">
                    {weekOffset !== 0 && (
                      <button
                        onClick={() => setWeekOffset(0)}
                        className="text-muted-foreground hover:text-foreground text-xs font-medium underline-offset-2 hover:underline"
                      >
                        {t("activity.thisWeek")}
                      </button>
                    )}
                    <button
                      onClick={navigateToWeek}
                      className="text-muted-foreground hover:text-foreground text-xs font-medium underline-offset-2 hover:underline"
                    >
                      {t("activity.viewLogs")}
                    </button>
                  </div>
                </div>

                {/* Weekly stats summary */}
                <div className="grid grid-cols-4 gap-2">
                  {[
                    {
                      label: t("stats.requests"),
                      value: formatCount(weekTotalReq),
                      icon: MessageSquare,
                      bg: "bg-blue-500/10",
                    },
                    {
                      label: t("stats.inputTokens"),
                      value: formatCount(weekTotalIn),
                      icon: ArrowDownToLine,
                      bg: "bg-orange-500/10",
                    },
                    {
                      label: t("stats.outputTokens"),
                      value: formatCount(weekTotalOut),
                      icon: ArrowUpFromLine,
                      bg: "bg-violet-500/10",
                    },
                    {
                      label: t("stats.cost"),
                      value: formatMoney(weekTotalCost),
                      icon: DollarSign,
                      bg: "bg-emerald-500/10",
                    },
                  ].map((stat) => (
                    <div
                      key={stat.label}
                      className="bg-muted/50 flex items-center gap-2 rounded-md px-2.5 py-2"
                    >
                      <div
                        className={`flex h-6 w-6 shrink-0 items-center justify-center rounded-md ${stat.bg}`}
                      >
                        <stat.icon className="text-primary h-3 w-3" />
                      </div>
                      <div className="min-w-0">
                        <div className="text-muted-foreground text-[10px] leading-tight">
                          {stat.label}
                        </div>
                        <div className="text-sm leading-tight font-bold tabular-nums">
                          {stat.value.value}
                          {stat.value.unit && (
                            <span className="text-muted-foreground ml-0.5 text-[10px] font-medium">
                              {stat.value.unit}
                            </span>
                          )}
                        </div>
                      </div>
                    </div>
                  ))}
                </div>

                {/* Bar chart — 7 vertical columns */}
                <div className="flex items-end gap-2 px-1" style={{ height: 180 }}>
                  {weekDays.map((day) => {
                    const dayOfWeek = new Date(
                      Number.parseInt(day.dateStr.slice(0, 4)),
                      Number.parseInt(day.dateStr.slice(4, 6)) - 1,
                      Number.parseInt(day.dateStr.slice(6, 8)),
                    ).getDay()
                    const count = day.daily
                      ? (day.daily.request_success ?? 0) + (day.daily.request_failed ?? 0)
                      : 0
                    const level = day.isFuture ? -1 : getActivityLevel(count)
                    const barHeight = day.isFuture ? 0 : Math.max(4, (count / weekMaxCount) * 150)
                    const isCurrentDay = day.dateStr === todayStr

                    return (
                      <div
                        key={day.dateStr}
                        className="flex flex-1 flex-col items-center gap-1.5"
                        style={{ height: "100%" }}
                      >
                        {/* Count label on top */}
                        <div className="text-muted-foreground text-[10px] font-bold tabular-nums">
                          {day.isFuture ? "" : count > 0 ? formatCount(count).value : "0"}
                        </div>

                        {/* Bar grows upward */}
                        <div className="relative flex w-full flex-1 items-end justify-center">
                          {day.isFuture ? (
                            <div
                              className="border-border/30 w-full rounded-t-md border border-b-0 border-dashed"
                              style={{ height: 30 }}
                            />
                          ) : (
                            <div
                              className={`w-full cursor-pointer rounded-t-md transition-all hover:opacity-80 ${isCurrentDay ? "border-foreground border-2 border-b-0" : ""}`}
                              style={{
                                height: barHeight,
                                backgroundColor: LEVEL_COLORS[level],
                                transition: "height 0.5s cubic-bezier(0.34, 1.56, 0.64, 1)",
                              }}
                              onClick={() => drillIntoDay(day.dateStr)}
                              onMouseEnter={(e) =>
                                handleMouseEnter(e, { label: day.displayDate, metrics: day.daily })
                              }
                              onMouseLeave={handleMouseLeave}
                            />
                          )}
                        </div>

                        {/* Bottom baseline */}
                        <div className="bg-border h-[2px] w-full" />

                        {/* Weekday label */}
                        <div
                          className={`text-center text-xs font-bold ${isCurrentDay ? "text-foreground" : "text-muted-foreground"}`}
                        >
                          {weekdayLabelsRaw[dayOfWeek]}
                        </div>
                        {/* Date label */}
                        <div className="text-muted-foreground text-center text-[10px] leading-none tabular-nums">
                          {day.dateStr.slice(4, 6)}/{day.dateStr.slice(6, 8)}
                        </div>
                      </div>
                    )
                  })}
                </div>
              </div>
            )
          })()}
      </div>

      {/* Legend */}
      <div className="text-muted-foreground flex items-center justify-end gap-1 px-4 pb-3 text-xs">
        <span>{t("activity.less")}</span>
        {LEVEL_COLORS.map((c, i) => (
          // eslint-disable-next-line react/no-array-index-key -- static array of CSS color strings used as legend; index is stable
          <div key={i} className="h-3 w-3 rounded-sm" style={{ backgroundColor: c }} />
        ))}
        <span>{t("activity.more")}</span>
      </div>

      {/* Floating tooltip */}
      {activeTooltip &&
        (() => {
          const m = activeTooltip.metrics
          const reqCount = m ? formatCount(m.request_success + m.request_failed) : null
          const inputTokens = m ? formatCount(m.input_token) : null
          const outputTokens = m ? formatCount(m.output_token) : null
          const cost = m ? formatMoney(m.input_cost + m.output_cost) : null
          return (
            <FloatingPortal>
              <div
                ref={refs.setFloating}
                style={floatingStyles}
                className="bg-popover text-popover-foreground border-border pointer-events-none z-50 w-fit min-w-max rounded-md border-2 p-3 text-sm shadow-[2px_2px_0_var(--nb-shadow)]"
              >
                <p className="mb-1 font-bold">{activeTooltip.label}</p>
                {m ? (
                  <div className="text-muted-foreground grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
                    <span>{t("stats.requests")}</span>
                    <span className="text-foreground text-right font-medium">
                      {reqCount!.value}
                      {reqCount!.unit}
                    </span>
                    <span>{t("stats.inputTokens")}</span>
                    <span className="text-foreground text-right font-medium">
                      {inputTokens!.value}
                      {inputTokens!.unit}
                    </span>
                    <span>{t("stats.outputTokens")}</span>
                    <span className="text-foreground text-right font-medium">
                      {outputTokens!.value}
                      {outputTokens!.unit}
                    </span>
                    <span>{t("stats.cost")}</span>
                    <span className="text-foreground text-right font-medium">{cost!.value}</span>
                  </div>
                ) : (
                  <p className="text-muted-foreground">{t("activity.noData")}</p>
                )}
              </div>
            </FloatingPortal>
          )
        })()}
    </Card>
  )
}

// ───────────── Shared sort type for ranking sections ─────────────

type RankSortKey = "requests" | "tokens" | "cost" | "latency"

/** Render a formatted value with its unit (if any) */
function Fmt({ fmt }: { fmt: { value: string; unit: string } }) {
  return (
    <span className="font-medium">
      {fmt.value}
      {fmt.unit && <span className="text-muted-foreground ml-0.5 text-[10px]">{fmt.unit}</span>}
    </span>
  )
}

// ───────────── Channel Ranking ─────────────

type ChannelSortKey = "requests" | "cost" | "latency"

function RankSection({ data }: { data?: ChannelStatsRow[] }) {
  const { t } = useTranslation("dashboard")
  const [sortBy, setSortBy] = useState<ChannelSortKey>("requests")

  const channelSortOptions = useMemo(
    () => [
      { key: "requests" as const, label: t("sort.requests") },
      { key: "cost" as const, label: t("sort.cost") },
      { key: "latency" as const, label: t("sort.latency") },
    ],
    [t],
  )

  const sorted = useMemo(() => {
    if (!data) return []
    return [...data].sort((a, b) => {
      switch (sortBy) {
        case "requests":
          return (b.totalRequests || 0) - (a.totalRequests || 0)
        case "cost":
          return (b.totalCost || 0) - (a.totalCost || 0)
        case "latency":
          return (b.avgLatency || 0) - (a.avgLatency || 0)
        default:
          return 0
      }
    })
  }, [data, sortBy])

  const maxVal = useMemo(() => {
    if (sorted.length === 0) return 1
    switch (sortBy) {
      case "requests":
        return sorted[0].totalRequests || 1
      case "cost":
        return sorted[0].totalCost || 1
      case "latency":
        return sorted[0].avgLatency || 1
    }
  }, [sorted, sortBy])

  const barPercent = (ch: ChannelStatsRow) => {
    switch (sortBy) {
      case "requests":
        return ((ch.totalRequests || 0) / maxVal) * 100
      case "cost":
        return ((ch.totalCost || 0) / maxVal) * 100
      case "latency":
        return ((ch.avgLatency || 0) / maxVal) * 100
    }
  }

  return (
    <Card>
      <Tabs value={sortBy} onValueChange={(v) => setSortBy(v as ChannelSortKey)}>
        <CardHeader className="flex flex-col gap-2 pb-2">
          <div className="flex items-center justify-between">
            <CardTitle className="text-base">{t("channelRanking.title")}</CardTitle>
          </div>
          <TabsList>
            {channelSortOptions.map((o) => (
              <TabsTrigger key={o.key} value={o.key}>
                {o.label}
              </TabsTrigger>
            ))}
          </TabsList>
        </CardHeader>
        <CardContent>
          {sorted.length === 0 ? (
            <div className="text-muted-foreground flex flex-col items-center justify-center py-8">
              <Bot className="mb-3 h-12 w-12 opacity-30" />
              <p className="text-sm">{t("channelRanking.noData")}</p>
            </div>
          ) : (
            <div className="max-h-[400px] space-y-1.5 overflow-y-auto">
              {sorted.map((ch) => {
                const reqFmt = formatCount(ch.totalRequests)
                const costFmt = formatMoney(ch.totalCost)
                const latFmt = formatTime(ch.avgLatency)
                return (
                  <Link
                    key={ch.channelId}
                    to={`/logs?channel=${ch.channelId}`}
                    className="hover:bg-muted/50 relative block rounded-md px-3 py-2 transition-colors"
                  >
                    <div
                      className="absolute inset-y-0 left-0 rounded-md opacity-10 transition-[width] duration-500 ease-out"
                      style={{
                        width: `${barPercent(ch)}%`,
                        backgroundColor: "var(--primary)",
                      }}
                    />
                    <div className="relative flex flex-col gap-1">
                      <div className="min-w-0 truncate text-sm font-medium">
                        {ch.channelName ||
                          t("channelRanking.channelFallback", { id: ch.channelId })}
                      </div>
                      <div className="flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs tabular-nums">
                        <div>
                          <span className="text-muted-foreground">{t("inline.req")} </span>
                          <Fmt fmt={reqFmt} />
                        </div>
                        <div>
                          <span className="text-muted-foreground">{t("inline.cost")} </span>
                          <Fmt fmt={costFmt} />
                        </div>
                        <div>
                          <span className="text-muted-foreground">{t("inline.avg")} </span>
                          <Fmt fmt={latFmt} />
                        </div>
                      </div>
                    </div>
                  </Link>
                )
              })}
            </div>
          )}
        </CardContent>
      </Tabs>
    </Card>
  )
}

// ───────────── Model Usage Stats ─────────────

function ModelStatsSection({ data }: { data?: ModelStatsItem[] }) {
  const { t } = useTranslation("dashboard")
  const [sortBy, setSortBy] = useState<RankSortKey>("requests")

  const rankSortOptions = useMemo(
    () => [
      { key: "requests" as const, label: t("sort.requests") },
      { key: "tokens" as const, label: t("sort.tokens") },
      { key: "cost" as const, label: t("sort.cost") },
      { key: "latency" as const, label: t("sort.latency") },
    ],
    [t],
  )

  const sorted = useMemo(() => {
    if (!data) return []
    return [...data].sort((a, b) => {
      switch (sortBy) {
        case "requests":
          return b.requestCount - a.requestCount
        case "tokens":
          return b.inputTokens + b.outputTokens - (a.inputTokens + a.outputTokens)
        case "cost":
          return b.totalCost - a.totalCost
        case "latency":
          return b.avgLatency - a.avgLatency
        default:
          return 0
      }
    })
  }, [data, sortBy])

  const maxVal = useMemo(() => {
    if (sorted.length === 0) return 1
    switch (sortBy) {
      case "requests":
        return sorted[0].requestCount || 1
      case "tokens":
        return sorted[0].inputTokens + sorted[0].outputTokens || 1
      case "cost":
        return sorted[0].totalCost || 1
      case "latency":
        return sorted[0].avgLatency || 1
    }
  }, [sorted, sortBy])

  const barPercent = (item: ModelStatsItem) => {
    switch (sortBy) {
      case "requests":
        return (item.requestCount / maxVal) * 100
      case "tokens":
        return ((item.inputTokens + item.outputTokens) / maxVal) * 100
      case "cost":
        return (item.totalCost / maxVal) * 100
      case "latency":
        return (item.avgLatency / maxVal) * 100
    }
  }

  return (
    <Card>
      <Tabs value={sortBy} onValueChange={(v) => setSortBy(v as RankSortKey)}>
        <CardHeader className="flex flex-col gap-2 pb-2">
          <div className="flex items-center justify-between">
            <CardTitle className="text-base">{t("modelUsage.title")}</CardTitle>
          </div>
          <TabsList>
            {rankSortOptions.map((o) => (
              <TabsTrigger key={o.key} value={o.key}>
                {o.label}
              </TabsTrigger>
            ))}
          </TabsList>
        </CardHeader>
        <CardContent>
          {sorted.length === 0 ? (
            <div className="text-muted-foreground flex flex-col items-center justify-center py-8">
              <Bot className="mb-3 h-12 w-12 opacity-30" />
              <p className="text-sm">{t("modelUsage.noData")}</p>
            </div>
          ) : (
            <div className="max-h-[400px] space-y-1.5 overflow-y-auto">
              {sorted.map((item) => {
                const reqFmt = formatCount(item.requestCount)
                const inFmt = formatCount(item.inputTokens)
                const outFmt = formatCount(item.outputTokens)
                const costFmt = formatMoney(item.totalCost)
                const latFmt = formatTime(item.avgLatency)
                return (
                  <Link
                    key={item.model}
                    to={`/logs?model=${encodeURIComponent(item.model)}`}
                    className="hover:bg-muted/50 relative block rounded-md px-3 py-2 transition-colors"
                  >
                    {/* Background bar */}
                    <div
                      className="absolute inset-y-0 left-0 rounded-md opacity-10 transition-[width] duration-500 ease-out"
                      style={{
                        width: `${barPercent(item)}%`,
                        backgroundColor: "var(--primary)",
                      }}
                    />
                    <div className="relative flex flex-col gap-1">
                      <div className="min-w-0 truncate text-sm font-medium">
                        <ModelBadge modelId={item.model} />
                      </div>
                      <div className="flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs tabular-nums">
                        <div>
                          <span className="text-muted-foreground">{t("inline.req")} </span>
                          <Fmt fmt={reqFmt} />
                        </div>
                        <div>
                          <span className="text-muted-foreground">{t("inline.in")} </span>
                          <Fmt fmt={inFmt} />
                        </div>
                        <div>
                          <span className="text-muted-foreground">{t("inline.out")} </span>
                          <Fmt fmt={outFmt} />
                        </div>
                        <div>
                          <span className="text-muted-foreground">{t("inline.cost")} </span>
                          <Fmt fmt={costFmt} />
                        </div>
                        <div>
                          <span className="text-muted-foreground">{t("inline.avg")} </span>
                          <Fmt fmt={latFmt} />
                        </div>
                      </div>
                    </div>
                  </Link>
                )
              })}
            </div>
          )}
        </CardContent>
      </Tabs>
    </Card>
  )
}

// ───────────── Page ─────────────

// Streaming increment entry for real-time dashboard updates
interface StreamIncrement {
  estimatedInputTokens: number
  outputTokens: number
  cost: number
  inputPrice: number
  outputPrice: number
}

export default function DashboardPage() {
  const { t } = useTranslation("dashboard")

  // Track streaming request increments for real-time stat updates
  const streamIncrementsRef = useRef(new Map<string, StreamIncrement>())
  const [incrementVersion, setIncrementVersion] = useState(0)

  useWsEvent("log-stream-start", (data) => {
    if (!data?.streamId) return
    streamIncrementsRef.current.set(data.streamId, {
      estimatedInputTokens: data.estimatedInputTokens ?? 0,
      outputTokens: 0,
      cost: 0,
      inputPrice: data.inputPrice ?? 0,
      outputPrice: data.outputPrice ?? 0,
    })
    setIncrementVersion((v) => v + 1)
  })

  useWsEvent("log-streaming", (data) => {
    if (!data?.streamId) return
    const entry = streamIncrementsRef.current.get(data.streamId)
    if (!entry) return
    const contentLen = (data.responseLength ?? 0) + (data.thinkingLength ?? 0)
    const outputTokens = Math.floor(contentLen / 3)
    const cost =
      (entry.estimatedInputTokens * entry.inputPrice + outputTokens * entry.outputPrice) / 1_000_000
    streamIncrementsRef.current.set(data.streamId, {
      ...entry,
      outputTokens,
      cost,
    })
    setIncrementVersion((v) => v + 1)
  })

  useWsEvent("log-created", (data) => {
    if (!data?.streamId) return
    streamIncrementsRef.current.delete(data.streamId)
    setIncrementVersion((v) => v + 1)
  })

  useWsEvent("log-stream-end", (data) => {
    if (!data?.streamId) return
    streamIncrementsRef.current.delete(data.streamId)
    setIncrementVersion((v) => v + 1)
  })

  // Compute aggregate streaming increments
  const streamingDelta = useMemo(() => {
    void incrementVersion // depend on version to recompute
    let inputTokens = 0
    let outputTokens = 0
    let inputCost = 0
    let outputCost = 0
    for (const entry of streamIncrementsRef.current.values()) {
      inputTokens += entry.estimatedInputTokens
      outputTokens += entry.outputTokens
      inputCost += (entry.estimatedInputTokens * entry.inputPrice) / 1_000_000
      outputCost += (entry.outputTokens * entry.outputPrice) / 1_000_000
    }
    return { inputTokens, outputTokens, inputCost, outputCost }
  }, [incrementVersion])

  const {
    data: totalData,
    isError: isTotalError,
    refetch: refetchTotal,
  } = useQuery({
    queryKey: ["stats", "total"],
    queryFn: getTotalStats,
  })

  const {
    data: dailyData,
    isError: isDailyError,
    refetch: refetchDaily,
  } = useQuery({
    queryKey: ["stats", "daily"],
    queryFn: getDailyStats,
  })

  const {
    data: hourlyData,
    isError: isHourlyError,
    refetch: refetchHourly,
  } = useQuery({
    queryKey: ["stats", "hourly"],
    queryFn: () => getHourlyStats(),
  })

  const {
    data: channelData,
    isError: isChannelError,
    refetch: refetchChannel,
  } = useQuery({
    queryKey: ["stats", "channel"],
    queryFn: getChannelStats,
  })

  const {
    data: modelData,
    isError: isModelError,
    refetch: refetchModel,
  } = useQuery({
    queryKey: ["stats", "model"],
    queryFn: getModelStats,
  })

  const isStatsError = isTotalError || isDailyError || isHourlyError
  const refetchStats = () => {
    if (isTotalError) refetchTotal()
    if (isDailyError) refetchDaily()
    if (isHourlyError) refetchHourly()
  }

  return (
    <div className="flex flex-col gap-6">
      {isStatsError ? (
        <InlineError message={t("errors.dashboardStats")} onRetry={refetchStats} />
      ) : (
        <>
          <TotalSection data={totalData?.data} streamingDelta={streamingDelta} />
          <ActivitySection data={dailyData?.data} />
          <Suspense fallback={<Skeleton className="h-[280px] w-full rounded-xl" />}>
            <LazyChartSection dailyData={dailyData?.data} hourlyData={hourlyData?.data} />
          </Suspense>
        </>
      )}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        {isModelError ? (
          <InlineError message={t("errors.modelStats")} onRetry={() => refetchModel()} />
        ) : (
          <motion.div
            initial={{ opacity: 0, x: -16 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{ duration: 0.4, delay: 0.2, ease: [0.33, 1, 0.68, 1] }}
          >
            <ModelStatsSection data={modelData?.data} />
          </motion.div>
        )}
        {isChannelError ? (
          <InlineError message={t("errors.channelStats")} onRetry={() => refetchChannel()} />
        ) : (
          <motion.div
            initial={{ opacity: 0, x: 16 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{ duration: 0.4, delay: 0.3, ease: [0.33, 1, 0.68, 1] }}
          >
            <RankSection data={channelData?.data} />
          </motion.div>
        )}
      </div>
    </div>
  )
}
