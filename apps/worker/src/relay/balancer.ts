import { GroupMode } from "@wheel/core"

interface GroupItem {
  id: number
  channelId: number
  modelName: string
  priority: number
  weight: number
}

// In-memory round-robin counters (per group)
const rrCounters = new Map<number, number>()

/**
 * Sort and order GroupItems based on the group's load-balancing mode.
 * Returns an ordered array of items to try sequentially.
 */
export function selectChannelOrder(
  mode: GroupMode,
  items: GroupItem[],
  groupId: number,
): GroupItem[] {
  if (items.length === 0) return []

  switch (mode) {
    case GroupMode.RoundRobin:
      return roundRobin(items, groupId)
    case GroupMode.Random:
      return randomOrder(items)
    case GroupMode.Failover:
      return failoverOrder(items)
    case GroupMode.Weighted:
      return weightedOrder(items)
    default:
      return items
  }
}

function roundRobin(items: GroupItem[], groupId: number): GroupItem[] {
  const idx = (rrCounters.get(groupId) ?? 0) % items.length
  rrCounters.set(groupId, idx + 1)
  // Rotate: start from idx, then wrap around
  return [...items.slice(idx), ...items.slice(0, idx)]
}

function randomOrder(items: GroupItem[]): GroupItem[] {
  const shuffled = [...items]
  for (let i = shuffled.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1))
    ;[shuffled[i], shuffled[j]] = [shuffled[j], shuffled[i]]
  }
  return shuffled
}

function failoverOrder(items: GroupItem[]): GroupItem[] {
  // Sort by priority ascending (lower = higher priority)
  return [...items].sort((a, b) => a.priority - b.priority)
}

function weightedOrder(items: GroupItem[]): GroupItem[] {
  // Weighted random: pick one item based on weight, then append the rest
  const totalWeight = items.reduce((sum, it) => sum + it.weight, 0)
  if (totalWeight === 0) return items

  let rand = Math.random() * totalWeight
  let selected = 0
  for (let i = 0; i < items.length; i++) {
    rand -= items[i].weight
    if (rand <= 0) {
      selected = i
      break
    }
  }

  // Put the selected item first, then the rest in order
  const result = [items[selected]]
  for (let i = 0; i < items.length; i++) {
    if (i !== selected) result.push(items[i])
  }
  return result
}
