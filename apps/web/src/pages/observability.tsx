import { useQuery } from "@tanstack/react-query"
import { Activity, AlertCircle, Clock, Radio } from "lucide-react"
import { useTranslation } from "react-i18next"
import {
  Area,
  AreaChart,
  CartesianGrid,
  Cell,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts"
import { Card, CardContent } from "@/components/ui/card"
import { getChannelStats, getHourlyStats, getModelStats, getTotalStats } from "@/lib/api"

const COLORS = [
  "hsl(var(--chart-1))",
  "hsl(var(--chart-2))",
  "hsl(var(--chart-3))",
  "hsl(var(--chart-4))",
  "hsl(var(--chart-5))",
]

function formatNumber(n: number) {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return n.toLocaleString()
}

export default function ObservabilityPage() {
  const { t } = useTranslation("observability")

  const { data: hourlyData } = useQuery({
    queryKey: ["stats", "hourly"],
    queryFn: () => getHourlyStats(),
    staleTime: 30_000,
  })

  const { data: channelData } = useQuery({
    queryKey: ["stats", "channel"],
    queryFn: getChannelStats,
    staleTime: 30_000,
  })

  const { data: modelData } = useQuery({
    queryKey: ["stats", "model"],
    queryFn: getModelStats,
    staleTime: 30_000,
  })

  const { data: totalData } = useQuery({
    queryKey: ["stats", "total"],
    queryFn: getTotalStats,
    staleTime: 30_000,
  })

  const totals = totalData?.data
  const channels = channelData?.data ?? []
  const models = modelData?.data ?? []
  const hourly = hourlyData?.data ?? []

  // Process hourly data for request rate chart
  const requestRateData = hourly.map((h) => ({
    hour: `${String(h.hour).padStart(2, "0")}:00`,
    requests: (h.request_success || 0) + (h.request_failed || 0),
    errors: h.request_failed || 0,
    avgLatency: Math.round(h.wait_time || 0),
  }))

  // Model usage distribution for pie chart
  const modelDistribution = models.slice(0, 8).map((m) => ({
    name: m.model || "unknown",
    value: m.requestCount || 0,
  }))

  // Summary metrics
  const totalRequests = totals ? (totals.request_success || 0) + (totals.request_failed || 0) : 0
  const avgLatency =
    channels.length > 0
      ? Math.round(channels.reduce((sum, ch) => sum + (ch.avgLatency || 0), 0) / channels.length)
      : 0
  const errorRate =
    totalRequests > 0 ? (((totals?.request_failed || 0) / totalRequests) * 100).toFixed(1) : "0.0"

  const summaryCards = [
    {
      label: t("stats.totalRequests"),
      value: formatNumber(totalRequests),
      icon: Activity,
    },
    {
      label: t("stats.avgLatency"),
      value: `${avgLatency}${t("unit.ms")}`,
      icon: Clock,
    },
    {
      label: t("stats.errorRate"),
      value: `${errorRate}%`,
      icon: AlertCircle,
    },
    {
      label: t("stats.totalModels"),
      value: String(models.length),
      icon: Radio,
    },
  ]

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="shrink-0 pb-4">
        <h2 className="text-2xl font-bold tracking-tight">{t("title")}</h2>
        <p className="text-muted-foreground text-sm">{t("description")}</p>
      </div>

      <div className="min-h-0 flex-1 space-y-6 overflow-auto">
        {/* Key metrics cards */}
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {summaryCards.map((card) => {
            const Icon = card.icon
            return (
              <Card key={card.label}>
                <CardContent className="flex items-center gap-4 pt-6">
                  <div className="bg-muted flex h-10 w-10 shrink-0 items-center justify-center rounded-lg">
                    <Icon className="h-5 w-5" />
                  </div>
                  <div>
                    <p className="text-muted-foreground text-xs">{card.label}</p>
                    <p className="text-xl font-bold">{card.value}</p>
                  </div>
                </CardContent>
              </Card>
            )
          })}
        </div>

        {/* Request Rate chart */}
        <div className="bg-card rounded-lg border p-4">
          <h3 className="mb-4 text-sm font-medium">{t("requestRate")}</h3>
          <ResponsiveContainer width="100%" height={250}>
            <AreaChart data={requestRateData}>
              <defs>
                <linearGradient id="reqGradient" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="hsl(var(--chart-1))" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="hsl(var(--chart-1))" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
              <XAxis dataKey="hour" tick={{ fontSize: 10 }} />
              <YAxis tick={{ fontSize: 10 }} />
              <Tooltip />
              <Area
                type="monotone"
                dataKey="requests"
                stroke="hsl(var(--chart-1))"
                fill="url(#reqGradient)"
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>

        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          {/* Latency chart */}
          <div className="bg-card rounded-lg border p-4">
            <h3 className="mb-4 text-sm font-medium">{t("latencyTrend")}</h3>
            <ResponsiveContainer width="100%" height={200}>
              <AreaChart data={requestRateData}>
                <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                <XAxis dataKey="hour" tick={{ fontSize: 10 }} />
                <YAxis tick={{ fontSize: 10 }} tickFormatter={(v) => `${v}ms`} />
                <Tooltip formatter={(value: any) => [`${Number(value)}ms`, t("unit.avgLatency")]} />
                <Area
                  type="monotone"
                  dataKey="avgLatency"
                  stroke="hsl(var(--chart-2))"
                  fill="hsl(var(--chart-2))"
                  fillOpacity={0.1}
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>

          {/* Model Usage pie chart */}
          <div className="bg-card rounded-lg border p-4">
            <h3 className="mb-4 text-sm font-medium">{t("modelUsage")}</h3>
            {modelDistribution.length > 0 ? (
              <ResponsiveContainer width="100%" height={200}>
                <PieChart>
                  <Pie
                    data={modelDistribution}
                    dataKey="value"
                    nameKey="name"
                    cx="50%"
                    cy="50%"
                    outerRadius={80}
                    label={({ name, percent }: any) =>
                      `${name ?? ""} ${((percent ?? 0) * 100).toFixed(0)}%`
                    }
                    labelLine={false}
                  >
                    {modelDistribution.map((_, i) => (
                      <Cell key={i} fill={COLORS[i % COLORS.length]} />
                    ))}
                  </Pie>
                  <Tooltip />
                </PieChart>
              </ResponsiveContainer>
            ) : (
              <div className="flex h-[200px] items-center justify-center">
                <p className="text-muted-foreground text-sm">{t("noData")}</p>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
