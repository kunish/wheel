import { apiFetch } from "./client"

// ── Channels ──

export interface ChannelInput {
  id?: number
  name: string
  type: number
  enabled: boolean
  baseUrls: { url: string; delay: number }[]
  keys: { channelKey: string; remark: string }[]
  model: string[]
  fetchedModel: string[]
  customModel: string
  paramOverride: string
}

interface SavedChannel {
  id: number
}

export function listChannels() {
  return apiFetch<{ success: boolean; data: { channels: unknown[] } }>("/api/v1/channel/list")
}

export function createChannel(data: ChannelInput) {
  return apiFetch<{ success: boolean; data: SavedChannel }>("/api/v1/channel/create", {
    method: "POST",
    body: data,
  })
}

export function updateChannel(data: ChannelInput) {
  return apiFetch<{ success: boolean; data: unknown }>("/api/v1/channel/update", {
    method: "POST",
    body: data,
  })
}

export function enableChannel(id: number, enabled: boolean) {
  return apiFetch<{ success: boolean }>("/api/v1/channel/enable", {
    method: "POST",
    body: { id, enabled },
  })
}

export function deleteChannel(id: number) {
  return apiFetch<{ success: boolean }>(`/api/v1/channel/delete/${id}`, {
    method: "DELETE",
  })
}

export function fetchChannelModelsPreview(data: { type: number; baseUrl: string; key: string }) {
  return apiFetch<{ success: boolean; data: { models: string[]; isFallback?: boolean } }>(
    "/api/v1/channel/fetch-model-preview",
    {
      method: "POST",
      body: data,
    },
  )
}

export function reorderChannels(orderedIds: number[]) {
  return apiFetch<{ success: boolean }>("/api/v1/channel/reorder", {
    method: "POST",
    body: { orderedIds },
  })
}

// ── Channel Health ──

export function getChannelHealth() {
  return apiFetch<{ success: boolean; data: { health: Record<string, number> } }>(
    "/api/v1/channel/health",
  )
}
