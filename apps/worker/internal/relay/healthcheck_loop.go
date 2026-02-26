package relay

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"
)

// Start begins periodic health checking in the background.
func (hc *HealthChecker) Start(ctx context.Context, getTargets func() []HealthCheckTarget) {
	if !hc.enabled {
		return
	}
	go func() {
		// Run initial check immediately instead of waiting for first tick
		targets := getTargets()
		hc.checkAll(ctx, targets)

		ticker := time.NewTicker(hc.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				targets := getTargets()
				hc.checkAll(ctx, targets)
			}
		}
	}()
}

const maxConcurrentChecks = 10

func (hc *HealthChecker) checkAll(ctx context.Context, targets []HealthCheckTarget) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentChecks)
	for _, t := range targets {
		wg.Add(1)
		sem <- struct{}{} // acquire
		go func(target HealthCheckTarget) {
			defer wg.Done()
			defer func() { <-sem }() // release
			hc.checkOne(ctx, target)
		}(t)
	}
	wg.Wait()
}

func (hc *HealthChecker) checkOne(ctx context.Context, target HealthCheckTarget) {
	// Skip recording failures if context is cancelled (shutdown)
	if ctx.Err() != nil {
		return
	}

	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, "GET", target.URL, nil)
	if err != nil {
		hc.recordFailure(target.ChannelID)
		return
	}
	for k, v := range target.Headers {
		req.Header.Set(k, v)
	}

	resp, err := hc.client.Do(req)
	latency := time.Since(start)

	if err != nil {
		// Don't record failure if the error is due to context cancellation
		if ctx.Err() != nil {
			return
		}
		hc.recordFailure(target.ChannelID)
		return
	}
	defer resp.Body.Close()

	// 2xx-4xx = alive (4xx means auth issue but server is up)
	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		hc.recordSuccess(target.ChannelID, latency)
	} else {
		hc.recordFailure(target.ChannelID)
	}
}

func (hc *HealthChecker) recordSuccess(channelID int, latency time.Duration) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	h := hc.getOrCreate(channelID)
	wasDown := h.Status == HealthDown
	h.Status = HealthHealthy
	h.LastCheck = time.Now()
	h.LastSuccess = time.Now()
	h.ConsecutiveFail = 0
	h.Latency = latency

	if wasDown {
		log.Printf("[healthcheck] channel %d recovered (latency: %v)", channelID, latency)
	}
}

func (hc *HealthChecker) recordFailure(channelID int) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	h := hc.getOrCreate(channelID)
	h.LastCheck = time.Now()
	h.ConsecutiveFail++

	switch {
	case h.ConsecutiveFail >= 3:
		if h.Status != HealthDown {
			log.Printf("[healthcheck] channel %d marked DOWN (%d consecutive failures)",
				channelID, h.ConsecutiveFail)
		}
		h.Status = HealthDown
	case h.ConsecutiveFail >= 1:
		h.Status = HealthDegraded
	}
}

func (hc *HealthChecker) getOrCreate(channelID int) *channelHealth {
	h, ok := hc.health[channelID]
	if !ok {
		h = &channelHealth{Status: HealthUnknown}
		hc.health[channelID] = h
	}
	return h
}
