package relay

import (
	"fmt"
	"sync"
	"time"
)

// SessionEntry holds the sticky channel assignment for an API key + model.
type SessionEntry struct {
	ChannelID    int
	ChannelKeyID int
	Timestamp    int64 // unix ms
}

var (
	sessions   = make(map[string]*SessionEntry)
	sessionsMu sync.RWMutex
)

func sessionKey(apiKeyID int, requestModel string) string {
	return fmt.Sprintf("%d:%s", apiKeyID, requestModel)
}

// GetSticky returns the sticky channel for an API key + model, or nil if expired/absent.
func GetSticky(apiKeyID int, model string, ttlSec int) *SessionEntry {
	if ttlSec <= 0 {
		return nil
	}

	key := sessionKey(apiKeyID, model)

	sessionsMu.RLock()
	entry, ok := sessions[key]
	sessionsMu.RUnlock()

	if !ok {
		return nil
	}

	if time.Now().UnixMilli()-entry.Timestamp > int64(ttlSec)*1000 {
		// Expired — lazy cleanup
		sessionsMu.Lock()
		delete(sessions, key)
		sessionsMu.Unlock()
		return nil
	}

	return entry
}

// SetSticky records the sticky channel for an API key + model.
func SetSticky(apiKeyID int, model string, channelID, channelKeyID int) {
	key := sessionKey(apiKeyID, model)

	sessionsMu.Lock()
	sessions[key] = &SessionEntry{
		ChannelID:    channelID,
		ChannelKeyID: channelKeyID,
		Timestamp:    time.Now().UnixMilli(),
	}
	sessionsMu.Unlock()
}
