import type { ChannelInput } from "@/lib/api/channels"

export const RUNTIME_MANAGED_CHANNEL_KEY = "managed-by-auth-files"

export type RuntimeProviderKey = "codex" | "copilot" | "codex-cli" | "antigravity"

interface SaveChannelResponse {
  data?: {
    id?: unknown
  } | null
}

interface RuntimeChannelModels {
  id?: number
  model?: string[] | null
  fetchedModel?: string[] | null
}

export function getRuntimeProviderKey(channelType?: number): RuntimeProviderKey | null {
  if (channelType === 33) {
    return "codex"
  }
  if (channelType === 34) {
    return "copilot"
  }
  if (channelType === 35) {
    return "codex-cli"
  }
  if (channelType === 36) {
    return "antigravity"
  }
  return null
}

export function isRuntimeChannelType(channelType?: number): boolean {
  return getRuntimeProviderKey(channelType) !== null
}

export function adaptChannelDraftForType<T extends ChannelInput>(form: T, channelType: number): T {
  const isRuntime = isRuntimeChannelType(channelType)
  const currentKey = form.keys[0]?.channelKey ?? ""
  const currentRemark = form.keys[0]?.remark ?? ""

  return {
    ...form,
    type: channelType,
    baseUrls: isRuntime ? [{ url: "", delay: form.baseUrls[0]?.delay ?? 0 }] : form.baseUrls,
    keys: isRuntime
      ? [{ channelKey: RUNTIME_MANAGED_CHANNEL_KEY, remark: currentRemark }]
      : currentKey === RUNTIME_MANAGED_CHANNEL_KEY
        ? [{ channelKey: "", remark: currentRemark }]
        : form.keys,
  }
}

export function shouldShowGenericModelFetch(channelType?: number): boolean {
  return !isRuntimeChannelType(channelType)
}

export async function ensureCodexChannelId({
  form,
  saveChannel,
}: {
  form: ChannelInput
  saveChannel: (form: ChannelInput) => Promise<SaveChannelResponse>
}) {
  if (typeof form.id === "number" && form.id > 0) {
    return form.id
  }

  const response = await saveChannel(form)
  const channelId = response.data?.id

  if (typeof channelId !== "number" || channelId <= 0) {
    throw new Error("Failed to save channel")
  }

  return channelId
}

export function mergeRuntimeChannelModels<T extends ChannelInput>(
  form: T,
  channel: RuntimeChannelModels | null | undefined,
): T {
  if (
    !channel ||
    typeof form.id !== "number" ||
    typeof channel.id !== "number" ||
    form.id !== channel.id
  ) {
    return form
  }

  return {
    ...form,
    model: [...(channel.model ?? [])],
    fetchedModel: [...(channel.fetchedModel ?? [])],
  }
}
