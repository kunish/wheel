import { describe, expect, it } from "vitest"
import {
  readPlaygroundMcpSnapshot,
  readPlaygroundSettings,
  writePlaygroundMcpSnapshot,
  writePlaygroundSettings,
} from "./persistence"

function createMemoryStorage() {
  const store = new Map<string, string>()
  return {
    getItem: (key: string) => (store.has(key) ? store.get(key)! : null),
    setItem: (key: string, value: string) => {
      store.set(key, value)
    },
  }
}

describe("playground persistence", () => {
  it("writes and reads chat settings snapshot", () => {
    const storage = createMemoryStorage()

    writePlaygroundSettings(
      {
        model: "gpt-4o",
        systemPrompt: "be concise",
        stream: false,
        temperature: 0.3,
        maxTokens: 2048,
        topP: 0.9,
      },
      storage,
    )

    expect(readPlaygroundSettings(storage)).toEqual({
      model: "gpt-4o",
      systemPrompt: "be concise",
      stream: false,
      temperature: 0.3,
      maxTokens: 2048,
      topP: 0.9,
    })
  })

  it("sanitizes invalid chat settings values", () => {
    const storage = createMemoryStorage()
    storage.setItem(
      "wheel.playground.settings.v1",
      JSON.stringify({
        model: 123,
        systemPrompt: 456,
        stream: "x",
        temperature: 9,
        maxTokens: -1,
        topP: -3,
      }),
    )

    expect(readPlaygroundSettings(storage)).toEqual({
      model: "",
      systemPrompt: "",
      stream: true,
      temperature: 2,
      maxTokens: 1,
      topP: 0,
    })
  })

  it("writes and reads mcp snapshot", () => {
    const storage = createMemoryStorage()

    writePlaygroundMcpSnapshot(
      {
        enabled: true,
        mode: "manual",
        selectedKeys: ["2:b", "1:a"],
        hasUserTouchedSelection: true,
      },
      storage,
    )

    expect(readPlaygroundMcpSnapshot(storage)).toEqual({
      enabled: true,
      mode: "manual",
      selectedKeys: ["1:a", "2:b"],
      hasUserTouchedSelection: true,
    })
  })

  it("falls back to safe mcp defaults", () => {
    const storage = createMemoryStorage()
    storage.setItem(
      "wheel.playground.mcp.v1",
      JSON.stringify({
        enabled: "yes",
        mode: "something",
        selectedKeys: ["1:a", 2],
      }),
    )

    expect(readPlaygroundMcpSnapshot(storage)).toEqual({
      enabled: false,
      mode: "auto",
      selectedKeys: ["1:a"],
      hasUserTouchedSelection: true,
    })
  })
})
