import type { ChatRunnerDeps, ChatRunnerRunInput, ManualToolOutput } from "./chat-runner"
import type { ChatMessage } from "./request-builders"
import { describe, expect, it, vi } from "vitest"
import { createChatRunner } from "./chat-runner"

function makeBaseInput(partial: Partial<ChatRunnerRunInput> = {}): ChatRunnerRunInput {
  return {
    apiKey: "sk-test",
    model: "gpt-4o",
    messages: [{ role: "user", content: "hi" } satisfies ChatMessage],
    mcpTools: [
      {
        type: "function",
        function: { name: "weather_search", description: "Search weather", parameters: {} },
      },
    ],
    aliasMap: {
      weather_search: {
        alias: "weather_search",
        clientId: 1,
        clientName: "weather",
        toolName: "search",
      },
    },
    mode: "auto",
    maxRounds: 4,
    ...partial,
  }
}

describe("chat-runner", () => {
  it("runs auto mode loop until assistant returns final content", async () => {
    const deps: ChatRunnerDeps = {
      createChatCompletion: vi
        .fn()
        .mockResolvedValueOnce({
          choices: [
            {
              message: {
                role: "assistant",
                tool_calls: [
                  {
                    id: "call_1",
                    function: { name: "weather_search", arguments: '{"city":"Tokyo"}' },
                  },
                ],
              },
            },
          ],
        })
        .mockResolvedValueOnce({ choices: [{ message: { role: "assistant", content: "Sunny" } }] }),
      executeTool: vi.fn().mockResolvedValue({ ok: true, temp: 26 }),
    }

    const runner = createChatRunner(deps)
    const result = await runner.run(makeBaseInput())

    expect(result.status).toBe("completed")
    if (result.status === "completed") {
      expect(result.responseText).toBe("Sunny")
    }
    expect(deps.executeTool).toHaveBeenCalledWith({
      apiKey: "sk-test",
      clientId: 1,
      toolName: "search",
      argumentsObj: { city: "Tokyo" },
      signal: undefined,
    })
    expect(deps.createChatCompletion).toHaveBeenCalledTimes(2)
  })

  it("pauses in manual mode when tool calls are returned", async () => {
    const deps: ChatRunnerDeps = {
      createChatCompletion: vi.fn().mockResolvedValue({
        choices: [
          {
            message: {
              role: "assistant",
              tool_calls: [
                {
                  id: "call_1",
                  function: { name: "weather_search", arguments: '{"city":"Tokyo"}' },
                },
              ],
            },
          },
        ],
      }),
      executeTool: vi.fn(),
    }

    const runner = createChatRunner(deps)
    const result = await runner.run(makeBaseInput({ mode: "manual" }))

    expect(result.status).toBe("paused")
    if (result.status === "paused") {
      expect(result.pendingCalls).toHaveLength(1)
      expect(result.pendingCalls[0].toolCallId).toBe("call_1")
      expect(result.pendingCalls[0].argumentsObj).toEqual({ city: "Tokyo" })
    }
    expect(deps.executeTool).not.toHaveBeenCalled()
  })

  it("continues manual session after tool outputs", async () => {
    const deps: ChatRunnerDeps = {
      createChatCompletion: vi
        .fn()
        .mockResolvedValueOnce({
          choices: [
            {
              message: {
                role: "assistant",
                tool_calls: [
                  {
                    id: "call_1",
                    function: { name: "weather_search", arguments: '{"city":"Tokyo"}' },
                  },
                ],
              },
            },
          ],
        })
        .mockResolvedValueOnce({ choices: [{ message: { role: "assistant", content: "Rainy" } }] }),
      executeTool: vi.fn(),
    }

    const runner = createChatRunner(deps)
    const first = await runner.run(makeBaseInput({ mode: "manual" }))
    expect(first.status).toBe("paused")
    if (first.status !== "paused") throw new Error("expected paused")

    const outputs: ManualToolOutput[] = [
      {
        toolCallId: "call_1",
        payload: { ok: true, temp: 12 },
      },
    ]
    const second = await runner.continueManual(first.session, outputs)
    expect(second.status).toBe("completed")
    if (second.status === "completed") {
      expect(second.responseText).toBe("Rainy")
    }
  })

  it("throws when tool alias is missing in auto mode", async () => {
    const deps: ChatRunnerDeps = {
      createChatCompletion: vi.fn().mockResolvedValue({
        choices: [
          {
            message: {
              role: "assistant",
              tool_calls: [{ id: "call_1", function: { name: "unknown_tool", arguments: "{}" } }],
            },
          },
        ],
      }),
      executeTool: vi.fn(),
    }

    const runner = createChatRunner(deps)
    await expect(
      runner.run(makeBaseInput({ aliasMap: {}, mcpTools: [], mode: "auto" })),
    ).rejects.toThrow(/missing tool alias/i)
  })
})
