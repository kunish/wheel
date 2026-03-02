import { apiFetch } from "./client"

// ── Tags ──

export interface Tag {
  id: number
  name: string
  color: string
  description: string
  channelCount: number
  keyCount: number
  createdAt?: string
}

export interface TagInput {
  id?: number
  name: string
  color: string
  description: string
}

export function listTags() {
  return apiFetch<{ success: boolean; data: { tags: Tag[] } }>("/api/v1/tag/list")
}

export function createTag(data: Omit<TagInput, "id">) {
  return apiFetch<{ success: boolean; data: Tag }>("/api/v1/tag/create", {
    method: "POST",
    body: data,
  })
}

export function updateTag(data: Partial<TagInput> & { id: number }) {
  return apiFetch<{ success: boolean }>("/api/v1/tag/update", {
    method: "POST",
    body: data,
  })
}

export function deleteTag(id: number) {
  return apiFetch<{ success: boolean }>(`/api/v1/tag/delete/${id}`, {
    method: "DELETE",
  })
}
