import type { LogStats } from "./types"
import { BarChart3, CheckCircle, Clock, DollarSign, Hash, Zap } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Card } from "@/components/ui/card"

function formatLatency(ms: number): string {
  if (ms === 0) return "—"
  if (ms < 1000) return `${Math.round(ms)}ms`
  return `${(ms / 1000).toFixed(2)}s`
}

function formatCost(cost: number): string {
  if (cost === 0) return "$0"
  if (cost < 0.000001) return `$${cost.toExponential(1)}`
  if (cost < 0.01) return `$${cost.toFixed(6)}`
  return `$${cost.toFixed(4)}`
}

export function LogStatsCards({ stats }: { stats: LogStats }) {
  const { t } = useTranslation("logs")

  const cards = [
    {
      label: t("stats.totalRequests"),
      value: stats.totalRequests.toLocaleString(),
      icon: BarChart3,
    },
    {
      label: t("stats.successRate"),
      value: stats.totalRequests > 0 ? `${stats.successRate.toFixed(1)}%` : "—",
      icon: CheckCircle,
    },
    {
      label: t("stats.avgLatency"),
      value: formatLatency(stats.averageLatency),
      icon: Clock,
    },
    {
      label: t("stats.totalTokens"),
      value: stats.totalTokens.toLocaleString(),
      icon: Hash,
    },
    {
      label: t("stats.totalCost"),
      value: formatCost(stats.totalCost),
      icon: DollarSign,
    },
    {
      label: t("stats.tokenSpeed"),
      value: stats.tokenSpeed > 0 ? `${stats.tokenSpeed.toFixed(1)} tok/s` : "—",
      icon: Zap,
    },
  ]

  return (
    <div className="grid grid-cols-3 gap-3 md:grid-cols-6">
      {cards.map((card) => (
        <Card key={card.label} className="gap-0 px-4 py-3">
          <div className="flex items-center gap-1.5">
            <card.icon className="text-muted-foreground h-3.5 w-3.5 shrink-0" />
            <span className="text-muted-foreground truncate text-xs">{card.label}</span>
          </div>
          <span className="mt-1 truncate font-mono text-lg font-semibold tracking-tight">
            {card.value}
          </span>
        </Card>
      ))}
    </div>
  )
}
