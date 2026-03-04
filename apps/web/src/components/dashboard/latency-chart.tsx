import type { ChartType } from "./chart-type-toggle"
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts"
import { formatTime } from "@/lib/format"
import { ChartTypeToggle } from "./chart-type-toggle"

interface LatencyChartProps {
  data: Array<{ date: string; latency: number }>
  title?: string
}

export function LatencyChart({ data, title }: LatencyChartProps) {
  const { t } = useTranslation("dashboard")
  const [chartType, setChartType] = useState<ChartType>("area")

  const formattedData = useMemo(
    () => data.map((d) => ({ ...d, latency: Math.round(d.latency) })),
    [data],
  )

  const formatLatencyTick = (v: number) => {
    const f = formatTime(v)
    return `${f.value}${f.unit}`
  }

  const formatLatencyTooltip = (value: any) => {
    const f = formatTime(Number(value))
    return [`${f.value}${f.unit}`, t("chart.latency", "Latency")]
  }

  return (
    <div className="bg-card rounded-lg border p-4">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-muted-foreground text-sm font-medium">
          {title || t("chart.latencyTrend", "Latency Trend")}
        </h3>
        <ChartTypeToggle value={chartType} onChange={setChartType} />
      </div>
      <ResponsiveContainer width="100%" height={200}>
        {chartType === "area" ? (
          <AreaChart data={formattedData}>
            <defs>
              <linearGradient id="latencyGradient" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="var(--chart-2, hsl(220 70% 50%))" stopOpacity={0.3} />
                <stop offset="95%" stopColor="var(--chart-2, hsl(220 70% 50%))" stopOpacity={0} />
              </linearGradient>
            </defs>
            <CartesianGrid strokeDasharray="3 3" vertical={false} className="stroke-border" />
            <XAxis
              dataKey="date"
              tickLine={false}
              axisLine={false}
              className="text-xs"
              tick={{ fontSize: 10 }}
            />
            <YAxis
              tickLine={false}
              axisLine={false}
              className="text-xs"
              tick={{ fontSize: 10 }}
              tickFormatter={formatLatencyTick}
            />
            <Tooltip formatter={formatLatencyTooltip} />
            <Area
              type="monotone"
              dataKey="latency"
              stroke="var(--chart-2, hsl(220 70% 50%))"
              fill="url(#latencyGradient)"
            />
          </AreaChart>
        ) : chartType === "line" ? (
          <LineChart data={formattedData}>
            <CartesianGrid strokeDasharray="3 3" vertical={false} className="stroke-border" />
            <XAxis
              dataKey="date"
              tickLine={false}
              axisLine={false}
              className="text-xs"
              tick={{ fontSize: 10 }}
            />
            <YAxis
              tickLine={false}
              axisLine={false}
              className="text-xs"
              tick={{ fontSize: 10 }}
              tickFormatter={formatLatencyTick}
            />
            <Tooltip formatter={formatLatencyTooltip} />
            <Line
              type="monotone"
              dataKey="latency"
              stroke="var(--chart-2, hsl(220 70% 50%))"
              strokeWidth={2}
              dot={false}
            />
          </LineChart>
        ) : (
          <BarChart data={formattedData}>
            <CartesianGrid strokeDasharray="3 3" vertical={false} className="stroke-border" />
            <XAxis
              dataKey="date"
              tickLine={false}
              axisLine={false}
              className="text-xs"
              tick={{ fontSize: 10 }}
            />
            <YAxis
              tickLine={false}
              axisLine={false}
              className="text-xs"
              tick={{ fontSize: 10 }}
              tickFormatter={formatLatencyTick}
            />
            <Tooltip formatter={formatLatencyTooltip} />
            <Bar dataKey="latency" fill="var(--chart-2, hsl(220 70% 50%))" radius={[4, 4, 0, 0]} />
          </BarChart>
        )}
      </ResponsiveContainer>
    </div>
  )
}
