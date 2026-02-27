import type { GroupItemForm } from "./group-dialog"

// ─── Shared record types ───────────────────────

export interface ModelPrice {
  id: number
  name: string
  inputPrice: number
  outputPrice: number
  source: string
  createdAt: string
  updatedAt: string
}

export interface ChannelRecord {
  id: number
  name: string
  type: number
  enabled: boolean
  model: string[]
  fetchedModel: string[]
  customModel: string
  baseUrls: { url: string; delay: number }[]
  keys: { channelKey: string; remark: string }[]
  paramOverride: string | null
}

export interface GroupRecord {
  id: number
  name: string
  mode: number
  firstTokenTimeOut: number
  sessionKeepTime: number
  order: number
  items: GroupItemForm[]
}

// ─── Drag data types ───────────────────────────

export interface DragDataModel {
  type: "model"
  model: string
  channelId: number
  channelName: string
}

export interface DragDataChannel {
  type: "channel"
  channel: ChannelRecord
}

export interface DragDataGroup {
  type: "group"
  groupId: number
}

export type DragData = DragDataModel | DragDataChannel | DragDataGroup

// ─── Helpers ───────────────────────────────────

export function parseModels(model: string[]): string[] {
  if (!model || !Array.isArray(model)) return []
  return model.filter(Boolean)
}
