package relay

import (
	"context"
	"log"
	"sync"
	"time"
)

// ProviderRequest represents a queued relay request for a specific provider/channel.
type ProviderRequest struct {
	Ctx       context.Context
	ChannelID int
	Execute   func() error
	ResultCh  chan error
}

// ProviderQueue manages a dedicated worker pool for a single provider/channel
// to prevent head-of-line blocking across providers.
type ProviderQueue struct {
	channelID int
	queue     chan *ProviderRequest
	workers   int
	wg        sync.WaitGroup
	cancel    context.CancelFunc
}

// NewProviderQueue creates a queue with the specified number of workers and buffer size.
func NewProviderQueue(channelID, workers, bufferSize int) *ProviderQueue {
	if workers <= 0 {
		workers = 2
	}
	if bufferSize <= 0 {
		bufferSize = 100
	}
	return &ProviderQueue{
		channelID: channelID,
		queue:     make(chan *ProviderRequest, bufferSize),
		workers:   workers,
	}
}

// Start launches the worker goroutines.
func (pq *ProviderQueue) Start(parentCtx context.Context) {
	ctx, cancel := context.WithCancel(parentCtx)
	pq.cancel = cancel

	for i := 0; i < pq.workers; i++ {
		pq.wg.Add(1)
		go pq.worker(ctx)
	}
}

func (pq *ProviderQueue) worker(ctx context.Context) {
	defer pq.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case req, ok := <-pq.queue:
			if !ok {
				return
			}
			err := req.Execute()
			if req.ResultCh != nil {
				req.ResultCh <- err
			}
		}
	}
}

// Submit adds a request to the queue. Returns an error if the queue is full.
func (pq *ProviderQueue) Submit(req *ProviderRequest) bool {
	select {
	case pq.queue <- req:
		return true
	default:
		return false
	}
}

// Stop gracefully shuts down the queue.
func (pq *ProviderQueue) Stop() {
	if pq.cancel != nil {
		pq.cancel()
	}
	close(pq.queue)
	pq.wg.Wait()
}

// QueueLength returns the current queue depth.
func (pq *ProviderQueue) QueueLength() int {
	return len(pq.queue)
}

// ProviderQueueManager manages per-channel queues.
type ProviderQueueManager struct {
	mu      sync.RWMutex
	queues  map[int]*ProviderQueue
	workers int
	buffer  int
	ctx     context.Context
}

// NewProviderQueueManager creates a queue manager.
func NewProviderQueueManager(ctx context.Context, workersPerQueue, bufferSize int) *ProviderQueueManager {
	if workersPerQueue <= 0 {
		workersPerQueue = 2
	}
	if bufferSize <= 0 {
		bufferSize = 100
	}
	return &ProviderQueueManager{
		queues:  make(map[int]*ProviderQueue),
		workers: workersPerQueue,
		buffer:  bufferSize,
		ctx:     ctx,
	}
}

// GetOrCreate returns the queue for a channel, creating one if needed.
func (m *ProviderQueueManager) GetOrCreate(channelID int) *ProviderQueue {
	m.mu.RLock()
	q, ok := m.queues[channelID]
	m.mu.RUnlock()
	if ok {
		return q
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if q, ok := m.queues[channelID]; ok {
		return q
	}

	q = NewProviderQueue(channelID, m.workers, m.buffer)
	q.Start(m.ctx)
	m.queues[channelID] = q
	log.Printf("[provider_queue] created queue for channel %d (workers=%d)", channelID, m.workers)
	return q
}

// StopAll gracefully stops all queues.
func (m *ProviderQueueManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, q := range m.queues {
		q.Stop()
		delete(m.queues, id)
	}
}

// StartCleanup periodically removes queues for channels with no recent activity.
func (m *ProviderQueueManager) StartCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.mu.Lock()
				for id, q := range m.queues {
					if q.QueueLength() == 0 {
						q.Stop()
						delete(m.queues, id)
					}
				}
				m.mu.Unlock()
			}
		}
	}()
}
