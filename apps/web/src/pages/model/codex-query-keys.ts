export const codexAuthFilesQueryKey = (channelId: number) =>
  ["codex-auth-files", channelId] as const

export const codexQuotaQueryKey = (channelId: number) => ["codex-quota", channelId] as const

export const channelsQueryKey = ["channels"] as const

export const codexUploadRefreshQueryKeys = (channelId: number) =>
  [codexAuthFilesQueryKey(channelId), codexQuotaQueryKey(channelId), channelsQueryKey] as const
