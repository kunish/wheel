package relay

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/kunish/wheel/apps/worker/internal/cache"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/uptrace/bun"
)

// CircuitState represents the state of a circuit breaker.
type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half_open"
)

type circuitEntry struct {
	state               CircuitState
	consecutiveFailures int
	lastFailureTime     int64 // unix ms
	tripCount           int
}

// CircuitObserver is called when circuit breaker state changes.
type CircuitObserver interface {
	SetCircuitBreakerState(ctx context.Context, channel string, delta int64)
}

// CircuitBreakerManager manages circuit breaker state for channel/key/model combos.
type CircuitBreakerManager struct {
	breakers map[string]*circuitEntry
	mu       sync.RWMutex
	observer CircuitObserver
	cache    *cache.MemoryKV
}

// NewCircuitBreakerManager creates a new CircuitBreakerManager with the given observer.
func NewCircuitBreakerManager(obs CircuitObserver, kv *cache.MemoryKV) *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[string]*circuitEntry),
		observer: obs,
		cache:    kv,
	}
}

func circuitKey(channelID, keyID int, modelName string) string {
	return fmt.Sprintf("%d:%d:%s", channelID, keyID, modelName)
}

func (m *CircuitBreakerManager) getOrCreate(key string) *circuitEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.breakers[key]
	if !ok {
		e = &circuitEntry{state: CircuitClosed}
		m.breakers[key] = e
	}
	return e
}

const cbConfigCacheKey = "cb_config"
const cbConfigTTL = 60 // seconds

type cbConfig struct {
	Threshold int `json:"threshold"`
	BaseSec   int `json:"baseSec"`
	MaxSec    int `json:"maxSec"`
}

func (m *CircuitBreakerManager) loadConfig(ctx context.Context, db *bun.DB) cbConfig {
	if m.cache != nil {
		if cached, ok := cache.Get[cbConfig](m.cache, cbConfigCacheKey); ok && cached != nil {
			return *cached
		}
	}

	cfg := cbConfig{Threshold: 5, BaseSec: 60, MaxSec: 600}

	if v, err := dal.GetSetting(ctx, db, "circuit_breaker_threshold"); err == nil && v != nil {
		if n, err := strconv.Atoi(*v); err == nil && n > 0 {
			cfg.Threshold = n
		}
	}
	if v, err := dal.GetSetting(ctx, db, "circuit_breaker_cooldown"); err == nil && v != nil {
		if n, err := strconv.Atoi(*v); err == nil && n > 0 {
			cfg.BaseSec = n
		}
	}
	if v, err := dal.GetSetting(ctx, db, "circuit_breaker_max_cooldown"); err == nil && v != nil {
		if n, err := strconv.Atoi(*v); err == nil && n > 0 {
			cfg.MaxSec = n
		}
	}

	if m.cache != nil {
		m.cache.Put(cbConfigCacheKey, cfg, cbConfigTTL)
	}
	return cfg
}

// GetCooldownConfig returns circuit breaker cooldown settings (cached for 60s).
func (m *CircuitBreakerManager) GetCooldownConfig(ctx context.Context, db *bun.DB) (baseSec, maxSec int) {
	cfg := m.loadConfig(ctx, db)
	return cfg.BaseSec, cfg.MaxSec
}

func getCooldownMs(tripCount, baseSec, maxSec int) int64 {
	cooldown := baseSec
	if tripCount > 1 {
		shift := tripCount - 1
		if shift > 20 {
			shift = 20
		}
		cooldown = baseSec * (1 << shift)
	}
	if cooldown > maxSec {
		cooldown = maxSec
	}
	return int64(cooldown) * 1000
}

// IsTripped checks if a channel/key/model combo is circuit-broken.
// Returns tripped=true to skip this channel, and remainingMs for cooldown info.
func (m *CircuitBreakerManager) IsTripped(channelID, keyID int, modelName string, baseSec, maxSec int) (tripped bool, remainingMs int64) {
	key := circuitKey(channelID, keyID, modelName)

	m.mu.RLock()
	entry, ok := m.breakers[key]
	m.mu.RUnlock()
	if !ok {
		return false, 0
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	switch entry.state {
	case CircuitClosed:
		return false, 0

	case CircuitOpen:
		cooldown := getCooldownMs(entry.tripCount, baseSec, maxSec)
		elapsed := time.Now().UnixMilli() - entry.lastFailureTime
		if elapsed >= cooldown {
			entry.state = CircuitHalfOpen
			return false, 0
		}
		return true, cooldown - elapsed

	case CircuitHalfOpen:
		// Already probing — block other requests
		return true, 0

	default:
		return false, 0
	}
}

// RecordSuccess resets the circuit breaker on a successful request.
func (m *CircuitBreakerManager) RecordSuccess(channelID, keyID int, modelName string) {
	key := circuitKey(channelID, keyID, modelName)

	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.breakers[key]
	if !ok {
		return
	}
	wasOpen := entry.state == CircuitOpen || entry.state == CircuitHalfOpen
	entry.state = CircuitClosed
	entry.consecutiveFailures = 0
	entry.tripCount = 0
	if wasOpen && m.observer != nil {
		go m.observer.SetCircuitBreakerState(context.Background(), key, -1)
	}
}

// RecordFailure records a failed request, potentially tripping the circuit breaker.
func (m *CircuitBreakerManager) RecordFailure(channelID, keyID int, modelName string, ctx context.Context, db *bun.DB) {
	key := circuitKey(channelID, keyID, modelName)
	entry := m.getOrCreate(key)

	m.mu.Lock()
	defer m.mu.Unlock()

	entry.lastFailureTime = time.Now().UnixMilli()

	switch entry.state {
	case CircuitClosed:
		entry.consecutiveFailures++
		threshold := m.loadConfig(ctx, db).Threshold
		if entry.consecutiveFailures >= threshold {
			entry.state = CircuitOpen
			entry.tripCount++
			if m.observer != nil {
				go m.observer.SetCircuitBreakerState(context.Background(), key, 1)
			}
		}

	case CircuitHalfOpen:
		// Probe failed — back to open with increased backoff
		entry.state = CircuitOpen
		entry.tripCount++
		entry.consecutiveFailures = 0

	case CircuitOpen:
		// Should not receive failures while open, but update time for safety
	}
}

// StartCleanup runs a background goroutine that removes stale closed breakers every 5 minutes.
// Breakers in closed state with no failure for 30+ minutes are considered stale.
func (m *CircuitBreakerManager) StartCleanup(ctx context.Context) {
	const interval = 5 * time.Minute
	const staleThreshold = 30 * time.Minute

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.mu.Lock()
				now := time.Now().UnixMilli()
				removed := 0
				for key, entry := range m.breakers {
					if entry.state == CircuitClosed && (now-entry.lastFailureTime) > staleThreshold.Milliseconds() {
						delete(m.breakers, key)
						removed++
					}
				}
				m.mu.Unlock()
				if removed > 0 {
					log.Printf("[cleanup] removed %d stale circuit breaker entries", removed)
				}
			}
		}
	}()
}
