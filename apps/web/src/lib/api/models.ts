import type { ModelMeta } from "../types/stats"
import { apiFetch } from "./client"

// ── Model Metadata ──

export function getModelMetadata() {
  return apiFetch<{ success: boolean; data: Record<string, ModelMeta> }>("/api/v1/model/metadata")
}

// ── Model Prices ──

export function listModelPrices() {
  return apiFetch<{
    success: boolean
    data: {
      models: Array<{
        id: number
        name: string
        inputPrice: number
        outputPrice: number
        source: string
        createdAt: string
        updatedAt: string
      }>
    }
  }>("/api/v1/model/list")
}

export function createModelPrice(data: { name: string; inputPrice: number; outputPrice: number }) {
  return apiFetch<{ success: boolean }>("/api/v1/model/create", {
    method: "POST",
    body: data,
  })
}

export function updateModelPrice(data: {
  id: number
  name: string
  inputPrice: number
  outputPrice: number
}) {
  return apiFetch<{ success: boolean }>("/api/v1/model/update", {
    method: "POST",
    body: data,
  })
}

export function deleteModelPrice(data: { name: string }) {
  return apiFetch<{ success: boolean }>("/api/v1/model/delete", {
    method: "POST",
    body: data,
  })
}

export function syncModelPrices() {
  return apiFetch<{ success: boolean }>("/api/v1/model/update-price", {
    method: "POST",
  })
}

export function getLastPriceUpdateTime() {
  return apiFetch<{ success: boolean; data: { lastUpdateTime: string | null } }>(
    "/api/v1/model/last-update-time",
  )
}
