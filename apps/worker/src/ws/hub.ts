/**
 * In-memory WebSocket connection hub.
 *
 * Broadcasts messages directly to all connected clients. No setInterval
 * polling — workerd treats active intervals as "pending work" which
 * prevents the request context from completing, causing "code had hung".
 */

const clients = new Set<WebSocket>()

export function addClient(ws: WebSocket) {
  clients.add(ws)

  function cleanup() {
    clients.delete(ws)
    try {
      ws.close()
    } catch {
      // already closed
    }
  }

  ws.addEventListener("close", cleanup)
  ws.addEventListener("error", cleanup)
}

export function broadcast(event: string, data?: Record<string, unknown>) {
  if (clients.size === 0) return
  const message = JSON.stringify({ event, data, ts: Date.now() })
  for (const ws of clients) {
    try {
      ws.send(message)
    } catch {
      clients.delete(ws)
      try {
        ws.close()
      } catch {
        // already closed
      }
    }
  }
}

export function clientCount() {
  return clients.size
}
