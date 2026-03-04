import type { ChannelStatsRow } from "@/lib/api-client"
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts"
import { formatCount, formatMoney } from "@/lib/format"

type MetricKey = "cost" | "requests"

interface ChannelComparisonChartProps {
  data?: ChannelStatsRow[]
  title?: string
}

export function ChannelComparisonChart({ data, title }: ChannelComparisonChartProps) {
  const { t } = useTranslation("dashboard")
  const [metric, setMetric] = useState<MetricKey>("cost")

  const chartData = useMemo(() => {
    if (!data || data.length === 0) return []
    const sorted = [...data].sort((a, b) => {
      if (metric === "cost") return b.totalCost - a.totalCost
      return b.totalRequests - a.totalRequests
    })
    return sorted.slice(0, 10).map((ch) => ({
      name: ch.channelName || `#${ch.channelId}`,
      cost: ch.totalCost,
      requests: ch.totalRequests,
    }))
  }, [data, metric])

  const formatTick = (v: number) => {
    if (metric === "cost") {
      const f = formatMoney(v)
      return `${f.value}${f.unit}`
    }
    const f = formatCount(v)
    return `${f.value}${f.unit}`
  }

  const formatTooltipValue = (value: any) => {
    const v = Number(value)
    if (metric === "cost") {
      const f = formatMoney(v)
      return [`${f.value}${f.unit}`, t("chart.cost")]
    }
    const f = formatCount(v)
    return [`${f.value}${f.unit}`, t("chart.totalRequests")]
  }

  if (!chartData.length) {
    return (
      <div className="bg-card rounded-lg border p-4">
        <h3 className="text-muted-foreground text-sm font-medium">
          {title || t("chart.channelComparison", "Channel Comparison")}
        </h3>
        <div className="text-muted-foreground flex h-[200px] items-center justify-center text-sm">
          {t("chart.noData", "No data")}
        </div>
      </div>
    )
  }

  return (
    <div className="bg-card rounded-lg border p-4">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-muted-foreground text-sm font-medium">
          {title || t("chart.channelComparison", "Channel Comparison")}
        </h3>
        <div className="bg-muted/50 flex items-center gap-0.5 rounded-md border p-0.5">
          {(
            [
              { key: "cost" as const, label: t("chart.cost") },
              { key: "requests" as const, label: t("chart.totalRequests") },
            ] as const
          ).map(({ key, label }) => (
            <button
              type="button"
              key={key}
              onClick={() => setMetric(key)}
              className={`rounded-sm px-2 py-1 text-xs transition-colors ${
                metric === key
                  ? "bg-background text-foreground shadow-sm"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              {label}
            </button>
          ))}
        </div>
      </div>
      <ResponsiveContainer width="100%" height={200}>
        <BarChart data={chartData} layout="vertical" margin={{ left: 10 }}>
          <CartesianGrid strokeDasharray="3 3" horizontal={false} className="stroke-border" />
          <XAxis
            type="number"
            tickLine={false}
            axisLine={false}
            className="text-xs"
            tick={{ fontSize: 10 }}
            tickFormatter={formatTick}
          />
          <YAxis
            type="category"
            dataKey="name"
            tickLine={false}
            axisLine={false}
            className="text-xs"
            tick={{ fontSize: 10 }}
            width={80}
          />
          <Tooltip formatter={formatTooltipValue} />
          <Bar dataKey={metric} fill="var(--chart-5, hsl(30 70% 50%))" radius={[0, 4, 4, 0]} />
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}
