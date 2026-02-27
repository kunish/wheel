import { apiFetch } from "./client"

// ── MCP Clients ──

export interface MCPStdioConfig {
  command: string
  args: string[]
  envs: string[]
}

export interface MCPHeaderEntry {
  key: string
  value: string
}

export interface MCPToolInfo {
  name: string
  description?: string
}

export interface MCPClientRecord {
  id: number
  name: string
  connectionType: "http" | "sse" | "stdio"
  connectionString: string
  stdioConfig?: MCPStdioConfig
  authType: "none" | "headers" | "oauth"
  headers?: MCPHeaderEntry[]
  oauthConfigId?: string
  toolsToExecute: string[]
  toolsToAutoExec: string[]
  enabled: boolean
  state: "connected" | "disconnected" | "error"
  errorMsg?: string
  tools: MCPToolInfo[]
  createdAt?: string
  updatedAt?: string
}

export interface MCPClientInput {
  id?: number
  name: string
  connectionType: "http" | "sse" | "stdio"
  connectionString?: string
  stdioConfig?: MCPStdioConfig
  authType: "none" | "headers" | "oauth"
  headers?: MCPHeaderEntry[]
  toolsToExecute?: string[]
  toolsToAutoExec?: string[]
  enabled: boolean
}

export function listMCPClients() {
  return apiFetch<{ success: boolean; data: { clients: MCPClientRecord[] } }>(
    "/api/v1/mcp/client/list",
  )
}

export function createMCPClient(data: Omit<MCPClientInput, "id">) {
  return apiFetch<{ success: boolean; data: MCPClientRecord }>("/api/v1/mcp/client/create", {
    method: "POST",
    body: data,
  })
}

export function updateMCPClient(data: Partial<MCPClientInput> & { id: number }) {
  return apiFetch<{ success: boolean }>("/api/v1/mcp/client/update", {
    method: "POST",
    body: data,
  })
}

export function deleteMCPClient(id: number) {
  return apiFetch<{ success: boolean }>(`/api/v1/mcp/client/delete/${id}`, {
    method: "DELETE",
  })
}

export function reconnectMCPClient(id: number) {
  return apiFetch<{ success: boolean }>(`/api/v1/mcp/client/reconnect/${id}`, {
    method: "POST",
  })
}
