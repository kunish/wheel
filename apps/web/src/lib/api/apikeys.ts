import { apiFetch } from "./client"

// ── API Keys ──

export interface ApiKeyRecord {
  id: number
  name: string
  apiKey: string
  enabled: boolean
  expireAt: number
  maxCost: number
  totalCost: number
  supportedModels: string
  rpmLimit: number
  tpmLimit: number
}

interface ApiKeyInput {
  id?: number
  name: string
  expireAt: number
  maxCost: number
  supportedModels: string
  rpmLimit: number
  tpmLimit: number
}

export function listApiKeys() {
  return apiFetch<{ success: boolean; data: { apiKeys: ApiKeyRecord[] } }>("/api/v1/apikey/list")
}

export function createApiKey(data: Omit<ApiKeyInput, "id">) {
  return apiFetch<{ success: boolean; data: { apiKey: string } }>("/api/v1/apikey/create", {
    method: "POST",
    body: data,
  })
}

export function updateApiKey(data: ApiKeyInput & { id: number }) {
  return apiFetch<{ success: boolean }>("/api/v1/apikey/update", {
    method: "POST",
    body: data,
  })
}

export function deleteApiKey(id: number) {
  return apiFetch<{ success: boolean }>(`/api/v1/apikey/delete/${id}`, {
    method: "DELETE",
  })
}
