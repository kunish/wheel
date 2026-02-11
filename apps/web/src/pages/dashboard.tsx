import type { StreamIncrement } from "./dashboard/types"
import { useQuery } from "@tanstack/react-query"
import { AlertCircle, RefreshCw } from "lucide-react"
import { useMemo, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { useWsEvent } from "@/hooks/use-stats-ws"
import {
  getChannelStats,
  getDailyStats,
  getHourlyStats,
  getModelStats,
  getTotalStats,
} from "@/lib/api"
import { ActivitySection } from "./dashboard/activity-section"

// ───────────── Inline Error ─────────────

function InlineError({ message, onRetry }: { message: string; onRetry: () => void }) {
  const { t } = useTranslation("common")
  return (
    <Card className="flex flex-col items-center justify-center gap-3 py-10">
      <AlertCircle className="text-destructive h-8 w-8" />
      <p className="text-muted-foreground text-sm">{message}</p>
      <Button variant="outline" size="sm" className="gap-1.5" onClick={onRetry}>
        <RefreshCw className="h-3.5 w-3.5" />
        {t("actions.retry")}
      </Button>
    </Card>
  )
}

// ───────────── Page ─────────────

export default function DashboardPage() {
  const { t } = useTranslation("dashboard")

  const streamIncrementsRef = useRef(new Map<string, StreamIncrement>())
  const [incrementVersion, setIncrementVersion] = useState(0)

  useWsEvent("log-stream-start", (data) => {
    if (!data?.streamId) return
    streamIncrementsRef.current.set(data.streamId, {
      estimatedInputTokens: data.estimatedInputTokens ?? 0,
      outputTokens: 0,
      cost: 0,
      inputPrice: data.inputPrice ?? 0,
      outputPrice: data.outputPrice ?? 0,
    })
    setIncrementVersion((v) => v + 1)
  })

  useWsEvent("log-streaming", (data) => {
    if (!data?.streamId) return
    const entry = streamIncrementsRef.current.get(data.streamId)
    if (!entry) return
    const contentLen = (data.responseLength ?? 0) + (data.thinkingLength ?? 0)
    const outputTokens = Math.floor(contentLen / 3)
    const cost =
      (entry.estimatedInputTokens * entry.inputPrice + outputTokens * entry.outputPrice) / 1_000_000
    streamIncrementsRef.current.set(data.streamId, {
      ...entry,
      outputTokens,
      cost,
    })
    setIncrementVersion((v) => v + 1)
  })

  useWsEvent("log-created", (data) => {
    if (!data?.streamId) return
    streamIncrementsRef.current.delete(data.streamId)
    setIncrementVersion((v) => v + 1)
  })

  useWsEvent("log-stream-end", (data) => {
    if (!data?.streamId) return
    streamIncrementsRef.current.delete(data.streamId)
    setIncrementVersion((v) => v + 1)
  })

  const streamingDelta = useMemo(() => {
    void incrementVersion
    let inputTokens = 0
    let outputTokens = 0
    let inputCost = 0
    let outputCost = 0
    for (const entry of streamIncrementsRef.current.values()) {
      inputTokens += entry.estimatedInputTokens
      outputTokens += entry.outputTokens
      inputCost += (entry.estimatedInputTokens * entry.inputPrice) / 1_000_000
      outputCost += (entry.outputTokens * entry.outputPrice) / 1_000_000
    }
    return { inputTokens, outputTokens, inputCost, outputCost }
  }, [incrementVersion])

  const {
    data: totalData,
    isError: isTotalError,
    refetch: refetchTotal,
  } = useQuery({ queryKey: ["stats", "total"], queryFn: getTotalStats })

  const {
    data: dailyData,
    isError: isDailyError,
    refetch: refetchDaily,
  } = useQuery({ queryKey: ["stats", "daily"], queryFn: getDailyStats })

  const {
    data: hourlyData,
    isError: isHourlyError,
    refetch: refetchHourly,
  } = useQuery({ queryKey: ["stats", "hourly"], queryFn: () => getHourlyStats() })

  const { data: channelData } = useQuery({
    queryKey: ["stats", "channel"],
    queryFn: getChannelStats,
  })

  const { data: modelData } = useQuery({
    queryKey: ["stats", "model"],
    queryFn: getModelStats,
  })

  const isStatsError = isTotalError || isDailyError || isHourlyError
  const refetchStats = () => {
    if (isTotalError) refetchTotal()
    if (isDailyError) refetchDaily()
    if (isHourlyError) refetchHourly()
  }

  if (isStatsError) {
    return <InlineError message={t("errors.dashboardStats")} onRetry={refetchStats} />
  }

  return (
    <div className="mx-auto flex h-full w-full max-w-[960px] flex-col">
      <ActivitySection
        data={dailyData?.data}
        totalData={totalData?.data}
        streamingDelta={streamingDelta}
        hourlyData={hourlyData?.data}
        modelData={modelData?.data}
        channelData={channelData?.data}
      />
    </div>
  )
}
