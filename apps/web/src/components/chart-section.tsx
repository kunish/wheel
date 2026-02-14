import type { StatsDaily, StatsHourly } from "@/lib/api-client"
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts"
import { AnimatedNumber } from "@/components/animated-number"
import { Card, CardContent, CardHeader } from "@/components/ui/card"
import { formatCount, formatMoney } from "@/lib/format"

const PERIODS = ["1", "7", "30"] as const

export interface ChartSectionProps {
  dailyData?: StatsDaily[]
  hourlyData?: StatsHourly[]
}

export function ChartSection({ dailyData, hourlyData }: ChartSectionProps) {
  const { t } = useTranslation("dashboard")
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
    const source = period === "1" ? hourlyData : sortedDaily.slice(-Number.parseInt(period))
    if (!source || source.length === 0)
      return { requests: 0, cost: 0, inputTokens: 0, outputTokens: 0 }
    let requests = 0
    let cost = 0
    let inputTokens = 0
    let outputTokens = 0
    for (const s of source) {
      requests += s.request_success + s.request_failed
      cost += s.input_cost + s.output_cost
      inputTokens += s.input_token
      outputTokens += s.output_token
    }
    return { requests, cost, inputTokens, outputTokens }
  }, [sortedDaily, hourlyData, period])

  const periodLabel: Record<string, string> = useMemo(
    () => ({
      "1": t("chart.today"),
      "7": t("chart.last7Days"),
      "30": t("chart.last30Days"),
    }),
    [t],
  )

  const handlePeriodClick = () => {
    const idx = PERIODS.indexOf(period)
    setPeriod(PERIODS[(idx + 1) % PERIODS.length])
  }

  return (
    <Card>
      <CardHeader className="flex flex-col gap-4 pb-2 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex min-w-0 flex-wrap gap-x-6 gap-y-2">
          <div>
            <p className="text-muted-foreground text-xs">{t("chart.totalRequests")}</p>
            <p className="text-xl font-semibold">
              <AnimatedNumber value={totals.requests} formatter={(n) => formatCount(n).value} />
              <span className="text-muted-foreground ml-0.5 text-sm">
                {formatCount(totals.requests).unit}
              </span>
            </p>
          </div>
          <div className="bg-border hidden w-px self-stretch sm:block" />
          <div>
            <p className="text-muted-foreground text-xs">{t("chart.inputTokens")}</p>
            <p className="text-xl font-semibold">
              <AnimatedNumber value={totals.inputTokens} formatter={(n) => formatCount(n).value} />
              <span className="text-muted-foreground ml-0.5 text-sm">
                {formatCount(totals.inputTokens).unit}
              </span>
            </p>
          </div>
          <div className="bg-border hidden w-px self-stretch sm:block" />
          <div>
            <p className="text-muted-foreground text-xs">{t("chart.outputTokens")}</p>
            <p className="text-xl font-semibold">
              <AnimatedNumber value={totals.outputTokens} formatter={(n) => formatCount(n).value} />
              <span className="text-muted-foreground ml-0.5 text-sm">
                {formatCount(totals.outputTokens).unit}
              </span>
            </p>
          </div>
          <div className="bg-border hidden w-px self-stretch sm:block" />
          <div>
            <p className="text-muted-foreground text-xs">{t("chart.totalCost")}</p>
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
          <p className="text-muted-foreground text-xs">{t("chart.period")}</p>
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
            <Tooltip
              formatter={(v: unknown) => [formatMoney(Number(v ?? 0)).value, t("chart.cost")]}
              labelClassName="text-foreground"
            />
            <Area type="monotone" dataKey="cost" stroke="var(--primary)" fill="url(#fillCost)" />
          </AreaChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  )
}
