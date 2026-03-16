export interface LogStats {
  totalRequests: number
  successRate: number
  averageLatency: number
  totalTokens: number
  totalCost: number
  tokenSpeed: number
}

export interface LogDetail {
  id: number
  time: number
  requestModelName: string
  actualModelName: string
  channelName: string
  channelId: number
  inputTokens: number
  outputTokens: number
  cacheReadTokens: number
  cacheCreationTokens: number
  cost: number
  ftut: number
  useTime: number
  requestContent: string
  requestHeaders: string
  upstreamContent: string | null
  responseContent: string
  responseHeaders: string
  error: string
  attempts: Array<{
    channelId: number
    channelKeyId?: number
    channelName: string
    modelName: string
    attemptNum: number
    status: "success" | "failed" | "circuit_break" | "skipped"
    duration: number
    sticky?: boolean
    msg?: string
  }>
  totalAttempts: number
}

export interface StreamingOverlay {
  thinkingContent: string
  responseContent: string
}
