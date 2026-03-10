package relay

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ── Sliding Window Rate Limiter ─────────────────────────────────

// RateLimitConfig defines per-key rate limits.
type RateLimitConfig struct {
	RPM int // requests per minute, 0 = unlimited
	TPM int // tokens per minute, 0 = unlimited
}

type windowEntry struct {
	timestamp int64
	tokens    int
}

// rateLimiter implements a sliding-window rate limiter for RPM and TPM.
type rateLimiter struct {
	mu      sync.Mutex
	windows map[string][]windowEntry // key -> sorted entries
}

// newRateLimiter creates a new rateLimiter.
func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		windows: make(map[string][]windowEntry),
	}
}

const rateLimitWindow = 60 * 1000 // 60 seconds in ms

// Check tests whether a request is allowed under the given limits.
// Returns (allowed, retryAfterMs). Does NOT record the request.
func (r *rateLimiter) Check(key string, config RateLimitConfig) (bool, int64) {
	if config.RPM == 0 && config.TPM == 0 {
		return true, 0
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UnixMilli()
	cutoff := now - int64(rateLimitWindow)

	entries := r.prune(key, cutoff)

	if config.RPM > 0 && len(entries) >= config.RPM {
		oldest := entries[0].timestamp
		retryAfter := oldest + int64(rateLimitWindow) - now
		if retryAfter < 100 {
			retryAfter = 100
		}
		return false, retryAfter
	}

	if config.TPM > 0 {
		totalTokens := 0
		for _, e := range entries {
			totalTokens += e.tokens
		}
		if totalTokens >= config.TPM {
			oldest := entries[0].timestamp
			retryAfter := oldest + int64(rateLimitWindow) - now
			if retryAfter < 100 {
				retryAfter = 100
			}
			return false, retryAfter
		}
	}

	return true, 0
}

// Record records a completed request with its token count.
func (r *rateLimiter) Record(key string, tokens int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UnixMilli()
	r.windows[key] = append(r.windows[key], windowEntry{
		timestamp: now,
		tokens:    tokens,
	})
}

// UpdateLastTokens updates the token count of the most recent entry for a key.
// Used by PostHook to fill in token counts recorded as 0 in PreHook.
func (r *rateLimiter) UpdateLastTokens(key string, tokens int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entries := r.windows[key]
	if len(entries) > 0 {
		entries[len(entries)-1].tokens = tokens
	}
}

// prune removes expired entries and returns the remaining ones.
// Must be called with r.mu held.
func (r *rateLimiter) prune(key string, cutoff int64) []windowEntry {
	entries := r.windows[key]
	start := 0
	for start < len(entries) && entries[start].timestamp < cutoff {
		start++
	}
	if start > 0 {
		// Copy to a new slice to release the old underlying array for GC
		remaining := make([]windowEntry, len(entries)-start)
		copy(remaining, entries[start:])
		r.windows[key] = remaining
		entries = remaining
	}
	return entries
}

// StartCleanup runs periodic cleanup of stale keys.
func (r *rateLimiter) StartCleanup(stop <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				r.cleanup()
			}
		}
	}()
}

func (r *rateLimiter) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := time.Now().UnixMilli() - int64(rateLimitWindow)
	removed := 0
	for key, entries := range r.windows {
		start := 0
		for start < len(entries) && entries[start].timestamp < cutoff {
			start++
		}
		if start >= len(entries) {
			delete(r.windows, key)
			removed++
		} else if start > 0 {
			r.windows[key] = entries[start:]
		}
	}
	if removed > 0 {
		log.Printf("[ratelimit] cleaned up %d stale keys", removed)
	}
}

// ── Rate Limit Plugin ───────────────────────────────────────────

// RateLimitPlugin is a relay plugin that enforces RPM/TPM limits
// per API key before the request reaches the provider.
type RateLimitPlugin struct {
	limiter    *rateLimiter
	configFunc func(ctx *RelayContext) RateLimitConfig
}

// NewRateLimitPlugin creates a rate limit plugin.
// configFunc returns the RateLimitConfig for a given request context.
func NewRateLimitPlugin(configFunc func(ctx *RelayContext) RateLimitConfig) *RateLimitPlugin {
	return &RateLimitPlugin{
		limiter:    newRateLimiter(),
		configFunc: configFunc,
	}
}

func (p *RateLimitPlugin) Name() string { return "rate_limit" }

func (p *RateLimitPlugin) PreHook(ctx *RelayContext) *ShortCircuit {
	config := p.configFunc(ctx)
	key := fmt.Sprintf("apikey:%d:%s", ctx.ApiKeyID, ctx.RequestModel)

	allowed, retryAfterMs := p.limiter.Check(key, config)
	if !allowed {
		ctx.Set("rate_limit_retry_after_ms", retryAfterMs)
		return &ShortCircuit{
			StatusCode: 429,
			Body: OpenAIErrorBody(
				"rate_limit_error",
				fmt.Sprintf(
					"Rate limit exceeded (RPM: %d, TPM: %d). Retry after %dms",
					config.RPM, config.TPM, retryAfterMs,
				),
			),
		}
	}

	// Record the request (tokens will be updated in PostHook)
	p.limiter.Record(key, 0)
	return nil
}

func (p *RateLimitPlugin) PostHook(ctx *RelayContext, resp *RelayPluginResponse) {
	// Update token count on the entry already created by PreHook (avoids double RPM counting)
	if resp.InputTokens+resp.OutputTokens > 0 {
		key := fmt.Sprintf("apikey:%d:%s", ctx.ApiKeyID, ctx.RequestModel)
		p.limiter.UpdateLastTokens(key, resp.InputTokens+resp.OutputTokens)
	}
}

// Limiter returns the underlying rateLimiter for cleanup registration.
func (p *RateLimitPlugin) Limiter() *rateLimiter {
	return p.limiter
}
