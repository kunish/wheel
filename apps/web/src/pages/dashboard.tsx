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
import { lazy, Suspense, useCallback, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { AnimatedNumber } from "@/components/animated-number"
import { ModelBadge } from "@/components/model-badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  getChannelStats,
  getDailyStats,
  getHourlyStats,
  getModelStats,
  getTotalStats,
} from "@/lib/api"
import { formatCount, formatMoney, formatTime } from "@/lib/format"

const LazyChartSection = lazy(() => import("@/components/chart-section"))

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

function TotalSection({ data, isLoading }: { data?: StatsMetrics; isLoading?: boolean }) {
  const { t } = useTranslation("dashboard")

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
            raw: (data?.input_token ?? 0) + (data?.output_token ?? 0),
            format: formatCount,
            icon: Bot,
            bg: "bg-emerald-500/10",
          },
          {
            label: t("stats.totalCost"),
            raw: (data?.input_cost ?? 0) + (data?.output_cost ?? 0),
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
            raw: data?.input_token ?? 0,
            format: formatCount,
            icon: Bot,
            bg: "bg-orange-500/10",
          },
          {
            label: t("stats.inputCost"),
            raw: data?.input_cost ?? 0,
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
            raw: data?.output_token ?? 0,
            format: formatCount,
            icon: Bot,
            bg: "bg-violet-500/10",
          },
          {
            label: t("stats.outputCost"),
            raw: data?.output_cost ?? 0,
            format: formatMoney,
            icon: DollarSign,
            bg: "bg-violet-500/10",
          },
        ],
      },
    ],
    [t, data],
  )

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
                    {isLoading ? (
                      <Skeleton className="mt-1 h-5 w-16" />
                    ) : (
                      <span className="text-lg leading-tight font-bold tabular-nums">
                        <AnimatedNumber value={item.raw} formatter={(n) => item.format(n).value} />
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

function ActivitySection({ data }: { data?: StatsDaily[] }) {
  const { t } = useTranslation("dashboard")
  const { t: tc } = useTranslation("common")
  const [view, setViewRaw] = useState<HeatmapView>(getStoredView)
  const setView = useCallback((v: HeatmapView) => {
    setViewRaw(v)
    try {
      localStorage.setItem(HEATMAP_VIEW_KEY, v)
    } catch {}
  }, [])
  const [activeTooltip, setActiveTooltip] = useState<HeatmapTooltip | null>(null)
  const [selectedDateStr, setSelectedDateStr] = useState<string | null>(null)
  const navigate = useNavigate()

  const [today] = useState(() => new Date())

  const { refs, floatingStyles } = useFloating({
    placement: "top",
    open: !!activeTooltip,
    middleware: [offset(8), flip(), shift({ padding: 8 })],
    whileElementsMounted: autoUpdate,
  })

  const dataMap = useMemo(() => new Map((data ?? []).map((d) => [d.date, d])), [data])

  const weekdayLabels = useMemo(
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
  const yearDays = useMemo(() => {
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
                  <button
                    onClick={() => navigateToDay(selectedDayDateStr)}
                    className="text-muted-foreground hover:text-foreground ml-auto text-xs font-medium underline-offset-2 hover:underline"
                  >
                    {t("activity.viewLogs")}
                  </button>
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

                {/* 24-hour heatmap grid */}
                <div className="flex gap-1">
                  <div className="flex flex-col gap-[3px]">
                    {Array.from({ length: 24 }, (_, h) => (
                      <div
                        key={h}
                        className="text-muted-foreground flex h-5 items-center text-[10px] leading-none tabular-nums"
                      >
                        {h.toString().padStart(2, "0")}
                      </div>
                    ))}
                  </div>
                  <div className="flex flex-1 flex-col gap-[3px]">
                    {Array.from({ length: 24 }, (_, h) => {
                      const isFutureHour = isToday && h > nowHour
                      if (isFutureHour) {
                        return (
                          <div
                            key={h}
                            className="border-border/30 h-5 rounded-sm border border-dashed"
                          />
                        )
                      }
                      const hourly = dayHourlyMap.get(h)
                      const hCount = (hourly?.request_success ?? 0) + (hourly?.request_failed ?? 0)
                      const hLevel = getActivityLevel(hCount)
                      return (
                        <div
                          key={h}
                          className="h-5 cursor-pointer rounded-sm transition-transform hover:scale-[1.02]"
                          style={{ backgroundColor: LEVEL_COLORS[hLevel] }}
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
                  </div>
                </div>
              </div>
            )
          })()}

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
              {weekdayLabels.map((label) => (
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
                // eslint-disable-next-line react/no-array-index-key -- padding cells for calendar alignment have no stable identity
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
                return (
                  <div
                    key={day.dateStr}
                    className="aspect-square cursor-pointer rounded-sm transition-transform hover:scale-125"
                    style={{ backgroundColor: LEVEL_COLORS[level] }}
                    onClick={() => drillIntoDay(day.dateStr)}
                    onMouseEnter={(e) =>
                      handleMouseEnter(e, { label: day.displayDate, metrics: day.daily })
                    }
                    onMouseLeave={handleMouseLeave}
                  />
                )
              })}
            </div>
          </div>
        )}

        {view === "week" && (
          <div>
            <div className="mb-1 grid grid-cols-7 gap-[3px]">
              {weekDays.map((day) => (
                <div
                  key={day.dateStr}
                  className="text-muted-foreground py-0.5 text-center text-xs font-medium"
                >
                  {
                    weekdayLabels[
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
            <div className="grid grid-cols-7 gap-[3px]">
              {weekDays.map((day) => renderCell(day, day.dateStr))}
            </div>
          </div>
        )}
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

export default function DashboardPage() {
  const { t } = useTranslation("dashboard")

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
          <TotalSection data={totalData?.data} />
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
          <ModelStatsSection data={modelData?.data} />
        )}
        {isChannelError ? (
          <InlineError message={t("errors.channelStats")} onRetry={() => refetchChannel()} />
        ) : (
          <RankSection data={channelData?.data} />
        )}
      </div>
    </div>
  )
}
