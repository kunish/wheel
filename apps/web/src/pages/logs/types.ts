export interface LogDetail {
  id: number
  time: number
  requestModelName: string
  actualModelName: string
  channelName: string
  channelId: number
  inputTokens: number
  outputTokens: number
  cost: number
  ftut: number
  useTime: number
  requestContent: string
  upstreamContent: string | null
  responseContent: string
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
