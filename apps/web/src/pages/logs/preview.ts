import { detectTruncation, getMessageTextContent, parseMessages } from "./detail/message-parsers"

function truncatePreview(text: string, maxChars = 140): string {
  if (text.length <= maxChars) return text
  return `${text.slice(0, maxChars)}...`
}

export function deriveLastMessagePreview(requestContent?: string): string {
  if (!requestContent) return ""
  const messages = parseMessages(requestContent)
  if (!messages || messages.length === 0) return ""

  const last = messages[messages.length - 1]
  const { text, hasImages } = getMessageTextContent(last.content)
  const { cleanText } = detectTruncation(text)

  let preview = (cleanText || text).trim().replace(/\s+/g, " ")

  if (hasImages) {
    preview = preview ? `${preview} [image]` : "[image]"
  }

  if (!preview && last.tool_calls?.length) {
    const count = last.tool_calls.length
    return count === 1 ? "[1 tool call]" : `[${count} tool calls]`
  }

  return truncatePreview(preview)
}
