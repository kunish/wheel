import { useState } from "react"
import { cn } from "@/lib/utils"

/**
 * Maps channel outbound type (number) to the models.dev provider key
 * used for logo URLs: https://models.dev/logos/{provider}.svg
 */
const PROVIDER_LOGO_MAP: Record<number, string> = {
  0: "openai", // OpenAI Chat
  1: "openai", // OpenAI
  2: "anthropic", // Anthropic
  3: "google", // Gemini
  4: "openai", // OpenAI Responses
  5: "openai", // OpenAI Embedding
  10: "azure", // Azure OpenAI
  11: "amazon", // AWS Bedrock
  12: "google", // Google Vertex AI
  13: "cohere", // Cohere
  20: "groq", // Groq
  21: "mistralai", // Mistral
  22: "deepseek", // DeepSeek
  23: "xai", // xAI (Grok)
  24: "cerebras", // Cerebras
  25: "openrouter", // OpenRouter
  26: "perplexity", // Perplexity
  27: "together", // Together AI
  28: "ollama", // Ollama
  29: "vllm", // vLLM
  30: "huggingface", // Hugging Face
  31: "novita", // Novita AI
  32: "siliconflow", // SiliconFlow
  33: "openai", // Codex
  34: "github", // GitHub Copilot
  35: "openai", // Codex CLI
  36: "google", // Antigravity
  37: "openai", // Cursor (no models.dev logo; use generic)
}

function getProviderLogoUrl(channelType: number): string | null {
  const provider = PROVIDER_LOGO_MAP[channelType]
  if (!provider) return null
  return `https://models.dev/logos/${provider}.svg`
}

interface ProviderIconProps {
  channelType: number
  size?: number
  className?: string
}

export function ProviderIcon({ channelType, size = 16, className }: ProviderIconProps) {
  const [error, setError] = useState(false)
  const logoUrl = getProviderLogoUrl(channelType)

  if (!logoUrl || error) return null

  return (
    <img
      src={logoUrl}
      alt=""
      width={size}
      height={size}
      className={cn("shrink-0 dark:invert", className)}
      onError={() => setError(true)}
    />
  )
}
