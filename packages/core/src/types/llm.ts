// Internal LLM request format (normalized from OpenAI/Anthropic/etc.)
export interface InternalLLMRequest {
  model: string
  messages: InternalMessage[]
  stream: boolean
  temperature?: number
  topP?: number
  maxTokens?: number
  stop?: string[]
  tools?: InternalTool[]
}

export interface InternalMessage {
  role: "system" | "user" | "assistant" | "tool"
  content: string | InternalContentPart[]
  name?: string
  toolCallId?: string
  toolCalls?: InternalToolCall[]
}

export interface InternalContentPart {
  type: "text" | "image_url"
  text?: string
  imageUrl?: { url: string; detail?: string }
}

export interface InternalTool {
  type: "function"
  function: {
    name: string
    description?: string
    parameters?: Record<string, unknown>
  }
}

export interface InternalToolCall {
  id: string
  type: "function"
  function: { name: string; arguments: string }
}

// Internal LLM response format
export interface InternalLLMResponse {
  id: string
  model: string
  choices: InternalChoice[]
  usage: InternalUsage
}

export interface InternalChoice {
  index: number
  message: InternalMessage
  finishReason: string | null
}

export interface InternalUsage {
  promptTokens: number
  completionTokens: number
  totalTokens: number
}

// SSE stream chunk (internal format)
export interface InternalStreamChunk {
  id: string
  model: string
  choices: {
    index: number
    delta: Partial<InternalMessage>
    finishReason: string | null
  }[]
  usage?: InternalUsage
}

// Request type detection
export type InboundRequestType = "openai-chat" | "anthropic-messages" | "openai-embeddings"
