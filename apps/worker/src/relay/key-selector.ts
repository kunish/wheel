/**
 * Select a usable key from a channel's key list.
 * Rules:
 * - Must be enabled
 * - Prefer keys NOT in 429 cooldown (last used within cooldown period)
 * - Among preferred keys, pick the one with lowest totalCost
 * - If ALL enabled keys are in 429 cooldown, fallback to the one
 *   with the oldest lastUseTimestamp (most likely to have recovered)
 */

interface ChannelKeyRecord {
  id: number
  channelId: number
  enabled: boolean
  channelKey: string
  statusCode: number
  lastUseTimestamp: number
  totalCost: number
  remark: string
}

const RATE_LIMIT_COOLDOWN = 300 // 300 seconds cooldown (5 minutes, aligned with Go)

export function selectKey(keys: ChannelKeyRecord[]): ChannelKeyRecord | null {
  const now = Math.floor(Date.now() / 1000)

  const enabledKeys = keys.filter((k) => k.enabled)
  if (enabledKeys.length === 0) return null

  // Prefer keys not in 429 cooldown
  const preferred = enabledKeys.filter((k) => {
    if (k.statusCode === 429 && now - k.lastUseTimestamp < RATE_LIMIT_COOLDOWN) {
      return false
    }
    return true
  })

  if (preferred.length > 0) {
    // Pick the key with lowest total cost
    preferred.sort((a, b) => a.totalCost - b.totalCost)
    return preferred[0]
  }

  // Fallback: all keys are rate-limited, pick the one with oldest timestamp
  // (most likely to have recovered from rate limit)
  enabledKeys.sort((a, b) => a.lastUseTimestamp - b.lastUseTimestamp)
  return enabledKeys[0]
}
