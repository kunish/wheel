import { apiFetch, apiRawFetch } from "./client"

// ── Settings ──

export function getSettings() {
  return apiFetch<{ success: boolean; data: { settings: Record<string, string> } }>(
    "/api/v1/setting",
  )
}

export function updateSettings(settings: Record<string, string>) {
  return apiFetch<{ success: boolean }>("/api/v1/setting/update", {
    method: "POST",
    body: { settings },
  })
}

export function getVersion() {
  return apiFetch<{ success: boolean; data: { version: string } }>("/api/v1/setting/version")
}

export function checkUpdate() {
  return apiFetch<{
    success: boolean
    data: {
      current: string
      latest: string
      hasUpdate: boolean
      releaseUrl: string
      releaseNotes: string
    }
  }>("/api/v1/setting/check-update")
}

export function applyUpdate() {
  return apiFetch<{ success: boolean }>("/api/v1/setting/apply-update", {
    method: "POST",
  })
}

export function resetCircuitBreakers() {
  return apiFetch<{ success: boolean; data: { reset: number } }>(
    "/api/v1/setting/reset-circuit-breakers",
    { method: "POST" },
  )
}

// ── Data Export/Import ──

export function exportData(includeLogs: boolean = false) {
  return apiRawFetch(`/api/v1/setting/export?include_logs=${includeLogs}`)
}

export interface ImportDataResult {
  success: boolean
  data?: {
    channels?: { added: number; skipped: number }
    groups?: { added: number; skipped: number }
    groupItems?: { added: number; skipped: number }
    apiKeys?: { added: number; skipped: number }
    settings?: { added: number; skipped: number }
  }
  error?: string
}

export function importData(file: File): Promise<ImportDataResult> {
  const formData = new FormData()
  formData.append("file", file)
  return apiRawFetch("/api/v1/setting/import", {
    method: "POST",
    body: formData,
  }).then((res) => res.json() as Promise<ImportDataResult>)
}
