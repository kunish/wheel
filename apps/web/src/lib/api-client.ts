import type { paths } from "./api.gen"
import type {
  ChannelStatsRow,
  ModelMeta,
  ModelStatsItem,
  StatsDaily,
  StatsHourly,
  StatsMetrics,
} from "./types/stats"
import createClient from "openapi-fetch"
import { useAuthStore } from "./store/auth"

// Re-export stats types for convenience
export type {
  ChannelStatsRow,
  ModelMeta,
  ModelStatsItem,
  StatsDaily,
  StatsHourly,
  StatsMetrics,
} from "./types/stats"

// ── OpenAPI client ──

export function createApiClient() {
  const { apiBaseUrl, token } = useAuthStore.getState()

  return createClient<paths>({
    baseUrl: apiBaseUrl || undefined,
    headers: {
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
  })
}

// ── Legacy fetch wrapper ──

interface ApiOptions {
  method?: string
  body?: unknown
  headers?: Record<string, string>
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message)
    this.name = "ApiError"
  }
}

export function getApiBaseUrl(): string {
  return useAuthStore.getState().apiBaseUrl || ""
}

async function apiFetch<T extends { success: boolean }>(
  endpoint: string,
  opts: ApiOptions = {},
): Promise<T> {
  const { method = "GET", body, headers = {} } = opts
  const token = useAuthStore.getState().token

  const reqHeaders: Record<string, string> = {
    "Content-Type": "application/json",
    ...headers,
  }

  if (token) {
    reqHeaders.Authorization = `Bearer ${token}`
  }

  const baseUrl = getApiBaseUrl()
  const url = baseUrl ? `${baseUrl}${endpoint}` : endpoint

  const resp = await fetch(url, {
    method,
    headers: reqHeaders,
    body: body ? JSON.stringify(body) : undefined,
  })

  if (resp.status === 401) {
    useAuthStore.getState().logout()
    if (typeof window !== "undefined") {
      window.location.href = "/login"
    }
    throw new ApiError(401, "Unauthorized")
  }

  if (!resp.ok) {
    const errBody: unknown = await resp.json().catch(() => ({}))
    const errMsg =
      typeof errBody === "object" && errBody !== null && "error" in errBody
        ? String((errBody as { error: unknown }).error)
        : `Request failed with status ${resp.status}`
    throw new ApiError(resp.status, errMsg)
  }

  const json: unknown = await resp.json()
  if (typeof json !== "object" || json === null || !("success" in json)) {
    throw new ApiError(resp.status, "Invalid API response: missing success field")
  }
  return json as T
}

function apiRawFetch(endpoint: string, init?: RequestInit): Promise<Response> {
  const token = useAuthStore.getState().token
  const baseUrl = getApiBaseUrl()
  const url = baseUrl ? `${baseUrl}${endpoint}` : endpoint
  return fetch(url, {
    ...init,
    headers: {
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...init?.headers,
    },
  })
}

// ── Auth ──

export function login(username: string, password: string) {
  return apiFetch<{ success: boolean; data: { token: string; expireAt: string } }>(
    "/api/v1/user/login",
    { method: "POST", body: { username, password } },
  )
}

export function changePassword(newPassword: string) {
  return apiFetch<{ success: boolean }>("/api/v1/user/change-password", {
    method: "POST",
    body: { newPassword },
  })
}

export function changeUsername(username: string) {
  return apiFetch<{ success: boolean }>("/api/v1/user/change-username", {
    method: "POST",
    body: { username },
  })
}

export function getAuthStatus() {
  return apiFetch<{ success: boolean; data: { authenticated: boolean } }>("/api/v1/user/status")
}

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

export function listChannels() {
  return apiFetch<{ success: boolean; data: { channels: unknown[] } }>("/api/v1/channel/list")
}

export function createChannel(data: ChannelInput) {
  return apiFetch<{ success: boolean; data: unknown }>("/api/v1/channel/create", {
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

export interface ApiKeyInput {
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

// ── Stats ──

export function getGlobalStats() {
  return apiFetch<{ success: boolean; data: Record<string, unknown> }>("/api/v1/stats/global")
}

export function getChannelStats() {
  return apiFetch<{ success: boolean; data: ChannelStatsRow[] }>("/api/v1/stats/channel")
}

export function getTotalStats() {
  return apiFetch<{ success: boolean; data: StatsMetrics }>("/api/v1/stats/total")
}

function getBrowserTz(): string {
  const offset = -new Date().getTimezoneOffset()
  const sign = offset >= 0 ? "+" : "-"
  const abs = Math.abs(offset)
  const h = String(Math.floor(abs / 60)).padStart(2, "0")
  const m = String(abs % 60).padStart(2, "0")
  return `${sign}${h}:${m}`
}

export function getTodayStats() {
  return apiFetch<{ success: boolean; data: StatsDaily }>(
    `/api/v1/stats/today?tz=${encodeURIComponent(getBrowserTz())}`,
  )
}

export function getDailyStats() {
  return apiFetch<{ success: boolean; data: StatsDaily[] }>(
    `/api/v1/stats/daily?tz=${encodeURIComponent(getBrowserTz())}`,
  )
}

export function getHourlyStats(start?: string, end?: string) {
  const params = new URLSearchParams()
  if (start) params.set("start", start)
  if (end) params.set("end", end)
  params.set("tz", getBrowserTz())
  return apiFetch<{ success: boolean; data: StatsHourly[] }>(
    `/api/v1/stats/hourly?${params.toString()}`,
  )
}

export function getModelStats() {
  return apiFetch<{ success: boolean; data: ModelStatsItem[] }>("/api/v1/stats/model")
}

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

// ── Model Profiles ──

export interface ModelProfile {
  id: number
  name: string
  provider: string
  models: string[]
  isBuiltin: boolean
  createdAt?: string
  updatedAt?: string
  groupCount: number
}

export function listProfiles() {
  return apiFetch<{ success: boolean; data: { profiles: ModelProfile[] } }>(
    "/api/v1/model/profiles",
  )
}

export function createProfile(data: { name: string; provider?: string; models?: string[] }) {
  return apiFetch<{ success: boolean; data: ModelProfile }>("/api/v1/model/profiles/create", {
    method: "POST",
    body: data,
  })
}

export function updateProfile(data: {
  id: number
  name: string
  provider?: string
  models?: string[]
}) {
  return apiFetch<{ success: boolean }>("/api/v1/model/profiles/update", {
    method: "POST",
    body: data,
  })
}

export function deleteProfile(id: number) {
  return apiFetch<{ success: boolean }>("/api/v1/model/profiles/delete", {
    method: "POST",
    body: { id },
  })
}

export interface ProfilePreviewGroup {
  key: string
  name: string
  model: string
  virtual: boolean
  materialized: boolean
  groupId?: number
}

export function listProfileGroupsPreview(profileId: number) {
  return apiFetch<{ success: boolean; data: { groups: ProfilePreviewGroup[] } }>(
    `/api/v1/model/profiles/${profileId}/groups-preview`,
  )
}

export function activateProfile(id: number) {
  return apiFetch<{ success: boolean }>("/api/v1/model/profiles/activate", {
    method: "POST",
    body: { id },
  })
}

export function materializeProfileGroups(profileId: number, data?: { models?: string[] }) {
  return apiFetch<{ success: boolean; data: { created: number; existing: number; total: number } }>(
    `/api/v1/model/profiles/${profileId}/groups-materialize`,
    {
      method: "POST",
      body: data ?? {},
    },
  )
}

// ── Routing Rules ──

export interface RoutingConditionItem {
  field: string
  operator: string
  value: string
}

export interface RoutingActionItem {
  type: "reject" | "route" | "rewrite"
  groupName?: string
  modelName?: string
  statusCode?: number
  message?: string
}

export interface RoutingRule {
  id: number
  name: string
  priority: number
  enabled: boolean
  conditions: RoutingConditionItem[]
  action: RoutingActionItem
}

export interface RoutingRuleInput {
  id?: number
  name: string
  priority: number
  enabled: boolean
  conditions: RoutingConditionItem[]
  action: RoutingActionItem
}

export function listRoutingRules() {
  return apiFetch<{ success: boolean; data: { rules: RoutingRule[] } }>("/api/v1/routing-rule/list")
}

export function createRoutingRule(data: Omit<RoutingRuleInput, "id">) {
  return apiFetch<{ success: boolean; data: RoutingRule }>("/api/v1/routing-rule/create", {
    method: "POST",
    body: data,
  })
}

export function updateRoutingRule(data: Partial<RoutingRuleInput> & { id: number }) {
  return apiFetch<{ success: boolean }>("/api/v1/routing-rule/update", {
    method: "POST",
    body: data,
  })
}

export function deleteRoutingRule(id: number) {
  return apiFetch<{ success: boolean }>(`/api/v1/routing-rule/delete/${id}`, {
    method: "DELETE",
  })
}

// ── Channel Health ──

export function getChannelHealth() {
  return apiFetch<{ success: boolean; data: { health: Record<string, number> } }>(
    "/api/v1/channel/health",
  )
}

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

// ── MCP Logs ──

export interface MCPLogRecord {
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

export function clearMCPLogs() {
  return apiFetch<{ success: boolean }>("/api/v1/mcp-log/clear", { method: "DELETE" })
}

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
