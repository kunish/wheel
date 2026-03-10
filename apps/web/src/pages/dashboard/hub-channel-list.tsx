import type { HubListItem, HubListSortOption } from "./hub-list"
import type { ChannelStatsRow } from "@/lib/api"
import { useCallback, useMemo } from "react"
import { useTranslation } from "react-i18next"
import { formatCount, formatMoney, formatTime } from "@/lib/format"
import { HubList } from "./hub-list"

type ChannelSortKey = "requests" | "cost" | "latency"

const SORT_OPTIONS: HubListSortOption<ChannelSortKey>[] = [
  { key: "requests", label: "REQ" },
  { key: "cost", label: "$" },
  { key: "latency", label: "LAT" },
]

export function HubChannelList({ data }: { data?: ChannelStatsRow[] }) {
  const { t } = useTranslation("dashboard")

  const items = useMemo<HubListItem[] | undefined>(() => {
    if (!data) return undefined
    return data.map((ch) => ({
      id: ch.channelId,
      label: ch.channelName || `#${ch.channelId}`,
      link: `/logs?channel=${ch.channelId}`,
      stats: (
        <>
          <span>{formatCount(ch.totalRequests).value}r</span>
          <span>{formatMoney(ch.totalCost).value}</span>
          <span>
            {formatTime(ch.avgLatency).value}
            {formatTime(ch.avgLatency).unit}
          </span>
        </>
      ),
    }))
  }, [data])

  const getSortValue = useCallback(
    (item: HubListItem, sortKey: ChannelSortKey) => {
      const raw = data?.find((d) => d.channelId === item.id)
      if (!raw) return 0
      switch (sortKey) {
        case "requests":
          return raw.totalRequests || 0
        case "cost":
          return raw.totalCost || 0
        case "latency":
          return raw.avgLatency || 0
      }
    },
    [data],
  )

  return (
    <HubList
      sortOptions={SORT_OPTIONS}
      defaultSort="requests"
      noDataMessage={t("channelRanking.noData")}
      items={items}
      getSortValue={getSortValue}
    />
  )
}
