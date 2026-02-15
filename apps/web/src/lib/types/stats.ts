export interface StatsMetrics {
  input_token: number
  output_token: number
  input_cost: number
  output_cost: number
  wait_time: number
  request_success: number
  request_failed: number
}

export interface StatsDaily extends StatsMetrics {
  date: string
}

export interface StatsHourly extends StatsMetrics {
  hour: number
  date: string
}

export interface ChannelStatsRow {
  channelId: number
  channelName: string
  totalRequests: number
  totalCost: number
  avgLatency: number
}

export interface ModelStatsItem {
  model: string
  requestCount: number
  inputTokens: number
  outputTokens: number
  totalCost: number
  avgLatency: number
  avgFirstTokenTime: number
}

export interface ModelMeta {
  name: string
  provider: string
  providerName: string
  logoUrl: string
}
