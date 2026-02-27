import { apiFetch } from "./client"

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

export interface ProfilePreviewGroup {
  key: string
  name: string
  model: string
  virtual: boolean
  materialized: boolean
  groupId?: number
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
