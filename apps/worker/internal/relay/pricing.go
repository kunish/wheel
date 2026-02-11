package relay

import (
	"context"
	"math"
	"regexp"
	"strings"

	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/uptrace/bun"
)

// CacheTokens holds cache token counts for cost calculation.
type CacheTokens struct {
	CacheReadTokens     int
	CacheCreationTokens int
}

// fallbackPrice holds per-million-token pricing.
type fallbackPrice struct {
	Input  float64
	Output float64
}

var fallbackPrices = map[string]fallbackPrice{
	// OpenAI
	"gpt-4o":         {2.5, 10},
	"gpt-4o-mini":    {0.15, 0.6},
	"gpt-4-turbo":    {10, 30},
	"gpt-4":          {30, 60},
	"gpt-3.5-turbo":  {0.5, 1.5},
	// Anthropic Claude 4.x / 3.x
	"claude-opus-4-6":             {15, 75},
	"claude-opus-4-5":             {15, 75},
	"claude-opus-4-5-20251101":    {15, 75},
	"claude-opus-4-1":             {15, 75},
	"claude-opus-4-0":             {15, 75},
	"claude-opus-4-20250514":      {15, 75},
	"claude-sonnet-4-5":           {3, 15},
	"claude-sonnet-4-5-20250929":  {3, 15},
	"claude-sonnet-4-20250514":    {3, 15},
	"claude-haiku-4-5":            {0.8, 4},
	"claude-sonnet-4-5-20250514":  {3, 15},
	"claude-3-5-sonnet-20241022":  {3, 15},
	"claude-3-5-sonnet-20240620":  {3, 15},
	"claude-3-5-haiku-20241022":   {0.8, 4},
	"claude-3-5-haiku-latest":     {0.8, 4},
	"claude-3-opus-20240229":      {15, 75},
	// Google
	"gemini-2.0-flash": {0.1, 0.4},
	"gemini-1.5-pro":   {1.25, 5},
	"gemini-1.5-flash": {0.075, 0.3},
	// DeepSeek
	"deepseek-chat":     {0.14, 0.28},
	"deepseek-reasoner": {0.55, 2.19},
}

var suffixPattern = regexp.MustCompile(`-(thinking|latest|online)$`)

// CalculateCost computes the cost of a request in dollars.
func CalculateCost(modelName string, inputTokens, outputTokens int, ctx context.Context, db *bun.DB, cacheTokens *CacheTokens) float64 {
	// DB lookup first
	price, err := dal.GetLLMPriceByName(ctx, db, modelName)
	if err == nil && price != nil {
		return computeCost(inputTokens, outputTokens, price.InputPrice, price.OutputPrice,
			price.CacheReadPrice, price.CacheWritePrice, cacheTokens)
	}

	// Fallback to built-in prices
	if fp, ok := fallbackPrices[modelName]; ok {
		return computeCost(inputTokens, outputTokens, fp.Input, fp.Output, 0, 0, cacheTokens)
	}

	// Prefix match: strip common suffixes
	base := suffixPattern.ReplaceAllString(modelName, "")
	if base != modelName {
		return CalculateCost(base, inputTokens, outputTokens, ctx, db, cacheTokens)
	}

	// Also try stripping everything after the last colon (for provider prefixes)
	if idx := strings.LastIndex(modelName, ":"); idx >= 0 {
		return CalculateCost(modelName[idx+1:], inputTokens, outputTokens, ctx, db, cacheTokens)
	}

	return 0
}

func computeCost(inputTokens, outputTokens int, inputPrice, outputPrice, cacheReadPrice, cacheWritePrice float64, cacheTokens *CacheTokens) float64 {
	var inputCost float64

	if cacheTokens != nil && (cacheTokens.CacheReadTokens > 0 || cacheTokens.CacheCreationTokens > 0) {
		nonCachedInput := math.Max(0, float64(inputTokens-cacheTokens.CacheReadTokens-cacheTokens.CacheCreationTokens))
		inputCost = (nonCachedInput*inputPrice +
			float64(cacheTokens.CacheReadTokens)*cacheReadPrice +
			float64(cacheTokens.CacheCreationTokens)*cacheWritePrice) / 1_000_000
	} else {
		inputCost = float64(inputTokens) * inputPrice / 1_000_000
	}

	outputCost := float64(outputTokens) * outputPrice / 1_000_000
	return inputCost + outputCost
}
