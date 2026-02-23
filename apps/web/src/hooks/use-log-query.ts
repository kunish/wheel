import type { LogEntry } from "@/pages/logs/columns"
import type { LogDetail } from "@/pages/logs/types"
import { keepPreviousData, useQuery, useQueryClient } from "@tanstack/react-query"
import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { useLocation, useNavigate, useSearchParams } from "react-router"
import { useWsEvent } from "@/hooks/use-stats-ws"
import { listChannels as apiListChannels, getLog, getModelList, listLogs } from "@/lib/api-client"
import { buildFilterSearchParams, parseLogFilters } from "@/pages/logs/log-filters"

export function useLogQuery() {
  const queryClient = useQueryClient()
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const { pathname } = useLocation()

  // Derive filter state from URL search params
  const filters = parseLogFilters(searchParams)
  const { page, model, status, channelId, keyword, pageSize, startTime, endTime } = filters

  // Local state for controlled text inputs (synced to URL via debounce)
  const [keywordInput, setKeywordInput] = useState(keyword)

  // Sync local input state when URL changes externally (e.g., deep links from dashboard)
  const prevKeywordRef = useRef(keyword)
  if (prevKeywordRef.current !== keyword) {
    prevKeywordRef.current = keyword
    setKeywordInput(keyword)
  }

  // Helper to update URL search params — resets page to 1 unless page itself is being updated
  const updateFilter = useCallback(
    (updates: Record<string, string | number | undefined | null>) => {
      const params = buildFilterSearchParams(searchParams, updates)
      const query = params.toString()
      navigate(query ? `${pathname}?${query}` : pathname, { replace: true })
    },
    [searchParams, pathname, navigate],
  )

  // Debounced sync for text inputs
  const debounceRef = useRef<ReturnType<typeof setTimeout>>(null)
  const debouncedUpdateFilter = useCallback(
    (key: string, value: string) => {
      if (debounceRef.current) clearTimeout(debounceRef.current)
      debounceRef.current = setTimeout(() => {
        updateFilter({ [key]: value || undefined })
      }, 300)
    },
    [updateFilter],
  )

  const [detailId, setDetailId] = useState<number | null>(null)
  const detailIdRef = useRef(detailId)
  detailIdRef.current = detailId
  const [detailTab, setDetailTab] = useState("overview")

  // Track which streamId the detail panel is viewing (null = viewing a DB log)
  const [detailStreamId, setDetailStreamId] = useState<string | null>(null)
  const detailStreamIdRef = useRef(detailStreamId)
  detailStreamIdRef.current = detailStreamId

  // Streaming overlay: real-time content from log-streaming WS events
  const [streamingOverlay, setStreamingOverlay] = useState<{
    thinkingContent: string
    responseContent: string
  } | null>(null)
  // Clear streaming overlay when switching detail panels
  const prevDetailIdRef = useRef(detailId)
  if (prevDetailIdRef.current !== detailId) {
    prevDetailIdRef.current = detailId
    setStreamingOverlay(null)
  }

  const [pendingStreams, setPendingStreams] = useState<Map<string, LogEntry>>(new Map())
  const [pendingCount, setPendingCount] = useState(0)

  const hasFilters =
    model !== "" ||
    status !== "all" ||
    keyword !== "" ||
    channelId !== undefined ||
    startTime !== undefined
  const isFirstPage = page === 1

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

  const { data: detailData } = useQuery({
    queryKey: ["log-detail", detailId],
    queryFn: () => getLog(detailId!),
    enabled: detailId !== null && detailStreamId === null,
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

  // Listen for log-created WebSocket events (reuses global WS connection)
  const filtersRef = useRef({
    page,
    pageSize,
    model,
    status,
    channelId,
    keyword,
    startTime,
    endTime,
    isFirstPage,
    hasFilters,
  })
  filtersRef.current = {
    page,
    pageSize,
    model,
    status,
    channelId,
    keyword,
    startTime,
    endTime,
    isFirstPage,
    hasFilters,
  }

  // Listen for log-stream-start: create a pending entry for the streaming request
  useWsEvent("log-stream-start", (data) => {
    if (!data?.streamId) return
    const f = filtersRef.current
    if (!f.isFirstPage || f.hasFilters) return
    setPendingStreams((prev) => {
      const next = new Map(prev)
      next.set(data.streamId, {
        id: -Date.now(),
        time: data.time ?? Math.floor(Date.now() / 1000),
        requestModelName: data.requestModelName ?? "",
        actualModelName: data.actualModelName ?? "",
        channelId: data.channelId ?? 0,
        channelName: data.channelName ?? "",
        inputTokens: data.estimatedInputTokens ?? 0,
        outputTokens: 0,
        ftut: 0,
        useTime: 0,
        cost: 0,
        error: "",
        totalAttempts: 0,
        _streaming: true,
        _streamId: data.streamId,
        _startedAt: Date.now(),
        _inputPrice: data.inputPrice ?? 0,
        _outputPrice: data.outputPrice ?? 0,
        _estimatedInputTokens: data.estimatedInputTokens ?? 0,
        _requestContent: data.requestContent ?? "",
      })
      return next
    })
  })

  // Listen for log-streaming WS events: update pending entry useTime + streaming overlay for detail panel
  useWsEvent("log-streaming", (data) => {
    if (!data?.streamId) return

    // Update pending entry useTime, estimated tokens, and cost
    setPendingStreams((prev) => {
      const entry = prev.get(data.streamId)
      if (!entry) return prev
      const next = new Map(prev)
      const contentLen = (data.responseLength ?? 0) + (data.thinkingLength ?? 0)
      const estimatedOutputTokens = Math.floor(contentLen / 3)
      const inputPrice = (entry as any)._inputPrice ?? 0
      const outputPrice = (entry as any)._outputPrice ?? 0
      const estimatedInputTokens = (entry as any)._estimatedInputTokens ?? 0
      const estimatedCost =
        (estimatedInputTokens * inputPrice + estimatedOutputTokens * outputPrice) / 1_000_000
      next.set(data.streamId, {
        ...entry,
        useTime: Date.now() - (entry._startedAt ?? Date.now()),
        outputTokens: estimatedOutputTokens,
        cost: estimatedCost,
      })
      return next
    })

    // Streaming overlay for detail panel (when viewing a pending stream)
    const currentStreamId = detailStreamIdRef.current
    if (currentStreamId === data.streamId) {
      setStreamingOverlay({
        thinkingContent: data.thinkingContent ?? "",
        responseContent: data.responseContent ?? "",
      })
    }
  })

  useWsEvent("log-created", (data) => {
    if (!data?.log) return

    // Remove corresponding pending stream entry
    if (data.streamId) {
      setPendingStreams((prev) => {
        if (!prev.has(data.streamId)) return prev
        const next = new Map(prev)
        next.delete(data.streamId)
        return next
      })

      // If detail panel is viewing this stream, switch to the real log ID
      if (detailStreamIdRef.current === data.streamId) {
        setDetailStreamId(null)
        setDetailId(data.log.id)
        setStreamingOverlay(null)
      }
    }

    // Clear streaming overlay & refresh detail panel if viewing this log
    const currentDetailId = detailIdRef.current
    if (currentDetailId !== null && data.log.id === currentDetailId) {
      setStreamingOverlay(null)
      queryClient.invalidateQueries({ queryKey: ["log-detail", currentDetailId] })
    }

    const f = filtersRef.current
    if (f.isFirstPage && !f.hasFilters) {
      queryClient.setQueryData(
        [
          "logs",
          f.page,
          f.pageSize,
          f.model,
          f.status,
          f.channelId,
          f.keyword,
          f.startTime,
          f.endTime,
        ],
        (
          old:
            | { data?: { logs: LogEntry[]; total: number; page: number; pageSize: number } }
            | undefined,
        ) => {
          if (!old?.data) return old
          const newLogs = [data.log as LogEntry, ...old.data.logs].slice(0, f.pageSize)
          return {
            ...old,
            data: {
              ...old.data,
              logs: newLogs,
              total: old.data.total + 1,
            },
          }
        },
      )
    } else {
      setPendingCount((c) => c + 1)
    }
  })

  // Listen for log-stream-end: remove pending entry (failed/exhausted stream)
  useWsEvent("log-stream-end", (data) => {
    if (!data?.streamId) return
    setPendingStreams((prev) => {
      if (!prev.has(data.streamId)) return prev
      const next = new Map(prev)
      next.delete(data.streamId)
      return next
    })
  })

  // Reset pending count when navigating to page 1 or clearing filters
  const prevFirstPageRef = useRef(isFirstPage)
  const prevHasFiltersRef = useRef(hasFilters)
  if (prevFirstPageRef.current !== isFirstPage || prevHasFiltersRef.current !== hasFilters) {
    prevFirstPageRef.current = isFirstPage
    prevHasFiltersRef.current = hasFilters
    if (isFirstPage && !hasFilters) {
      setPendingCount(0)
    }
  }

  const handleShowNew = useCallback(() => {
    // Clear all filters and go to page 1
    navigate(pathname, { replace: true })
    setKeywordInput("")
    setPendingCount(0)
    queryClient.invalidateQueries({ queryKey: ["logs"] })
  }, [queryClient, navigate, pathname])

  const logs = useMemo(() => {
    const dbLogs = (data?.data?.logs ?? []) as LogEntry[]
    if (pendingStreams.size === 0) return dbLogs
    const pending = Array.from(pendingStreams.values()).sort((a, b) => b.time - a.time)
    return [...pending, ...dbLogs]
  }, [data, pendingStreams])
  const total = data?.data?.total ?? 0
  const totalPages = Math.ceil(total / pageSize)
  const detail = (detailData?.data ?? null) as LogDetail | null

  // Real-time elapsed time update for pending streams (1s interval)
  useEffect(() => {
    if (pendingStreams.size === 0) return
    const interval = setInterval(() => {
      setPendingStreams((prev) => {
        const next = new Map(prev)
        for (const [key, entry] of next) {
          next.set(key, {
            ...entry,
            useTime: Date.now() - (entry._startedAt ?? Date.now()),
          })
        }
        return next
      })
    }, 1000)
    return () => clearInterval(interval)
  }, [pendingStreams.size > 0]) // eslint-disable-line react-hooks/exhaustive-deps

  return {
    // Filter state
    filters,
    page,
    model,
    status,
    channelId,
    keyword,
    pageSize,
    startTime,
    endTime,
    hasFilters,
    isFirstPage,
    keywordInput,
    setKeywordInput,
    updateFilter,
    debouncedUpdateFilter,
    pathname,
    navigate,

    // Query state
    logs,
    total,
    totalPages,
    isLoading,
    isFetching,
    isError,
    refetch,

    // Detail panel state
    detail,
    detailId,
    setDetailId,
    detailStreamId,
    setDetailStreamId,
    detailTab,
    setDetailTab,
    streamingOverlay,

    // Streaming state
    pendingStreams,
    pendingCount,
    handleShowNew,

    // Filter options
    channels,
    modelOptions,
  }
}
