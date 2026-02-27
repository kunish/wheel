import { apiFetch } from "./client"

// ── Audit Logs ──

export interface AuditLogRecord {
  id: number
  time: number
  user: string
  action: string
  target: string
  detail: string
}

export function listAuditLogs(params: {
  page?: number
  pageSize?: number
  user?: string
  action?: string
  startTime?: number
  endTime?: number
}) {
  const q = new URLSearchParams()
  if (params.page) q.set("page", String(params.page))
  if (params.pageSize) q.set("pageSize", String(params.pageSize))
  if (params.user) q.set("user", params.user)
  if (params.action) q.set("action", params.action)
  if (params.startTime) q.set("startTime", String(params.startTime))
  if (params.endTime) q.set("endTime", String(params.endTime))
  return apiFetch<{
    success: boolean
    data: { logs: AuditLogRecord[]; total: number; page: number; pageSize: number }
  }>(`/api/v1/audit-log/list?${q.toString()}`)
}

export function clearAuditLogs() {
  return apiFetch<{ success: boolean }>("/api/v1/audit-log/clear", { method: "DELETE" })
}
