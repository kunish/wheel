package relay

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"sync"
	"time"
)

// SemanticCacheConfig defines the configuration for the semantic cache plugin.
type SemanticCacheConfig struct {
	TTL           time.Duration // cache entry TTL
	MaxEntries    int           // max number of cached entries
	Enabled       bool
	ExcludeModels []string // models to exclude from caching
}

type cacheEntry struct {
	Response     map[string]any
	ExpiresAt    time.Time
	InputTokens  int
	OutputTokens int
}

// SemanticCachePlugin caches non-streaming responses keyed by
// a SHA-256 hash of model + messages (exact match).
type SemanticCachePlugin struct {
	config  SemanticCacheConfig
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	exclude map[string]bool
}

// NewSemanticCachePlugin creates a new semantic cache plugin.
func NewSemanticCachePlugin(config SemanticCacheConfig) *SemanticCachePlugin {
	exclude := make(map[string]bool, len(config.ExcludeModels))
	for _, m := range config.ExcludeModels {
		exclude[m] = true
	}
	if config.MaxEntries <= 0 {
		config.MaxEntries = 10000
	}
	if config.TTL <= 0 {
		config.TTL = 5 * time.Minute
	}
	return &SemanticCachePlugin{
		config:  config,
		entries: make(map[string]*cacheEntry),
		exclude: exclude,
	}
}

func (p *SemanticCachePlugin) Name() string { return "semantic_cache" }

func (p *SemanticCachePlugin) PreHook(ctx *RelayContext) *ShortCircuit {
	if !p.config.Enabled || ctx.IsStream {
		return nil
	}
	if p.exclude[ctx.RequestModel] {
		return nil
	}

	key := cacheKey(ctx.RequestModel, ctx.Body["messages"])
	ctx.Set("cache_key", key)

	p.mu.RLock()
	entry, ok := p.entries[key]
	p.mu.RUnlock()

	if ok && time.Now().Before(entry.ExpiresAt) {
		// Cache hit
		ctx.Set("cache_hit", true)
		return &ShortCircuit{
			StatusCode: 200,
			Body:       entry.Response,
		}
	}

	return nil
}

func (p *SemanticCachePlugin) PostHook(ctx *RelayContext, resp *RelayPluginResponse) {
	if !p.config.Enabled || !resp.Success || ctx.IsStream {
		return
	}
	if resp.Body == nil {
		return
	}

	keyVal, ok := ctx.Get("cache_key")
	if !ok {
		return
	}
	key, _ := keyVal.(string)
	if key == "" {
		return
	}

	// Don't cache if it was already a cache hit
	if hit, _ := ctx.Get("cache_hit"); hit == true {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Evict if at capacity
	if len(p.entries) >= p.config.MaxEntries {
		p.evictOldest()
	}

	p.entries[key] = &cacheEntry{
		Response:     resp.Body,
		ExpiresAt:    time.Now().Add(p.config.TTL),
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
	}
}

// evictOldest removes the entry with the earliest expiration. Must be called with mu held.
func (p *SemanticCachePlugin) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	first := true
	for k, e := range p.entries {
		if first || e.ExpiresAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = e.ExpiresAt
			first = false
		}
	}
	if oldestKey != "" {
		delete(p.entries, oldestKey)
	}
}

// StartCleanup runs periodic cleanup of expired cache entries.
func (p *SemanticCachePlugin) StartCleanup(stop <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				p.cleanup()
			}
		}
	}()
}

func (p *SemanticCachePlugin) cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	removed := 0
	for k, e := range p.entries {
		if now.After(e.ExpiresAt) {
			delete(p.entries, k)
			removed++
		}
	}
	if removed > 0 {
		log.Printf("[semantic_cache] cleaned up %d expired entries", removed)
	}
}

// cacheKey generates a deterministic cache key from model and messages.
func cacheKey(model string, messages any) string {
	h := sha256.New()
	h.Write([]byte(model))
	h.Write([]byte{0})
	data, _ := json.Marshal(messages)
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
