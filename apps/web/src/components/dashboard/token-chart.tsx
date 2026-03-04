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
import { formatCount } from "@/lib/format"
import { ChartTypeToggle } from "./chart-type-toggle"

interface TokenChartProps {
  data: Array<{ date: string; inputTokens: number; outputTokens: number }>
  title?: string
}

export function TokenChart({ data, title }: TokenChartProps) {
  const { t } = useTranslation("dashboard")
  const [chartType, setChartType] = useState<ChartType>("area")

  const formattedData = useMemo(() => data, [data])

  const formatTokenTick = (v: number) => {
    const f = formatCount(v)
    return `${f.value}${f.unit}`
  }

  const formatTokenTooltip = (value: any, name: any) => {
    const f = formatCount(Number(value))
    const label =
      name === "inputTokens"
        ? t("chart.inputTokens", "Input Tokens")
        : t("chart.outputTokens", "Output Tokens")
    return [`${f.value}${f.unit}`, label]
  }

  return (
    <div className="bg-card rounded-lg border p-4">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-muted-foreground text-sm font-medium">
          {title || t("chart.tokenUsage", "Token Usage")}
        </h3>
        <ChartTypeToggle value={chartType} onChange={setChartType} />
      </div>
      <ResponsiveContainer width="100%" height={200}>
        {chartType === "area" ? (
          <AreaChart data={formattedData}>
            <defs>
              <linearGradient id="inputTokenGradient" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="var(--chart-3, hsl(270 70% 50%))" stopOpacity={0.3} />
                <stop offset="95%" stopColor="var(--chart-3, hsl(270 70% 50%))" stopOpacity={0} />
              </linearGradient>
              <linearGradient id="outputTokenGradient" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="var(--chart-4, hsl(150 70% 50%))" stopOpacity={0.3} />
                <stop offset="95%" stopColor="var(--chart-4, hsl(150 70% 50%))" stopOpacity={0} />
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
              tickFormatter={formatTokenTick}
            />
            <Tooltip formatter={formatTokenTooltip} />
            <Area
              type="monotone"
              dataKey="inputTokens"
              stackId="tokens"
              stroke="var(--chart-3, hsl(270 70% 50%))"
              fill="url(#inputTokenGradient)"
            />
            <Area
              type="monotone"
              dataKey="outputTokens"
              stackId="tokens"
              stroke="var(--chart-4, hsl(150 70% 50%))"
              fill="url(#outputTokenGradient)"
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
              tickFormatter={formatTokenTick}
            />
            <Tooltip formatter={formatTokenTooltip} />
            <Line
              type="monotone"
              dataKey="inputTokens"
              stroke="var(--chart-3, hsl(270 70% 50%))"
              strokeWidth={2}
              dot={false}
            />
            <Line
              type="monotone"
              dataKey="outputTokens"
              stroke="var(--chart-4, hsl(150 70% 50%))"
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
              tickFormatter={formatTokenTick}
            />
            <Tooltip formatter={formatTokenTooltip} />
            <Bar
              dataKey="inputTokens"
              stackId="tokens"
              fill="var(--chart-3, hsl(270 70% 50%))"
              radius={[0, 0, 0, 0]}
            />
            <Bar
              dataKey="outputTokens"
              stackId="tokens"
              fill="var(--chart-4, hsl(150 70% 50%))"
              radius={[4, 4, 0, 0]}
            />
          </BarChart>
        )}
      </ResponsiveContainer>
    </div>
  )
}
