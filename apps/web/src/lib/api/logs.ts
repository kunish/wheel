import { apiFetch, apiRawFetch } from "./client"

// ── Logs ──

export function listLogs(params: Record<string, string | number | undefined>) {
  const searchParams = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== "") {
      searchParams.set(key, String(value))
    }
  }
  return apiFetch<{
    success: boolean
    data: { logs: unknown[]; total: number; page: number; pageSize: number }
  }>(`/api/v1/log/list?${searchParams.toString()}`)
}

export function getLog(id: number) {
  return apiFetch<{ success: boolean; data: unknown }>(`/api/v1/log/${id}`)
}

export function deleteLog(id: number) {
  return apiFetch<{ success: boolean }>(`/api/v1/log/delete/${id}`, {
    method: "DELETE",
  })
}

export function replayLog(id: number): Promise<Response> {
  return apiRawFetch(`/api/v1/log/replay/${id}`, { method: "POST" })
}
