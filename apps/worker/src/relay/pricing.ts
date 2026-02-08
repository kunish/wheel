import type { Database } from "../runtime/types"
import { getLLMPriceByName } from "../db/dal/models"

// Fallback prices for common models ($/M tokens)
const FALLBACK_PRICES: Record<string, { input: number; output: number }> = {
  // OpenAI
  "gpt-4o": { input: 2.5, output: 10 },
  "gpt-4o-mini": { input: 0.15, output: 0.6 },
  "gpt-4-turbo": { input: 10, output: 30 },
  "gpt-4": { input: 30, output: 60 },
  "gpt-3.5-turbo": { input: 0.5, output: 1.5 },
  // Anthropic Claude 4.x / 3.x
  "claude-opus-4-6": { input: 15, output: 75 },
  "claude-opus-4-5": { input: 15, output: 75 },
  "claude-opus-4-5-20251101": { input: 15, output: 75 },
  "claude-opus-4-1": { input: 15, output: 75 },
  "claude-opus-4-0": { input: 15, output: 75 },
  "claude-opus-4-20250514": { input: 15, output: 75 },
  "claude-sonnet-4-5": { input: 3, output: 15 },
  "claude-sonnet-4-5-20250929": { input: 3, output: 15 },
  "claude-sonnet-4-20250514": { input: 3, output: 15 },
  "claude-haiku-4-5": { input: 0.8, output: 4 },
  "claude-sonnet-4-5-20250514": { input: 3, output: 15 },
  "claude-3-5-sonnet-20241022": { input: 3, output: 15 },
  "claude-3-5-sonnet-20240620": { input: 3, output: 15 },
  "claude-3-5-haiku-20241022": { input: 0.8, output: 4 },
  "claude-3-5-haiku-latest": { input: 0.8, output: 4 },
  "claude-3-opus-20240229": { input: 15, output: 75 },
  // Google
  "gemini-2.0-flash": { input: 0.1, output: 0.4 },
  "gemini-1.5-pro": { input: 1.25, output: 5 },
  "gemini-1.5-flash": { input: 0.075, output: 0.3 },
  // DeepSeek
  "deepseek-chat": { input: 0.14, output: 0.28 },
  "deepseek-reasoner": { input: 0.55, output: 2.19 },
}

export async function calculateCost(
  modelName: string,
  inputTokens: number,
  outputTokens: number,
  db: Database,
  cacheTokens?: { cacheReadTokens: number; cacheCreationTokens: number },
): Promise<number> {
  // Look up in database first (exact match)
  const price = await getLLMPriceByName(db, modelName)
  if (price) {
    return computeCost(
      inputTokens,
      outputTokens,
      price.inputPrice,
      price.outputPrice,
      price.cacheReadPrice,
      price.cacheWritePrice,
      cacheTokens,
    )
  }

  // Fallback to built-in prices (exact match)
  const fallback = FALLBACK_PRICES[modelName]
  if (fallback) {
    return computeCost(
      inputTokens,
      outputTokens,
      fallback.input,
      fallback.output,
      0,
      0,
      cacheTokens,
    )
  }

  // Prefix match: strip common suffixes like "-thinking", "-latest", etc.
  // e.g. "claude-opus-4-6-thinking" → try "claude-opus-4-6"
  const base = modelName.replace(/-(thinking|latest|online)$/, "")
  if (base !== modelName) {
    return calculateCost(base, inputTokens, outputTokens, db, cacheTokens)
  }

  // Unknown model — return 0
  return 0
}

function computeCost(
  inputTokens: number,
  outputTokens: number,
  inputPrice: number,
  outputPrice: number,
  cacheReadPrice: number,
  cacheWritePrice: number,
  cacheTokens?: { cacheReadTokens: number; cacheCreationTokens: number },
): number {
  let inputCost: number

  if (cacheTokens && (cacheTokens.cacheReadTokens > 0 || cacheTokens.cacheCreationTokens > 0)) {
    // Anthropic: cache_creation_input_tokens + cache_read_input_tokens
    // OpenAI: cached_tokens (only cacheReadTokens, cacheCreationTokens=0)
    const nonCachedInput = Math.max(
      0,
      inputTokens - cacheTokens.cacheReadTokens - cacheTokens.cacheCreationTokens,
    )
    inputCost =
      (nonCachedInput * inputPrice +
        cacheTokens.cacheReadTokens * cacheReadPrice +
        cacheTokens.cacheCreationTokens * cacheWritePrice) /
      1_000_000
  } else {
    inputCost = (inputTokens * inputPrice) / 1_000_000
  }

  const outputCost = (outputTokens * outputPrice) / 1_000_000
  return inputCost + outputCost
}
