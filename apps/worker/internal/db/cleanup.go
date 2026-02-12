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
// It runs immediately once, then repeats every hour.
func StartLogCleanup(ctx context.Context, mainDB, logDB *bun.DB) {
	go func() {
		cleanupLogs(mainDB, logDB)

		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				cleanupLogs(mainDB, logDB)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func cleanupLogs(mainDB, logDB *bun.DB) {
	ctx := context.Background()

	days, err := dal.GetSetting(ctx, mainDB, "log_retention_days")
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

	deleted, err := dal.CleanupOldLogs(ctx, logDB, retentionDays)
	if err != nil {
		log.Printf("[cleanup] failed to clean up logs: %v", err)
		return
	}
	if deleted > 0 {
		log.Printf("[cleanup] deleted %d expired logs (retention: %d days)", deleted, retentionDays)
	}
}
