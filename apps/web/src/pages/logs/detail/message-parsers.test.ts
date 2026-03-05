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
})
