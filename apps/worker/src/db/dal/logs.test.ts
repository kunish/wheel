import BetterSqlite3 from "better-sqlite3"
import { drizzle } from "drizzle-orm/better-sqlite3"
import { beforeEach, describe, expect, it } from "vitest"
import * as schema from "../schema"
import { createLog, listLogs } from "./logs"

function createTestDb() {
  const sqlite = new BetterSqlite3(":memory:")
  sqlite.pragma("journal_mode = WAL")

  sqlite.exec(`
    CREATE TABLE relay_logs (
      id INTEGER PRIMARY KEY,
      time INTEGER NOT NULL,
      request_model_name TEXT NOT NULL DEFAULT '',
      channel_id INTEGER NOT NULL DEFAULT 0,
      channel_name TEXT NOT NULL DEFAULT '',
      actual_model_name TEXT NOT NULL DEFAULT '',
      input_tokens INTEGER NOT NULL DEFAULT 0,
      output_tokens INTEGER NOT NULL DEFAULT 0,
      ftut INTEGER NOT NULL DEFAULT 0,
      use_time INTEGER NOT NULL DEFAULT 0,
      cost REAL NOT NULL DEFAULT 0,
      request_content TEXT NOT NULL DEFAULT '',
      response_content TEXT NOT NULL DEFAULT '',
      error TEXT NOT NULL DEFAULT '',
      attempts TEXT NOT NULL DEFAULT '[]',
      total_attempts INTEGER NOT NULL DEFAULT 0
    )
  `)

  return drizzle(sqlite, { schema }) as any
}

describe("listLogs DAL", () => {
  let db: ReturnType<typeof createTestDb>

  beforeEach(async () => {
    db = createTestDb()

    await createLog(db, {
      time: 1700000100,
      requestModelName: "gpt-4o",
      channelId: 1,
      channelName: "OpenAI",
      inputTokens: 100,
      outputTokens: 50,
      useTime: 300,
      cost: 0.01,
      requestContent: '{"messages":[{"role":"user","content":"Hello"}]}',
      responseContent: '{"choices":[{"message":{"content":"Hi"}}]}',
      error: "",
      totalAttempts: 1,
    })

    await createLog(db, {
      time: 1700000200,
      requestModelName: "claude-sonnet-4-20250514",
      channelId: 2,
      channelName: "Anthropic",
      inputTokens: 500,
      outputTokens: 200,
      useTime: 1500,
      cost: 0.05,
      requestContent: '{"messages":[{"role":"user","content":"Write code"}]}',
      responseContent: '{"content":[{"text":"def hello(): pass"}]}',
      error: "",
      totalAttempts: 1,
    })

    await createLog(db, {
      time: 1700000300,
      requestModelName: "gpt-4o",
      channelId: 1,
      channelName: "OpenAI",
      inputTokens: 200,
      outputTokens: 0,
      useTime: 100,
      cost: 0,
      requestContent: '{"messages":[{"role":"user","content":"Fail"}]}',
      responseContent: "",
      error: "rate_limit_exceeded",
      totalAttempts: 2,
    })
  })

  it("returns all logs with default pagination", async () => {
    const result = await listLogs(db, {})
    expect(result.logs).toHaveLength(3)
    expect(result.total).toBe(3)
    expect(result.page).toBe(1)
    expect(result.pageSize).toBe(20)
  })

  it("returns logs ordered by time descending", async () => {
    const result = await listLogs(db, {})
    expect(result.logs[0].time).toBe(1700000300)
    expect(result.logs[2].time).toBe(1700000100)
  })

  it("filters by keyword matching requestModelName", async () => {
    const result = await listLogs(db, { keyword: "claude" })
    expect(result.logs).toHaveLength(1)
    expect(result.logs[0].requestModelName).toBe("claude-sonnet-4-20250514")
  })

  it("filters by keyword matching channelName", async () => {
    const result = await listLogs(db, { keyword: "Anthropic" })
    expect(result.logs).toHaveLength(1)
    expect(result.logs[0].channelName).toBe("Anthropic")
  })

  it("filters by keyword matching error field", async () => {
    const result = await listLogs(db, { keyword: "rate_limit" })
    expect(result.logs).toHaveLength(1)
    expect(result.logs[0].error).toBe("rate_limit_exceeded")
  })

  it("filters by keyword matching requestContent", async () => {
    const result = await listLogs(db, { keyword: "Write code" })
    expect(result.logs).toHaveLength(1)
    expect(result.logs[0].channelName).toBe("Anthropic")
  })

  it("filters by keyword matching responseContent", async () => {
    const result = await listLogs(db, { keyword: "def hello" })
    expect(result.logs).toHaveLength(1)
  })

  it("keyword uses OR logic across fields", async () => {
    const result = await listLogs(db, { keyword: "OpenAI" })
    expect(result.logs).toHaveLength(2)
  })

  it("filters by channelId", async () => {
    const result = await listLogs(db, { channelId: 2 })
    expect(result.logs).toHaveLength(1)
    expect(result.logs[0].channelName).toBe("Anthropic")
  })

  it("returns empty for non-existent channelId", async () => {
    const result = await listLogs(db, { channelId: 999 })
    expect(result.logs).toHaveLength(0)
    expect(result.total).toBe(0)
  })

  it("filters by startTime", async () => {
    const result = await listLogs(db, { startTime: 1700000250 })
    expect(result.logs).toHaveLength(1)
    expect(result.logs[0].time).toBe(1700000300)
  })

  it("filters by endTime", async () => {
    const result = await listLogs(db, { endTime: 1700000150 })
    expect(result.logs).toHaveLength(1)
    expect(result.logs[0].time).toBe(1700000100)
  })

  it("filters by time range (start and end)", async () => {
    const result = await listLogs(db, { startTime: 1700000150, endTime: 1700000250 })
    expect(result.logs).toHaveLength(1)
    expect(result.logs[0].time).toBe(1700000200)
  })

  it("filters for errors only", async () => {
    const result = await listLogs(db, { hasError: true })
    expect(result.logs).toHaveLength(1)
    expect(result.logs[0].error).toBe("rate_limit_exceeded")
  })

  it("filters for success only", async () => {
    const result = await listLogs(db, { hasError: false })
    expect(result.logs).toHaveLength(2)
    expect(result.logs.every((l) => l.error === "")).toBe(true)
  })

  it("filters by model name (LIKE match)", async () => {
    const result = await listLogs(db, { model: "gpt-4o" })
    expect(result.logs).toHaveLength(2)
  })

  it("filters by partial model name", async () => {
    const result = await listLogs(db, { model: "claude" })
    expect(result.logs).toHaveLength(1)
  })

  it("combines channelId and hasError filters", async () => {
    const result = await listLogs(db, { channelId: 1, hasError: true })
    expect(result.logs).toHaveLength(1)
    expect(result.logs[0].error).toBe("rate_limit_exceeded")
  })

  it("combines keyword and time range filters", async () => {
    const result = await listLogs(db, { keyword: "gpt", startTime: 1700000250 })
    expect(result.logs).toHaveLength(1)
    expect(result.logs[0].time).toBe(1700000300)
  })

  it("combines all filters returning empty result", async () => {
    const result = await listLogs(db, {
      model: "claude",
      channelId: 1,
      hasError: true,
    })
    expect(result.logs).toHaveLength(0)
    expect(result.total).toBe(0)
  })

  it("paginates correctly", async () => {
    const page1 = await listLogs(db, { page: 1, pageSize: 2 })
    expect(page1.logs).toHaveLength(2)
    expect(page1.total).toBe(3)

    const page2 = await listLogs(db, { page: 2, pageSize: 2 })
    expect(page2.logs).toHaveLength(1)
    expect(page2.total).toBe(3)
  })

  it("returns empty logs for out-of-range page", async () => {
    const result = await listLogs(db, { page: 10, pageSize: 20 })
    expect(result.logs).toHaveLength(0)
    expect(result.total).toBe(3)
  })

  it("returns empty for no matching keyword", async () => {
    const result = await listLogs(db, { keyword: "nonexistent_query_xyz" })
    expect(result.logs).toHaveLength(0)
    expect(result.total).toBe(0)
  })
})
