"use client"

import type { QueryClient } from "@tanstack/react-query"
import { useEffect, useRef } from "react"

const WS_RECONNECT_INTERVAL = 3000

/**
 * Connects to the worker WebSocket endpoint and invalidates
 * all stats-related TanStack Query caches on "stats-updated" events.
 * Auto-reconnects on disconnect.
 */
export function useStatsWebSocket(queryClient: QueryClient) {
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    function connect() {
      const proto = window.location.protocol === "https:" ? "wss:" : "ws:"
      const wsUrl = `${proto}//${window.location.host}/api/v1/ws`

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
