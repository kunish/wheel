import type { ModelMeta } from "@/lib/api-client"

export interface ProviderGroup {
  provider: string
  providerName: string
  logoUrl: string | null
  models: string[]
}

export function groupModelsByProvider(
  modelIds: string[],
  metadataMap: Record<string, ModelMeta> | undefined,
): ProviderGroup[] {
  if (!metadataMap) {
    return [{ provider: "other", providerName: "Other", logoUrl: null, models: modelIds }]
  }

  const groups = new Map<string, ProviderGroup>()

  for (const id of modelIds) {
    const meta = metadataMap[id]
    const key = meta?.provider ?? "other"

    let group = groups.get(key)
    if (!group) {
      group = {
        provider: key,
        providerName: meta?.providerName ?? "Other",
        logoUrl: meta?.logoUrl ?? null,
        models: [],
      }
      groups.set(key, group)
    }
    group.models.push(id)
  }

  return Array.from(groups.values()).sort((a, b) => {
    if (a.provider === "other") return 1
    if (b.provider === "other") return -1
    return a.providerName.localeCompare(b.providerName)
  })
}
