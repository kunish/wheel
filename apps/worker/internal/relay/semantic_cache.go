package relay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"math"
	"sync"
	"time"
)

// EmbeddingProvider generates embeddings for text.
type EmbeddingProvider interface {
	Embed(ctx context.Context, text string) ([]float64, error)
}

// NoopEmbeddingProvider always returns nil (used when similarity mode is disabled).
type NoopEmbeddingProvider struct{}

func (n *NoopEmbeddingProvider) Embed(ctx context.Context, text string) ([]float64, error) {
	return nil, nil
}

// SemanticCacheConfig defines the configuration for the semantic cache plugin.
type SemanticCacheConfig struct {
	TTL           time.Duration // cache entry TTL
	MaxEntries    int           // max number of cached entries
	Enabled       bool
	ExcludeModels []string // models to exclude from caching

	// Vector similarity fields
	SimilarityMode      bool    // enable vector similarity matching
	SimilarityThreshold float64 // cosine similarity threshold (0.0-1.0, default 0.95)
	EmbeddingModel      string  // model to use for embeddings (e.g. "text-embedding-3-small")
}

type cacheEntry struct {
	Response     map[string]any
	ExpiresAt    time.Time
	InputTokens  int
	OutputTokens int
	Embedding    []float64 // vector embedding of the query (nil when similarity mode is off)
	QueryHash    string    // exact SHA-256 hash for fast lookup
}

// SemanticCachePlugin caches non-streaming responses keyed by
// a SHA-256 hash of model + messages (exact match), with optional
// vector similarity matching via an embedding provider.
type SemanticCachePlugin struct {
	config   SemanticCacheConfig
	mu       sync.RWMutex
	entries  map[string]*cacheEntry
	exclude  map[string]bool
	embedder EmbeddingProvider
}

// NewSemanticCachePlugin creates a new semantic cache plugin.
// An optional EmbeddingProvider can be passed for similarity mode;
// when omitted a NoopEmbeddingProvider is used.
func NewSemanticCachePlugin(config SemanticCacheConfig, embedder ...EmbeddingProvider) *SemanticCachePlugin {
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
	if config.SimilarityThreshold <= 0 || config.SimilarityThreshold > 1.0 {
		config.SimilarityThreshold = 0.95
	}

	var ep EmbeddingProvider
	if len(embedder) > 0 && embedder[0] != nil {
		ep = embedder[0]
	} else {
		ep = &NoopEmbeddingProvider{}
	}

	return &SemanticCachePlugin{
		config:   config,
		entries:  make(map[string]*cacheEntry),
		exclude:  exclude,
		embedder: ep,
	}
}

func (p *SemanticCachePlugin) Name() string { return "semantic_cache" }

func (p *SemanticCachePlugin) PreHook(ctx *RelayContext) *ShortCircuit {
	if !p.config.Enabled {
		return nil
	}
	if p.exclude[ctx.RequestModel] {
		return nil
	}

	key := cacheKey(ctx.RequestModel, ctx.Body["messages"])
	ctx.Set("cache_key", key)

	// For streaming requests, we only generate the cache key so PostHook can
	// cache the accumulated response. We don't short-circuit streaming requests
	// because the cached response is a non-streaming JSON body.
	if ctx.IsStream {
		return nil
	}

	// 1. Try exact match first (fast path)
	p.mu.RLock()
	entry, ok := p.entries[key]
	p.mu.RUnlock()

	if ok && time.Now().Before(entry.ExpiresAt) {
		// Cache hit — exact match
		ctx.Set("cache_hit", true)
		return &ShortCircuit{
			StatusCode: 200,
			Body:       entry.Response,
		}
	}

	// 2. Try similarity match if enabled
	if p.config.SimilarityMode {
		queryText := buildQueryText(ctx.RequestModel, ctx.Body["messages"])
		embedding, err := p.embedder.Embed(context.Background(), queryText)
		if err != nil {
			log.Printf("[semantic_cache] embedding error: %v", err)
			return nil
		}
		if embedding != nil {
			ctx.Set("cache_embedding", embedding)

			if similar := p.findSimilarEntry(embedding, p.config.SimilarityThreshold); similar != nil {
				ctx.Set("cache_hit", true)
				return &ShortCircuit{
					StatusCode: 200,
					Body:       similar.Response,
				}
			}
		}
	}

	return nil
}

func (p *SemanticCachePlugin) PostHook(ctx *RelayContext, resp *RelayPluginResponse) {
	if !p.config.Enabled || !resp.Success {
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

	// Retrieve embedding if it was computed during PreHook
	var embedding []float64
	if embVal, ok := ctx.Get("cache_embedding"); ok {
		embedding, _ = embVal.([]float64)
	}

	// If similarity mode is on and we don't have an embedding yet, compute one
	if p.config.SimilarityMode && embedding == nil {
		queryText := buildQueryText(ctx.RequestModel, ctx.Body["messages"])
		emb, err := p.embedder.Embed(context.Background(), queryText)
		if err != nil {
			log.Printf("[semantic_cache] embedding error on store: %v", err)
		} else {
			embedding = emb
		}
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
		Embedding:    embedding,
		QueryHash:    key,
	}
}

// findSimilarEntry searches the cache for an entry whose embedding is similar
// enough to the given embedding (above the threshold). Must be called without
// the write lock held; acquires a read lock internally.
func (p *SemanticCachePlugin) findSimilarEntry(embedding []float64, threshold float64) *cacheEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()

	now := time.Now()
	var bestEntry *cacheEntry
	bestScore := threshold // only return entries above threshold

	for _, e := range p.entries {
		if now.After(e.ExpiresAt) {
			continue
		}
		if len(e.Embedding) == 0 {
			continue
		}
		score := cosineSimilarity(embedding, e.Embedding)
		if score > bestScore {
			bestScore = score
			bestEntry = e
		}
	}

	return bestEntry
}

// cosineSimilarity computes the cosine similarity between two vectors.
// Returns 0 if either vector is empty or they have different lengths.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
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

// buildQueryText creates a text representation of model + messages for embedding.
func buildQueryText(model string, messages any) string {
	data, _ := json.Marshal(messages)
	return model + "\n" + string(data)
}
