package relay

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// sessionEntry holds the sticky channel assignment for an API key + model.
type sessionEntry struct {
	ChannelID    int
	ChannelKeyID int
	Timestamp    int64 // unix ms
}

// SessionManager manages session sticky state for API key + model combos.
type SessionManager struct {
	sessions map[string]*sessionEntry
	mu       sync.RWMutex
}

// NewSessionManager creates a new SessionManager.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*sessionEntry),
	}
}

func sessionKey(apiKeyID int, requestModel string) string {
	return fmt.Sprintf("%d:%s", apiKeyID, requestModel)
}

// GetSticky returns the sticky channel for an API key + model, or nil if expired/absent.
func (m *SessionManager) GetSticky(apiKeyID int, model string, ttlSec int) *sessionEntry {
	if ttlSec <= 0 {
		return nil
	}

	key := sessionKey(apiKeyID, model)

	m.mu.RLock()
	entry, ok := m.sessions[key]
	m.mu.RUnlock()

	if !ok {
		return nil
	}

	if time.Now().UnixMilli()-entry.Timestamp > int64(ttlSec)*1000 {
		// Expired — lazy cleanup
		m.mu.Lock()
		delete(m.sessions, key)
		m.mu.Unlock()
		return nil
	}

	return entry
}

// SetSticky records the sticky channel for an API key + model.
func (m *SessionManager) SetSticky(apiKeyID int, model string, channelID, channelKeyID int) {
	key := sessionKey(apiKeyID, model)

	m.mu.Lock()
	m.sessions[key] = &sessionEntry{
		ChannelID:    channelID,
		ChannelKeyID: channelKeyID,
		Timestamp:    time.Now().UnixMilli(),
	}
	m.mu.Unlock()
}

// StartCleanup runs a background goroutine that removes expired sessions every 5 minutes.
// Sessions older than 1 hour are always removed regardless of their configured TTL.
func (m *SessionManager) StartCleanup(ctx context.Context) {
	const interval = 5 * time.Minute
	const maxAge = 1 * time.Hour

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
				for key, entry := range m.sessions {
					if (now - entry.Timestamp) > maxAge.Milliseconds() {
						delete(m.sessions, key)
						removed++
					}
				}
				m.mu.Unlock()
				if removed > 0 {
					log.Printf("[cleanup] removed %d expired session entries", removed)
				}
			}
		}
	}()
}
