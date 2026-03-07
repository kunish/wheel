interface RuntimeListQueryKeyInput {
  page?: number
  pageSize?: number
  search?: string
  channelType?: number
}

export const codexAuthFilesQueryKey = (channelId: number, input?: RuntimeListQueryKeyInput) =>
  ["codex-auth-files", channelId, input ?? {}] as const

export const codexQuotaQueryKey = (channelId: number, input?: RuntimeListQueryKeyInput) =>
  ["codex-quota", channelId, input ?? {}] as const

export const channelsQueryKey = ["channels"] as const

export const codexUploadRefreshQueryKeys = (channelId: number) =>
  [codexAuthFilesQueryKey(channelId), codexQuotaQueryKey(channelId), channelsQueryKey] as const
