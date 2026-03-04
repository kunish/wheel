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
import { ChartTypeToggle } from "./chart-type-toggle"

interface CostChartProps {
  data: Array<{ date: string; cost: number }>
  title?: string
}

export function CostChart({ data, title }: CostChartProps) {
  const { t } = useTranslation("dashboard")
  const [chartType, setChartType] = useState<ChartType>("area")

  const formattedData = useMemo(
    () => data.map((d) => ({ ...d, cost: Number(d.cost.toFixed(4)) })),
    [data],
  )

  return (
    <div className="bg-card rounded-lg border p-4">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-muted-foreground text-sm font-medium">
          {title || t("chart.costTrend", "Cost Trend")}
        </h3>
        <ChartTypeToggle value={chartType} onChange={setChartType} />
      </div>
      <ResponsiveContainer width="100%" height={200}>
        {chartType === "area" ? (
          <AreaChart data={formattedData}>
            <defs>
              <linearGradient id="costGradient" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="var(--primary)" stopOpacity={0.3} />
                <stop offset="95%" stopColor="var(--primary)" stopOpacity={0} />
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
              tickFormatter={(v) => `$${v}`}
            />
            <Tooltip formatter={(value: any) => [`${Number(value).toFixed(4)}`, t("chart.cost")]} />
            <Area
              type="monotone"
              dataKey="cost"
              stroke="var(--primary)"
              fill="url(#costGradient)"
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
              tickFormatter={(v) => `$${v}`}
            />
            <Tooltip formatter={(value: any) => [`${Number(value).toFixed(4)}`, t("chart.cost")]} />
            <Line
              type="monotone"
              dataKey="cost"
              stroke="var(--primary)"
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
              tickFormatter={(v) => `$${v}`}
            />
            <Tooltip formatter={(value: any) => [`${Number(value).toFixed(4)}`, t("chart.cost")]} />
            <Bar dataKey="cost" fill="var(--primary)" radius={[4, 4, 0, 0]} />
          </BarChart>
        )}
      </ResponsiveContainer>
    </div>
  )
}
