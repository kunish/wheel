import { describe, expect, it } from "vitest"
import { parseResponseContent } from "./message-parsers"

describe("parseResponseContent", () => {
  it("parses anthropic style response metadata and usage", () => {
    const payload = JSON.stringify({
      id: "msg_123",
      model: "claude-3-7-sonnet-20250219",
      stop_reason: "end_turn",
      usage: {
        input_tokens: 120,
        output_tokens: 45,
        cache_read_input_tokens: 32,
      },
      content: [
        { type: "thinking", thinking: "analyzing" },
        { type: "text", text: "hello" },
      ],
    })

    const parsed = parseResponseContent(payload)
    expect(parsed).not.toBeNull()
    expect(parsed?.id).toBe("msg_123")
    expect(parsed?.model).toBe("claude-3-7-sonnet-20250219")
    expect(parsed?.usage?.prompt_tokens).toBe(120)
    expect(parsed?.usage?.completion_tokens).toBe(45)
    expect(parsed?.choices[0]?.assistantContent).toBe("hello")
    expect(parsed?.choices[0]?.thinkingContent).toBe("analyzing")
  })

  it("extracts reasoning_content from OpenAI-format response (Copilot Claude)", () => {
    const payload = JSON.stringify({
      id: "chatcmpl-abc",
      model: "claude-opus-4.6",
      choices: [
        {
          message: {
            role: "assistant",
            content: "The answer is 4.",
            reasoning_content: "Let me think step by step...",
          },
          finish_reason: "stop",
          index: 0,
        },
      ],
      usage: { prompt_tokens: 10, completion_tokens: 20, total_tokens: 30 },
    })

    const parsed = parseResponseContent(payload)
    expect(parsed).not.toBeNull()
    expect(parsed?.choices[0]?.assistantContent).toBe("The answer is 4.")
    expect(parsed?.choices[0]?.thinkingContent).toBe("Let me think step by step...")
  })

  it("falls back to tag-based thinking when reasoning_content is absent", () => {
    const payload = `<|thinking|>reasoning here<|/thinking|>${JSON.stringify({
      id: "chatcmpl-xyz",
      model: "gpt-4o",
      choices: [
        {
          message: { role: "assistant", content: "hello" },
          finish_reason: "stop",
          index: 0,
        },
      ],
    })}`

    const parsed = parseResponseContent(payload)
    expect(parsed).not.toBeNull()
    expect(parsed?.choices[0]?.assistantContent).toBe("hello")
    expect(parsed?.choices[0]?.thinkingContent).toBe("reasoning here")
  })

  it("prefers reasoning_content over tag-based thinking extraction", () => {
    const payload = `<|thinking|>old thinking<|/thinking|>${JSON.stringify({
      id: "chatcmpl-pref",
      model: "claude-opus-4.6",
      choices: [
        {
          message: {
            role: "assistant",
            content: "result",
            reasoning_content: "actual reasoning",
          },
          finish_reason: "stop",
          index: 0,
        },
      ],
    })}`

    const parsed = parseResponseContent(payload)
    expect(parsed).not.toBeNull()
    expect(parsed?.choices[0]?.thinkingContent).toBe("actual reasoning")
  })
})
