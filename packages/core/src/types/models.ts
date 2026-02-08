import type { AutoGroupType, GroupMode, OutboundType } from "./enums.js"

export interface OpenAIModel {
  id: string
  object: "model"
  created: number
  owned_by: string
}

export interface AnthropicModel {
  id: string
  created_at: string
  display_name: string
  type: "model"
}

export interface BaseUrl {
  url: string
  delay: number
}

export interface ChannelKey {
  id: number
  channelId: number
  enabled: boolean
  channelKey: string
  statusCode: number
  lastUseTimestamp: number
  totalCost: number
  remark: string
}

export interface CustomHeader {
  key: string
  value: string
}

export interface Channel {
  id: number
  name: string
  type: OutboundType
  enabled: boolean
  baseUrls: BaseUrl[]
  keys: ChannelKey[]
  model: string[]
  customModel: string
  proxy: boolean
  autoSync: boolean
  autoGroup: AutoGroupType
  customHeader: CustomHeader[]
  paramOverride: string | null
  channelProxy: string | null
  matchRegex: string | null
}

export interface GroupItem {
  id: number
  groupId: number
  channelId: number
  modelName: string
  priority: number
  weight: number
}

export interface Group {
  id: number
  name: string
  mode: GroupMode
  matchRegex: string
  firstTokenTimeOut: number
  items: GroupItem[]
}

export interface APIKey {
  id: number
  name: string
  apiKey: string
  enabled: boolean
  expireAt: number
  maxCost: number
  supportedModels: string
}

export interface ChannelAttempt {
  channelId: number
  channelName: string
  modelName: string
  round: number
  attemptNum: number
  success: boolean
  error: string
  duration: number
}

export interface RelayLog {
  id: number
  time: number
  requestModelName: string
  channelId: number
  channelName: string
  actualModelName: string
  inputTokens: number
  outputTokens: number
  ftut: number
  useTime: number
  cost: number
  requestContent: string
  responseContent: string
  error: string
  attempts: ChannelAttempt[]
  totalAttempts: number
  successfulRound: number
}

export interface User {
  id: number
  username: string
  password: string
}

export interface Setting {
  key: string
  value: string
}

export interface LLMPrice {
  inputPrice: number // $/M tokens
  outputPrice: number // $/M tokens
}

export interface LLMInfo {
  id?: number
  name: string
  inputPrice: number
  outputPrice: number
  source: "manual" | "sync"
  createdAt?: string
  updatedAt?: string
}
