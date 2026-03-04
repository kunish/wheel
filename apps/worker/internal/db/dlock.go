package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/uptrace/bun"
)

// DistributedLock provides database-based distributed locking
// using a table-based advisory lock for MySQL/TiDB compatibility.
type DistributedLock struct {
	db *bun.DB
}

// NewDistributedLock creates a new distributed lock manager.
func NewDistributedLock(db *bun.DB) *DistributedLock {
	return &DistributedLock{db: db}
}

// TryAcquire attempts to acquire a named lock with a TTL.
// Returns true if the lock was acquired, false otherwise.
func (d *DistributedLock) TryAcquire(ctx context.Context, lockName, holderID string, ttl time.Duration) (bool, error) {
	expiresAt := time.Now().Add(ttl)

	// Try to insert the lock (first time)
	// Or update if the lock has expired or held by same holder (reclaim)
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO distributed_locks (lock_name, holder_id, acquired_at, expires_at)
		VALUES (?, ?, NOW(), ?)
		ON DUPLICATE KEY UPDATE
			holder_id = IF(expires_at < NOW() OR holder_id = VALUES(holder_id), VALUES(holder_id), holder_id),
			acquired_at = IF(expires_at < NOW() OR holder_id = VALUES(holder_id), NOW(), acquired_at),
			expires_at = IF(expires_at < NOW() OR holder_id = VALUES(holder_id), VALUES(expires_at), expires_at)
	`, lockName, holderID, expiresAt)
	if err != nil {
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}

	// Check if we actually hold the lock now
	var currentHolder string
	err = d.db.QueryRowContext(ctx,
		"SELECT holder_id FROM distributed_locks WHERE lock_name = ?", lockName,
	).Scan(&currentHolder)
	if err != nil {
		return false, fmt.Errorf("failed to verify lock: %w", err)
	}

	return currentHolder == holderID, nil
}

// Release releases a named lock if held by the specified holder.
func (d *DistributedLock) Release(ctx context.Context, lockName, holderID string) error {
	_, err := d.db.ExecContext(ctx,
		"DELETE FROM distributed_locks WHERE lock_name = ? AND holder_id = ?",
		lockName, holderID,
	)
	return err
}

// Renew extends the TTL of a lock held by the specified holder.
func (d *DistributedLock) Renew(ctx context.Context, lockName, holderID string, ttl time.Duration) (bool, error) {
	expiresAt := time.Now().Add(ttl)
	result, err := d.db.ExecContext(ctx,
		"UPDATE distributed_locks SET expires_at = ? WHERE lock_name = ? AND holder_id = ?",
		expiresAt, lockName, holderID,
	)
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows > 0, nil
}

// WithLock acquires a lock, runs the function, and releases the lock.
// It will retry acquiring the lock for up to the specified timeout.
func (d *DistributedLock) WithLock(ctx context.Context, lockName, holderID string, ttl, timeout time.Duration, fn func() error) error {
	deadline := time.Now().Add(timeout)

	for {
		acquired, err := d.TryAcquire(ctx, lockName, holderID, ttl)
		if err != nil {
			return fmt.Errorf("lock acquire error: %w", err)
		}
		if acquired {
			defer func() {
				if err := d.Release(ctx, lockName, holderID); err != nil {
					log.Printf("[dlock] failed to release lock %s: %v", lockName, err)
				}
			}()
			return fn()
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for lock %s", lockName)
		}

		// Wait before retrying
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// CleanupExpired removes all expired locks.
func (d *DistributedLock) CleanupExpired(ctx context.Context) (int64, error) {
	result, err := d.db.ExecContext(ctx,
		"DELETE FROM distributed_locks WHERE expires_at < NOW()",
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
