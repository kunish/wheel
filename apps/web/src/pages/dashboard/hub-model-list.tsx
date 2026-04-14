import type { HubListItem, HubListSortOption } from "./hub-list"
import type { ModelStatsItem } from "@/lib/api"
import { useCallback, useMemo } from "react"
import { useTranslation } from "react-i18next"
import { formatCount, formatMoney, formatTime } from "@/lib/format"
import { HubList } from "./hub-list"

type ModelSortKey = "requests" | "tokens" | "cost" | "latency"

const SORT_OPTIONS: HubListSortOption<ModelSortKey>[] = [
  { key: "requests", label: "REQ" },
  { key: "tokens", label: "TOK" },
  { key: "cost", label: "$" },
  { key: "latency", label: "LAT" },
]

export function HubModelList({ data }: { data?: ModelStatsItem[] }) {
  const { t } = useTranslation("dashboard")

  const items = useMemo<HubListItem[] | undefined>(() => {
    if (!data) return undefined
    return data.map((item) => ({
      id: item.model,
      label: item.model.split("/").pop() ?? item.model,
      link: `/logs?model=${encodeURIComponent(item.model)}`,
      stats: (
        <>
          <span>{formatCount(item.requestCount).value}r</span>
          <span>{formatMoney(item.totalCost).value}</span>
          <span>
            {formatTime(item.avgLatency).value}
            {formatTime(item.avgLatency).unit}
          </span>
        </>
      ),
      _raw: item,
    }))
  }, [data])

  const getSortValue = useCallback(
    (item: HubListItem, sortKey: ModelSortKey) => {
      const raw = data?.find((d) => d.model === item.id)
      if (!raw) return 0
      switch (sortKey) {
        case "requests":
          return raw.requestCount
        case "tokens":
          return raw.inputTokens + raw.outputTokens
        case "cost":
          return raw.totalCost
        case "latency":
          return raw.avgLatency
      }
    },
    [data],
  )

  return (
    <HubList
      sortOptions={SORT_OPTIONS}
      defaultSort="requests"
      noDataMessage={t("modelUsage.noData")}
      items={items}
      getSortValue={getSortValue}
    />
  )
}
