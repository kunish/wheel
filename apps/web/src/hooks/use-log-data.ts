import type { LogEntry } from "@/pages/logs/columns"
import type { LogStats } from "@/pages/logs/types"
import { keepPreviousData, useQuery } from "@tanstack/react-query"
import { useMemo } from "react"
import { listChannels as apiListChannels, getModelList, listLogs } from "@/lib/api"

interface LogDataParams {
  page: number
  pageSize: number
  model: string
  status: string
  channelId: number | undefined
  keyword: string
  startTime: number | undefined
  endTime: number | undefined
}

export function useLogData(params: LogDataParams, pendingStreams: Map<string, LogEntry>) {
  const { page, pageSize, model, status, channelId, keyword, startTime, endTime } = params

  const { data, isLoading, isFetching, isError, refetch } = useQuery({
    queryKey: ["logs", page, pageSize, model, status, channelId, keyword, startTime, endTime],
    queryFn: () =>
      listLogs({
        page,
        pageSize,
        ...(model ? { model } : {}),
        ...(status !== "all" ? { status } : {}),
        ...(channelId ? { channelId } : {}),
        ...(keyword ? { keyword } : {}),
        ...(startTime ? { startTime } : {}),
        ...(endTime ? { endTime } : {}),
      }),
    placeholderData: keepPreviousData,
  })

  const { data: channelsData } = useQuery({
    queryKey: ["channels-for-filter"],
    queryFn: apiListChannels,
    staleTime: 5 * 60 * 1000,
  })
  const channels = (channelsData?.data?.channels ?? []) as Array<{ id: number; name: string }>

  const { data: modelsData } = useQuery({
    queryKey: ["models-for-filter"],
    queryFn: getModelList,
    staleTime: 5 * 60 * 1000,
  })
  const modelOptions = (modelsData?.data?.models ?? []) as string[]

  const logs = useMemo(() => {
    const dbLogs = (data?.data?.logs ?? []) as LogEntry[]
    if (pendingStreams.size === 0) return dbLogs
    const pending = Array.from(pendingStreams.values()).sort((a, b) => b.time - a.time)
    return [...pending, ...dbLogs]
  }, [data, pendingStreams])

  const total = data?.data?.total ?? 0
  const totalPages = Math.ceil(total / pageSize)

  const stats = useMemo<LogStats>(() => {
    // Only compute stats from non-streaming, completed logs
    const completedLogs = logs.filter((l) => !l._streaming)
    const count = completedLogs.length

    if (count === 0) {
      return {
        totalRequests: total,
        successRate: 0,
        averageLatency: 0,
        totalTokens: 0,
        totalCost: 0,
        tokenSpeed: 0,
      }
    }

    const successCount = completedLogs.filter((l) => !l.error).length
    const successRate = (successCount / count) * 100

    const totalLatency = completedLogs.reduce((sum, l) => sum + l.useTime, 0)
    const averageLatency = totalLatency / count

    const totalTokens = completedLogs.reduce((sum, l) => sum + l.inputTokens + l.outputTokens, 0)

    const totalCost = completedLogs.reduce((sum, l) => sum + (l.cost ?? 0), 0)

    // Token speed: average output tokens per second across logs with valid useTime
    const speedLogs = completedLogs.filter((l) => l.useTime > 0 && l.outputTokens > 0)
    const tokenSpeed =
      speedLogs.length > 0
        ? speedLogs.reduce((sum, l) => sum + l.outputTokens / (l.useTime / 1000), 0) /
          speedLogs.length
        : 0

    return {
      totalRequests: total,
      successRate,
      averageLatency,
      totalTokens,
      totalCost,
      tokenSpeed,
    }
  }, [logs, total])

  return {
    logs,
    total,
    totalPages,
    stats,
    isLoading,
    isFetching,
    isError,
    refetch,
    channels,
    modelOptions,
  }
}
