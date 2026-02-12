package db

import (
	"context"
	"log"
	"time"

	"github.com/uptrace/bun"

	"github.com/kunish/wheel/apps/worker/internal/db/dal"
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

// BroadcastFunc is the signature for WebSocket broadcast.
type BroadcastFunc func(event string, data ...any)

// StreamTracker tracks active streams so new WS clients get a snapshot.
type StreamTracker interface {
	UntrackStream(streamId string)
}

// LogWriter buffers relay logs and flushes them in batches.
type LogWriter struct {
	ch            chan logEntry
	countThresh   int
	timeThresh    time.Duration
	logDB         *bun.DB
	mainDB        *bun.DB
	broadcast     BroadcastFunc
	streamTracker StreamTracker
}

// NewLogWriter creates a LogWriter with sensible defaults.
func NewLogWriter(logDB, mainDB *bun.DB, broadcast BroadcastFunc, streamTracker StreamTracker) *LogWriter {
	return &LogWriter{
		ch:            make(chan logEntry, 1000),
		countThresh:   50,
		timeThresh:    2 * time.Second,
		logDB:         logDB,
		mainDB:        mainDB,
		broadcast:     broadcast,
		streamTracker: streamTracker,
	}
}

// Submit enqueues a log entry for batched persistence.
// It blocks if the internal buffer is full (backpressure).
func (w *LogWriter) Submit(rl types.RelayLog, cost *CostInfo, streamID string) {
	w.ch <- logEntry{Log: rl, Cost: cost, StreamID: streamID}
}

// Run consumes the channel in a loop, flushing on count or time threshold.
// It blocks until ctx is cancelled or the channel is closed.
func (w *LogWriter) Run(ctx context.Context) {
	buf := make([]logEntry, 0, w.countThresh)
	timer := time.NewTimer(w.timeThresh)
	defer timer.Stop()

	for {
		select {
		case entry, ok := <-w.ch:
			if !ok {
				// Channel closed — flush remaining
				if len(buf) > 0 {
					w.flush(buf)
				}
				return
			}
			buf = append(buf, entry)
			if len(buf) >= w.countThresh {
				w.flush(buf)
				buf = buf[:0]
				timer.Reset(w.timeThresh)
			}

		case <-timer.C:
			if len(buf) > 0 {
				w.flush(buf)
				buf = buf[:0]
			}
			timer.Reset(w.timeThresh)

		case <-ctx.Done():
			// Drain remaining items from channel
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

	err := w.logDB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return dal.CreateLogsBatch(ctx, tx, logs)
	})
	if err != nil {
		log.Printf("[logwriter] batch insert failed: %v", err)
		return
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
		err := w.mainDB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
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

	// 3. Broadcast log-created events (after successful write)
	if w.broadcast != nil {
		w.broadcast("stats-updated")
		for i, e := range entries {
			summary := logSummary(&logs[i])
			if e.StreamID != "" {
				summary["streamId"] = e.StreamID
			}
			w.broadcast("log-created", summary)

			if e.StreamID != "" && w.streamTracker != nil {
				w.streamTracker.UntrackStream(e.StreamID)
			}
		}
	}
}

// Shutdown closes the channel and waits for Run to drain remaining items.
// Call this during graceful shutdown. Run() will return after draining.
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
