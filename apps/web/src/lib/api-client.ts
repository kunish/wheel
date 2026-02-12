import type { paths } from "./api.gen"
import createClient from "openapi-fetch"
import { useAuthStore } from "./store/auth"

export function createApiClient() {
  const { apiBaseUrl, token } = useAuthStore.getState()

  return createClient<paths>({
    baseUrl: apiBaseUrl || undefined,
    headers: {
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
  })
}
