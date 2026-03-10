import { useAuthStore } from "../store/auth"

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

export async function apiFetch<T extends { success: boolean }>(
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

export function apiRawFetch(endpoint: string, init?: RequestInit): Promise<Response> {
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
