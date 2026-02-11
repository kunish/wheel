package cache

import (
	"encoding/json"
	"sync"
	"time"
)

type entry struct {
	value   []byte
	expires int64 // unix ms, 0 = no expiry
}

// MemoryKV is a TTL-aware in-memory key-value store protected by sync.RWMutex.
type MemoryKV struct {
	mu    sync.RWMutex
	store map[string]entry
	stop  chan struct{}
}

// New creates a MemoryKV and starts a background cleanup goroutine.
func New() *MemoryKV {
	m := &MemoryKV{
		store: make(map[string]entry),
		stop:  make(chan struct{}),
	}
	go m.cleanup()
	return m
}

// Get retrieves a value by key, returning nil if missing or expired.
func Get[T any](m *MemoryKV, key string) (*T, bool) {
	m.mu.RLock()
	e, ok := m.store[key]
	m.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if e.expires > 0 && time.Now().UnixMilli() > e.expires {
		m.mu.Lock()
		delete(m.store, key)
		m.mu.Unlock()
		return nil, false
	}
	var v T
	if err := json.Unmarshal(e.value, &v); err != nil {
		return nil, false
	}
	return &v, true
}

// Put stores a value with an optional TTL in seconds. Pass 0 for no expiry.
func (m *MemoryKV) Put(key string, value any, ttlSeconds int) {
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	var expires int64
	if ttlSeconds > 0 {
		expires = time.Now().UnixMilli() + int64(ttlSeconds)*1000
	}
	m.mu.Lock()
	m.store[key] = entry{value: data, expires: expires}
	m.mu.Unlock()
}

// Delete removes a key from the store.
func (m *MemoryKV) Delete(key string) {
	m.mu.Lock()
	delete(m.store, key)
	m.mu.Unlock()
}

// Close stops the background cleanup goroutine.
func (m *MemoryKV) Close() {
	close(m.stop)
}

// cleanup removes expired entries every 60 seconds.
func (m *MemoryKV) cleanup() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now().UnixMilli()
			m.mu.Lock()
			for k, e := range m.store {
				if e.expires > 0 && now > e.expires {
					delete(m.store, k)
				}
			}
			m.mu.Unlock()
		case <-m.stop:
			return
		}
	}
}
