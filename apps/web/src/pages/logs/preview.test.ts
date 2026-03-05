import { describe, expect, it } from "vitest"
import { deriveLastMessagePreview } from "./preview"

describe("deriveLastMessagePreview", () => {
  it("extracts last message text", () => {
    const payload = JSON.stringify({
      messages: [
        { role: "user", content: "hi" },
        { role: "user", content: "show me latest logs" },
      ],
    })
    expect(deriveLastMessagePreview(payload)).toBe("show me latest logs")
  })

  it("keeps image marker when message has image content", () => {
    const payload = JSON.stringify({
      messages: [
        {
          role: "user",
          content: [
            { type: "text", text: "analyze" },
            { type: "image_url", image_url: { url: "https://example.com/a.png" } },
          ],
        },
      ],
    })
    expect(deriveLastMessagePreview(payload)).toBe("analyze [image]")
  })

  it("falls back to tool calls label", () => {
    const payload = JSON.stringify({
      messages: [{ role: "assistant", content: "", tool_calls: [{ id: "a" }] }],
    })
    expect(deriveLastMessagePreview(payload)).toBe("[1 tool call]")
  })
})
