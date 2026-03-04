const MODEL_PAYLOAD_CHAR_LIMIT = 4_000
const DISPLAY_PAYLOAD_CHAR_LIMIT = 2_000

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null
  return value as Record<string, unknown>
}

function clampText(text: string, maxChars: number): string {
  if (text.length <= maxChars) return text
  const suffix = "\n\n... (truncated)"
  return `${text.slice(0, maxChars - suffix.length)}${suffix}`
}

function extractTextSegments(payload: unknown): string[] {
  const record = asRecord(payload)
  if (!record) return []
  const content = record.content
  if (!Array.isArray(content)) return []

  const texts: string[] = []
  for (const item of content) {
    const part = asRecord(item)
    if (!part || part.type !== "text") continue
    if (typeof part.text === "string" && part.text.trim()) {
      texts.push(part.text)
    }
  }
  return texts
}

function prettyMaybeJSON(text: string): string {
  const trimmed = text.trim()
  if (!trimmed) return ""
  if (!(trimmed.startsWith("{") || trimmed.startsWith("["))) return text
  try {
    const parsed = JSON.parse(trimmed) as unknown
    return JSON.stringify(parsed, null, 2)
  } catch {
    return text
  }
}

function fallbackJSONString(value: unknown, spaced: boolean): string {
  try {
    return JSON.stringify(value, null, spaced ? 2 : undefined)
  } catch {
    return String(value)
  }
}

export function normalizeToolPayloadForModel(payload: unknown): string {
  const record = asRecord(payload)
  const textSegments = extractTextSegments(payload)
  const textBody = textSegments.join("\n\n").trim()

  if (record?.isError === true) {
    const errorText =
      typeof record.error === "string" && record.error.trim()
        ? record.error
        : textBody || "Tool execution failed"
    return clampText(`[tool_error]\n${errorText}`, MODEL_PAYLOAD_CHAR_LIMIT)
  }

  if (textBody) {
    return clampText(textBody, MODEL_PAYLOAD_CHAR_LIMIT)
  }

  return clampText(fallbackJSONString(payload, false), MODEL_PAYLOAD_CHAR_LIMIT)
}

export function formatToolPayloadForDisplay(payload: unknown): string {
  const record = asRecord(payload)
  const textSegments = extractTextSegments(payload)

  if (record?.isError === true) {
    const errorText =
      typeof record.error === "string" && record.error.trim()
        ? record.error
        : textSegments.join("\n\n").trim() || "Tool execution failed"
    return clampText(`Error: ${errorText}`, DISPLAY_PAYLOAD_CHAR_LIMIT)
  }

  if (textSegments.length > 0) {
    const pretty = textSegments.map((segment) => prettyMaybeJSON(segment)).join("\n\n")
    return clampText(pretty, DISPLAY_PAYLOAD_CHAR_LIMIT)
  }

  return clampText(fallbackJSONString(payload, true), DISPLAY_PAYLOAD_CHAR_LIMIT)
}
