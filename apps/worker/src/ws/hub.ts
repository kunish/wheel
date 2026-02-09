/**
 * In-memory WebSocket connection hub.
 *
 * In Cloudflare Workers (workerd), WebSocket.send() only works within the
 * request context that created the WebSocket — calling ws.send() from a
 * different HTTP request handler silently fails. To work around this, we
 * use a message queue: broadcast() pushes to the queue, and each WebSocket
 * connection drains the queue via setInterval in its own context.
 *
 * Note: This is per-isolate. For production with multiple isolates,
 * consider Durable Objects.
 */

const messageQueue: string[] = []
let messageSeq = 0

interface ClientState {
  ws: WebSocket
  lastSeq: number
  interval: ReturnType<typeof setInterval>
}

const clients = new Map<WebSocket, ClientState>()

const POLL_INTERVAL = 200 // ms

export function addClient(ws: WebSocket) {
  const state: ClientState = {
    ws,
    lastSeq: messageSeq,
    interval: setInterval(() => {
      // Drain any pending messages
      while (state.lastSeq < messageSeq) {
        const idx = state.lastSeq - (messageSeq - messageQueue.length)
        if (idx >= 0 && idx < messageQueue.length) {
          try {
            ws.send(messageQueue[idx])
          } catch {
            cleanup()
            return
          }
        }
        state.lastSeq++
      }
    }, POLL_INTERVAL),
  }

  clients.set(ws, state)

  function cleanup() {
    clearInterval(state.interval)
    clients.delete(ws)
  }

  ws.addEventListener("close", cleanup)
  ws.addEventListener("error", cleanup)
}

export function broadcast(event: string, data?: Record<string, unknown>) {
  const message = JSON.stringify({ event, data, ts: Date.now() })
  messageQueue.push(message)
  messageSeq++

  // Keep queue bounded — trim old messages that all clients have consumed
  if (messageQueue.length > 1000) {
    let minConsumed = messageSeq
    for (const state of clients.values()) {
      if (state.lastSeq < minConsumed) minConsumed = state.lastSeq
    }
    const trimCount = minConsumed - (messageSeq - messageQueue.length)
    if (trimCount > 0) {
      messageQueue.splice(0, trimCount)
    }
  }
}

export function clientCount() {
  return clients.size
}
