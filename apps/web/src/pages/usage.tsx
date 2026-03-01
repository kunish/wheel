import { useQuery } from "@tanstack/react-query"
import { BarChart3, DollarSign, Hash, Zap } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Card, CardContent } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { getChannelStats, getModelStats, getTotalStats, listApiKeys } from "@/lib/api-client"

function formatCost(cost: number) {
  return `$${cost.toFixed(4)}`
}

function formatNumber(n: number) {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return n.toLocaleString()
}

// ── Summary Cards ──

function SummaryCards({
  totalCost,
  totalRequests,
  totalTokens,
}: {
  totalCost: number
  totalRequests: number
  totalTokens: number
}) {
  const { t } = useTranslation("usage")
  const avgCost = totalRequests > 0 ? totalCost / totalRequests : 0

  const cards = [
    { label: t("summary.totalCost"), value: formatCost(totalCost), icon: DollarSign },
    { label: t("summary.totalRequests"), value: formatNumber(totalRequests), icon: Hash },
    { label: t("summary.totalTokens"), value: formatNumber(totalTokens), icon: Zap },
    { label: t("summary.avgCostPerRequest"), value: formatCost(avgCost), icon: BarChart3 },
  ]

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      {cards.map((card) => {
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
  )
}

// ── By API Key Tab ──

function ByKeyTab() {
  const { t } = useTranslation("usage")
  const { data: keysData, isLoading } = useQuery({
    queryKey: ["api-keys"],
    queryFn: listApiKeys,
  })

  const keys = keysData?.data?.apiKeys ?? []

  if (isLoading) {
    return (
      <p className="text-muted-foreground py-8 text-center text-sm">
        {t("actions.loading", { ns: "common" })}
      </p>
    )
  }

  if (keys.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center gap-2 py-16">
        <BarChart3 className="text-muted-foreground h-10 w-10" />
        <p className="text-muted-foreground font-medium">{t("noData")}</p>
        <p className="text-muted-foreground text-sm">{t("noDataHint")}</p>
      </div>
    )
  }

  const sorted = [...keys].sort((a, b) => b.totalCost - a.totalCost)

  return (
    <div className="overflow-x-auto">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("table.name")}</TableHead>
            <TableHead className="text-right">{t("table.cost")}</TableHead>
            <TableHead className="text-right">{t("table.quota")}</TableHead>
            <TableHead className="text-right">{t("table.percentage")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {sorted.map((key) => {
            const percent = key.maxCost > 0 ? (key.totalCost / key.maxCost) * 100 : 0
            return (
              <TableRow key={key.id}>
                <TableCell className="font-medium">{key.name}</TableCell>
                <TableCell className="text-right font-mono">{formatCost(key.totalCost)}</TableCell>
                <TableCell className="text-right font-mono">
                  {key.maxCost > 0 ? formatCost(key.maxCost) : t("unlimited")}
                </TableCell>
                <TableCell className="text-right">
                  {key.maxCost > 0 ? (
                    <div className="flex items-center justify-end gap-2">
                      <div className="bg-muted h-2 w-16 overflow-hidden rounded-full">
                        <div
                          className={`h-full rounded-full ${
                            percent > 90
                              ? "bg-destructive"
                              : percent > 70
                                ? "bg-yellow-500"
                                : "bg-primary"
                          }`}
                          style={{ width: `${Math.min(percent, 100)}%` }}
                        />
                      </div>
                      <span className="text-muted-foreground text-xs">{percent.toFixed(1)}%</span>
                    </div>
                  ) : (
                    <span className="text-muted-foreground text-xs">—</span>
                  )}
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </div>
  )
}

// ── By Model Tab ──

function ByModelTab() {
  const { t } = useTranslation("usage")
  const { data: modelData, isLoading } = useQuery({
    queryKey: ["stats", "model"],
    queryFn: getModelStats,
  })

  const models = modelData?.data ?? []

  if (isLoading) {
    return (
      <p className="text-muted-foreground py-8 text-center text-sm">
        {t("actions.loading", { ns: "common" })}
      </p>
    )
  }

  if (models.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center gap-2 py-16">
        <BarChart3 className="text-muted-foreground h-10 w-10" />
        <p className="text-muted-foreground font-medium">{t("noData")}</p>
      </div>
    )
  }

  const sorted = [...models].sort((a, b) => b.totalCost - a.totalCost)

  return (
    <div className="overflow-x-auto">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("table.model")}</TableHead>
            <TableHead className="text-right">{t("table.requests")}</TableHead>
            <TableHead className="text-right">{t("table.inputTokens")}</TableHead>
            <TableHead className="text-right">{t("table.outputTokens")}</TableHead>
            <TableHead className="text-right">{t("table.cost")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {sorted.map((m) => (
            <TableRow key={m.model}>
              <TableCell className="font-medium">{m.model}</TableCell>
              <TableCell className="text-right font-mono">{formatNumber(m.requestCount)}</TableCell>
              <TableCell className="text-right font-mono">{formatNumber(m.inputTokens)}</TableCell>
              <TableCell className="text-right font-mono">{formatNumber(m.outputTokens)}</TableCell>
              <TableCell className="text-right font-mono">{formatCost(m.totalCost)}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

// ── By Channel Tab ──

function ByChannelTab() {
  const { t } = useTranslation("usage")
  const { data: channelData, isLoading } = useQuery({
    queryKey: ["stats", "channel"],
    queryFn: getChannelStats,
  })

  const channels = channelData?.data ?? []

  if (isLoading) {
    return (
      <p className="text-muted-foreground py-8 text-center text-sm">
        {t("actions.loading", { ns: "common" })}
      </p>
    )
  }

  if (channels.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center gap-2 py-16">
        <BarChart3 className="text-muted-foreground h-10 w-10" />
        <p className="text-muted-foreground font-medium">{t("noData")}</p>
      </div>
    )
  }

  const sorted = [...channels].sort((a, b) => b.totalCost - a.totalCost)

  return (
    <div className="overflow-x-auto">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("table.channel")}</TableHead>
            <TableHead className="text-right">{t("table.requests")}</TableHead>
            <TableHead className="text-right">{t("table.cost")}</TableHead>
            <TableHead className="text-right">{t("table.cost")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {sorted.map((ch) => (
            <TableRow key={ch.channelId}>
              <TableCell className="font-medium">{ch.channelName}</TableCell>
              <TableCell className="text-right font-mono">
                {formatNumber(ch.totalRequests)}
              </TableCell>
              <TableCell className="text-right font-mono">
                {formatNumber(ch.totalRequests)}
              </TableCell>
              <TableCell className="text-right font-mono">{formatCost(ch.totalCost)}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

// ── Usage Page ──

export default function UsagePage() {
  const { t } = useTranslation("usage")

  const { data: totalData } = useQuery({
    queryKey: ["stats", "total"],
    queryFn: getTotalStats,
  })

  const totals = totalData?.data

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="shrink-0 pb-4">
        <h2 className="text-2xl font-bold tracking-tight">{t("title")}</h2>
        <p className="text-muted-foreground text-sm">{t("description")}</p>
      </div>

      <div className="min-h-0 flex-1 space-y-6 overflow-auto">
        <SummaryCards
          totalCost={(totals?.input_cost ?? 0) + (totals?.output_cost ?? 0)}
          totalRequests={(totals?.request_success ?? 0) + (totals?.request_failed ?? 0)}
          totalTokens={(totals?.input_token ?? 0) + (totals?.output_token ?? 0)}
        />

        <Card>
          <CardContent className="pt-6">
            <Tabs defaultValue="byKey">
              <TabsList variant="line" className="shrink-0">
                <TabsTrigger value="byKey">{t("tabs.byKey")}</TabsTrigger>
                <TabsTrigger value="byModel">{t("tabs.byModel")}</TabsTrigger>
                <TabsTrigger value="byChannel">{t("tabs.byChannel")}</TabsTrigger>
              </TabsList>

              <TabsContent value="byKey" className="pt-4">
                <ByKeyTab />
              </TabsContent>
              <TabsContent value="byModel" className="pt-4">
                <ByModelTab />
              </TabsContent>
              <TabsContent value="byChannel" className="pt-4">
                <ByChannelTab />
              </TabsContent>
            </Tabs>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
