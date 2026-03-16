package relay

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/uptrace/bun"
)

// SessionStore defines the interface for session sticky storage.
// Implementations can be in-memory (default) or database-backed (multi-instance).
type SessionStore interface {
	Get(ctx context.Context, key string, ttlSec int) *sessionEntry
	Set(ctx context.Context, key string, entry *sessionEntry)
	Cleanup(ctx context.Context)
}

// InMemorySessionStore wraps the existing SessionManager as a SessionStore.
type InMemorySessionStore struct {
	mgr *SessionManager
}

// NewInMemorySessionStore creates an in-memory session store.
func NewInMemorySessionStore() *InMemorySessionStore {
	return &InMemorySessionStore{mgr: NewSessionManager()}
}

func (s *InMemorySessionStore) Get(_ context.Context, key string, ttlSec int) *sessionEntry {
	parts := parseSessionKey(key)
	if parts == nil {
		return nil
	}
	return s.mgr.GetSticky(parts.apiKeyID, parts.model, ttlSec)
}

func (s *InMemorySessionStore) Set(_ context.Context, key string, entry *sessionEntry) {
	parts := parseSessionKey(key)
	if parts == nil {
		return
	}
	s.mgr.SetSticky(parts.apiKeyID, parts.model, entry.ChannelID, entry.ChannelKeyID)
}

func (s *InMemorySessionStore) Cleanup(_ context.Context) {}

// DBSessionStore stores session mappings in the database for multi-instance deployments.
type DBSessionStore struct {
	db *bun.DB
}

// dbSessionRow represents a row in the session_sticky table.
type dbSessionRow struct {
	bun.BaseModel `bun:"table:session_sticky"`
	Key           string `bun:"key,pk"`
	ChannelID     int    `bun:"channel_id"`
	ChannelKeyID  int    `bun:"channel_key_id"`
	UpdatedAt     int64  `bun:"updated_at"`
}

// NewDBSessionStore creates a DB-backed session store.
func NewDBSessionStore(db *bun.DB) *DBSessionStore {
	ctx := context.Background()
	_, err := db.NewCreateTable().Model((*dbSessionRow)(nil)).IfNotExists().Exec(ctx)
	if err != nil {
		log.Printf("[session_store] failed to create session_sticky table: %v", err)
	}
	return &DBSessionStore{db: db}
}

func (s *DBSessionStore) Get(ctx context.Context, key string, ttlSec int) *sessionEntry {
	if ttlSec <= 0 {
		return nil
	}
	var row dbSessionRow
	err := s.db.NewSelect().Model(&row).Where("\"key\" = ?", key).Scan(ctx)
	if err != nil {
		return nil
	}
	if time.Now().UnixMilli()-row.UpdatedAt > int64(ttlSec)*1000 {
		_, _ = s.db.NewDelete().Model((*dbSessionRow)(nil)).Where("\"key\" = ?", key).Exec(ctx)
		return nil
	}
	return &sessionEntry{
		ChannelID:    row.ChannelID,
		ChannelKeyID: row.ChannelKeyID,
		Timestamp:    row.UpdatedAt,
	}
}

func (s *DBSessionStore) Set(ctx context.Context, key string, entry *sessionEntry) {
	row := &dbSessionRow{
		Key:          key,
		ChannelID:    entry.ChannelID,
		ChannelKeyID: entry.ChannelKeyID,
		UpdatedAt:    time.Now().UnixMilli(),
	}
	_, err := s.db.NewInsert().Model(row).
		On("CONFLICT (\"key\") DO UPDATE").
		Set("channel_id = EXCLUDED.channel_id").
		Set("channel_key_id = EXCLUDED.channel_key_id").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	if err != nil {
		log.Printf("[session_store] failed to upsert session: %v", err)
	}
}

func (s *DBSessionStore) Cleanup(ctx context.Context) {
	cutoff := time.Now().Add(-1 * time.Hour).UnixMilli()
	result, err := s.db.NewDelete().Model((*dbSessionRow)(nil)).
		Where("updated_at < ?", cutoff).Exec(ctx)
	if err != nil {
		log.Printf("[session_store] cleanup error: %v", err)
		return
	}
	if n, _ := result.RowsAffected(); n > 0 {
		log.Printf("[session_store] cleaned up %d expired sessions", n)
	}
}

type sessionKeyParts struct {
	apiKeyID int
	model    string
}

func parseSessionKey(key string) *sessionKeyParts {
	var apiKeyID int
	var model string
	n, _ := fmt.Sscanf(key, "%d:", &apiKeyID)
	if n == 1 {
		for i := 0; i < len(key); i++ {
			if key[i] == ':' {
				model = key[i+1:]
				break
			}
		}
	}
	if model == "" {
		return nil
	}
	return &sessionKeyParts{apiKeyID: apiKeyID, model: model}
}

// ExternalizedSessionManager wraps a SessionStore and provides the same API as SessionManager.
type ExternalizedSessionManager struct {
	store SessionStore
}

// NewExternalizedSessionManager creates a session manager backed by the given store.
func NewExternalizedSessionManager(store SessionStore) *ExternalizedSessionManager {
	return &ExternalizedSessionManager{store: store}
}

// GetSticky returns the sticky channel for an API key + model.
func (m *ExternalizedSessionManager) GetSticky(ctx context.Context, apiKeyID int, model string, ttlSec int) *sessionEntry {
	key := sessionKey(apiKeyID, model)
	return m.store.Get(ctx, key, ttlSec)
}

// SetSticky records the sticky channel for an API key + model.
func (m *ExternalizedSessionManager) SetSticky(ctx context.Context, apiKeyID int, model string, channelID, channelKeyID int) {
	key := sessionKey(apiKeyID, model)
	m.store.Set(ctx, key, &sessionEntry{
		ChannelID:    channelID,
		ChannelKeyID: channelKeyID,
		Timestamp:    time.Now().UnixMilli(),
	})
}

// StartCleanup runs periodic cleanup in the background.
func (m *ExternalizedSessionManager) StartCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.store.Cleanup(ctx)
			}
		}
	}()
}
