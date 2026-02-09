import type { APIKey, Channel, Group, LLMInfo, RelayLog } from "./models.js"

// Generic API response wrapper
export interface ApiResponse<T> {
  success: boolean
  data?: T
  error?: string
}

// Auth
export interface LoginRequest {
  username: string
  password: string
}

export interface LoginResponse {
  token: string
  expireAt: string
}

export interface ChangePasswordRequest {
  oldPassword: string
  newPassword: string
}

export interface ChangeUsernameRequest {
  username: string
}

// Channel
export interface ChannelCreateRequest {
  name: string
  type: number
  enabled: boolean
  baseUrls: { url: string }[]
  keys: { channelKey: string; remark?: string }[]
  model: string[]
  customModel?: string
  autoSync?: boolean
  autoGroup?: number
  customHeader?: { key: string; value: string }[]
  paramOverride?: string
}

export interface ChannelUpdateRequest extends Partial<ChannelCreateRequest> {
  id: number
}

export interface ChannelEnableRequest {
  id: number
  enabled: boolean
}

export interface ChannelListResponse {
  channels: Channel[]
}

// Group
export interface GroupCreateRequest {
  name: string
  mode: number
  firstTokenTimeOut?: number
  items: {
    channelId: number
    modelName: string
    priority?: number
    weight?: number
  }[]
}

export interface GroupUpdateRequest extends Partial<GroupCreateRequest> {
  id: number
}

export interface GroupListResponse {
  groups: Group[]
}

export interface ModelListResponse {
  models: string[]
}

// API Key
export interface APIKeyCreateRequest {
  name: string
  expireAt?: number
  maxCost?: number
  supportedModels?: string
}

export interface APIKeyUpdateRequest extends Partial<APIKeyCreateRequest> {
  id: number
  enabled?: boolean
}

export interface APIKeyListResponse {
  apiKeys: APIKey[]
}

export interface APIKeyStatsResponse {
  totalRequests: number
  totalCost: number
  totalInputTokens: number
  totalOutputTokens: number
}

// Log
export interface LogListRequest {
  page?: number
  pageSize?: number
  model?: string
  channelId?: number
  status?: "success" | "error"
  startTime?: number
  endTime?: number
}

export interface LogListResponse {
  logs: RelayLog[]
  total: number
  page: number
  pageSize: number
}

// Stats
export interface GlobalStatsResponse {
  totalRequests: number
  totalInputTokens: number
  totalOutputTokens: number
  totalCost: number
  activeChannels: number
  activeGroups: number
}

export interface ChannelStatsResponse {
  channels: {
    channelId: number
    channelName: string
    totalRequests: number
    totalCost: number
    avgLatency: number
  }[]
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

export interface DailyStatsItem {
  date: string
  input_token: number
  output_token: number
  input_cost: number
  output_cost: number
  wait_time: number
  request_success: number
  request_failed: number
}

export interface HourlyStatsItem {
  hour: number
  date: string
  input_token: number
  output_token: number
  input_cost: number
  output_cost: number
  wait_time: number
  request_success: number
  request_failed: number
}

// Sync
export interface SyncResult {
  syncedChannels: number
  newModels: string[]
  removedModels: string[]
  errors: string[]
}

export interface FetchModelResponse {
  models: string[]
}

// Settings
export interface SettingsResponse {
  settings: Record<string, string>
}

export interface SettingsUpdateRequest {
  settings: Record<string, string>
}

// LLM Price
export interface LLMListResponse {
  models: LLMInfo[]
}

export interface LLMCreateRequest {
  name: string
  inputPrice: number
  outputPrice: number
}

export interface LLMUpdateRequest {
  id: number
  name?: string
  inputPrice?: number
  outputPrice?: number
}

export interface LLMDeleteRequest {
  id: number
}

export interface LLMPriceSyncResponse {
  synced: number
  updated: number
}

// Data Export / Import
export interface DBDump {
  version: number
  exportedAt: string
  channels: Channel[]
  groups: Group[]
  apiKeys: APIKey[]
  settings: { key: string; value: string }[]
  relayLogs?: RelayLog[]
}

export interface ImportResult {
  channels: { added: number; skipped: number }
  groups: { added: number; skipped: number }
  apiKeys: { added: number; skipped: number }
  settings: { added: number; skipped: number }
}
