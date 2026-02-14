import type { ChannelStatsRow } from "@/lib/api-client"
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router"
import { formatCount, formatMoney, formatTime } from "@/lib/format"

type ChannelSortKey = "requests" | "cost" | "latency"

export function HubChannelList({ data }: { data?: ChannelStatsRow[] }) {
  const { t } = useTranslation("dashboard")
  const [sortBy, setSortBy] = useState<ChannelSortKey>("requests")

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

  const barPct = (ch: ChannelStatsRow) => {
    switch (sortBy) {
      case "requests":
        return ((ch.totalRequests || 0) / maxVal) * 100
      case "cost":
        return ((ch.totalCost || 0) / maxVal) * 100
      case "latency":
        return ((ch.avgLatency || 0) / maxVal) * 100
    }
  }

  if (!data || data.length === 0) {
    return (
      <p className="text-muted-foreground text-center text-[9px]">{t("channelRanking.noData")}</p>
    )
  }

  return (
    <div className="flex flex-col gap-1">
      <div className="flex gap-0.5">
        {(["requests", "cost", "latency"] as const).map((key) => (
          <button
            key={key}
            onClick={() => setSortBy(key)}
            className={`rounded px-1 py-px text-[7px] font-bold transition-all ${
              sortBy === key
                ? "bg-foreground/10 text-foreground"
                : "text-muted-foreground/50 hover:text-muted-foreground"
            }`}
          >
            {key === "requests" ? "REQ" : key === "cost" ? "$" : "LAT"}
          </button>
        ))}
      </div>
      <div className="max-h-[90px] space-y-px overflow-y-auto">
        {sorted.map((ch) => (
          <Link
            key={ch.channelId}
            to={`/logs?channel=${ch.channelId}`}
            className="hover:bg-muted/40 relative block rounded px-1 py-0.5 transition-colors"
          >
            <div
              className="absolute inset-y-0 left-0 rounded opacity-[0.07]"
              style={{ width: `${barPct(ch)}%`, backgroundColor: "var(--primary)" }}
            />
            <div className="relative flex items-center justify-between gap-1">
              <span className="min-w-0 truncate text-[9px] font-medium">
                {ch.channelName || `#${ch.channelId}`}
              </span>
              <span className="text-muted-foreground flex shrink-0 gap-1.5 text-[7px] tabular-nums">
                <span>{formatCount(ch.totalRequests).value}r</span>
                <span>{formatMoney(ch.totalCost).value}</span>
                <span>
                  {formatTime(ch.avgLatency).value}
                  {formatTime(ch.avgLatency).unit}
                </span>
              </span>
            </div>
          </Link>
        ))}
      </div>
    </div>
  )
}
