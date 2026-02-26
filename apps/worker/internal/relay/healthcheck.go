package relay

import (
	"net/http"
	"sync"
	"time"
)

// HealthStatus represents the health state of a channel.
type HealthStatus int

const (
	HealthUnknown  HealthStatus = 0
	HealthHealthy  HealthStatus = 1
	HealthDegraded HealthStatus = 2
	HealthDown     HealthStatus = 3
)

type channelHealth struct {
	Status          HealthStatus
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
	return h.Status != HealthDown
}

// GetAllHealth returns a snapshot of all channel health statuses.
func (hc *HealthChecker) GetAllHealth() map[int]HealthStatus {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	result := make(map[int]HealthStatus, len(hc.health))
	for id, h := range hc.health {
		result[id] = h.Status
	}
	return result
}
