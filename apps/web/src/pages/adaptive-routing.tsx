import { useQuery } from "@tanstack/react-query"
import { Activity, AlertTriangle, Shield, Zap } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts"
import { Card, CardContent } from "@/components/ui/card"
import { getChannelStats, getTotalStats } from "@/lib/api"

function formatNumber(n: number) {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return n.toLocaleString()
}

export default function AdaptiveRoutingPage() {
  const { t } = useTranslation("adaptive-routing")

  const { data: channelData } = useQuery({
    queryKey: ["stats", "channel"],
    queryFn: getChannelStats,
    staleTime: 30_000,
  })

  const { data: totalData } = useQuery({
    queryKey: ["stats", "total"],
    queryFn: getTotalStats,
    staleTime: 30_000,
  })

  const channels = channelData?.data ?? []

  // Build traffic distribution data from channel stats
  const trafficData = channels.map((ch) => ({
    name: ch.channelName || `Channel ${ch.channelId}`,
    requests: ch.totalRequests || 0,
    avgLatency: Math.round(ch.avgLatency || 0),
    cost: ch.totalCost || 0,
  }))

  // Summary metrics
  const totalRequests = channels.reduce((sum, ch) => sum + (ch.totalRequests || 0), 0)
  const avgLatency =
    channels.length > 0
      ? Math.round(channels.reduce((sum, ch) => sum + (ch.avgLatency || 0), 0) / channels.length)
      : 0
  const activeChannels = channels.length
  const totalSuccess = totalData?.data?.request_success ?? 0
  const totalFailed = totalData?.data?.request_failed ?? 0
  const errorRate =
    totalSuccess + totalFailed > 0 ? (totalFailed / (totalSuccess + totalFailed)) * 100 : 0

  const summaryCards = [
    {
      label: t("stats.totalRequests"),
      value: formatNumber(totalRequests),
      icon: Activity,
    },
    {
      label: t("stats.avgLatency"),
      value: `${avgLatency}${t("unit.ms")}`,
      icon: Zap,
    },
    {
      label: t("stats.errorRate"),
      value: `${errorRate.toFixed(1)}%`,
      icon: AlertTriangle,
    },
    {
      label: t("stats.activeChannels"),
      value: String(activeChannels),
      icon: Shield,
    },
  ]

  if (channels.length === 0) {
    return (
      <div className="flex min-h-0 flex-1 flex-col">
        <div className="shrink-0 pb-4">
          <h2 className="text-2xl font-bold tracking-tight">{t("title")}</h2>
          <p className="text-muted-foreground text-sm">{t("description")}</p>
        </div>
        <div className="flex flex-1 flex-col items-center justify-center gap-2 py-16">
          <Activity className="text-muted-foreground h-10 w-10" />
          <p className="text-muted-foreground text-sm">{t("noData")}</p>
        </div>
      </div>
    )
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="shrink-0 pb-4">
        <h2 className="text-2xl font-bold tracking-tight">{t("title")}</h2>
        <p className="text-muted-foreground text-sm">{t("description")}</p>
      </div>

      <div className="min-h-0 flex-1 space-y-6 overflow-auto">
        {/* Stats cards */}
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

        {/* Traffic Distribution Chart */}
        <div className="bg-card rounded-lg border p-4">
          <h3 className="mb-4 text-sm font-medium">{t("trafficDistribution")}</h3>
          <ResponsiveContainer width="100%" height={300}>
            <BarChart data={trafficData} layout="vertical">
              <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
              <XAxis type="number" tick={{ fontSize: 12 }} />
              <YAxis dataKey="name" type="category" width={150} tick={{ fontSize: 12 }} />
              <Tooltip />
              <Bar dataKey="requests" fill="hsl(var(--chart-1))" radius={[0, 4, 4, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>

        {/* Channel Health Table */}
        <div className="bg-card rounded-lg border p-4">
          <h3 className="mb-4 text-sm font-medium">{t("channelHealth")}</h3>
          <div className="space-y-2">
            {trafficData.map((ch, i) => (
              <div key={i} className="bg-muted/50 flex items-center justify-between rounded-md p-3">
                <div className="flex items-center gap-2">
                  <div className="h-2 w-2 rounded-full bg-slate-400" />
                  <span className="text-sm font-medium">{ch.name}</span>
                </div>
                <div className="text-muted-foreground flex items-center gap-6 text-sm">
                  <span>
                    {formatNumber(ch.requests)} {t("unit.req")}
                  </span>
                  <span>
                    {ch.avgLatency}
                    {t("unit.avgMs")}
                  </span>
                  <span>{t("noData")}</span>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
