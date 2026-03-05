import type { LogDetail } from "@/pages/logs/types"
import { useQuery } from "@tanstack/react-query"
import { useRef, useState } from "react"
import { getLog } from "@/lib/api-client"

export function useLogDetail() {
  const [detailId, setDetailId] = useState<number | null>(null)
  const detailIdRef = useRef(detailId)
  detailIdRef.current = detailId

  const [detailStreamId, setDetailStreamId] = useState<string | null>(null)
  const detailStreamIdRef = useRef(detailStreamId)
  detailStreamIdRef.current = detailStreamId

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

  const { data: detailData } = useQuery({
    queryKey: ["log-detail", detailId],
    queryFn: () => getLog(detailId!),
    enabled: detailId !== null && detailStreamId === null,
  })

  const detail = (detailData?.data ?? null) as LogDetail | null

  return {
    detail,
    detailId,
    setDetailId,
    detailIdRef,
    detailStreamId,
    setDetailStreamId,
    detailStreamIdRef,
    streamingOverlay,
    setStreamingOverlay,
  }
}
