package relay

import (
	"log"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

func randFloat64() float64 {
	return rand.Float64()
}

// channelMetrics tracks real-time performance metrics for a channel.
type channelMetrics struct {
	mu              sync.Mutex
	totalRequests   int64
	totalErrors     int64
	totalLatencyMs  int64
	windowRequests  int64
	windowErrors    int64
	windowLatencyMs int64
	lastReset       time.Time

	errorRate float64
	avgLatMs  float64
	score     float64
	weight    float64
}

const (
	metricsWindowDuration = 5 * time.Second
	weightErrorRate       = 0.50
	weightLatency         = 0.20
	weightUtilization     = 0.05
	momentumFactor        = 0.10
	minWeight             = 0.05
	maxWeight             = 1.0
)

// AdaptiveBalancer extends BalancerState with metrics-driven weight adjustment.
type AdaptiveBalancer struct {
	base    *BalancerState
	metrics sync.Map // channelID -> *channelMetrics
	stopCh  chan struct{}
}

// NewAdaptiveBalancer creates an adaptive balancer wrapping the base balancer.
func NewAdaptiveBalancer(base *BalancerState) *AdaptiveBalancer {
	ab := &AdaptiveBalancer{
		base:   base,
		stopCh: make(chan struct{}),
	}
	return ab
}

// Start begins the periodic weight recalculation loop.
func (ab *AdaptiveBalancer) Start() {
	go func() {
		ticker := time.NewTicker(metricsWindowDuration)
		defer ticker.Stop()
		for {
			select {
			case <-ab.stopCh:
				return
			case <-ticker.C:
				ab.recalculateWeights()
			}
		}
	}()
}

// Stop halts the recalculation loop.
func (ab *AdaptiveBalancer) Stop() {
	close(ab.stopCh)
}

func (ab *AdaptiveBalancer) getMetrics(channelID int) *channelMetrics {
	val, ok := ab.metrics.Load(channelID)
	if ok {
		return val.(*channelMetrics)
	}
	m := &channelMetrics{
		lastReset: time.Now(),
		weight:    1.0,
	}
	actual, _ := ab.metrics.LoadOrStore(channelID, m)
	return actual.(*channelMetrics)
}

// RecordSuccess records a successful request with its latency.
func (ab *AdaptiveBalancer) RecordSuccess(channelID int, latencyMs int64) {
	m := ab.getMetrics(channelID)
	m.mu.Lock()
	m.totalRequests++
	m.totalLatencyMs += latencyMs
	m.windowRequests++
	m.windowLatencyMs += latencyMs
	m.mu.Unlock()
}

// RecordFailure records a failed request.
func (ab *AdaptiveBalancer) RecordFailure(channelID int, latencyMs int64) {
	m := ab.getMetrics(channelID)
	m.mu.Lock()
	m.totalRequests++
	m.totalErrors++
	m.totalLatencyMs += latencyMs
	m.windowRequests++
	m.windowErrors++
	m.windowLatencyMs += latencyMs
	m.mu.Unlock()
}

// GetWeight returns the adaptive weight for a channel (0.0-1.0).
func (ab *AdaptiveBalancer) GetWeight(channelID int) float64 {
	m := ab.getMetrics(channelID)
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.weight
}

func (ab *AdaptiveBalancer) recalculateWeights() {
	var maxLatency float64

	// First pass: collect max latency for normalization
	ab.metrics.Range(func(key, value any) bool {
		m := value.(*channelMetrics)
		m.mu.Lock()
		if m.windowRequests > 0 {
			avg := float64(m.windowLatencyMs) / float64(m.windowRequests)
			if avg > maxLatency {
				maxLatency = avg
			}
		}
		m.mu.Unlock()
		return true
	})

	if maxLatency == 0 {
		maxLatency = 1
	}

	// Second pass: compute scores and weights
	ab.metrics.Range(func(key, value any) bool {
		m := value.(*channelMetrics)
		m.mu.Lock()
		defer m.mu.Unlock()

		if m.windowRequests == 0 {
			m.windowErrors = 0
			m.windowLatencyMs = 0
			m.lastReset = time.Now()
			return true
		}

		errRate := float64(m.windowErrors) / float64(m.windowRequests)
		avgLat := float64(m.windowLatencyMs) / float64(m.windowRequests)
		normalizedLat := avgLat / maxLatency

		// Simple utilization proxy (capped at 1.0)
		utilization := math.Min(float64(m.windowRequests)/100.0, 1.0)

		prevScore := m.score
		rawScore := (errRate * weightErrorRate) + (normalizedLat * weightLatency) + (utilization * weightUtilization)
		momentum := 0.0
		if rawScore < prevScore {
			momentum = momentumFactor
		}

		m.score = rawScore
		m.errorRate = errRate
		m.avgLatMs = avgLat
		m.weight = minWeight + (1.0-math.Min(rawScore-momentum, 1.0))*(maxWeight-minWeight)
		if m.weight < minWeight {
			m.weight = minWeight
		}
		if m.weight > maxWeight {
			m.weight = maxWeight
		}

		// Reset window
		m.windowRequests = 0
		m.windowErrors = 0
		m.windowLatencyMs = 0
		m.lastReset = time.Now()

		return true
	})
}

// SelectChannelOrder returns items ordered by adaptive weights when mode is Adaptive.
func (ab *AdaptiveBalancer) SelectChannelOrder(mode types.GroupMode, items []types.GroupItem, groupID int) []types.GroupItem {
	if mode != types.GroupModeAdaptive {
		return ab.base.SelectChannelOrder(mode, items, groupID)
	}

	enabled := make([]types.GroupItem, 0, len(items))
	for _, it := range items {
		if it.Enabled {
			enabled = append(enabled, it)
		}
	}
	if len(enabled) == 0 {
		return enabled
	}

	return ab.adaptiveOrder(enabled)
}

func (ab *AdaptiveBalancer) adaptiveOrder(items []types.GroupItem) []types.GroupItem {
	type weighted struct {
		item   types.GroupItem
		weight float64
	}

	ws := make([]weighted, len(items))
	totalWeight := 0.0
	for i, it := range items {
		w := ab.GetWeight(it.ChannelID)
		ws[i] = weighted{item: it, weight: w}
		totalWeight += w
	}

	if totalWeight == 0 {
		return items
	}

	// Weighted random selection for primary, rest as fallbacks
	result := make([]types.GroupItem, 0, len(items))
	remaining := make([]weighted, len(ws))
	copy(remaining, ws)

	for len(remaining) > 0 {
		r := randFloat64() * totalWeight
		cumulative := 0.0
		selectedIdx := len(remaining) - 1
		for i, w := range remaining {
			cumulative += w.weight
			if r < cumulative {
				selectedIdx = i
				break
			}
		}

		result = append(result, remaining[selectedIdx].item)
		totalWeight -= remaining[selectedIdx].weight
		remaining = append(remaining[:selectedIdx], remaining[selectedIdx+1:]...)
	}

	return result
}

// LogMetrics logs current channel metrics for debugging.
func (ab *AdaptiveBalancer) LogMetrics() {
	ab.metrics.Range(func(key, value any) bool {
		id := key.(int)
		m := value.(*channelMetrics)
		m.mu.Lock()
		log.Printf("[adaptive] channel=%d errorRate=%.2f avgLatMs=%.0f weight=%.3f total=%d",
			id, m.errorRate, m.avgLatMs, m.weight, m.totalRequests)
		m.mu.Unlock()
		return true
	})
}
