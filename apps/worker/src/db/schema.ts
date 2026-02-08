import { sql } from "drizzle-orm"
import { integer, real, sqliteTable, text } from "drizzle-orm/sqlite-core"

export const users = sqliteTable("users", {
  id: integer("id").primaryKey({ autoIncrement: true }),
  username: text("username").notNull().unique(),
  password: text("password").notNull(),
})

export const channels = sqliteTable("channels", {
  id: integer("id").primaryKey({ autoIncrement: true }),
  name: text("name").notNull().unique(),
  type: integer("type").notNull().default(1),
  enabled: integer("enabled", { mode: "boolean" }).notNull().default(true),
  baseUrls: text("base_urls", { mode: "json" })
    .notNull()
    .default("[]")
    .$type<{ url: string; delay: number }[]>(),
  model: text("model", { mode: "json" }).notNull().default("[]").$type<string[]>(),
  customModel: text("custom_model").notNull().default(""),
  proxy: integer("proxy", { mode: "boolean" }).notNull().default(false),
  autoSync: integer("auto_sync", { mode: "boolean" }).notNull().default(false),
  autoGroup: integer("auto_group").notNull().default(0),
  customHeader: text("custom_header", { mode: "json" })
    .notNull()
    .default("[]")
    .$type<{ key: string; value: string }[]>(),
  paramOverride: text("param_override"),
  channelProxy: text("channel_proxy"),
  matchRegex: text("match_regex"),
})

export const channelKeys = sqliteTable("channel_keys", {
  id: integer("id").primaryKey({ autoIncrement: true }),
  channelId: integer("channel_id")
    .notNull()
    .references(() => channels.id, { onDelete: "cascade" }),
  enabled: integer("enabled", { mode: "boolean" }).notNull().default(true),
  channelKey: text("channel_key").notNull(),
  statusCode: integer("status_code").notNull().default(0),
  lastUseTimestamp: integer("last_use_timestamp").notNull().default(0),
  totalCost: real("total_cost").notNull().default(0),
  remark: text("remark").notNull().default(""),
})

export const groups = sqliteTable("groups", {
  id: integer("id").primaryKey({ autoIncrement: true }),
  name: text("name").notNull(),
  mode: integer("mode").notNull().default(1),
  matchRegex: text("match_regex").notNull().default(""),
  firstTokenTimeOut: integer("first_token_time_out").notNull().default(30),
})

export const groupItems = sqliteTable("group_items", {
  id: integer("id").primaryKey({ autoIncrement: true }),
  groupId: integer("group_id")
    .notNull()
    .references(() => groups.id, { onDelete: "cascade" }),
  channelId: integer("channel_id")
    .notNull()
    .references(() => channels.id, { onDelete: "cascade" }),
  modelName: text("model_name").notNull().default(""),
  priority: integer("priority").notNull().default(0),
  weight: integer("weight").notNull().default(1),
})

export const apiKeys = sqliteTable("api_keys", {
  id: integer("id").primaryKey({ autoIncrement: true }),
  name: text("name").notNull(),
  apiKey: text("api_key").notNull().unique(),
  enabled: integer("enabled", { mode: "boolean" }).notNull().default(true),
  expireAt: integer("expire_at").notNull().default(0),
  maxCost: real("max_cost").notNull().default(0),
  supportedModels: text("supported_models").notNull().default(""),
  totalCost: real("total_cost").notNull().default(0),
})

export const relayLogs = sqliteTable("relay_logs", {
  id: integer("id").primaryKey(),
  time: integer("time").notNull(),
  requestModelName: text("request_model_name").notNull().default(""),
  channelId: integer("channel_id").notNull().default(0),
  channelName: text("channel_name").notNull().default(""),
  actualModelName: text("actual_model_name").notNull().default(""),
  inputTokens: integer("input_tokens").notNull().default(0),
  outputTokens: integer("output_tokens").notNull().default(0),
  ftut: integer("ftut").notNull().default(0),
  useTime: integer("use_time").notNull().default(0),
  cost: real("cost").notNull().default(0),
  requestContent: text("request_content").notNull().default(""),
  responseContent: text("response_content").notNull().default(""),
  error: text("error").notNull().default(""),
  attempts: text("attempts", { mode: "json" }).notNull().default("[]").$type<
    {
      channelId: number
      channelName: string
      modelName: string
      round: number
      attemptNum: number
      success: boolean
      error: string
      duration: number
    }[]
  >(),
  totalAttempts: integer("total_attempts").notNull().default(0),
  successfulRound: integer("successful_round").notNull().default(0),
})

export const settings = sqliteTable("settings", {
  key: text("key").primaryKey(),
  value: text("value").notNull().default(""),
})

export const llmPrices = sqliteTable("llm_prices", {
  id: integer("id").primaryKey({ autoIncrement: true }),
  name: text("name").notNull().unique(),
  inputPrice: real("input_price").notNull().default(0),
  outputPrice: real("output_price").notNull().default(0),
  cacheReadPrice: real("cache_read_price").notNull().default(0),
  cacheWritePrice: real("cache_write_price").notNull().default(0),
  source: text("source").notNull().default("manual"),
  createdAt: text("created_at").default(sql`(datetime('now'))`),
  updatedAt: text("updated_at").default(sql`(datetime('now'))`),
})
