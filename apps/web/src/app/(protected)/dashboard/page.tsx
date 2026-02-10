"use client"

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
  ArrowDownToLine,
  ArrowUpFromLine,
  Bot,
  ChartColumnBig,
  Clock,
  DollarSign,
  MessageSquare,
  TrendingUp,
} from "lucide-react"
import { LayoutGroup, motion } from "motion/react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { useCallback, useEffect, useMemo, useState } from "react"
import {
  Area,
  AreaChart,
  CartesianGrid,
  Tooltip as RechartsTooltip,
  ResponsiveContainer,
  XAxis,
  YAxis,
} from "recharts"
import { AnimatedNumber } from "@/components/animated-number"
import { ModelBadge } from "@/components/model-badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  getChannelStats,
  getDailyStats,
  getHourlyStats,
  getModelStats,
  getTotalStats,
} from "@/lib/api"

// ───────────── formatting helpers ─────────────

function formatCount(n: number): { value: string; unit: string; raw: number } {
  if (n >= 1_000_000_000) return { value: (n / 1_000_000_000).toFixed(1), unit: "B", raw: n }
  if (n >= 1_000_000) return { value: (n / 1_000_000).toFixed(1), unit: "M", raw: n }
  if (n >= 1_000) return { value: (n / 1_000).toFixed(1), unit: "K", raw: n }
  return { value: String(Math.round(n)), unit: "", raw: n }
}

function formatMoney(n: number): { value: string; unit: string; raw: number } {
  if (n >= 1_000_000) return { value: `$${(n / 1_000_000).toFixed(2)}`, unit: "M", raw: n }
  if (n >= 1_000) return { value: `$${(n / 1_000).toFixed(2)}`, unit: "K", raw: n }
  if (n >= 1) return { value: `$${n.toFixed(2)}`, unit: "", raw: n }
  return { value: `$${n.toFixed(4)}`, unit: "", raw: n }
}

function formatTime(ms: number): { value: string; unit: string; raw: number } {
  if (ms >= 3600000) return { value: (ms / 3600000).toFixed(1), unit: "h", raw: ms }
  if (ms >= 60000) return { value: (ms / 60000).toFixed(1), unit: "m", raw: ms }
  if (ms >= 1000) return { value: (ms / 1000).toFixed(1), unit: "s", raw: ms }
  return { value: String(Math.round(ms)), unit: "ms", raw: ms }
}

// ───────────── Total (4 stat cards) ─────────────

function TotalSection({ data }: { data?: StatsMetrics }) {
  const cards = [
    {
      title: "Request Stats",
      headerIcon: Activity,
      items: [
        {
          label: "Requests",
          raw: (data?.request_success ?? 0) + (data?.request_failed ?? 0),
          format: formatCount,
          icon: MessageSquare,
          bg: "bg-blue-500/10",
        },
        {
          label: "Time Used",
          raw: data?.wait_time ?? 0,
          format: formatTime,
          icon: Clock,
          bg: "bg-blue-500/10",
        },
      ],
    },
    {
      title: "Overview",
      headerIcon: ChartColumnBig,
      items: [
        {
          label: "Total Tokens",
          raw: (data?.input_token ?? 0) + (data?.output_token ?? 0),
          format: formatCount,
          icon: Bot,
          bg: "bg-emerald-500/10",
        },
        {
          label: "Total Cost",
          raw: (data?.input_cost ?? 0) + (data?.output_cost ?? 0),
          format: formatMoney,
          icon: DollarSign,
          bg: "bg-emerald-500/10",
        },
      ],
    },
    {
      title: "Input",
      headerIcon: ArrowDownToLine,
      items: [
        {
          label: "Input Tokens",
          raw: data?.input_token ?? 0,
          format: formatCount,
          icon: Bot,
          bg: "bg-orange-500/10",
        },
        {
          label: "Input Cost",
          raw: data?.input_cost ?? 0,
          format: formatMoney,
          icon: DollarSign,
          bg: "bg-orange-500/10",
        },
      ],
    },
    {
      title: "Output",
      headerIcon: ArrowUpFromLine,
      items: [
        {
          label: "Output Tokens",
          raw: data?.output_token ?? 0,
          format: formatCount,
          icon: Bot,
          bg: "bg-violet-500/10",
        },
        {
          label: "Output Cost",
          raw: data?.output_cost ?? 0,
          format: formatMoney,
          icon: DollarSign,
          bg: "bg-violet-500/10",
        },
      ],
    },
  ]

  return (
    <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
      {cards.map((card) => (
        <Card key={card.title} className="gap-3 p-4">
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
                    <span className="text-lg leading-tight font-bold tabular-nums">
                      <AnimatedNumber value={item.raw} formatter={(n) => item.format(n).value} />
                      {formatted.unit && (
                        <span className="text-muted-foreground ml-0.5 text-xs font-medium">
                          {formatted.unit}
                        </span>
                      )}
                    </span>
                  </div>
                </div>
              )
            })}
          </div>
        </Card>
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

type HeatmapView = "year" | "month" | "week"

const LEVEL_COLORS = [
  "var(--muted)",
  "color-mix(in srgb, var(--primary) 25%, transparent)",
  "color-mix(in srgb, var(--primary) 50%, transparent)",
  "color-mix(in srgb, var(--primary) 75%, transparent)",
  "var(--primary)",
]

const WEEKDAY_LABELS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"]

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

function ActivitySection({
  data,
  hourlyData,
}: {
  data?: StatsDaily[]
  hourlyData?: StatsHourly[]
}) {
  const [view, setView] = useState<HeatmapView>("week")
  const [activeTooltip, setActiveTooltip] = useState<HeatmapTooltip | null>(null)
  const router = useRouter()

  const [today, setToday] = useState<Date | null>(null)

  useEffect(() => {
    setToday(new Date())
  }, [])

  const { refs, floatingStyles } = useFloating({
    placement: "top",
    open: !!activeTooltip,
    middleware: [offset(8), flip(), shift({ padding: 8 })],
    whileElementsMounted: autoUpdate,
  })

  const dataMap = useMemo(() => new Map((data ?? []).map((d) => [d.date, d])), [data])

  // ── Year view: 53 weeks × 7 days, column-first grid ──
  const yearDays = useMemo(() => {
    if (!today) return []
    const todayDay = today.getDay()
    const start = new Date(today)
    start.setDate(start.getDate() - todayDay - 52 * 7)
    const result: DayData[] = []
    for (let i = 0; i < 53 * 7; i++) {
      const d = new Date(start)
      d.setDate(d.getDate() + i)
      result.push(buildDayData(d, dataMap, today))
    }
    return result
  }, [dataMap, today])

  // ── Month view: current month's days ──
  const monthDays = useMemo(() => {
    if (!today) return []
    const year = today.getFullYear()
    const month = today.getMonth()
    const firstDay = new Date(year, month, 1)
    const lastDay = new Date(year, month + 1, 0)
    const startPad = firstDay.getDay()
    const result: (DayData | null)[] = []
    for (let i = 0; i < startPad; i++) result.push(null)
    for (let d = 1; d <= lastDay.getDate(); d++) {
      const date = new Date(year, month, d)
      result.push(buildDayData(date, dataMap, today))
    }
    return result
  }, [dataMap, today])

  // ── Week view: current week (Sun–Sat) ──
  const weekDays = useMemo(() => {
    if (!today) return []
    const todayDay = today.getDay()
    const start = new Date(today)
    start.setDate(start.getDate() - todayDay)
    const result: DayData[] = []
    for (let i = 0; i < 7; i++) {
      const d = new Date(start)
      d.setDate(d.getDate() + i)
      result.push(buildDayData(d, dataMap, today))
    }
    return result
  }, [dataMap, today])

  // ── Week view: hourly data grouped by day (7 days × 24 hours) ──
  const weekHourlyMap = useMemo(() => {
    if (!today || !hourlyData) return new Map<string, Map<number, StatsHourly>>()
    const todayDay = today.getDay()
    const start = new Date(today)
    start.setDate(start.getDate() - todayDay)
    const map = new Map<string, Map<number, StatsHourly>>()
    for (const s of hourlyData) {
      const sDate = new Date(
        Number.parseInt(s.date.slice(0, 4)),
        Number.parseInt(s.date.slice(4, 6)) - 1,
        Number.parseInt(s.date.slice(6, 8)),
      )
      if (sDate >= start && sDate <= today) {
        let hourMap = map.get(s.date)
        if (!hourMap) {
          hourMap = new Map<number, StatsHourly>()
          map.set(s.date, hourMap)
        }
        hourMap.set(s.hour, s)
      }
    }
    return map
  }, [hourlyData, today])

  // ── Month view: date range for hourly query ──
  const monthRange = useMemo(() => {
    if (!today) return { start: "", end: "" }
    const year = today.getFullYear()
    const month = today.getMonth()
    const firstDay = new Date(year, month, 1)
    const startStr =
      firstDay.getFullYear().toString() +
      (firstDay.getMonth() + 1).toString().padStart(2, "0") +
      firstDay.getDate().toString().padStart(2, "0")
    const todayStr =
      today.getFullYear().toString() +
      (today.getMonth() + 1).toString().padStart(2, "0") +
      today.getDate().toString().padStart(2, "0")
    return { start: startStr, end: todayStr }
  }, [today])

  const { data: monthHourlyData } = useQuery({
    queryKey: ["stats", "hourly", monthRange.start, monthRange.end],
    queryFn: () => getHourlyStats(monthRange.start, monthRange.end),
    enabled: view === "month" && !!monthRange.start,
  })

  const monthHourlyMap = useMemo(() => {
    const raw = monthHourlyData?.data
    if (!raw) return new Map<string, Map<number, StatsHourly>>()
    const map = new Map<string, Map<number, StatsHourly>>()
    for (const s of raw) {
      let hourMap = map.get(s.date)
      if (!hourMap) {
        hourMap = new Map<number, StatsHourly>()
        map.set(s.date, hourMap)
      }
      hourMap.set(s.hour, s)
    }
    return map
  }, [monthHourlyData])

  // ── Month view: selected day for hourly breakdown ──
  const [selectedMonthDay, setSelectedMonthDay] = useState<DayData | null>(null)

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

  /** Navigate to logs filtered by a day's time range */
  const navigateToDay = useCallback(
    (dateStr: string) => {
      const y = Number.parseInt(dateStr.slice(0, 4))
      const m = Number.parseInt(dateStr.slice(4, 6)) - 1
      const d = Number.parseInt(dateStr.slice(6, 8))
      const from = Math.floor(new Date(y, m, d).getTime() / 1000)
      const to = from + 86400 - 1
      router.push(`/logs?from=${from}&to=${to}`)
    },
    [router],
  )

  /** Navigate to logs filtered by an hour's time range */
  const navigateToHour = useCallback(
    (dateStr: string, hour: number) => {
      const y = Number.parseInt(dateStr.slice(0, 4))
      const m = Number.parseInt(dateStr.slice(4, 6)) - 1
      const d = Number.parseInt(dateStr.slice(6, 8))
      const from = Math.floor(new Date(y, m, d, hour).getTime() / 1000)
      const to = from + 3600 - 1
      router.push(`/logs?from=${from}&to=${to}`)
    },
    [router],
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
        onClick={() => navigateToDay(day.dateStr)}
        onMouseEnter={(e) => handleMouseEnter(e, { label: day.displayDate, metrics: day.daily })}
        onMouseLeave={handleMouseLeave}
      />
    )
  }

  return (
    <Card className="gap-0">
      <div className="flex items-center justify-between px-4 pt-4 pb-2">
        <span className="text-sm font-bold">Activity</span>
        <div className="flex gap-1">
          {(["week", "month", "year"] as const).map((v) => (
            <button
              key={v}
              onClick={() => setView(v)}
              className={`rounded-md border-2 px-2.5 py-1 text-xs font-bold transition-all ${
                view === v
                  ? "border-border bg-primary text-primary-foreground shadow-[2px_2px_0_var(--nb-shadow)]"
                  : "text-muted-foreground hover:text-foreground border-transparent"
              }`}
            >
              {v === "year" ? "Year" : v === "month" ? "Month" : "Week"}
            </button>
          ))}
        </div>
      </div>

      <div className="px-4 pb-2">
        {view === "year" && (
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
        )}

        {view === "month" && (
          <div>
            <div className="mb-1 grid grid-cols-7 gap-[3px]">
              {WEEKDAY_LABELS.map((label) => (
                <div
                  key={label}
                  className="text-muted-foreground py-0.5 text-center text-xs font-medium"
                >
                  {label}
                </div>
              ))}
            </div>
            <div className="grid grid-cols-7 gap-[3px]">
              {monthDays.map((day, i) => {
                if (!day) return <div key={`pad-${i}`} />
                if (day.isFuture)
                  return (
                    <div
                      key={day.dateStr}
                      className="border-border/30 aspect-square rounded-sm border border-dashed"
                    />
                  )
                const count = (day.daily?.request_success ?? 0) + (day.daily?.request_failed ?? 0)
                const level = getActivityLevel(count)
                const isSelected = selectedMonthDay?.dateStr === day.dateStr
                return (
                  <div
                    key={day.dateStr}
                    className={`aspect-square cursor-pointer rounded-sm transition-transform hover:scale-125 ${isSelected ? "ring-foreground ring-2 ring-offset-1" : ""}`}
                    style={{ backgroundColor: LEVEL_COLORS[level] }}
                    onClick={() => setSelectedMonthDay(isSelected ? null : day)}
                    onMouseEnter={(e) =>
                      handleMouseEnter(e, { label: day.displayDate, metrics: day.daily })
                    }
                    onMouseLeave={handleMouseLeave}
                  />
                )
              })}
            </div>
            {selectedMonthDay && !selectedMonthDay.isFuture && (
              <div className="mt-3 flex gap-1">
                <div className="flex flex-col gap-[3px]">
                  {Array.from({ length: 24 }, (_, h) => (
                    <div
                      key={h}
                      className="text-muted-foreground flex h-[14px] items-center text-[10px] leading-none tabular-nums"
                    >
                      {h % 3 === 0 ? `${h.toString().padStart(2, "0")}` : ""}
                    </div>
                  ))}
                </div>
                <div className="flex-1">
                  <div className="text-muted-foreground mb-0.5 text-[10px] font-medium">
                    {selectedMonthDay.displayDate}
                  </div>
                  <div className="flex flex-col gap-[3px]">
                    {Array.from({ length: 24 }, (_, h) => {
                      const now = new Date()
                      const todayStr = `${now.getFullYear()}${(now.getMonth() + 1).toString().padStart(2, "0")}${now.getDate().toString().padStart(2, "0")}`
                      const isFutureHour =
                        selectedMonthDay.dateStr === todayStr && h > now.getHours()
                      if (isFutureHour) {
                        return (
                          <div
                            key={h}
                            className="border-border/30 h-[14px] rounded-sm border border-dashed"
                          />
                        )
                      }
                      const hourly = monthHourlyMap.get(selectedMonthDay.dateStr)?.get(h)
                      const hCount = (hourly?.request_success ?? 0) + (hourly?.request_failed ?? 0)
                      const hLevel = getActivityLevel(hCount)
                      return (
                        <div
                          key={h}
                          className="h-[14px] cursor-pointer rounded-sm transition-transform hover:scale-110"
                          style={{ backgroundColor: LEVEL_COLORS[hLevel] }}
                          onClick={() => navigateToHour(selectedMonthDay.dateStr, h)}
                          onMouseEnter={(e) =>
                            handleMouseEnter(e, {
                              label: `${selectedMonthDay.displayDate} ${h.toString().padStart(2, "0")}:00`,
                              metrics: hourly ?? null,
                            })
                          }
                          onMouseLeave={handleMouseLeave}
                        />
                      )
                    })}
                  </div>
                </div>
              </div>
            )}
          </div>
        )}

        {view === "week" && (
          <div className="flex gap-1">
            {/* Hour labels */}
            <div className="flex flex-col gap-[3px] pt-[18px]">
              {Array.from({ length: 24 }, (_, h) => (
                <div
                  key={h}
                  className="text-muted-foreground flex h-[14px] items-center text-[10px] leading-none tabular-nums"
                >
                  {h % 3 === 0 ? `${h.toString().padStart(2, "0")}` : ""}
                </div>
              ))}
            </div>
            {/* Grid */}
            <div className="flex-1">
              <div className="mb-0.5 grid grid-cols-7 gap-[3px]">
                {weekDays.map((day) => (
                  <div
                    key={day.dateStr}
                    className="text-muted-foreground text-center text-[10px] font-medium"
                  >
                    {
                      WEEKDAY_LABELS[
                        new Date(
                          Number.parseInt(day.dateStr.slice(0, 4)),
                          Number.parseInt(day.dateStr.slice(4, 6)) - 1,
                          Number.parseInt(day.dateStr.slice(6, 8)),
                        ).getDay()
                      ]
                    }
                  </div>
                ))}
              </div>
              <div
                className="grid gap-[3px]"
                style={{
                  gridTemplateColumns: "repeat(7, 1fr)",
                  gridTemplateRows: "repeat(24, 1fr)",
                }}
              >
                {Array.from({ length: 24 }, (_, h) =>
                  weekDays.map((day) => {
                    const now = new Date()
                    const isFutureHour =
                      day.isFuture ||
                      (day.dateStr ===
                        `${now.getFullYear()}${(now.getMonth() + 1).toString().padStart(2, "0")}${now.getDate().toString().padStart(2, "0")}` &&
                        h > now.getHours())
                    if (isFutureHour) {
                      return (
                        <div
                          key={`${day.dateStr}-${h}`}
                          className="border-border/30 h-[14px] rounded-sm border border-dashed"
                        />
                      )
                    }
                    const hourly = weekHourlyMap.get(day.dateStr)?.get(h)
                    const count = (hourly?.request_success ?? 0) + (hourly?.request_failed ?? 0)
                    const level = getActivityLevel(count)
                    return (
                      <div
                        key={`${day.dateStr}-${h}`}
                        className="h-[14px] cursor-pointer rounded-sm transition-transform hover:scale-125"
                        style={{ backgroundColor: LEVEL_COLORS[level] }}
                        onClick={() => navigateToHour(day.dateStr, h)}
                        onMouseEnter={(e) =>
                          handleMouseEnter(e, {
                            label: `${day.displayDate} ${h.toString().padStart(2, "0")}:00`,
                            metrics: hourly ?? null,
                          })
                        }
                        onMouseLeave={handleMouseLeave}
                      />
                    )
                  }),
                )}
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Legend */}
      <div className="text-muted-foreground flex items-center justify-end gap-1 px-4 pb-3 text-xs">
        <span>Less</span>
        {LEVEL_COLORS.map((c) => (
          <div key={c} className="h-3 w-3 rounded-sm" style={{ backgroundColor: c }} />
        ))}
        <span>More</span>
      </div>

      {/* Floating tooltip */}
      {activeTooltip && (
        <FloatingPortal>
          <div
            ref={refs.setFloating}
            style={floatingStyles}
            className="bg-popover text-popover-foreground border-border pointer-events-none z-50 w-fit min-w-max rounded-md border-2 p-3 text-sm shadow-[2px_2px_0_var(--nb-shadow)]"
          >
            <p className="mb-1 font-bold">{activeTooltip.label}</p>
            {activeTooltip.metrics ? (
              <div className="text-muted-foreground grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
                <span>Requests</span>
                <span className="text-foreground text-right font-medium">
                  {
                    formatCount(
                      activeTooltip.metrics.request_success + activeTooltip.metrics.request_failed,
                    ).value
                  }
                  {
                    formatCount(
                      activeTooltip.metrics.request_success + activeTooltip.metrics.request_failed,
                    ).unit
                  }
                </span>
                <span>Input Tokens</span>
                <span className="text-foreground text-right font-medium">
                  {formatCount(activeTooltip.metrics.input_token).value}
                  {formatCount(activeTooltip.metrics.input_token).unit}
                </span>
                <span>Output Tokens</span>
                <span className="text-foreground text-right font-medium">
                  {formatCount(activeTooltip.metrics.output_token).value}
                  {formatCount(activeTooltip.metrics.output_token).unit}
                </span>
                <span>Cost</span>
                <span className="text-foreground text-right font-medium">
                  {
                    formatMoney(
                      activeTooltip.metrics.input_cost + activeTooltip.metrics.output_cost,
                    ).value
                  }
                </span>
              </div>
            ) : (
              <p className="text-muted-foreground">No data</p>
            )}
          </div>
        </FloatingPortal>
      )}
    </Card>
  )
}

// ───────────── Cost Trend Chart ─────────────

const PERIODS = ["1", "7", "30"] as const

function ChartSection({
  dailyData,
  hourlyData,
}: {
  dailyData?: StatsDaily[]
  hourlyData?: StatsHourly[]
}) {
  const [period, setPeriod] = useState<(typeof PERIODS)[number]>("1")

  const sortedDaily = useMemo(() => {
    if (!dailyData) return []
    return [...dailyData].sort((a, b) => a.date.localeCompare(b.date))
  }, [dailyData])

  const chartData = useMemo(() => {
    if (period === "1") {
      if (!hourlyData) return []
      return hourlyData.map((s) => ({
        date: `${s.hour}:00`,
        cost: s.input_cost + s.output_cost,
      }))
    }
    const days = Number.parseInt(period)
    return sortedDaily.slice(-days).map((s) => ({
      date: s.date.replace(/(\d{4})(\d{2})(\d{2})/, "$2/$3"),
      cost: s.input_cost + s.output_cost,
    }))
  }, [sortedDaily, hourlyData, period])

  const totals = useMemo(() => {
    if (period === "1") {
      if (!hourlyData) return { requests: 0, cost: 0, inputTokens: 0, outputTokens: 0 }
      return {
        requests: hourlyData.reduce((a, s) => a + s.request_success + s.request_failed, 0),
        cost: hourlyData.reduce((a, s) => a + s.input_cost + s.output_cost, 0),
        inputTokens: hourlyData.reduce((a, s) => a + s.input_token, 0),
        outputTokens: hourlyData.reduce((a, s) => a + s.output_token, 0),
      }
    }
    const days = Number.parseInt(period)
    const recent = sortedDaily.slice(-days)
    return {
      requests: recent.reduce((a, s) => a + s.request_success + s.request_failed, 0),
      cost: recent.reduce((a, s) => a + s.input_cost + s.output_cost, 0),
      inputTokens: recent.reduce((a, s) => a + s.input_token, 0),
      outputTokens: recent.reduce((a, s) => a + s.output_token, 0),
    }
  }, [sortedDaily, hourlyData, period])

  const periodLabel: Record<string, string> = {
    "1": "Today",
    "7": "Last 7 Days",
    "30": "Last 30 Days",
  }

  const handlePeriodClick = () => {
    const idx = PERIODS.indexOf(period)
    setPeriod(PERIODS[(idx + 1) % PERIODS.length])
  }

  return (
    <Card>
      <CardHeader className="flex flex-col gap-4 pb-2 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex min-w-0 flex-wrap gap-x-6 gap-y-2">
          <div>
            <p className="text-muted-foreground text-xs">Total Requests</p>
            <p className="text-xl font-semibold">
              <AnimatedNumber value={totals.requests} formatter={(n) => formatCount(n).value} />
              <span className="text-muted-foreground ml-0.5 text-sm">
                {formatCount(totals.requests).unit}
              </span>
            </p>
          </div>
          <div className="bg-border hidden w-px self-stretch sm:block" />
          <div>
            <p className="text-muted-foreground text-xs">Input Tokens</p>
            <p className="text-xl font-semibold">
              <AnimatedNumber value={totals.inputTokens} formatter={(n) => formatCount(n).value} />
              <span className="text-muted-foreground ml-0.5 text-sm">
                {formatCount(totals.inputTokens).unit}
              </span>
            </p>
          </div>
          <div className="bg-border hidden w-px self-stretch sm:block" />
          <div>
            <p className="text-muted-foreground text-xs">Output Tokens</p>
            <p className="text-xl font-semibold">
              <AnimatedNumber value={totals.outputTokens} formatter={(n) => formatCount(n).value} />
              <span className="text-muted-foreground ml-0.5 text-sm">
                {formatCount(totals.outputTokens).unit}
              </span>
            </p>
          </div>
          <div className="bg-border hidden w-px self-stretch sm:block" />
          <div>
            <p className="text-muted-foreground text-xs">Total Cost</p>
            <p className="text-xl font-semibold">
              <AnimatedNumber value={totals.cost} formatter={(n) => formatMoney(n).value} />
              <span className="text-muted-foreground ml-0.5 text-sm">
                {formatMoney(totals.cost).unit}
              </span>
            </p>
          </div>
        </div>
        <button
          className="shrink-0 cursor-pointer text-left transition-opacity hover:opacity-80 sm:text-right"
          onClick={handlePeriodClick}
        >
          <p className="text-muted-foreground text-xs">Period</p>
          <p className="text-xl font-semibold">{periodLabel[period]}</p>
        </button>
      </CardHeader>
      <CardContent className="pb-2">
        <ResponsiveContainer width="100%" height={160}>
          <AreaChart data={chartData} margin={{ left: 10 }}>
            <defs>
              <linearGradient id="fillCost" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="var(--primary)" stopOpacity={0.8} />
                <stop offset="95%" stopColor="var(--primary)" stopOpacity={0.1} />
              </linearGradient>
            </defs>
            <CartesianGrid strokeDasharray="3 3" vertical={false} className="stroke-border" />
            <XAxis dataKey="date" tickLine={false} axisLine={false} className="text-xs" />
            <YAxis
              tickLine={false}
              axisLine={false}
              className="text-xs"
              tickFormatter={(v: number) => {
                const f = formatMoney(v)
                return `${f.value}${f.unit}`
              }}
            />
            <RechartsTooltip
              formatter={(v: unknown) => [formatMoney(Number(v ?? 0)).value, "Cost"]}
              labelClassName="text-foreground"
            />
            <Area type="monotone" dataKey="cost" stroke="var(--primary)" fill="url(#fillCost)" />
          </AreaChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  )
}

// ───────────── Channel Ranking ─────────────

function RankSection({ data }: { data?: ChannelStatsRow[] }) {
  const rankedByCost = useMemo(() => {
    if (!data) return []
    return [...data].sort((a, b) => b.totalCost - a.totalCost)
  }, [data])

  const rankedByCount = useMemo(() => {
    if (!data) return []
    return [...data].sort((a, b) => b.totalRequests - a.totalRequests)
  }, [data])

  const medal = (rank: number) => {
    if (rank === 1) return "\u{1F947}"
    if (rank === 2) return "\u{1F948}"
    if (rank === 3) return "\u{1F949}"
    return String(rank)
  }

  const renderList = (channels: ChannelStatsRow[], mode: "cost" | "count") => {
    if (channels.length === 0) {
      return (
        <div className="text-muted-foreground flex flex-col items-center justify-center py-8">
          <TrendingUp className="mb-3 h-12 w-12 opacity-30" />
          <p className="text-sm">No data yet</p>
        </div>
      )
    }
    return (
      <div className="max-h-[300px] space-y-2 overflow-y-auto">
        <LayoutGroup>
          {channels.map((ch, index) => {
            const rank = index + 1
            const successRate =
              ch.totalRequests > 0 ? (ch.request_success / ch.totalRequests) * 100 : 0
            return (
              <motion.div
                key={ch.channelId}
                layout
                transition={{ type: "spring", stiffness: 300, damping: 30 }}
                className="hover:bg-muted/50 flex items-center gap-3 rounded-lg p-3 transition-colors"
              >
                <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-lg font-bold">
                  {medal(rank)}
                </div>
                <div className="min-w-0 flex-1">
                  <Link
                    href={`/logs?channel=${ch.channelId}`}
                    className="block truncate text-sm font-medium hover:underline"
                  >
                    {ch.channelName || `Channel ${ch.channelId}`}
                  </Link>
                  <div className="text-muted-foreground mt-0.5 flex items-center gap-2 text-xs">
                    {mode === "count" && <span>Success: {successRate.toFixed(1)}%</span>}
                    <span>
                      In: {formatCount(ch.input_token).value}
                      {formatCount(ch.input_token).unit}
                    </span>
                    <span>
                      Out: {formatCount(ch.output_token).value}
                      {formatCount(ch.output_token).unit}
                    </span>
                  </div>
                </div>
                <div className="shrink-0 text-right">
                  {mode === "count" ? (
                    <div className="flex items-center gap-1 text-sm font-medium tabular-nums">
                      <span className="text-green-600 dark:text-green-400">
                        {formatCount(ch.request_success).value}
                        <span className="text-muted-foreground text-xs">
                          {formatCount(ch.request_success).unit}
                        </span>
                      </span>
                      <span className="text-muted-foreground/40">/</span>
                      <span className="text-red-600 dark:text-red-400">
                        {formatCount(ch.request_failed).value}
                        <span className="text-muted-foreground text-xs">
                          {formatCount(ch.request_failed).unit}
                        </span>
                      </span>
                    </div>
                  ) : (
                    <span className="font-semibold">
                      {formatMoney(ch.totalCost).value}
                      <span className="text-muted-foreground ml-0.5 text-xs">
                        {formatMoney(ch.totalCost).unit}
                      </span>
                    </span>
                  )}
                </div>
              </motion.div>
            )
          })}
        </LayoutGroup>
      </div>
    )
  }

  return (
    <Card>
      <Tabs defaultValue="cost">
        <CardHeader className="flex flex-row items-center justify-between pb-2">
          <CardTitle className="text-base">Channel Ranking</CardTitle>
          <TabsList>
            <TabsTrigger value="cost">By Cost</TabsTrigger>
            <TabsTrigger value="count">By Count</TabsTrigger>
          </TabsList>
        </CardHeader>
        <CardContent>
          <TabsContent value="cost" className="mt-0">
            {renderList(rankedByCost, "cost")}
          </TabsContent>
          <TabsContent value="count" className="mt-0">
            {renderList(rankedByCount, "count")}
          </TabsContent>
        </CardContent>
      </Tabs>
    </Card>
  )
}

// ───────────── Model Usage Stats ─────────────

type ModelSortKey = "requests" | "tokens" | "cost" | "latency"

function ModelStatsSection({ data }: { data?: ModelStatsItem[] }) {
  const [sortBy, setSortBy] = useState<ModelSortKey>("requests")

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

  const sortOptions: { key: ModelSortKey; label: string }[] = [
    { key: "requests", label: "Requests" },
    { key: "tokens", label: "Tokens" },
    { key: "cost", label: "Cost" },
    { key: "latency", label: "Latency" },
  ]

  return (
    <Card>
      <Tabs value={sortBy} onValueChange={(v) => setSortBy(v as ModelSortKey)}>
        <CardHeader className="flex flex-col gap-2 pb-2">
          <div className="flex items-center justify-between">
            <CardTitle className="text-base">Model Usage</CardTitle>
          </div>
          <TabsList>
            {sortOptions.map((o) => (
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
              <p className="text-sm">No data yet</p>
            </div>
          ) : (
            <div className="max-h-[400px] space-y-1.5 overflow-y-auto">
              {sorted.map((item) => (
                <Link
                  key={item.model}
                  href={`/logs?model=${encodeURIComponent(item.model)}`}
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
                        <span className="text-muted-foreground">Req </span>
                        <span className="font-medium">
                          {formatCount(item.requestCount).value}
                          {formatCount(item.requestCount).unit}
                        </span>
                      </div>
                      <div>
                        <span className="text-muted-foreground">In </span>
                        <span className="font-medium">
                          {formatCount(item.inputTokens).value}
                          {formatCount(item.inputTokens).unit}
                        </span>
                      </div>
                      <div>
                        <span className="text-muted-foreground">Out </span>
                        <span className="font-medium">
                          {formatCount(item.outputTokens).value}
                          {formatCount(item.outputTokens).unit}
                        </span>
                      </div>
                      <div>
                        <span className="text-muted-foreground">Cost </span>
                        <span className="font-medium">{formatMoney(item.totalCost).value}</span>
                      </div>
                      <div>
                        <span className="text-muted-foreground">Avg </span>
                        <span className="font-medium">
                          {formatTime(item.avgLatency).value}
                          {formatTime(item.avgLatency).unit}
                        </span>
                      </div>
                    </div>
                  </div>
                </Link>
              ))}
            </div>
          )}
        </CardContent>
      </Tabs>
    </Card>
  )
}

// ───────────── Page ─────────────

export default function DashboardPage() {
  const { data: totalData } = useQuery({
    queryKey: ["stats", "total"],
    queryFn: getTotalStats,
  })

  const { data: dailyData } = useQuery({
    queryKey: ["stats", "daily"],
    queryFn: getDailyStats,
  })

  const { data: hourlyData } = useQuery({
    queryKey: ["stats", "hourly"],
    queryFn: () => getHourlyStats(),
  })

  const { data: channelData } = useQuery({
    queryKey: ["stats", "channel"],
    queryFn: getChannelStats,
  })

  const { data: modelData } = useQuery({
    queryKey: ["stats", "model"],
    queryFn: getModelStats,
  })

  return (
    <div className="flex flex-col gap-6">
      <TotalSection data={totalData?.data} />
      <ActivitySection data={dailyData?.data} hourlyData={hourlyData?.data} />
      <ChartSection dailyData={dailyData?.data} hourlyData={hourlyData?.data} />
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <ModelStatsSection data={modelData?.data} />
        <RankSection data={channelData?.data} />
      </div>
    </div>
  )
}
