import type { ModelStatsItem } from "@/lib/api-client"
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router"
import { formatCount, formatMoney, formatTime } from "@/lib/format"

type ModelSortKey = "requests" | "tokens" | "cost" | "latency"

export function HubModelList({ data }: { data?: ModelStatsItem[] }) {
  const { t } = useTranslation("dashboard")
  const [sortBy, setSortBy] = useState<ModelSortKey>("requests")

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
    const first = sorted[0]
    switch (sortBy) {
      case "requests":
        return first.requestCount || 1
      case "tokens":
        return first.inputTokens + first.outputTokens || 1
      case "cost":
        return first.totalCost || 1
      case "latency":
        return first.avgLatency || 1
    }
  }, [sorted, sortBy])

  const barPct = (item: ModelStatsItem) => {
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

  if (!data || data.length === 0) {
    return <p className="text-muted-foreground text-center text-[9px]">{t("modelUsage.noData")}</p>
  }

  return (
    <div className="flex flex-col gap-1">
      <div className="flex gap-0.5">
        {(["requests", "tokens", "cost", "latency"] as const).map((key) => (
          <button
            key={key}
            onClick={() => setSortBy(key)}
            className={`rounded px-1 py-px text-[7px] font-bold transition-all ${
              sortBy === key
                ? "bg-foreground/10 text-foreground"
                : "text-muted-foreground/50 hover:text-muted-foreground"
            }`}
          >
            {key === "requests" ? "REQ" : key === "tokens" ? "TOK" : key === "cost" ? "$" : "LAT"}
          </button>
        ))}
      </div>
      <div className="max-h-[90px] space-y-px overflow-y-auto">
        {sorted.map((item) => (
          <Link
            key={item.model}
            to={`/logs?model=${encodeURIComponent(item.model)}`}
            className="hover:bg-muted/40 relative block rounded px-1 py-0.5 transition-colors"
          >
            <div
              className="absolute inset-y-0 left-0 rounded opacity-[0.07]"
              style={{ width: `${barPct(item)}%`, backgroundColor: "var(--primary)" }}
            />
            <div className="relative flex items-center justify-between gap-1">
              <span className="min-w-0 truncate text-[9px] font-medium">
                {item.model.split("/").pop()}
              </span>
              <span className="text-muted-foreground flex shrink-0 gap-1.5 text-[7px] tabular-nums">
                <span>{formatCount(item.requestCount).value}r</span>
                <span>{formatMoney(item.totalCost).value}</span>
                <span>
                  {formatTime(item.avgLatency).value}
                  {formatTime(item.avgLatency).unit}
                </span>
              </span>
            </div>
          </Link>
        ))}
      </div>
    </div>
  )
}
