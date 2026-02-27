import { apiFetch } from "./client"

// ── Groups ──

export interface GroupItemInput {
  channelId: number
  modelName: string
  priority: number
  weight: number
  enabled: boolean
}

export interface GroupInput {
  id?: number
  name: string
  mode: number
  firstTokenTimeOut: number
  sessionKeepTime?: number
  items: GroupItemInput[]
  profileId?: number
}

export function listGroups(profileId?: number) {
  const qs = profileId ? `?profileId=${profileId}` : ""
  return apiFetch<{ success: boolean; data: { groups: unknown[] } }>(`/api/v1/group/list${qs}`)
}

export function createGroup(data: GroupInput) {
  return apiFetch<{ success: boolean; data: unknown }>("/api/v1/group/create", {
    method: "POST",
    body: data,
  })
}

export function updateGroup(data: GroupInput) {
  return apiFetch<{ success: boolean; data: unknown }>("/api/v1/group/update", {
    method: "POST",
    body: data,
  })
}

export function deleteGroup(id: number) {
  return apiFetch<{ success: boolean }>(`/api/v1/group/delete/${id}`, {
    method: "DELETE",
  })
}

export function reorderGroups(orderedIds: number[]) {
  return apiFetch<{ success: boolean }>("/api/v1/group/reorder", {
    method: "POST",
    body: { orderedIds },
  })
}

export function getModelList() {
  return apiFetch<{ success: boolean; data: { models: string[] } }>("/api/v1/group/model-list")
}
