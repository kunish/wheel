import type { ChannelSortKey, RankSortKey } from "./types"
import type { ChannelStatsRow, ModelStatsItem } from "@/lib/api-client"
import { Bot } from "lucide-react"
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router"
import { ModelBadge } from "@/components/model-badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { formatCount, formatMoney, formatTime } from "@/lib/format"
import { Fmt } from "./types"

// ───────────── Channel Ranking ─────────────

export function RankSection({ data }: { data?: ChannelStatsRow[] }) {
  const { t } = useTranslation("dashboard")
  const [sortBy, setSortBy] = useState<ChannelSortKey>("requests")

  const channelSortOptions = useMemo(
    () => [
      { key: "requests" as const, label: t("sort.requests") },
      { key: "cost" as const, label: t("sort.cost") },
      { key: "latency" as const, label: t("sort.latency") },
    ],
    [t],
  )

  const sorted = useMemo(() => {
    if (!data) return []
    return [...data].sort((a, b) => {
      switch (sortBy) {
        case "requests":
          return (b.totalRequests || 0) - (a.totalRequests || 0)
        case "cost":
          return (b.totalCost || 0) - (a.totalCost || 0)
        case "latency":
          return (b.avgLatency || 0) - (a.avgLatency || 0)
        default:
          return 0
      }
    })
  }, [data, sortBy])

  const maxVal = useMemo(() => {
    if (sorted.length === 0) return 1
    switch (sortBy) {
      case "requests":
        return sorted[0].totalRequests || 1
      case "cost":
        return sorted[0].totalCost || 1
      case "latency":
        return sorted[0].avgLatency || 1
    }
  }, [sorted, sortBy])

  const barPercent = (ch: ChannelStatsRow) => {
    switch (sortBy) {
      case "requests":
        return ((ch.totalRequests || 0) / maxVal) * 100
      case "cost":
        return ((ch.totalCost || 0) / maxVal) * 100
      case "latency":
        return ((ch.avgLatency || 0) / maxVal) * 100
    }
  }

  return (
    <Card>
      <Tabs value={sortBy} onValueChange={(v) => setSortBy(v as ChannelSortKey)}>
        <CardHeader className="flex flex-col gap-2 pb-2">
          <div className="flex items-center justify-between">
            <CardTitle className="text-base">{t("channelRanking.title")}</CardTitle>
          </div>
          <TabsList>
            {channelSortOptions.map((o) => (
              <TabsTrigger key={o.key} value={o.key}>
                {o.label}
              </TabsTrigger>
            ))}
          </TabsList>
        </CardHeader>
        <CardContent>
          {sorted.length === 0 ? (
            <div className="text-muted-foreground flex flex-col items-center justify-center py-8">
              <Bot className="mb-3 h-12 w-12 opacity-30" />
              <p className="text-sm">{t("channelRanking.noData")}</p>
            </div>
          ) : (
            <div className="max-h-[400px] space-y-1.5 overflow-y-auto">
              {sorted.map((ch) => {
                const reqFmt = formatCount(ch.totalRequests)
                const costFmt = formatMoney(ch.totalCost)
                const latFmt = formatTime(ch.avgLatency)
                return (
                  <Link
                    key={ch.channelId}
                    to={`/logs?channel=${ch.channelId}`}
                    className="hover:bg-muted/50 relative block rounded-md px-3 py-2 transition-colors"
                  >
                    <div
                      className="absolute inset-y-0 left-0 rounded-md opacity-10 transition-[width] duration-500 ease-out"
                      style={{
                        width: `${barPercent(ch)}%`,
                        backgroundColor: "var(--primary)",
                      }}
                    />
                    <div className="relative flex flex-col gap-1">
                      <div className="min-w-0 truncate text-sm font-medium">
                        {ch.channelName ||
                          t("channelRanking.channelFallback", { id: ch.channelId })}
                      </div>
                      <div className="flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs tabular-nums">
                        <div>
                          <span className="text-muted-foreground">{t("inline.req")} </span>
                          <Fmt fmt={reqFmt} />
                        </div>
                        <div>
                          <span className="text-muted-foreground">{t("inline.cost")} </span>
                          <Fmt fmt={costFmt} />
                        </div>
                        <div>
                          <span className="text-muted-foreground">{t("inline.avg")} </span>
                          <Fmt fmt={latFmt} />
                        </div>
                      </div>
                    </div>
                  </Link>
                )
              })}
            </div>
          )}
        </CardContent>
      </Tabs>
    </Card>
  )
}

// ───────────── Model Usage Stats ─────────────

export function ModelStatsSection({ data }: { data?: ModelStatsItem[] }) {
  const { t } = useTranslation("dashboard")
  const [sortBy, setSortBy] = useState<RankSortKey>("requests")

  const rankSortOptions = useMemo(
    () => [
      { key: "requests" as const, label: t("sort.requests") },
      { key: "tokens" as const, label: t("sort.tokens") },
      { key: "cost" as const, label: t("sort.cost") },
      { key: "latency" as const, label: t("sort.latency") },
    ],
    [t],
  )

  const sorted = useMemo(() => {
    if (!data) return []
    return [...data].sort((a, b) => {
      switch (sortBy) {
        case "requests":
          return b.requestCount - a.requestCount
        case "tokens":
          return b.inputTokens + b.outputTokens - (a.inputTokens + a.outputTokens)
        case "cost":
          return b.totalCost - a.totalCost
        case "latency":
          return b.avgLatency - a.avgLatency
        default:
          return 0
      }
    })
  }, [data, sortBy])

  const maxVal = useMemo(() => {
    if (sorted.length === 0) return 1
    switch (sortBy) {
      case "requests":
        return sorted[0].requestCount || 1
      case "tokens":
        return sorted[0].inputTokens + sorted[0].outputTokens || 1
      case "cost":
        return sorted[0].totalCost || 1
      case "latency":
        return sorted[0].avgLatency || 1
    }
  }, [sorted, sortBy])

  const barPercent = (item: ModelStatsItem) => {
    switch (sortBy) {
      case "requests":
        return (item.requestCount / maxVal) * 100
      case "tokens":
        return ((item.inputTokens + item.outputTokens) / maxVal) * 100
      case "cost":
        return (item.totalCost / maxVal) * 100
      case "latency":
        return (item.avgLatency / maxVal) * 100
    }
  }

  return (
    <Card>
      <Tabs value={sortBy} onValueChange={(v) => setSortBy(v as RankSortKey)}>
        <CardHeader className="flex flex-col gap-2 pb-2">
          <div className="flex items-center justify-between">
            <CardTitle className="text-base">{t("modelUsage.title")}</CardTitle>
          </div>
          <TabsList>
            {rankSortOptions.map((o) => (
              <TabsTrigger key={o.key} value={o.key}>
                {o.label}
              </TabsTrigger>
            ))}
          </TabsList>
        </CardHeader>
        <CardContent>
          {sorted.length === 0 ? (
            <div className="text-muted-foreground flex flex-col items-center justify-center py-8">
              <Bot className="mb-3 h-12 w-12 opacity-30" />
              <p className="text-sm">{t("modelUsage.noData")}</p>
            </div>
          ) : (
            <div className="max-h-[400px] space-y-1.5 overflow-y-auto">
              {sorted.map((item) => {
                const reqFmt = formatCount(item.requestCount)
                const inFmt = formatCount(item.inputTokens)
                const outFmt = formatCount(item.outputTokens)
                const costFmt = formatMoney(item.totalCost)
                const latFmt = formatTime(item.avgLatency)
                return (
                  <Link
                    key={item.model}
                    to={`/logs?model=${encodeURIComponent(item.model)}`}
                    className="hover:bg-muted/50 relative block rounded-md px-3 py-2 transition-colors"
                  >
                    {/* Background bar */}
                    <div
                      className="absolute inset-y-0 left-0 rounded-md opacity-10 transition-[width] duration-500 ease-out"
                      style={{
                        width: `${barPercent(item)}%`,
                        backgroundColor: "var(--primary)",
                      }}
                    />
                    <div className="relative flex flex-col gap-1">
                      <div className="min-w-0 truncate text-sm font-medium">
                        <ModelBadge modelId={item.model} />
                      </div>
                      <div className="flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs tabular-nums">
                        <div>
                          <span className="text-muted-foreground">{t("inline.req")} </span>
                          <Fmt fmt={reqFmt} />
                        </div>
                        <div>
                          <span className="text-muted-foreground">{t("inline.in")} </span>
                          <Fmt fmt={inFmt} />
                        </div>
                        <div>
                          <span className="text-muted-foreground">{t("inline.out")} </span>
                          <Fmt fmt={outFmt} />
                        </div>
                        <div>
                          <span className="text-muted-foreground">{t("inline.cost")} </span>
                          <Fmt fmt={costFmt} />
                        </div>
                        <div>
                          <span className="text-muted-foreground">{t("inline.avg")} </span>
                          <Fmt fmt={latFmt} />
                        </div>
                      </div>
                    </div>
                  </Link>
                )
              })}
            </div>
          )}
        </CardContent>
      </Tabs>
    </Card>
  )
}
