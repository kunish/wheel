import type { LogEntry } from "@/pages/logs/columns"
import { useQueryClient } from "@tanstack/react-query"
import { useCallback, useEffect, useRef, useState } from "react"
import { useWsEvent } from "@/hooks/use-stats-ws"

interface StreamRefs {
  detailIdRef: React.RefObject<number | null>
  detailStreamIdRef: React.RefObject<string | null>
  setDetailStreamId: (id: string | null) => void
  setDetailId: (id: number | null) => void
  setStreamingOverlay: (
    overlay: { thinkingContent: string; responseContent: string } | null,
  ) => void
}

interface FilterRefs {
  page: number
  pageSize: number
  model: string
  status: string
  channelId: number | undefined
  keyword: string
  startTime: number | undefined
  endTime: number | undefined
  isFirstPage: boolean
  hasFilters: boolean
}

export type ConnectionState = "connected" | "disconnected" | "reconnecting"

interface BufferedEvent {
  type: "log-stream-start" | "log-streaming" | "log-created" | "log-stream-end"
  data: any
}

const MAX_BUFFERED_EVENTS = 2000

export function useLogStream(filterState: FilterRefs, streamRefs: StreamRefs) {
  const queryClient = useQueryClient()
  const [pendingStreams, setPendingStreams] = useState<Map<string, LogEntry>>(() => new Map())
  const [pendingCount, setPendingCount] = useState(0)
  const [isPaused, setIsPaused] = useState(false)
  const [connectionState, setConnectionState] = useState<ConnectionState>("connected")

  const isPausedRef = useRef(isPaused)
  isPausedRef.current = isPaused

  const bufferedEventsRef = useRef<BufferedEvent[]>([])

  const filtersRef = useRef(filterState)
  filtersRef.current = filterState

  // ── Helper to process a single event (shared by live + flush) ──

  const processEvent = useCallback(
    (event: BufferedEvent) => {
      const { type, data } = event

      if (type === "log-stream-start") {
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
            cacheReadTokens: 0,
            cacheCreationTokens: 0,
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
      }

      if (type === "log-streaming") {
        if (!data?.streamId) return
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

        const currentStreamId = streamRefs.detailStreamIdRef.current
        if (currentStreamId === data.streamId) {
          streamRefs.setStreamingOverlay({
            thinkingContent: data.thinkingContent ?? "",
            responseContent: data.responseContent ?? "",
          })
        }
      }

      if (type === "log-created") {
        if (!data?.log) return

        if (data.streamId) {
          setPendingStreams((prev) => {
            if (!prev.has(data.streamId)) return prev
            const next = new Map(prev)
            next.delete(data.streamId)
            return next
          })

          if (streamRefs.detailStreamIdRef.current === data.streamId) {
            streamRefs.setDetailStreamId(null)
            streamRefs.setDetailId(data.log.id)
            streamRefs.setStreamingOverlay(null)
          }
        }

        const currentDetailId = streamRefs.detailIdRef.current
        if (currentDetailId !== null && data.log.id === currentDetailId) {
          streamRefs.setStreamingOverlay(null)
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
              const logFromEvent = {
                ...(data.log as LogEntry),
                ...(data.streamId ? { _streamId: data.streamId } : {}),
              }
              const newLogs = [logFromEvent, ...old.data.logs].slice(0, f.pageSize)
              return { ...old, data: { ...old.data, logs: newLogs, total: old.data.total + 1 } }
            },
          )
        } else {
          setPendingCount((c) => c + 1)
        }
      }

      if (type === "log-stream-end") {
        if (!data?.streamId) return
        setPendingStreams((prev) => {
          if (!prev.has(data.streamId)) return prev
          const next = new Map(prev)
          next.delete(data.streamId)
          return next
        })
      }
    },
    [queryClient, streamRefs],
  )

  // ── WS event handlers — buffer when paused, process when live ──

  const bufferEvent = useCallback((event: BufferedEvent) => {
    const next = bufferedEventsRef.current
    if (next.length >= MAX_BUFFERED_EVENTS) {
      next.splice(0, next.length - MAX_BUFFERED_EVENTS + 1)
    }
    next.push(event)
  }, [])

  useWsEvent("log-stream-start", (data) => {
    if (isPausedRef.current) {
      bufferEvent({ type: "log-stream-start", data })
      return
    }
    processEvent({ type: "log-stream-start", data })
  })

  useWsEvent("log-streaming", (data) => {
    if (isPausedRef.current) {
      bufferEvent({ type: "log-streaming", data })
      return
    }
    processEvent({ type: "log-streaming", data })
  })

  useWsEvent("log-created", (data) => {
    if (isPausedRef.current) {
      bufferEvent({ type: "log-created", data })
      return
    }
    processEvent({ type: "log-created", data })
  })

  useWsEvent("log-stream-end", (data) => {
    if (isPausedRef.current) {
      bufferEvent({ type: "log-stream-end", data })
      return
    }
    processEvent({ type: "log-stream-end", data })
  })

  // ── Connection state tracking ──

  useWsEvent("ws-state", (data) => {
    const state = data?.state
    if (state === "connected" || state === "reconnecting" || state === "disconnected") {
      setConnectionState(state)
    }
  })

  // ── Pause / Resume ──

  const togglePause = useCallback(() => {
    setIsPaused((prev) => {
      if (prev) {
        const events = bufferedEventsRef.current
        bufferedEventsRef.current = []
        queueMicrotask(() => {
          for (const event of events) {
            processEvent(event)
          }
        })
      }
      return !prev
    })
  }, [processEvent])

  // Reset pending count when navigating to page 1 or clearing filters
  const prevFirstPageRef = useRef(filterState.isFirstPage)
  const prevHasFiltersRef = useRef(filterState.hasFilters)
  if (
    prevFirstPageRef.current !== filterState.isFirstPage ||
    prevHasFiltersRef.current !== filterState.hasFilters
  ) {
    prevFirstPageRef.current = filterState.isFirstPage
    prevHasFiltersRef.current = filterState.hasFilters
    if (filterState.isFirstPage && !filterState.hasFilters) {
      setPendingCount(0)
    }
  }

  // Real-time elapsed time update for pending streams
  const hasActiveStreams = pendingStreams.size > 0
  useEffect(() => {
    if (!hasActiveStreams) return
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
  }, [hasActiveStreams])

  const handleShowNew = useCallback(
    (pathname: string, navigate: (path: string, opts?: { replace?: boolean }) => void) => {
      navigate(pathname, { replace: true })
      setPendingCount(0)
      queryClient.invalidateQueries({ queryKey: ["logs"] })
    },
    [queryClient],
  )

  return {
    pendingStreams,
    pendingCount,
    isPaused,
    connectionState,
    togglePause,
    handleShowNew,
  }
}
