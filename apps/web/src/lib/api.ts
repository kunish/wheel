import { useAuthStore } from "@/lib/store/auth"

const API_BASE_URL = ""

interface ApiOptions {
  method?: string
  body?: unknown
  headers?: Record<string, string>
}

class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message)
    this.name = "ApiError"
  }
}

async function apiFetch<T>(endpoint: string, opts: ApiOptions = {}): Promise<T> {
  const { method = "GET", body, headers = {} } = opts
  const token = useAuthStore.getState().token

  const reqHeaders: Record<string, string> = {
    "Content-Type": "application/json",
    ...headers,
  }

  if (token) {
    reqHeaders.Authorization = `Bearer ${token}`
  }

  const resp = await fetch(`${API_BASE_URL}${endpoint}`, {
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
    const data = await resp.json().catch(() => ({}))
    throw new ApiError(
      resp.status,
      (data as { error?: string }).error ?? `Request failed with status ${resp.status}`,
    )
  }

  return resp.json() as Promise<T>
}

// Auth
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

// Channels
export function listChannels() {
  return apiFetch<{ success: boolean; data: { channels: unknown[] } }>("/api/v1/channel/list")
}

export function createChannel(data: object) {
  return apiFetch<{ success: boolean; data: unknown }>("/api/v1/channel/create", {
    method: "POST",
    body: data,
  })
}

export function updateChannel(data: object) {
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
  return apiFetch<{ success: boolean; data: { models: string[] } }>(
    "/api/v1/channel/fetch-model-preview",
    {
      method: "POST",
      body: data,
    },
  )
}

// Groups
export function listGroups() {
  return apiFetch<{ success: boolean; data: { groups: unknown[] } }>("/api/v1/group/list")
}

export function createGroup(data: object) {
  return apiFetch<{ success: boolean; data: unknown }>("/api/v1/group/create", {
    method: "POST",
    body: data,
  })
}

export function updateGroup(data: object) {
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

// API Keys
export function listApiKeys() {
  return apiFetch<{ success: boolean; data: { apiKeys: unknown[] } }>("/api/v1/apikey/list")
}

export function createApiKey(data: object) {
  return apiFetch<{ success: boolean; data: unknown }>("/api/v1/apikey/create", {
    method: "POST",
    body: data,
  })
}

export function updateApiKey(data: object) {
  return apiFetch<{ success: boolean; data: unknown }>("/api/v1/apikey/update", {
    method: "POST",
    body: data,
  })
}

export function deleteApiKey(id: number) {
  return apiFetch<{ success: boolean }>(`/api/v1/apikey/delete/${id}`, {
    method: "DELETE",
  })
}

// Logs
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
  const token = useAuthStore.getState().token
  return fetch(`${API_BASE_URL}/api/v1/log/replay/${id}`, {
    method: "POST",
    headers: {
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
  })
}

// Stats
export interface StatsMetrics {
  input_token: number
  output_token: number
  input_cost: number
  output_cost: number
  wait_time: number
  request_success: number
  request_failed: number
}

export interface StatsDaily extends StatsMetrics {
  date: string
}

export interface StatsHourly extends StatsMetrics {
  hour: number
  date: string
}

export interface ChannelStatsRow extends StatsMetrics {
  channelId: number
  channelName: string
  totalRequests: number
  totalCost: number
  avgLatency: number
}

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

export interface ModelStatsItem {
  model: string
  requestCount: number
  inputTokens: number
  outputTokens: number
  totalCost: number
  avgLatency: number
  avgFirstTokenTime: number
}

export function getModelStats() {
  return apiFetch<{ success: boolean; data: ModelStatsItem[] }>("/api/v1/stats/model")
}

// Settings
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

// Model Metadata
export interface ModelMeta {
  name: string
  provider: string
  providerName: string
  logoUrl: string
}

export function getModelMetadata() {
  return apiFetch<{ success: boolean; data: Record<string, ModelMeta> }>("/api/v1/model/metadata")
}

// Model Prices
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

// Data Export/Import
export function exportData(includeLogs: boolean = false) {
  const token = useAuthStore.getState().token
  return fetch(`${API_BASE_URL}/api/v1/setting/export?include_logs=${includeLogs}`, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  })
}

export function importData(file: File) {
  const token = useAuthStore.getState().token
  const formData = new FormData()
  formData.append("file", file)
  return fetch(`${API_BASE_URL}/api/v1/setting/import`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
    },
    body: formData,
  }).then((res) => res.json())
}
