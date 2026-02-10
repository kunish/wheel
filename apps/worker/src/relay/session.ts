// ── Session Stickiness ─────────────────────────────────────────────
// Keeps requests from the same API Key + Model routed to the same channel
// within the group's sessionKeepTime window.

interface SessionEntry {
  channelId: number
  channelKeyId: number
  timestamp: number
}

// Global in-memory session storage
// Key format: apiKeyId:requestModel
const sessions = new Map<string, SessionEntry>()

function sessionKey(apiKeyId: number, requestModel: string): string {
  return `${apiKeyId}:${requestModel}`
}

/**
 * Get the sticky channel for an API key + model combination.
 * Returns null if no valid (non-expired) session exists.
 */
export function getSticky(
  apiKeyId: number,
  requestModel: string,
  ttlSec: number,
): SessionEntry | null {
  if (ttlSec <= 0) return null

  const key = sessionKey(apiKeyId, requestModel)
  const entry = sessions.get(key)
  if (!entry) return null

  if (Date.now() - entry.timestamp > ttlSec * 1000) {
    // Expired — lazy cleanup
    sessions.delete(key)
    return null
  }

  return entry
}

/**
 * Set the sticky channel for an API key + model combination.
 */
export function setSticky(
  apiKeyId: number,
  requestModel: string,
  channelId: number,
  channelKeyId: number,
): void {
  const key = sessionKey(apiKeyId, requestModel)
  sessions.set(key, { channelId, channelKeyId, timestamp: Date.now() })
}
