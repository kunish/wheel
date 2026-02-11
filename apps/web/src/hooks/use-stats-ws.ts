import type { QueryClient } from "@tanstack/react-query"
import { useEffect, useRef } from "react"

const WS_RECONNECT_INTERVAL = 3000

function getWsUrl() {
  const wsBase = import.meta.env.VITE_API_BASE_URL
  if (wsBase) {
    const url = new URL(wsBase)
    const proto = url.protocol === "https:" ? "wss:" : "ws:"
    return `${proto}//${url.host}/api/v1/ws`
  }
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:"
  if (import.meta.env.DEV) {
    return `${proto}//${window.location.hostname}:8787/api/v1/ws`
  }
  return `${proto}//${window.location.host}/api/v1/ws`
}

// ── Global singleton WS with pub/sub ──

type WsListener = (msg: { event: string; data?: any; ts: number }) => void

let globalWs: WebSocket | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let refCount = 0
const listeners = new Set<WsListener>()

function ensureConnection() {
  if (globalWs && globalWs.readyState <= WebSocket.OPEN) return

  // Clear any pending reconnect to avoid duplicate connections
  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }

  const ws = new WebSocket(getWsUrl())
  globalWs = ws

  ws.onmessage = (ev) => {
    try {
      const msg = JSON.parse(ev.data)
      for (const fn of listeners) fn(msg)
    } catch {
      // ignore non-JSON
    }
  }

  ws.onclose = () => {
    // Only nullify if this is still the current connection
    if (globalWs === ws) {
      globalWs = null
      if (refCount > 0) {
        reconnectTimer = setTimeout(ensureConnection, WS_RECONNECT_INTERVAL)
      }
    }
  }

  ws.onerror = () => {
    ws.close()
  }
}

function addRef() {
  refCount++
  if (reconnectTimer) {
    // Cancel any pending reconnect — we'll connect fresh
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
  ensureConnection()
}

function releaseRef() {
  refCount = Math.max(0, refCount - 1)
  // Don't close the WS on releaseRef — keep it alive as long as the page is open.
  // The connection is cheap and avoids churn from React StrictMode / route transitions.
}

export function subscribe(fn: WsListener) {
  listeners.add(fn)
  return () => listeners.delete(fn)
}

// ── Hooks ──

/**
 * Maintains the global WS connection and invalidates stats queries.
 * Mount once in the protected layout.
 */
export function useStatsWebSocket(queryClient: QueryClient) {
  useEffect(() => {
    addRef()
    const unsub = subscribe((msg) => {
      if (msg.event === "stats-updated") {
        queryClient.invalidateQueries({ queryKey: ["stats"] })
      }
    })
    return () => {
      unsub()
      releaseRef()
    }
  }, [queryClient])
}

/**
 * Subscribe to a specific WS event. Reuses the global WS connection.
 */
export function useWsEvent(event: string, handler: (data: any) => void) {
  const handlerRef = useRef(handler)
  handlerRef.current = handler

  useEffect(() => {
    addRef()
    const unsub = subscribe((msg) => {
      if (msg.event === event) {
        handlerRef.current(msg.data)
      }
    })
    return () => {
      unsub()
      releaseRef()
    }
  }, [event])
}
