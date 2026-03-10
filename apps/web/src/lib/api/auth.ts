import { apiFetch } from "./client"

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
