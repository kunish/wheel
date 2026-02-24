import type { DataPanelPopoverProps } from "./data-panel-popover"
import type { DayData, HeatmapTooltip, PeriodTotals, StreamingDelta } from "./types"
import type {
  ChannelStatsRow,
  ModelStatsItem,
  StatsDaily,
  StatsHourly,
  StatsMetrics,
} from "@/lib/api-client"
import { autoUpdate, flip, FloatingPortal, offset, shift, useFloating } from "@floating-ui/react"
import { ArrowDownToLine, ArrowUpFromLine, DollarSign, MessageSquare } from "lucide-react"
import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { AnimatedNumber } from "@/components/animated-number"
import { useDateNavigation } from "@/hooks/use-date-navigation"
import { formatCount, formatMoney } from "@/lib/format"
import { DataPanelPopover } from "./data-panel-popover"
import { HeroGearClock } from "./hero-gear-clock"
import { InlineStats } from "./inline-stats"
import { PeriodNavBar } from "./period-nav-bar"
import { PowerPipeline } from "./power-pipeline"
import { ReactorGrid } from "./reactor-grid"
import {
  computePeriodTotals,
  getActivityLevel,
  LEVEL_COLORS,
  toDateStr,
  useGearRotation,
} from "./types"

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
      className="text-muted-foreground hover:text-foreground hover:bg-muted hover:border-border rounded-md border border-transparent p-1 transition-colors disabled:cursor-not-allowed disabled:opacity-30 disabled:hover:border-transparent disabled:hover:bg-transparent"
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
  const { t } = useTranslation("dashboard")
  const gearAngle = useGearRotation()

  const dataMap = useMemo(() => new Map((data ?? []).map((d) => [d.date, d])), [data])
  const nav = useDateNavigation(dataMap)

  const [activeTooltip, setActiveTooltip] = useState<HeatmapTooltip | null>(null)
  const [dataTab, setDataTab] = useState<DataPanelPopoverProps["dataTab"]>(null)

  const { refs, floatingStyles } = useFloating({
    placement: "top",
    open: !!activeTooltip,
    middleware: [offset(8), flip(), shift({ padding: 8 })],
    whileElementsMounted: autoUpdate,
  })

  // ── Gear clock center data ──
  const gearData = useMemo(() => {
    const sd = streamingDelta ?? { inputTokens: 0, outputTokens: 0, inputCost: 0, outputCost: 0 }
    if (nav.view === "day") {
      const dayMetrics = nav.selectedDayData
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
  }, [nav.view, nav.selectedDayData, totalData, streamingDelta])

  // ── Tooltip handlers ──
  const handleMouseEnter = useCallback(
    (e: React.MouseEvent, tooltip: HeatmapTooltip) => {
      refs.setReference(e.currentTarget)
      setActiveTooltip(tooltip)
    },
    [refs],
  )
  const handleMouseLeave = useCallback(() => setActiveTooltip(null), [])

  // ── Shared data panel props ──
  const dataPanelProps: DataPanelPopoverProps = {
    dataTab,
    setDataTab,
    dailyData: data,
    hourlyData,
    modelData,
    channelData,
  }

  // ── Period totals ──
  const yearTotals = useMemo(
    () => computePeriodTotals(nav.yearDays.map((d) => d.daily)),
    [nav.yearDays],
  )
  const monthTotals = useMemo(
    () => computePeriodTotals(nav.monthDays.map((d) => d?.daily)),
    [nav.monthDays],
  )
  const weekTotals = useMemo(
    () => computePeriodTotals(nav.weekDays.map((d) => d.daily)),
    [nav.weekDays],
  )

  function buildStatsItems(totals: PeriodTotals) {
    return [
      { label: t("stats.requests"), raw: totals.req, format: formatCount, icon: MessageSquare },
      {
        label: t("stats.inputTokens"),
        raw: totals.inTok,
        format: formatCount,
        icon: ArrowDownToLine,
      },
      {
        label: t("stats.outputTokens"),
        raw: totals.outTok,
        format: formatCount,
        icon: ArrowUpFromLine,
      },
      { label: t("stats.cost"), raw: totals.cost, format: formatMoney, icon: DollarSign },
    ]
  }

  // ── Heatmap cell renderer ──
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
        onClick={() => nav.drillIntoDay(day.dateStr)}
        onMouseEnter={(e) => handleMouseEnter(e, { label: day.displayDate, metrics: day.daily })}
        onMouseLeave={handleMouseLeave}
      />
    )
  }

  const todayStr = toDateStr(nav.today)

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {/* ── View tabs ── */}
      <div className="flex shrink-0 items-center justify-between px-2 pb-2">
        <span className="text-sm font-bold">{t("activity.title")}</span>
        <div className="flex items-center gap-1">
          {(["day", "week", "month", "year"] as const).map((v) => (
            <button
              key={v}
              onClick={() => nav.setView(v)}
              className={`rounded-md px-2.5 py-1 text-xs font-medium transition-colors ${
                nav.view === v
                  ? "bg-primary text-primary-foreground shadow-sm"
                  : "text-muted-foreground hover:bg-muted hover:text-foreground"
              }`}
            >
              {nav.viewLabels[v]}
            </button>
          ))}
        </div>
      </div>

      {/* ── Day view ── */}
      {nav.view === "day" && (
        <div className="flex min-h-0 flex-1 flex-col">
          <GearClockFit>
            <HeroGearClock
              dayHourlyMap={nav.dayHourlyMap}
              isToday={nav.selectedDayDateStr === toDateStr(new Date())}
              nowHour={new Date().getHours()}
              now={new Date()}
              selectedDayDateStr={nav.selectedDayDateStr}
              selectedDisplayDate={nav.selectedDisplayDate}
              gearAngle={gearAngle}
              navigateToHour={nav.navigateToHour}
              handleMouseEnter={handleMouseEnter}
              handleMouseLeave={handleMouseLeave}
            >
              <div className="flex flex-col items-center gap-1">
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
                <div className="bg-border h-px w-10" />
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
                <span
                  className="text-xs font-bold tabular-nums"
                  style={{
                    color: "color-mix(in srgb, var(--nb-lime) 60%, var(--foreground))",
                  }}
                >
                  {formatMoney(gearData.totalCost).value}
                </span>
                <DataPanelPopover {...dataPanelProps} />
              </div>
            </HeroGearClock>
          </GearClockFit>

          <div className="shrink-0 px-2">
            {(() => {
              const isToday = nav.selectedDayDateStr === todayStr
              const isFutureDate = (() => {
                const d = new Date(
                  Number.parseInt(nav.selectedDayDateStr.slice(0, 4)),
                  Number.parseInt(nav.selectedDayDateStr.slice(4, 6)) - 1,
                  Number.parseInt(nav.selectedDayDateStr.slice(6, 8)),
                )
                return d > nav.today
              })()

              return (
                <div className="relative flex items-center gap-2">
                  <NavArrow direction="left" onClick={() => nav.shiftDay(-1)} />
                  <div className="flex items-baseline gap-1.5">
                    <span className="text-sm font-bold">{nav.selectedDisplayDate}</span>
                    <span className="text-muted-foreground text-[11px] font-medium">
                      {nav.selectedDayWeekday}
                    </span>
                  </div>
                  <NavArrow
                    direction="right"
                    onClick={() => nav.shiftDay(1)}
                    disabled={isFutureDate || isToday}
                  />
                  <div className="ml-auto flex items-center gap-3">
                    {!isToday && (
                      <button
                        onClick={() => nav.setSelectedDateStr(null)}
                        className="text-muted-foreground hover:text-foreground text-xs font-medium underline-offset-2 hover:underline"
                      >
                        {t("activity.today")}
                      </button>
                    )}
                    <button
                      onClick={() => nav.navigateToDay(nav.selectedDayDateStr)}
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
      {nav.view === "year" && (
        <div className="flex min-h-0 flex-1 flex-col gap-2 px-2">
          <InlineStats items={buildStatsItems(yearTotals)} />
          <div className="flex min-h-0 flex-1 items-center justify-center">
            <div
              className="grid w-full gap-[3px]"
              style={{
                gridTemplateColumns: "repeat(53, 1fr)",
                gridTemplateRows: "repeat(7, 1fr)",
                gridAutoFlow: "column",
              }}
            >
              {nav.yearDays.map((day) => renderCell(day, day.dateStr))}
            </div>
          </div>
          <div className="mt-auto shrink-0">
            <PeriodNavBar
              label={nav.yearLabel}
              onPrev={() => nav.setYearOffset((o) => o - 1)}
              onNext={() => nav.setYearOffset((o) => o + 1)}
              nextDisabled={nav.yearOffset >= 0}
              resetLabel={nav.yearOffset !== 0 ? t("activity.thisYear") : undefined}
              onReset={() => nav.setYearOffset(0)}
              viewLogsLabel={t("activity.viewLogs")}
              onViewLogs={nav.navigateToYear}
              dataPanelProps={dataPanelProps}
            />
          </div>
        </div>
      )}

      {/* ── Month view ── */}
      {nav.view === "month" && (
        <div className="flex min-h-0 flex-1 flex-col px-2">
          <InlineStats items={buildStatsItems(monthTotals)} />
          <div className="flex min-h-0 flex-1 items-center justify-center overflow-y-auto">
            <ReactorGrid
              monthDays={nav.monthDays}
              weekdayLabels={nav.weekdayLabels}
              todayStr={todayStr}
              gearAngle={gearAngle}
              drillIntoDay={nav.drillIntoDay}
              handleMouseEnter={handleMouseEnter}
              handleMouseLeave={handleMouseLeave}
            />
          </div>
          <div className="shrink-0">
            <PeriodNavBar
              label={nav.monthLabel}
              onPrev={() => nav.setMonthOffset((o) => o - 1)}
              onNext={() => nav.setMonthOffset((o) => o + 1)}
              nextDisabled={nav.monthOffset >= 0}
              resetLabel={nav.monthOffset !== 0 ? t("activity.thisMonth") : undefined}
              onReset={() => nav.setMonthOffset(0)}
              viewLogsLabel={t("activity.viewLogs")}
              onViewLogs={nav.navigateToMonth}
              dataPanelProps={dataPanelProps}
            />
          </div>
        </div>
      )}

      {/* ── Week view ── */}
      {nav.view === "week" && (
        <div className="flex min-h-0 flex-1 flex-col gap-3 px-2">
          <InlineStats items={buildStatsItems(weekTotals)} />
          <div className="flex min-h-0 flex-1 items-center justify-center">
            <PowerPipeline
              weekDays={nav.weekDays}
              weekdayLabels={nav.weekdayLabelsRaw}
              todayStr={todayStr}
              gearAngle={gearAngle}
              drillIntoDay={nav.drillIntoDay}
              handleMouseEnter={handleMouseEnter}
              handleMouseLeave={handleMouseLeave}
            />
          </div>
          <div className="mt-auto shrink-0">
            <PeriodNavBar
              label={nav.weekLabel}
              onPrev={() => nav.setWeekOffset((o) => o - 1)}
              onNext={() => nav.setWeekOffset((o) => o + 1)}
              nextDisabled={nav.weekOffset >= 0}
              resetLabel={nav.weekOffset !== 0 ? t("activity.thisWeek") : undefined}
              onReset={() => nav.setWeekOffset(0)}
              viewLogsLabel={t("activity.viewLogs")}
              onViewLogs={nav.navigateToWeek}
              dataPanelProps={dataPanelProps}
            />
          </div>
        </div>
      )}

      {/* ── Floating tooltip ── */}
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
                className="bg-popover text-popover-foreground pointer-events-none z-50 w-fit min-w-max rounded-md border p-3 text-sm shadow-md"
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
