import { apiFetch } from "./client"

// ── Model Limits ──

export interface ModelLimitRecord {
  id: number
  model: string
  rpm: number
  tpm: number
  dailyRequests: number
  dailyTokens: number
  enabled: boolean
}

export function listModelLimits() {
  return apiFetch<{ success: boolean; data: { limits: ModelLimitRecord[] } }>(
    "/api/v1/model-limit/list",
  )
}

export function createModelLimit(data: {
  model: string
  rpm: number
  tpm: number
  dailyRequests: number
  dailyTokens: number
  enabled: boolean
}) {
  return apiFetch<{ success: boolean; data: ModelLimitRecord }>("/api/v1/model-limit/create", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  })
}

export function updateModelLimit(data: {
  id: number
  model?: string
  rpm?: number
  tpm?: number
  dailyRequests?: number
  dailyTokens?: number
  enabled?: boolean
}) {
  return apiFetch<{ success: boolean }>("/api/v1/model-limit/update", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  })
}

export function deleteModelLimit(id: number) {
  return apiFetch<{ success: boolean }>(`/api/v1/model-limit/delete/${id}`, {
    method: "DELETE",
  })
}
