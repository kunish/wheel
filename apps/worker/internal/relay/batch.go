package relay

import (
	"fmt"
	"sync"
	"time"
)

// BatchStatus represents the status of a batch job.
type BatchStatus string

const (
	BatchStatusPending    BatchStatus = "pending"
	BatchStatusProcessing BatchStatus = "processing"
	BatchStatusCompleted  BatchStatus = "completed"
	BatchStatusFailed     BatchStatus = "failed"
	BatchStatusCancelled  BatchStatus = "cancelled"
)

// BatchRequest represents a single request within a batch.
type BatchRequest struct {
	CustomID string         `json:"custom_id"`
	Method   string         `json:"method"`
	URL      string         `json:"url"`
	Body     map[string]any `json:"body"`
}

// BatchResponse represents the response for a single batch request.
type BatchResponse struct {
	CustomID string       `json:"custom_id"`
	Response *BatchResult `json:"response,omitempty"`
	Error    *BatchError  `json:"error,omitempty"`
}

// BatchResult is the successful result for a batch item.
type BatchResult struct {
	StatusCode int            `json:"status_code"`
	Body       map[string]any `json:"body"`
}

// BatchError is the error result for a batch item.
type BatchError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// BatchJob represents a batch processing job.
type BatchJob struct {
	ID            string          `json:"id"`
	Object        string          `json:"object"`
	Status        BatchStatus     `json:"status"`
	CreatedAt     int64           `json:"created_at"`
	CompletedAt   *int64          `json:"completed_at,omitempty"`
	ApiKeyID      int             `json:"-"`
	RequestCounts BatchCounts     `json:"request_counts"`
	Requests      []BatchRequest  `json:"-"`
	Responses     []BatchResponse `json:"responses,omitempty"`
	Model         string          `json:"model,omitempty"`
}

// BatchCounts tracks batch request counts.
type BatchCounts struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

// BatchStore manages batch jobs in memory.
type BatchStore struct {
	mu   sync.RWMutex
	jobs map[string]*BatchJob
}

// NewBatchStore creates a new batch store.
func NewBatchStore() *BatchStore {
	return &BatchStore{
		jobs: make(map[string]*BatchJob),
	}
}

// CreateJob creates a new batch job and returns it.
func (s *BatchStore) CreateJob(requests []BatchRequest, model string, apiKeyID int) *BatchJob {
	job := &BatchJob{
		ID:        fmt.Sprintf("batch_%d", time.Now().UnixNano()),
		Object:    "batch",
		Status:    BatchStatusPending,
		CreatedAt: time.Now().Unix(),
		ApiKeyID:  apiKeyID,
		RequestCounts: BatchCounts{
			Total: len(requests),
		},
		Requests: requests,
		Model:    model,
	}
	s.mu.Lock()
	s.jobs[job.ID] = job
	s.mu.Unlock()
	return job
}

// GetJob retrieves a batch job by ID.
func (s *BatchStore) GetJob(id string) *BatchJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.jobs[id]
}

// ListJobs returns all batch jobs.
func (s *BatchStore) ListJobs() []*BatchJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := make([]*BatchJob, 0, len(s.jobs))
	for _, j := range s.jobs {
		jobs = append(jobs, j)
	}
	return jobs
}

// CancelJob cancels a pending/processing batch job.
func (s *BatchStore) CancelJob(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return false
	}
	if job.Status == BatchStatusPending || job.Status == BatchStatusProcessing {
		job.Status = BatchStatusCancelled
		return true
	}
	return false
}

// UpdateJob updates a batch job's status and results.
func (s *BatchStore) UpdateJob(id string, status BatchStatus, responses []BatchResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return
	}
	if job.Status == BatchStatusCancelled && status != BatchStatusCancelled {
		if responses != nil {
			job.Responses = responses
			completed := 0
			failed := 0
			for _, r := range responses {
				if r.Error != nil {
					failed++
				} else {
					completed++
				}
			}
			job.RequestCounts.Completed = completed
			job.RequestCounts.Failed = failed
		}
		return
	}
	job.Status = status
	job.Responses = responses
	completed := 0
	failed := 0
	for _, r := range responses {
		if r.Error != nil {
			failed++
		} else {
			completed++
		}
	}
	job.RequestCounts.Completed = completed
	job.RequestCounts.Failed = failed
	if status == BatchStatusCompleted || status == BatchStatusFailed {
		now := time.Now().Unix()
		job.CompletedAt = &now
	}
}

// Cleanup removes old completed/failed/cancelled jobs older than the given duration.
func (s *BatchStore) Cleanup(maxAge time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-maxAge).Unix()
	for id, job := range s.jobs {
		if job.CreatedAt < cutoff && (job.Status == BatchStatusCompleted || job.Status == BatchStatusFailed || job.Status == BatchStatusCancelled) {
			delete(s.jobs, id)
		}
	}
}
