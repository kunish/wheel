/**
 * Match a model name against configured Groups.
 * Each Group has a matchRegex field. We test the model against all groups
 * and return the first matching group. If no regex matches, return null.
 */

interface GroupWithItems {
  id: number
  name: string
  mode: number
  matchRegex: string
  firstTokenTimeOut: number
  items: {
    id: number
    groupId: number
    channelId: number
    modelName: string
    priority: number
    weight: number
  }[]
}

export function matchGroup(model: string, groups: GroupWithItems[]): GroupWithItems | null {
  // Priority 1: exact match by group name
  for (const group of groups) {
    if (group.name === model) {
      return group
    }
  }

  // Priority 2: regex match
  for (const group of groups) {
    if (!group.matchRegex) continue
    try {
      const regex = new RegExp(group.matchRegex)
      if (regex.test(model)) {
        return group
      }
    } catch {
      // Invalid regex, skip
      continue
    }
  }
  return null
}
