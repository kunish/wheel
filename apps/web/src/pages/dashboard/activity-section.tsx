import type { DayData, HeatmapTooltip, HeatmapView, StreamingDelta } from "./types"
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
  ArrowDownToLine,
  ArrowUpFromLine,
  Bot,
  DollarSign,
  MessageSquare,
  Radio,
  TrendingUp,
} from "lucide-react"
import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { Area, AreaChart, ResponsiveContainer } from "recharts"
import { AnimatedNumber } from "@/components/animated-number"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { getHourlyStats } from "@/lib/api"
import { formatCount, formatMoney, formatTime } from "@/lib/format"
import { HeroGearClock } from "./hero-gear-clock"
import { PowerPipeline } from "./power-pipeline"
import { ReactorGrid } from "./reactor-grid"
import {
  buildDayData,
  getActivityLevel,
  getFirstDayOfWeek,
  getStoredView,
  HEATMAP_VIEW_KEY,
  LEVEL_COLORS,
  toDateStr,
  useGearRotation,
} from "./types"

// ───────────── Compact inline stats ─────────────

/** Measures its container and renders children at min(width, height) square size */
function GearClockFit({ children }: { children: React.ReactNode }) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [size, setSize] = useState(0)

  useEffect(() => {
    const el = containerRef.current
    if (!el) return
    const ro = new ResizeObserver(([entry]) => {
      const { width, height } = entry.contentRect
      setSize(Math.min(width, height))
    })
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  return (
    <div ref={containerRef} className="flex min-h-0 flex-1 items-center justify-center">
      <div style={{ width: size, height: size }}>{children}</div>
    </div>
  )
}

function InlineStats({
  items,
}: {
  items: { label: string; raw: number; format: typeof formatCount; icon: typeof MessageSquare }[]
}) {
  return (
    <div className="flex flex-wrap items-center justify-center gap-x-4 gap-y-1.5 py-2">
      {items.map((item) => {
        const formatted = item.format(item.raw)
        return (
          <div key={item.label} className="flex items-center gap-1.5">
            <item.icon className="text-muted-foreground h-3 w-3 shrink-0" />
            <span className="text-muted-foreground text-[11px]">{item.label}</span>
            <span className="text-xs font-bold tabular-nums">
              <AnimatedNumber value={item.raw} formatter={(n) => item.format(n).value} />
              {formatted.unit && (
                <span className="text-muted-foreground ml-0.5 text-[10px] font-medium">
                  {formatted.unit}
                </span>
              )}
            </span>
          </div>
        )
      })}
    </div>
  )
}

// ───────────── Data panel tab type ─────────────

type DataTab = "cost" | "models" | "channels"

type ModelSortKey = "requests" | "tokens" | "cost" | "latency"
type ChannelSortKey = "requests" | "cost" | "latency"

// ───────────── Hub panels ─────────────

function HubCostChart({
  dailyData,
  hourlyData,
}: {
  dailyData?: StatsDaily[]
  hourlyData?: StatsHourly[]
}) {
  const sortedDaily = useMemo(() => {
    if (!dailyData) return []
    return [...dailyData].sort((a, b) => a.date.localeCompare(b.date))
  }, [dailyData])

  const chartData = useMemo(() => {
    if (hourlyData && hourlyData.length > 0) {
      return hourlyData.map((s) => ({
        date: `${s.hour}h`,
        cost: s.input_cost + s.output_cost,
      }))
    }
    return sortedDaily.slice(-7).map((s) => ({
      date: `${s.date.slice(4, 6)}/${s.date.slice(6)}`,
      cost: s.input_cost + s.output_cost,
    }))
  }, [sortedDaily, hourlyData])

  // Compute period totals for summary
  const periodTotals = useMemo(() => {
    const source = hourlyData && hourlyData.length > 0 ? hourlyData : sortedDaily.slice(-7)
    let req = 0
    let inTok = 0
    let outTok = 0
    let cost = 0
    for (const s of source) {
      req += (s.request_success ?? 0) + (s.request_failed ?? 0)
      inTok += s.input_token ?? 0
      outTok += s.output_token ?? 0
      cost += (s.input_cost ?? 0) + (s.output_cost ?? 0)
    }
    return { req, inTok, outTok, cost }
  }, [hourlyData, sortedDaily])

  return (
    <div className="flex flex-col gap-1.5">
      {/* Summary metrics row */}
      <div className="flex justify-between text-[9px] tabular-nums">
        {[
          { label: "REQ", value: formatCount(periodTotals.req) },
          { label: "IN", value: formatCount(periodTotals.inTok) },
          { label: "OUT", value: formatCount(periodTotals.outTok) },
          { label: "$", value: formatMoney(periodTotals.cost) },
        ].map((m) => (
          <div key={m.label} className="flex flex-col items-center">
            <span className="text-muted-foreground text-[7px] font-bold">{m.label}</span>
            <span className="font-bold">
              {m.value.value}
              {m.value.unit && (
                <span className="text-muted-foreground ml-px text-[7px]">{m.value.unit}</span>
              )}
            </span>
          </div>
        ))}
      </div>
      <ResponsiveContainer width="100%" height={60}>
        <AreaChart data={chartData} margin={{ left: 0, right: 0, top: 2, bottom: 0 }}>
          <defs>
            <linearGradient id="fillCostMini" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="var(--primary)" stopOpacity={0.6} />
              <stop offset="95%" stopColor="var(--primary)" stopOpacity={0.05} />
            </linearGradient>
          </defs>
          <Area
            type="monotone"
            dataKey="cost"
            stroke="var(--primary)"
            strokeWidth={1.5}
            fill="url(#fillCostMini)"
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  )
}

function HubModelList({ data }: { data?: ModelStatsItem[] }) {
  const { t } = useTranslation("dashboard")
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
    const first = sorted[0]
    switch (sortBy) {
      case "requests":
        return first.requestCount || 1
      case "tokens":
        return first.inputTokens + first.outputTokens || 1
      case "cost":
        return first.totalCost || 1
      case "latency":
        return first.avgLatency || 1
    }
  }, [sorted, sortBy])

  const barPct = (item: ModelStatsItem) => {
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

  if (!data || data.length === 0) {
    return <p className="text-muted-foreground text-center text-[9px]">{t("modelUsage.noData")}</p>
  }

  return (
    <div className="flex flex-col gap-1">
      {/* Sort tabs */}
      <div className="flex gap-0.5">
        {(["requests", "tokens", "cost", "latency"] as const).map((key) => (
          <button
            key={key}
            onClick={() => setSortBy(key)}
            className={`rounded px-1 py-px text-[7px] font-bold transition-all ${
              sortBy === key
                ? "bg-foreground/10 text-foreground"
                : "text-muted-foreground/50 hover:text-muted-foreground"
            }`}
          >
            {key === "requests" ? "REQ" : key === "tokens" ? "TOK" : key === "cost" ? "$" : "LAT"}
          </button>
        ))}
      </div>
      {/* Model rows */}
      <div className="max-h-[90px] space-y-px overflow-y-auto">
        {sorted.map((item) => (
          <Link
            key={item.model}
            to={`/logs?model=${encodeURIComponent(item.model)}`}
            className="hover:bg-muted/40 relative block rounded px-1 py-0.5 transition-colors"
          >
            {/* Background bar */}
            <div
              className="absolute inset-y-0 left-0 rounded opacity-[0.07]"
              style={{ width: `${barPct(item)}%`, backgroundColor: "var(--primary)" }}
            />
            <div className="relative flex items-center justify-between gap-1">
              <span className="min-w-0 truncate text-[9px] font-medium">
                {item.model.split("/").pop()}
              </span>
              <span className="text-muted-foreground flex shrink-0 gap-1.5 text-[7px] tabular-nums">
                <span>{formatCount(item.requestCount).value}r</span>
                <span>{formatMoney(item.totalCost).value}</span>
                <span>
                  {formatTime(item.avgLatency).value}
                  {formatTime(item.avgLatency).unit}
                </span>
              </span>
            </div>
          </Link>
        ))}
      </div>
    </div>
  )
}

function HubChannelList({ data }: { data?: ChannelStatsRow[] }) {
  const { t } = useTranslation("dashboard")
  const [sortBy, setSortBy] = useState<ChannelSortKey>("requests")

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

  const barPct = (ch: ChannelStatsRow) => {
    switch (sortBy) {
      case "requests":
        return ((ch.totalRequests || 0) / maxVal) * 100
      case "cost":
        return ((ch.totalCost || 0) / maxVal) * 100
      case "latency":
        return ((ch.avgLatency || 0) / maxVal) * 100
    }
  }

  if (!data || data.length === 0) {
    return (
      <p className="text-muted-foreground text-center text-[9px]">{t("channelRanking.noData")}</p>
    )
  }

  return (
    <div className="flex flex-col gap-1">
      {/* Sort tabs */}
      <div className="flex gap-0.5">
        {(["requests", "cost", "latency"] as const).map((key) => (
          <button
            key={key}
            onClick={() => setSortBy(key)}
            className={`rounded px-1 py-px text-[7px] font-bold transition-all ${
              sortBy === key
                ? "bg-foreground/10 text-foreground"
                : "text-muted-foreground/50 hover:text-muted-foreground"
            }`}
          >
            {key === "requests" ? "REQ" : key === "cost" ? "$" : "LAT"}
          </button>
        ))}
      </div>
      {/* Channel rows */}
      <div className="max-h-[90px] space-y-px overflow-y-auto">
        {sorted.map((ch) => (
          <Link
            key={ch.channelId}
            to={`/logs?channel=${ch.channelId}`}
            className="hover:bg-muted/40 relative block rounded px-1 py-0.5 transition-colors"
          >
            <div
              className="absolute inset-y-0 left-0 rounded opacity-[0.07]"
              style={{ width: `${barPct(ch)}%`, backgroundColor: "var(--primary)" }}
            />
            <div className="relative flex items-center justify-between gap-1">
              <span className="min-w-0 truncate text-[9px] font-medium">
                {ch.channelName || `#${ch.channelId}`}
              </span>
              <span className="text-muted-foreground flex shrink-0 gap-1.5 text-[7px] tabular-nums">
                <span>{formatCount(ch.totalRequests).value}r</span>
                <span>{formatMoney(ch.totalCost).value}</span>
                <span>
                  {formatTime(ch.avgLatency).value}
                  {formatTime(ch.avgLatency).unit}
                </span>
              </span>
            </div>
          </Link>
        ))}
      </div>
    </div>
  )
}

// ───────────── Data Panel Popover ─────────────

function DataPanelPopover({
  dataTab,
  setDataTab,
  dailyData,
  hourlyData,
  modelData,
  channelData,
}: {
  dataTab: DataTab | null
  setDataTab: React.Dispatch<React.SetStateAction<DataTab | null>>
  dailyData?: StatsDaily[]
  hourlyData?: StatsHourly[]
  modelData?: ModelStatsItem[]
  channelData?: ChannelStatsRow[]
}) {
  // Keep last active tab so content remains visible during close animation
  const lastTabRef = useRef<DataTab>("cost")
  if (dataTab !== null) lastTabRef.current = dataTab
  const displayTab = dataTab ?? lastTabRef.current

  return (
    <div>
      <Popover
        open={dataTab !== null}
        onOpenChange={(open) => {
          if (!open) setDataTab(null)
        }}
      >
        <PopoverTrigger asChild>
          <button
            onClick={() => setDataTab((prev) => (prev === null ? "cost" : null))}
            className={`rounded-md border-2 p-1 transition-all ${
              dataTab !== null
                ? "border-border bg-primary text-primary-foreground shadow-[2px_2px_0_var(--nb-shadow)]"
                : "text-muted-foreground hover:text-foreground border-transparent"
            }`}
          >
            <TrendingUp className="h-3.5 w-3.5" />
          </button>
        </PopoverTrigger>
        <PopoverContent side="top" align="center" className="w-[360px] p-4">
          {/* Tab switcher — icon only */}
          <div className="mb-3 flex gap-1">
            {[
              { key: "cost" as const, Icon: TrendingUp },
              { key: "models" as const, Icon: Bot },
              { key: "channels" as const, Icon: Radio },
            ].map(({ key, Icon }) => (
              <button
                key={key}
                onClick={() => setDataTab(key)}
                className={`rounded-md border-2 p-1.5 transition-all ${
                  displayTab === key
                    ? "border-border bg-primary text-primary-foreground shadow-[2px_2px_0_var(--nb-shadow)]"
                    : "text-muted-foreground hover:text-foreground border-transparent"
                }`}
              >
                <Icon className="h-4 w-4" />
              </button>
            ))}
          </div>
          {/* Panel content */}
          <div>
            {displayTab === "cost" && (
              <HubCostChart dailyData={dailyData} hourlyData={hourlyData} />
            )}
            {displayTab === "models" && <HubModelList data={modelData} />}
            {displayTab === "channels" && <HubChannelList data={channelData} />}
          </div>
        </PopoverContent>
      </Popover>
    </div>
  )
}

// ───────────── Nav Arrow ─────────────

function NavArrow({
  direction,
  onClick,
  disabled,
}: {
  direction: "left" | "right"
  onClick: () => void
  disabled?: boolean
}) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className="text-muted-foreground hover:text-foreground hover:bg-muted hover:border-border rounded-md border-2 border-transparent p-1 transition-colors disabled:cursor-not-allowed disabled:opacity-30 disabled:hover:border-transparent disabled:hover:bg-transparent"
    >
      <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
        <path
          d={direction === "left" ? "M10 4L6 8L10 12" : "M6 4L10 8L6 12"}
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    </button>
  )
}

// ───────────── Activity Section ─────────────

export function ActivitySection({
  data,
  totalData,
  streamingDelta,
  hourlyData,
  modelData,
  channelData,
}: {
  data?: StatsDaily[]
  totalData?: StatsMetrics
  streamingDelta?: StreamingDelta
  hourlyData?: StatsHourly[]
  modelData?: ModelStatsItem[]
  channelData?: ChannelStatsRow[]
}) {
  const { t, i18n } = useTranslation("dashboard")
  const { t: tc } = useTranslation("common")
  const gearAngle = useGearRotation()
  const firstDay = useMemo(() => getFirstDayOfWeek(i18n.language), [i18n.language])
  const [view, setView] = useState<HeatmapView>(getStoredView)
  useEffect(() => {
    try {
      localStorage.setItem(HEATMAP_VIEW_KEY, view)
    } catch {}
  }, [view])
  const [activeTooltip, setActiveTooltip] = useState<HeatmapTooltip | null>(null)
  const [selectedDateStr, setSelectedDateStr] = useState<string | null>(null)
  const [weekOffset, setWeekOffset] = useState(0)
  const [monthOffset, setMonthOffset] = useState(0)
  const [yearOffset, setYearOffset] = useState(0)
  const [dataTab, setDataTab] = useState<DataTab | null>(null)
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

  // ── Year view ──
  const yearAnchor = useMemo(() => {
    const d = new Date(today.getFullYear() + yearOffset, 11, 31)
    if (yearOffset === 0) return today
    return d
  }, [today, yearOffset])

  const yearLabel = useMemo(() => `${today.getFullYear() + yearOffset}`, [today, yearOffset])

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

  // ── Month view ──
  const monthAnchor = useMemo(
    () => new Date(today.getFullYear(), today.getMonth() + monthOffset, 1),
    [today, monthOffset],
  )

  const monthLabel = useMemo(
    () =>
      `${monthAnchor.getFullYear()}-${(monthAnchor.getMonth() + 1).toString().padStart(2, "0")}`,
    [monthAnchor],
  )

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

  // ── Week view ──
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

  // ── Day view ──
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
      if (s.date === selectedDayDateStr) map.set(s.hour, s)
    }
    return map
  }, [dayHourlyData, selectedDayDateStr])

  const selectedDayData = useMemo(
    () => dataMap.get(selectedDayDateStr) ?? null,
    [dataMap, selectedDayDateStr],
  )

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

  const gearData = useMemo(() => {
    const sd = streamingDelta ?? { inputTokens: 0, outputTokens: 0, inputCost: 0, outputCost: 0 }
    if (view === "day") {
      const dayMetrics = selectedDayData
      return {
        reqCount: dayMetrics
          ? (dayMetrics.request_success ?? 0) + (dayMetrics.request_failed ?? 0)
          : 0,
        inTokens: dayMetrics?.input_token ?? 0,
        outTokens: dayMetrics?.output_token ?? 0,
        totalCost: dayMetrics ? (dayMetrics.input_cost ?? 0) + (dayMetrics.output_cost ?? 0) : 0,
      }
    }
    return {
      reqCount: (totalData?.request_success ?? 0) + (totalData?.request_failed ?? 0),
      inTokens: (totalData?.input_token ?? 0) + sd.inputTokens,
      outTokens: (totalData?.output_token ?? 0) + sd.outputTokens,
      totalCost:
        (totalData?.input_cost ?? 0) + (totalData?.output_cost ?? 0) + sd.inputCost + sd.outputCost,
    }
  }, [view, selectedDayData, totalData, streamingDelta])

  const handleMouseEnter = useCallback(
    (e: React.MouseEvent, tooltip: HeatmapTooltip) => {
      refs.setReference(e.currentTarget)
      setActiveTooltip(tooltip)
    },
    [refs],
  )

  const handleMouseLeave = useCallback(() => setActiveTooltip(null), [])

  const drillIntoDay = useCallback((dateStr: string) => {
    setSelectedDateStr(dateStr)
    setView("day")
  }, [])

  const navigateToDay = useCallback(
    (dateStr: string) => {
      const y = Number.parseInt(dateStr.slice(0, 4))
      const m = Number.parseInt(dateStr.slice(4, 6)) - 1
      const d = Number.parseInt(dateStr.slice(6, 8))
      const from = Math.floor(new Date(y, m, d).getTime() / 1000)
      navigate(`/logs?from=${from}&to=${from + 86400 - 1}`)
    },
    [navigate],
  )

  const navigateToHour = useCallback(
    (dateStr: string, hour: number) => {
      const y = Number.parseInt(dateStr.slice(0, 4))
      const m = Number.parseInt(dateStr.slice(4, 6)) - 1
      const d = Number.parseInt(dateStr.slice(6, 8))
      const from = Math.floor(new Date(y, m, d, hour).getTime() / 1000)
      navigate(`/logs?from=${from}&to=${from + 3600 - 1}`)
    },
    [navigate],
  )

  const navigateToWeek = useCallback(() => {
    const from = Math.floor(weekStart.getTime() / 1000)
    const end = new Date(weekStart)
    end.setDate(end.getDate() + 7)
    navigate(`/logs?from=${from}&to=${Math.floor(end.getTime() / 1000) - 1}`)
  }, [weekStart, navigate])

  const navigateToMonth = useCallback(() => {
    const from = Math.floor(monthAnchor.getTime() / 1000)
    const end = new Date(monthAnchor.getFullYear(), monthAnchor.getMonth() + 1, 1)
    navigate(`/logs?from=${from}&to=${Math.floor(end.getTime() / 1000) - 1}`)
  }, [monthAnchor, navigate])

  const navigateToYear = useCallback(() => {
    const y = today.getFullYear() + yearOffset
    const from = Math.floor(new Date(y, 0, 1).getTime() / 1000)
    navigate(`/logs?from=${from}&to=${Math.floor(new Date(y + 1, 0, 1).getTime() / 1000) - 1}`)
  }, [today, yearOffset, navigate])

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
    <div className="flex min-h-0 flex-1 flex-col">
      {/* ── View tabs ── */}
      <div className="flex shrink-0 items-center justify-between px-2 pb-2">
        <span className="text-sm font-bold">{t("activity.title")}</span>
        <div className="flex items-center gap-1">
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

      {/* ── Day view: Gear Clock + day nav ── */}
      {view === "day" && (
        <div className="flex min-h-0 flex-1 flex-col">
          <GearClockFit>
            <HeroGearClock
              dayHourlyMap={dayHourlyMap}
              isToday={selectedDayDateStr === toDateStr(new Date())}
              nowHour={new Date().getHours()}
              now={new Date()}
              selectedDayDateStr={selectedDayDateStr}
              selectedDisplayDate={selectedDisplayDate}
              gearAngle={gearAngle}
              navigateToHour={navigateToHour}
              handleMouseEnter={handleMouseEnter}
              handleMouseLeave={handleMouseLeave}
            >
              <div className="flex flex-col items-center gap-1">
                {/* Primary metric: requests */}
                <div className="flex flex-col items-center">
                  <span className="text-muted-foreground text-[10px] font-bold tracking-[0.15em]">
                    REQ
                  </span>
                  <span className="text-2xl leading-none font-black tabular-nums">
                    <AnimatedNumber
                      value={gearData.reqCount}
                      formatter={(n) => formatCount(n).value}
                    />
                    {formatCount(gearData.reqCount).unit && (
                      <span className="text-muted-foreground ml-0.5 text-xs font-bold">
                        {formatCount(gearData.reqCount).unit}
                      </span>
                    )}
                  </span>
                </div>

                {/* Divider */}
                <div className="bg-border h-px w-10" />

                {/* Token flow: in / out */}
                <div className="text-muted-foreground flex items-center gap-2 text-[10px] tabular-nums">
                  <span className="flex items-center gap-0.5">
                    <ArrowDownToLine className="h-3 w-3 opacity-50" />
                    {formatCount(gearData.inTokens).value}
                    {formatCount(gearData.inTokens).unit}
                  </span>
                  <span className="flex items-center gap-0.5">
                    <ArrowUpFromLine className="h-3 w-3 opacity-50" />
                    {formatCount(gearData.outTokens).value}
                    {formatCount(gearData.outTokens).unit}
                  </span>
                </div>

                {/* Cost */}
                <span
                  className="text-xs font-bold tabular-nums"
                  style={{
                    color: "color-mix(in srgb, var(--nb-lime) 60%, var(--foreground))",
                  }}
                >
                  {formatMoney(gearData.totalCost).value}
                </span>

                {/* Data panel popover button */}
                <DataPanelPopover
                  dataTab={dataTab}
                  setDataTab={setDataTab}
                  dailyData={data}
                  hourlyData={hourlyData}
                  modelData={modelData}
                  channelData={channelData}
                />
              </div>
            </HeroGearClock>
          </GearClockFit>

          <div className="shrink-0 px-2">
            {(() => {
              const isToday = selectedDayDateStr === toDateStr(new Date())
              const isFutureDate = (() => {
                const d = new Date(
                  Number.parseInt(selectedDayDateStr.slice(0, 4)),
                  Number.parseInt(selectedDayDateStr.slice(4, 6)) - 1,
                  Number.parseInt(selectedDayDateStr.slice(6, 8)),
                )
                return d > today
              })()

              return (
                <div className="relative flex items-center gap-2">
                  <NavArrow direction="left" onClick={() => shiftDay(-1)} />
                  <div className="flex items-baseline gap-1.5">
                    <span className="text-sm font-bold">{selectedDisplayDate}</span>
                    <span className="text-muted-foreground text-[11px] font-medium">
                      {selectedDayWeekday}
                    </span>
                  </div>
                  <NavArrow
                    direction="right"
                    onClick={() => shiftDay(1)}
                    disabled={isFutureDate || isToday}
                  />

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
              )
            })()}
          </div>
        </div>
      )}

      {/* ── Year view ── */}
      {view === "year" &&
        (() => {
          const yearTotalReq = yearDays.reduce(
            (s, d) =>
              s + (d.daily ? (d.daily.request_success ?? 0) + (d.daily.request_failed ?? 0) : 0),
            0,
          )
          const yearTotalIn = yearDays.reduce((s, d) => s + (d.daily?.input_token ?? 0), 0)
          const yearTotalOut = yearDays.reduce((s, d) => s + (d.daily?.output_token ?? 0), 0)
          const yearTotalCost = yearDays.reduce(
            (s, d) => s + (d.daily ? (d.daily.input_cost ?? 0) + (d.daily.output_cost ?? 0) : 0),
            0,
          )

          return (
            <div className="flex min-h-0 flex-1 flex-col gap-2 px-2">
              <InlineStats
                items={[
                  {
                    label: t("stats.requests"),
                    raw: yearTotalReq,
                    format: formatCount,
                    icon: MessageSquare,
                  },
                  {
                    label: t("stats.inputTokens"),
                    raw: yearTotalIn,
                    format: formatCount,
                    icon: ArrowDownToLine,
                  },
                  {
                    label: t("stats.outputTokens"),
                    raw: yearTotalOut,
                    format: formatCount,
                    icon: ArrowUpFromLine,
                  },
                  {
                    label: t("stats.cost"),
                    raw: yearTotalCost,
                    format: formatMoney,
                    icon: DollarSign,
                  },
                ]}
              />

              <div className="flex min-h-0 flex-1 items-center justify-center">
                <div
                  className="grid w-full gap-[3px]"
                  style={{
                    gridTemplateColumns: "repeat(53, 1fr)",
                    gridTemplateRows: "repeat(7, 1fr)",
                    gridAutoFlow: "column",
                  }}
                >
                  {yearDays.map((day) => renderCell(day, day.dateStr))}
                </div>
              </div>

              <div className="mt-auto shrink-0">
                <div className="relative flex items-center gap-3">
                  <NavArrow direction="left" onClick={() => setYearOffset((o) => o - 1)} />
                  <span className="text-base font-bold">{yearLabel}</span>
                  <NavArrow
                    direction="right"
                    onClick={() => setYearOffset((o) => o + 1)}
                    disabled={yearOffset >= 0}
                  />
                  <DataPanelPopover
                    dataTab={dataTab}
                    setDataTab={setDataTab}
                    dailyData={data}
                    hourlyData={hourlyData}
                    modelData={modelData}
                    channelData={channelData}
                  />
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
              </div>
            </div>
          )
        })()}

      {/* ── Month view ── */}
      {view === "month" &&
        (() => {
          const todayStr = toDateStr(today)
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
            <div className="flex min-h-0 flex-1 flex-col px-2">
              {/* Stats row */}
              <div className="flex shrink-0 flex-wrap items-center justify-center gap-x-3 gap-y-0.5 py-1">
                {[
                  { label: "REQ", raw: monthTotalReq, format: formatCount },
                  { label: "IN", raw: monthTotalIn, format: formatCount },
                  { label: "OUT", raw: monthTotalOut, format: formatCount },
                  { label: "$", raw: monthTotalCost, format: formatMoney },
                ].map((item) => {
                  const formatted = item.format(item.raw)
                  return (
                    <div key={item.label} className="flex items-center gap-1">
                      <span className="text-muted-foreground text-[10px] font-bold">
                        {item.label}
                      </span>
                      <span className="text-[11px] font-bold tabular-nums">
                        <AnimatedNumber value={item.raw} formatter={(n) => item.format(n).value} />
                        {formatted.unit && (
                          <span className="text-muted-foreground ml-0.5 text-[9px] font-medium">
                            {formatted.unit}
                          </span>
                        )}
                      </span>
                    </div>
                  )
                })}
              </div>

              <div className="min-h-0 flex-1 overflow-y-auto">
                <ReactorGrid
                  monthDays={monthDays}
                  weekdayLabels={weekdayLabels}
                  todayStr={todayStr}
                  gearAngle={gearAngle}
                  drillIntoDay={drillIntoDay}
                  handleMouseEnter={handleMouseEnter}
                  handleMouseLeave={handleMouseLeave}
                />
              </div>

              <div className="shrink-0">
                <div className="relative flex items-center gap-3">
                  <NavArrow direction="left" onClick={() => setMonthOffset((o) => o - 1)} />
                  <span className="text-base font-bold">{monthLabel}</span>
                  <NavArrow
                    direction="right"
                    onClick={() => setMonthOffset((o) => o + 1)}
                    disabled={monthOffset >= 0}
                  />
                  <DataPanelPopover
                    dataTab={dataTab}
                    setDataTab={setDataTab}
                    dailyData={data}
                    hourlyData={hourlyData}
                    modelData={modelData}
                    channelData={channelData}
                  />
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
              </div>
            </div>
          )
        })()}

      {/* ── Week view ── */}
      {view === "week" &&
        (() => {
          const todayStr = toDateStr(today)
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
            <div className="flex min-h-0 flex-1 flex-col gap-3 px-2">
              <InlineStats
                items={[
                  {
                    label: t("stats.requests"),
                    raw: weekTotalReq,
                    format: formatCount,
                    icon: MessageSquare,
                  },
                  {
                    label: t("stats.inputTokens"),
                    raw: weekTotalIn,
                    format: formatCount,
                    icon: ArrowDownToLine,
                  },
                  {
                    label: t("stats.outputTokens"),
                    raw: weekTotalOut,
                    format: formatCount,
                    icon: ArrowUpFromLine,
                  },
                  {
                    label: t("stats.cost"),
                    raw: weekTotalCost,
                    format: formatMoney,
                    icon: DollarSign,
                  },
                ]}
              />

              <div className="flex min-h-0 flex-1 items-center justify-center">
                <PowerPipeline
                  weekDays={weekDays}
                  weekdayLabels={weekdayLabelsRaw}
                  todayStr={todayStr}
                  gearAngle={gearAngle}
                  drillIntoDay={drillIntoDay}
                  handleMouseEnter={handleMouseEnter}
                  handleMouseLeave={handleMouseLeave}
                />
              </div>

              <div className="mt-auto shrink-0">
                <div className="relative flex items-center gap-3">
                  <NavArrow direction="left" onClick={() => setWeekOffset((o) => o - 1)} />
                  <span className="text-base font-bold">{weekLabel}</span>
                  <NavArrow
                    direction="right"
                    onClick={() => setWeekOffset((o) => o + 1)}
                    disabled={weekOffset >= 0}
                  />
                  <DataPanelPopover
                    dataTab={dataTab}
                    setDataTab={setDataTab}
                    dailyData={data}
                    hourlyData={hourlyData}
                    modelData={modelData}
                    channelData={channelData}
                  />
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
              </div>
            </div>
          )
        })()}

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
    </div>
  )
}
