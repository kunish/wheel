import { useCallback } from "react"
import { useLogData } from "@/hooks/use-log-data"
import { useLogDetail } from "@/hooks/use-log-detail"
import { useLogFilters } from "@/hooks/use-log-filters"
import { useLogStream } from "@/hooks/use-log-stream"

export function useLogQuery() {
  const filterState = useLogFilters()
  const detailState = useLogDetail()

  const streamState = useLogStream(
    {
      page: filterState.page,
      pageSize: filterState.pageSize,
      model: filterState.model,
      status: filterState.status,
      channelId: filterState.channelId,
      keyword: filterState.keyword,
      startTime: filterState.startTime,
      endTime: filterState.endTime,
      isFirstPage: filterState.isFirstPage,
      hasFilters: filterState.hasFilters,
    },
    {
      detailIdRef: detailState.detailIdRef,
      detailStreamIdRef: detailState.detailStreamIdRef,
      setDetailStreamId: detailState.setDetailStreamId,
      setDetailId: detailState.setDetailId,
      setStreamingOverlay: detailState.setStreamingOverlay,
    },
  )

  const dataState = useLogData(
    {
      page: filterState.page,
      pageSize: filterState.pageSize,
      model: filterState.model,
      status: filterState.status,
      channelId: filterState.channelId,
      keyword: filterState.keyword,
      startTime: filterState.startTime,
      endTime: filterState.endTime,
    },
    streamState.pendingStreams,
  )

  const handleShowNew = useCallback(() => {
    streamState.handleShowNew(filterState.pathname, filterState.navigate)
    filterState.setKeywordInput("")
  }, [streamState, filterState])

  return {
    // Filter state
    ...filterState,

    // Query state
    ...dataState,

    // Detail panel state
    detail: detailState.detail,
    detailId: detailState.detailId,
    setDetailId: detailState.setDetailId,
    detailStreamId: detailState.detailStreamId,
    setDetailStreamId: detailState.setDetailStreamId,
    streamingOverlay: detailState.streamingOverlay,

    // Streaming state
    pendingStreams: streamState.pendingStreams,
    pendingCount: streamState.pendingCount,
    isPaused: streamState.isPaused,
    connectionState: streamState.connectionState,
    togglePause: streamState.togglePause,
    handleShowNew,
  }
}
