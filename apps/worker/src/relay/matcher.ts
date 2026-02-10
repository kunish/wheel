/**
 * Match a model name against configured Groups.
 * We match by exact group name. If no match, return null.
 */

interface GroupWithItems {
  id: number
  name: string
  mode: number
  firstTokenTimeOut: number
  sessionKeepTime: number
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
  // Exact match by group name
  for (const group of groups) {
    if (group.name === model) {
      return group
    }
  }
  return null
}
