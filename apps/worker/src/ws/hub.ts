/**
 * In-memory WebSocket connection hub.
 * Maintains a set of active server-side WebSocket connections and
 * broadcasts JSON messages to all of them.
 *
 * Note: This is per-isolate — works perfectly for wrangler dev (single process).
 * For production with multiple isolates, consider Durable Objects.
 */

const clients = new Set<WebSocket>()

export function addClient(ws: WebSocket) {
  clients.add(ws)
  ws.addEventListener("close", () => clients.delete(ws))
  ws.addEventListener("error", () => clients.delete(ws))
}

export function broadcast(event: string, data?: Record<string, unknown>) {
  const message = JSON.stringify({ event, data, ts: Date.now() })
  for (const ws of clients) {
    try {
      ws.send(message)
    } catch {
      clients.delete(ws)
    }
  }
}

export function clientCount() {
  return clients.size
}
