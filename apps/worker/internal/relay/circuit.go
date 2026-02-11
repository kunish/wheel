package relay

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

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

var (
	breakers   = make(map[string]*circuitEntry)
	breakersMu sync.RWMutex
)

func circuitKey(channelID, keyID int, modelName string) string {
	return fmt.Sprintf("%d:%d:%s", channelID, keyID, modelName)
}

func getOrCreate(key string) *circuitEntry {
	breakersMu.Lock()
	defer breakersMu.Unlock()
	e, ok := breakers[key]
	if !ok {
		e = &circuitEntry{state: CircuitClosed}
		breakers[key] = e
	}
	return e
}

func getThreshold(ctx context.Context, db *bun.DB) int {
	v, err := dal.GetSetting(ctx, db, "circuit_breaker_threshold")
	if err != nil || v == nil {
		return 5
	}
	n, err := strconv.Atoi(*v)
	if err != nil || n <= 0 {
		return 5
	}
	return n
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

// GetCooldownConfig reads circuit breaker cooldown settings from DB.
func GetCooldownConfig(ctx context.Context, db *bun.DB) (baseSec, maxSec int) {
	baseSec, maxSec = 60, 600

	baseVal, err := dal.GetSetting(ctx, db, "circuit_breaker_cooldown")
	if err == nil && baseVal != nil {
		if n, err := strconv.Atoi(*baseVal); err == nil && n > 0 {
			baseSec = n
		}
	}

	maxVal, err := dal.GetSetting(ctx, db, "circuit_breaker_max_cooldown")
	if err == nil && maxVal != nil {
		if n, err := strconv.Atoi(*maxVal); err == nil && n > 0 {
			maxSec = n
		}
	}
	return
}

// IsTripped checks if a channel/key/model combo is circuit-broken.
// Returns tripped=true to skip this channel, and remainingMs for cooldown info.
func IsTripped(channelID, keyID int, modelName string, baseSec, maxSec int) (tripped bool, remainingMs int64) {
	key := circuitKey(channelID, keyID, modelName)

	breakersMu.RLock()
	entry, ok := breakers[key]
	breakersMu.RUnlock()
	if !ok {
		return false, 0
	}

	breakersMu.Lock()
	defer breakersMu.Unlock()

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
func RecordSuccess(channelID, keyID int, modelName string) {
	key := circuitKey(channelID, keyID, modelName)

	breakersMu.Lock()
	defer breakersMu.Unlock()

	entry, ok := breakers[key]
	if !ok {
		return
	}
	entry.state = CircuitClosed
	entry.consecutiveFailures = 0
	entry.tripCount = 0
}

// RecordFailure records a failed request, potentially tripping the circuit breaker.
func RecordFailure(channelID, keyID int, modelName string, ctx context.Context, db *bun.DB) {
	key := circuitKey(channelID, keyID, modelName)
	entry := getOrCreate(key)

	breakersMu.Lock()
	defer breakersMu.Unlock()

	entry.lastFailureTime = time.Now().UnixMilli()

	switch entry.state {
	case CircuitClosed:
		entry.consecutiveFailures++
		threshold := getThreshold(ctx, db)
		if entry.consecutiveFailures >= threshold {
			entry.state = CircuitOpen
			entry.tripCount++
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
