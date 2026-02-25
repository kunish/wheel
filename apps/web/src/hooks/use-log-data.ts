import type { LogEntry } from "@/pages/logs/columns"
import { keepPreviousData, useQuery } from "@tanstack/react-query"
import { useMemo } from "react"
import { listChannels as apiListChannels, getModelList, listLogs } from "@/lib/api-client"

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

  return {
    logs,
    total,
    totalPages,
    isLoading,
    isFetching,
    isError,
    refetch,
    channels,
    modelOptions,
  }
}
