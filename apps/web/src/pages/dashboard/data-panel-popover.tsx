import type { ChannelStatsRow, ModelStatsItem, StatsDaily, StatsHourly } from "@/lib/api-client"
import { Bot, Radio, TrendingUp } from "lucide-react"
import { useMemo, useRef } from "react"
import { Area, AreaChart, ResponsiveContainer } from "recharts"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { formatCount, formatMoney } from "@/lib/format"
import { HubChannelList } from "./hub-channel-list"
import { HubModelList } from "./hub-model-list"
import { computePeriodTotals } from "./types"

type DataTab = "cost" | "models" | "channels"

function HubCostChart({
  dailyData,
  hourlyData,
}: {
  dailyData?: StatsDaily[]
  hourlyData?: StatsHourly[]
}) {
  const sortedDaily = useMemo(() => {
    if (!dailyData) return []
    return [...dailyData].sort((a, b) => a.date.localeCompare(b.date))
  }, [dailyData])

  const chartData = useMemo(() => {
    if (hourlyData && hourlyData.length > 0) {
      return hourlyData.map((s) => ({
        date: `${s.hour}h`,
        cost: s.input_cost + s.output_cost,
      }))
    }
    return sortedDaily.slice(-7).map((s) => ({
      date: `${s.date.slice(4, 6)}/${s.date.slice(6)}`,
      cost: s.input_cost + s.output_cost,
    }))
  }, [sortedDaily, hourlyData])

  const periodTotals = useMemo(() => {
    const source = hourlyData && hourlyData.length > 0 ? hourlyData : sortedDaily.slice(-7)
    return computePeriodTotals(source)
  }, [hourlyData, sortedDaily])

  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex justify-between text-[10px] tabular-nums">
        {[
          { label: "REQ", value: formatCount(periodTotals.req) },
          { label: "IN", value: formatCount(periodTotals.inTok) },
          { label: "OUT", value: formatCount(periodTotals.outTok) },
          { label: "$", value: formatMoney(periodTotals.cost) },
        ].map((m) => (
          <div key={m.label} className="flex flex-col items-center">
            <span className="text-muted-foreground text-[10px] font-bold">{m.label}</span>
            <span className="font-bold">
              {m.value.value}
              {m.value.unit && (
                <span className="text-muted-foreground ml-px text-[10px]">{m.value.unit}</span>
              )}
            </span>
          </div>
        ))}
      </div>
      <ResponsiveContainer width="100%" height={60}>
        <AreaChart data={chartData} margin={{ left: 0, right: 0, top: 2, bottom: 0 }}>
          <defs>
            <linearGradient id="fillCostMini" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="var(--primary)" stopOpacity={0.6} />
              <stop offset="95%" stopColor="var(--primary)" stopOpacity={0.05} />
            </linearGradient>
          </defs>
          <Area
            type="monotone"
            dataKey="cost"
            stroke="var(--primary)"
            strokeWidth={1.5}
            fill="url(#fillCostMini)"
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  )
}

export interface DataPanelPopoverProps {
  dataTab: DataTab | null
  setDataTab: React.Dispatch<React.SetStateAction<DataTab | null>>
  dailyData?: StatsDaily[]
  hourlyData?: StatsHourly[]
  modelData?: ModelStatsItem[]
  channelData?: ChannelStatsRow[]
}

export function DataPanelPopover({
  dataTab,
  setDataTab,
  dailyData,
  hourlyData,
  modelData,
  channelData,
}: DataPanelPopoverProps) {
  const lastTabRef = useRef<DataTab>("cost")
  if (dataTab !== null) lastTabRef.current = dataTab
  const displayTab = dataTab ?? lastTabRef.current

  return (
    <div>
      <Popover
        open={dataTab !== null}
        onOpenChange={(open) => {
          if (!open) setDataTab(null)
        }}
      >
        <PopoverTrigger asChild>
          <button
            onClick={() => setDataTab((prev) => (prev === null ? "cost" : null))}
            className={`rounded-md p-1.5 transition-colors ${
              dataTab !== null
                ? "bg-primary text-primary-foreground shadow-sm"
                : "text-muted-foreground hover:bg-muted hover:text-foreground"
            }`}
          >
            <TrendingUp className="h-3.5 w-3.5" />
          </button>
        </PopoverTrigger>
        <PopoverContent side="top" align="center" className="w-[360px] p-4">
          <div className="mb-3 flex gap-1">
            {[
              { key: "cost" as const, Icon: TrendingUp },
              { key: "models" as const, Icon: Bot },
              { key: "channels" as const, Icon: Radio },
            ].map(({ key, Icon }) => (
              <button
                key={key}
                onClick={() => setDataTab(key)}
                className={`rounded-md p-1.5 transition-colors ${
                  displayTab === key
                    ? "bg-primary text-primary-foreground shadow-sm"
                    : "text-muted-foreground hover:bg-muted hover:text-foreground"
                }`}
              >
                <Icon className="h-4 w-4" />
              </button>
            ))}
          </div>
          <div>
            {displayTab === "cost" && (
              <HubCostChart dailyData={dailyData} hourlyData={hourlyData} />
            )}
            {displayTab === "models" && <HubModelList data={modelData} />}
            {displayTab === "channels" && <HubChannelList data={channelData} />}
          </div>
        </PopoverContent>
      </Popover>
    </div>
  )
}
