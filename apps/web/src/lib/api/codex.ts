import { ApiError, apiFetch, apiRawFetch } from "./client"

/**
 * Returns the API URL prefix for runtime-managed channels (Codex / Copilot).
 * Copilot channels (type 34) use `/copilot/` routes, everything else uses `/codex/`.
 */
function runtimePrefix(channelType?: number): string {
  return channelType === 34 ? "copilot" : "codex"
}

/**
 * Returns the auth-file provider filter string for runtime-managed channels.
 * Copilot channels filter by "copilot", Codex channels filter by "codex".
 */
export function runtimeProviderFilter(channelType?: number): string {
  return channelType === 34 ? "copilot" : "codex"
}

export interface CodexAuthFile {
  name: string
  provider: string
  type: string
  email?: string
  disabled?: boolean
  authIndex?: string
  auth_index?: string
}

export interface CodexCapabilities {
  localEnabled: boolean
  managementEnabled: boolean
  oauthEnabled: boolean
  modelsEnabled: boolean
}

export interface CodexQuotaWindow {
  usedPercent: number
  limitWindowSeconds: number
  resetAfterSeconds: number
  resetAt: string
  allowed: boolean
  limitReached: boolean
}

export interface CodexQuotaItem {
  name: string
  email?: string
  authIndex?: string
  planType?: string
  weekly: CodexQuotaWindow
  codeReview: CodexQuotaWindow
  snapshots?: Array<{
    id: string
    label: string
    percentRemaining: number
    remaining?: number
    entitlement?: number
    unlimited?: boolean
  }>
  resetAt?: string
  error?: string
}

export interface CodexAuthUploadResult {
  name: string
  status: string
  error?: string
}

export interface CodexAuthUploadBatchResult {
  total: number
  successCount: number
  failedCount: number
  results: CodexAuthUploadResult[]
}

export interface CodexAuthUploadToastState {
  level: "success" | "info" | "error"
  key: "codex.uploadSummarySuccess" | "codex.uploadSummaryPartial" | "codex.uploadSummaryError"
  values: {
    total: number
    successCount: number
    failedCount: number
  }
}

export interface CodexAuthBatchScope {
  names?: string[]
  allMatching?: boolean
  search?: string
  provider?: string
  excludeNames?: string[]
}

export function buildCodexAuthUploadFormData(files: File[]) {
  const formData = new FormData()
  for (const file of files) {
    formData.append("files", file)
  }
  return formData
}

export function getCodexAuthUploadToastState(
  result: CodexAuthUploadBatchResult,
): CodexAuthUploadToastState {
  const values = {
    total: result.total,
    successCount: result.successCount,
    failedCount: result.failedCount,
  }

  if (result.successCount > 0 && result.failedCount === 0) {
    return { level: "success", key: "codex.uploadSummarySuccess", values }
  }
  if (result.successCount > 0) {
    return { level: "info", key: "codex.uploadSummaryPartial", values }
  }
  return { level: "error", key: "codex.uploadSummaryError", values }
}

export function listCodexAuthFiles(
  channelId: number,
  params?: {
    provider?: string
    search?: string
    disabled?: string
    status?: string
    page?: number
    pageSize?: number
    channelType?: number
  },
) {
  const prefix = runtimePrefix(params?.channelType)
  const query = new URLSearchParams()
  if (params?.provider) query.set("provider", params.provider)
  if (params?.search) query.set("search", params.search)
  if (params?.disabled) query.set("disabled", params.disabled)
  if (params?.status) query.set("status", params.status)
  if (params?.page) query.set("page", String(params.page))
  if (params?.pageSize) query.set("pageSize", String(params.pageSize))
  const suffix = query.toString()
  return apiFetch<{
    success: boolean
    data: {
      files: CodexAuthFile[]
      total: number
      page: number
      pageSize: number
      capabilities: CodexCapabilities
      quotaItems?: CodexQuotaItem[]
      cachedCount?: number
      totalUnfiltered?: number
    }
  }>(`/api/v1/channel/${channelId}/${prefix}/auth-files${suffix ? `?${suffix}` : ""}`)
}

export function patchCodexAuthFileStatus(
  channelId: number,
  input: { name: string; disabled: boolean },
  channelType?: number,
) {
  const prefix = runtimePrefix(channelType)
  return apiFetch<{ success: boolean; data: { status: string; disabled: boolean } }>(
    `/api/v1/channel/${channelId}/${prefix}/auth-files/status`,
    { method: "PATCH", body: input },
  )
}

export function patchCodexAuthFileStatusBatch(
  channelId: number,
  input: CodexAuthBatchScope & { disabled: boolean },
  channelType?: number,
) {
  const prefix = runtimePrefix(channelType)
  return apiFetch<{ success: boolean; data: CodexAuthUploadBatchResult }>(
    `/api/v1/channel/${channelId}/${prefix}/auth-files/status/batch`,
    { method: "PATCH", body: input },
  )
}

export function deleteCodexAuthFile(
  channelId: number,
  input: { name?: string; all?: boolean },
  channelType?: number,
) {
  const prefix = runtimePrefix(channelType)
  const query = new URLSearchParams()
  if (input.name) query.set("name", input.name)
  if (input.all) query.set("all", "true")
  const suffix = query.toString()
  return apiFetch<{ success: boolean; data: { status: string; deleted?: number } }>(
    `/api/v1/channel/${channelId}/${prefix}/auth-files${suffix ? `?${suffix}` : ""}`,
    { method: "DELETE" },
  )
}

export function deleteCodexAuthFileBatch(
  channelId: number,
  input: CodexAuthBatchScope,
  channelType?: number,
) {
  const prefix = runtimePrefix(channelType)
  return apiFetch<{ success: boolean; data: CodexAuthUploadBatchResult }>(
    `/api/v1/channel/${channelId}/${prefix}/auth-files/delete/batch`,
    { method: "POST", body: input },
  )
}

export async function uploadCodexAuthFile(channelId: number, files: File[], channelType?: number) {
  const prefix = runtimePrefix(channelType)
  const formData = buildCodexAuthUploadFormData(files)

  const resp = await apiRawFetch(`/api/v1/channel/${channelId}/${prefix}/auth-files`, {
    method: "POST",
    body: formData,
  })

  if (!resp.ok) {
    const errBody: unknown = await resp.json().catch(() => ({}))
    const errMsg =
      typeof errBody === "object" && errBody !== null && "error" in errBody
        ? String((errBody as { error: unknown }).error)
        : `Request failed with status ${resp.status}`
    throw new ApiError(resp.status, errMsg)
  }

  return resp.json() as Promise<{ success: boolean; data: CodexAuthUploadBatchResult }>
}

export function getCodexAuthFileModels(channelId: number, name: string, channelType?: number) {
  const prefix = runtimePrefix(channelType)
  const query = new URLSearchParams({ name })
  return apiFetch<{ success: boolean; data: { models: Array<Record<string, unknown>> } }>(
    `/api/v1/channel/${channelId}/${prefix}/models?${query.toString()}`,
  )
}

export function listCodexQuota(
  channelId: number,
  params?: {
    search?: string
    page?: number
    pageSize?: number
    channelType?: number
  },
) {
  const prefix = runtimePrefix(params?.channelType)
  const query = new URLSearchParams()
  if (params?.search) query.set("search", params.search)
  if (params?.page) query.set("page", String(params.page))
  if (params?.pageSize) query.set("pageSize", String(params.pageSize))
  const suffix = query.toString()
  return apiFetch<{
    success: boolean
    data: { items: CodexQuotaItem[]; total: number; page: number; pageSize: number }
  }>(`/api/v1/channel/${channelId}/${prefix}/quota${suffix ? `?${suffix}` : ""}`)
}

export function syncCodexKeys(channelId: number, channelType?: number) {
  const prefix = runtimePrefix(channelType)
  return apiFetch<{ success: boolean; data: { synced: number; authFiles: number } }>(
    `/api/v1/channel/${channelId}/${prefix}/sync-keys`,
    { method: "POST" },
  )
}

export function startCodexOAuth(channelId: number, channelType?: number) {
  const prefix = runtimePrefix(channelType)
  return apiFetch<{ success: boolean; data: { url: string; state: string } }>(
    `/api/v1/channel/${channelId}/${prefix}/oauth/start`,
    { method: "POST" },
  )
}

export function getCodexOAuthStatus(channelId: number, state: string, channelType?: number) {
  const prefix = runtimePrefix(channelType)
  const query = new URLSearchParams({ state })
  return apiFetch<{ success: boolean; data: { status: string; error?: string } }>(
    `/api/v1/channel/${channelId}/${prefix}/oauth/status?${query.toString()}`,
  )
}
