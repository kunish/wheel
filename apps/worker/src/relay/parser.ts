import type { InboundRequestType } from "@wheel/core"

/**
 * Parse the incoming request to determine the request type and extract the model name.
 */
export function detectRequestType(path: string): InboundRequestType | null {
  if (path.includes("/chat/completions")) return "openai-chat"
  if (path.includes("/v1/messages")) return "anthropic-messages"
  if (path.includes("/embeddings")) return "openai-embeddings"
  return null
}

export async function extractModel(
  request: Request,
  _requestType: InboundRequestType,
): Promise<{ model: string; body: Record<string, unknown>; stream: boolean }> {
  const body = (await request.json()) as Record<string, unknown>
  const model = (body.model as string) ?? ""
  const stream = (body.stream as boolean) ?? false
  return { model, body, stream }
}
