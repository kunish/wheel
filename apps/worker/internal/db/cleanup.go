package db

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/uptrace/bun"

	"github.com/kunish/wheel/apps/worker/internal/db/dal"
)

// StartLogCleanup launches a background goroutine that cleans up expired logs.
func StartLogCleanup(ctx context.Context, db *bun.DB) {
	go func() {
		cleanupLogs(db)

		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				cleanupLogs(db)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func cleanupLogs(db *bun.DB) {
	ctx := context.Background()

	days, err := dal.GetSetting(ctx, db, "log_retention_days")
	if err != nil {
		log.Printf("[cleanup] failed to read log_retention_days: %v", err)
		return
	}

	retentionDays := 30
	if days != nil {
		if n, err := strconv.Atoi(*days); err == nil && n > 0 {
			retentionDays = n
		}
	}

	deleted, err := dal.CleanupOldLogs(ctx, db, retentionDays)
	if err != nil {
		log.Printf("[cleanup] failed to clean up logs: %v", err)
		return
	}
	if deleted > 0 {
		log.Printf("[cleanup] deleted %d expired logs (retention: %d days)", deleted, retentionDays)
	}
}
