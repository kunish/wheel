package relay

import (
	"net/http"
	"sync"
	"time"
)

// healthStatus represents the health state of a channel.
type healthStatus int

const (
	healthUnknown  healthStatus = 0
	healthHealthy  healthStatus = 1
	healthDegraded healthStatus = 2
	healthDown     healthStatus = 3
)

type channelHealth struct {
	Status          healthStatus
	LastCheck       time.Time
	LastSuccess     time.Time
	ConsecutiveFail int
	Latency         time.Duration
}

// HealthCheckTarget describes a channel endpoint to probe.
type HealthCheckTarget struct {
	ChannelID int
	URL       string
	Headers   map[string]string
}

// HealthChecker performs periodic health checks on upstream channels.
type HealthChecker struct {
	mu       sync.RWMutex
	health   map[int]*channelHealth
	client   *http.Client
	interval time.Duration
	enabled  bool
}

// NewHealthChecker creates a HealthChecker. Pass 0 interval to disable.
func NewHealthChecker(interval time.Duration) *HealthChecker {
	return &HealthChecker{
		health:   make(map[int]*channelHealth),
		client:   &http.Client{Timeout: 10 * time.Second},
		interval: interval,
		enabled:  interval > 0,
	}
}

// IsHealthy returns whether a channel is considered healthy.
// Unknown or disabled channels are assumed healthy.
func (hc *HealthChecker) IsHealthy(channelID int) bool {
	if !hc.enabled {
		return true
	}
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	h, ok := hc.health[channelID]
	if !ok {
		return true
	}
	return h.Status != healthDown
}

// GetAllHealth returns a snapshot of all channel health statuses.
func (hc *HealthChecker) GetAllHealth() map[int]healthStatus {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	result := make(map[int]healthStatus, len(hc.health))
	for id, h := range hc.health {
		result[id] = h.Status
	}
	return result
}
