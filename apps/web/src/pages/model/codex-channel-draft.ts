import type { ChannelInput } from "@/lib/api/channels"

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
