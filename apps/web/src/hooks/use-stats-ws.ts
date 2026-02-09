"use client"

import type { QueryClient } from "@tanstack/react-query"
import { useEffect, useRef } from "react"

const WS_RECONNECT_INTERVAL = 3000

function getWsUrl() {
  const wsBase = process.env.NEXT_PUBLIC_API_BASE_URL
  if (wsBase) {
    const url = new URL(wsBase)
    const proto = url.protocol === "https:" ? "wss:" : "ws:"
    return `${proto}//${url.host}/api/v1/ws`
  }
  // Dev fallback: connect directly to worker
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:"
  return `${proto}//${window.location.hostname}:8787/api/v1/ws`
}

/**
 * Connects to the worker WebSocket endpoint and invalidates
 * all stats-related TanStack Query caches on "stats-updated" events.
 * Auto-reconnects on disconnect.
 */
export { getWsUrl }
export function useStatsWebSocket(queryClient: QueryClient) {
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    function connect() {
      const wsUrl = getWsUrl()

      const ws = new WebSocket(wsUrl)
      wsRef.current = ws

      ws.onmessage = (ev) => {
        try {
          const msg = JSON.parse(ev.data)
          if (msg.event === "stats-updated") {
            queryClient.invalidateQueries({ queryKey: ["stats"] })
          }
        } catch {
          // ignore non-JSON messages
        }
      }

      ws.onclose = () => {
        wsRef.current = null
        reconnectTimerRef.current = setTimeout(connect, WS_RECONNECT_INTERVAL)
      }

      ws.onerror = () => {
        ws.close()
      }
    }

    connect()

    return () => {
      if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current)
      wsRef.current?.close()
      wsRef.current = null
    }
  }, [queryClient])
}
