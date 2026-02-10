import type { Database } from "../runtime/types"
import { getSetting } from "../db/dal/settings"

// ── Circuit Breaker State Machine ──────────────────────────────────
// Closed → Open → HalfOpen → Closed (on success) or back to Open (on failure)

type CircuitState = "closed" | "open" | "half_open"

interface CircuitEntry {
  state: CircuitState
  consecutiveFailures: number
  lastFailureTime: number
  tripCount: number // cumulative trips for exponential backoff
}

// Global in-memory circuit breaker storage
// Key format: channelId:channelKeyId:modelName
const breakers = new Map<string, CircuitEntry>()

function circuitKey(channelId: number, keyId: number, modelName: string): string {
  return `${channelId}:${keyId}:${modelName}`
}

function getOrCreate(key: string): CircuitEntry {
  let entry = breakers.get(key)
  if (!entry) {
    entry = { state: "closed", consecutiveFailures: 0, lastFailureTime: 0, tripCount: 0 }
    breakers.set(key, entry)
  }
  return entry
}

// ── Config ─────────────────────────────────────────────────────────

async function getThreshold(db: Database): Promise<number> {
  const v = await getSetting(db, "circuit_breaker_threshold")
  const n = v ? Number.parseInt(v, 10) : 0
  return n > 0 ? n : 5
}

function getCooldownMs(tripCount: number, baseSec: number, maxSec: number): number {
  let cooldown = baseSec
  if (tripCount > 1) {
    const shift = Math.min(tripCount - 1, 20) // prevent overflow
    cooldown = baseSec * (1 << shift)
  }
  if (cooldown > maxSec) cooldown = maxSec
  return cooldown * 1000
}

export async function getCooldownConfig(
  db: Database,
): Promise<{ baseSec: number; maxSec: number }> {
  const [baseVal, maxVal] = await Promise.all([
    getSetting(db, "circuit_breaker_cooldown"),
    getSetting(db, "circuit_breaker_max_cooldown"),
  ])
  const baseSec = baseVal ? Number.parseInt(baseVal, 10) : 60
  const maxSec = maxVal ? Number.parseInt(maxVal, 10) : 600
  return { baseSec: baseSec > 0 ? baseSec : 60, maxSec: maxSec > 0 ? maxSec : 600 }
}

// ── Public API ─────────────────────────────────────────────────────

/**
 * Check if a channel is circuit-broken.
 * Returns { tripped, remainingMs } — tripped=true means skip this channel.
 */
export function isTripped(
  channelId: number,
  keyId: number,
  modelName: string,
  baseSec: number,
  maxSec: number,
): { tripped: boolean; remainingMs: number } {
  const key = circuitKey(channelId, keyId, modelName)
  const entry = breakers.get(key)
  if (!entry) return { tripped: false, remainingMs: 0 }

  switch (entry.state) {
    case "closed":
      return { tripped: false, remainingMs: 0 }

    case "open": {
      const cooldown = getCooldownMs(entry.tripCount, baseSec, maxSec)
      const elapsed = Date.now() - entry.lastFailureTime
      if (elapsed >= cooldown) {
        // Transition to half-open
        entry.state = "half_open"
        return { tripped: false, remainingMs: 0 }
      }
      return { tripped: true, remainingMs: cooldown - elapsed }
    }

    case "half_open":
      // Already probing — block other requests
      return { tripped: true, remainingMs: 0 }

    default:
      return { tripped: false, remainingMs: 0 }
  }
}

/**
 * Record a successful request — reset circuit breaker.
 */
export function recordSuccess(channelId: number, keyId: number, modelName: string): void {
  const key = circuitKey(channelId, keyId, modelName)
  const entry = breakers.get(key)
  if (!entry) return

  entry.state = "closed"
  entry.consecutiveFailures = 0
  entry.tripCount = 0
}

/**
 * Record a failed request — may trigger circuit breaker.
 */
export async function recordFailure(
  channelId: number,
  keyId: number,
  modelName: string,
  db: Database,
): Promise<void> {
  const key = circuitKey(channelId, keyId, modelName)
  const entry = getOrCreate(key)

  entry.lastFailureTime = Date.now()

  switch (entry.state) {
    case "closed": {
      entry.consecutiveFailures++
      const threshold = await getThreshold(db)
      if (entry.consecutiveFailures >= threshold) {
        entry.state = "open"
        entry.tripCount++
      }
      break
    }

    case "half_open":
      // Probe failed — back to open with increased backoff
      entry.state = "open"
      entry.tripCount++
      entry.consecutiveFailures = 0
      break

    case "open":
      // Should not receive failures while open, but update time for safety
      break
  }
}
