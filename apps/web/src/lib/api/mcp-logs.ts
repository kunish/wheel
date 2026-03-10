import { apiFetch } from "./client"

// ── MCP Logs ──

interface MCPLogRecord {
  id: number
  time: number
  clientId: number
  clientName: string
  toolName: string
  status: string
  duration: number
  error: string
}

export function listMCPLogs(params: {
  page?: number
  pageSize?: number
  clientId?: number
  toolName?: string
  status?: string
  startTime?: number
  endTime?: number
}) {
  const q = new URLSearchParams()
  if (params.page) q.set("page", String(params.page))
  if (params.pageSize) q.set("pageSize", String(params.pageSize))
  if (params.clientId) q.set("clientId", String(params.clientId))
  if (params.toolName) q.set("toolName", params.toolName)
  if (params.status) q.set("status", params.status)
  if (params.startTime) q.set("startTime", String(params.startTime))
  if (params.endTime) q.set("endTime", String(params.endTime))
  return apiFetch<{
    success: boolean
    data: { logs: MCPLogRecord[]; total: number; page: number; pageSize: number }
  }>(`/api/v1/mcp-log/list?${q.toString()}`)
}
