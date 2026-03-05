package db

import (
	"context"
	"log"
	"sync/atomic"
	"time"

	"github.com/uptrace/bun"

	"github.com/kunish/wheel/apps/worker/internal/cache"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/observe"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// CostInfo holds the cost data associated with a log entry.
type CostInfo struct {
	ApiKeyID     int
	ChannelKeyID int
	Cost         float64
}

// logEntry pairs a relay log with its optional cost info.
type logEntry struct {
	Log      types.RelayLog
	Cost     *CostInfo
	StreamID string // non-empty for streaming logs
}

// LogWriter buffers relay logs and flushes them in batches.
type LogWriter struct {
	ch            chan logEntry
	countThresh   int
	timeThresh    time.Duration
	db            *bun.DB
	broadcast     types.BroadcastFunc
	streamTracker types.StreamTracker
	observer      *observe.Observer
	statsCache    *cache.MemoryKV
	dropCount     atomic.Int64
}

// NewLogWriter creates a LogWriter with sensible defaults.
func NewLogWriter(db *bun.DB, broadcast types.BroadcastFunc, streamTracker types.StreamTracker, obs *observe.Observer, statsCache *cache.MemoryKV) *LogWriter {
	return &LogWriter{
		ch:            make(chan logEntry, 1000),
		countThresh:   50,
		timeThresh:    200 * time.Millisecond,
		db:            db,
		broadcast:     broadcast,
		streamTracker: streamTracker,
		observer:      obs,
		statsCache:    statsCache,
	}
}

// Submit enqueues a log entry for batched persistence.
// It never blocks: if the buffer is full the entry is dropped and false is returned.
func (w *LogWriter) Submit(rl types.RelayLog, cost *CostInfo, streamID string) bool {
	select {
	case w.ch <- logEntry{Log: rl, Cost: cost, StreamID: streamID}:
		return true
	default:
		w.dropCount.Add(1)
		w.observer.RecordLogDrop(context.Background())
		return false
	}
}

// DroppedCount returns the total number of log entries dropped due to a full buffer.
func (w *LogWriter) DroppedCount() int64 {
	return w.dropCount.Load()
}

// Run consumes the channel in a loop, flushing on count or time threshold.
// It blocks until ctx is cancelled or the channel is closed.
func (w *LogWriter) Run(ctx context.Context) {
	buf := make([]logEntry, 0, w.countThresh)
	var timer *time.Timer
	var timerC <-chan time.Time

	for {
		select {
		case entry, ok := <-w.ch:
			if !ok {
				if len(buf) > 0 {
					w.flush(buf)
				}
				return
			}
			buf = append(buf, entry)
			if len(buf) >= w.countThresh {
				w.flush(buf)
				buf = buf[:0]
				if timer != nil {
					timer.Stop()
					timer = nil
					timerC = nil
				}
			} else if timer == nil {
				// First entry after idle — start flush timer
				timer = time.NewTimer(w.timeThresh)
				timerC = timer.C
			}

		case <-timerC:
			if len(buf) > 0 {
				w.flush(buf)
				buf = buf[:0]
			}
			timer = nil
			timerC = nil

		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			close(w.ch)
			for entry := range w.ch {
				buf = append(buf, entry)
			}
			if len(buf) > 0 {
				w.flush(buf)
			}
			return
		}
	}
}

// flush persists a batch of logs and their associated cost updates.
func (w *LogWriter) flush(entries []logEntry) {
	ctx := context.Background()

	// 1. Batch INSERT logs into log database within a transaction
	logs := make([]types.RelayLog, len(entries))
	for i := range entries {
		logs[i] = entries[i].Log
	}

	err := w.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return dal.CreateLogsBatch(ctx, tx, logs)
	})
	if err != nil {
		log.Printf("[logwriter] batch insert failed: %v", err)
		return
	}

	// Invalidate stats cache after successful log insert
	if w.statsCache != nil {
		w.statsCache.DeletePrefix("stats:")
	}

	// 2. Aggregate cost updates per api_key and channel_key, then apply in a single transaction
	apiKeyCosts := make(map[int]float64)
	channelKeyCosts := make(map[int]float64)
	for _, e := range entries {
		if e.Cost != nil && e.Cost.Cost > 0 {
			apiKeyCosts[e.Cost.ApiKeyID] += e.Cost.Cost
			channelKeyCosts[e.Cost.ChannelKeyID] += e.Cost.Cost
		}
	}

	if len(apiKeyCosts) > 0 || len(channelKeyCosts) > 0 {
		err := w.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			for id, cost := range apiKeyCosts {
				if err := dal.IncrementApiKeyCostTx(ctx, tx, id, cost); err != nil {
					return err
				}
			}
			for id, cost := range channelKeyCosts {
				if err := dal.IncrementChannelKeyCostTx(ctx, tx, id, cost); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			log.Printf("[logwriter] cost update failed: %v", err)
		}
	}

	// 3. Broadcast log-created events asynchronously (after successful write)
	if w.broadcast != nil {
		// Build broadcast payloads before launching goroutine since entries
		// shares backing array with the caller's buffer and may be overwritten.
		type broadcastItem struct {
			summary  map[string]any
			streamID string
		}
		items := make([]broadcastItem, len(entries))
		for i, e := range entries {
			summary := logSummary(&logs[i])
			if e.StreamID != "" {
				summary["streamId"] = e.StreamID
			}
			items[i] = broadcastItem{summary: summary, streamID: e.StreamID}
		}

		go func() {
			w.broadcast("stats-updated")
			for _, item := range items {
				w.broadcast("log-created", item.summary)
				if item.streamID != "" && w.streamTracker != nil {
					w.streamTracker.UntrackStream(item.streamID)
				}
			}
		}()
	}
}

// Shutdown closes the input channel and returns immediately.
// Run() drains remaining items asynchronously before returning.
func (w *LogWriter) Shutdown() {
	close(w.ch)
}

// logSummary builds the WebSocket payload for a log entry.
func logSummary(l *types.RelayLog) map[string]any {
	return map[string]any{
		"log": map[string]any{
			"id":               l.ID,
			"time":             l.Time,
			"requestModelName": l.RequestModelName,
			"actualModelName":  l.ActualModelName,
			"channelId":        l.ChannelID,
			"channelName":      l.ChannelName,
			"inputTokens":      l.InputTokens,
			"outputTokens":     l.OutputTokens,
			"ftut":             l.FTUT,
			"useTime":          l.UseTime,
			"error":            l.Error,
			"cost":             l.Cost,
			"totalAttempts":    l.TotalAttempts,
		},
	}
}
