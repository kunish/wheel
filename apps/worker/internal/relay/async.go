package relay

import (
	"fmt"
	"sync"
	"time"
)

// AsyncJobStatus represents the status of an async inference job.
type AsyncJobStatus string

const (
	AsyncStatusQueued     AsyncJobStatus = "queued"
	AsyncStatusProcessing AsyncJobStatus = "processing"
	AsyncStatusCompleted  AsyncJobStatus = "completed"
	AsyncStatusFailed     AsyncJobStatus = "failed"
)

// AsyncJob represents an async inference job.
type AsyncJob struct {
	ID          string         `json:"id"`
	Object      string         `json:"object"`
	Status      AsyncJobStatus `json:"status"`
	CreatedAt   int64          `json:"created_at"`
	UpdatedAt   int64          `json:"updated_at"`
	CompletedAt *int64         `json:"completed_at,omitempty"`
	Model       string         `json:"model"`
	Request     map[string]any `json:"request,omitempty"`
	Response    map[string]any `json:"response,omitempty"`
	Error       *string        `json:"error,omitempty"`
	Usage       map[string]any `json:"usage,omitempty"`
}

// AsyncStore manages async inference jobs in memory.
type AsyncStore struct {
	mu   sync.RWMutex
	jobs map[string]*AsyncJob
}

// NewAsyncStore creates a new async store.
func NewAsyncStore() *AsyncStore {
	return &AsyncStore{
		jobs: make(map[string]*AsyncJob),
	}
}

// CreateJob creates a new async inference job.
func (s *AsyncStore) CreateJob(model string, request map[string]any) *AsyncJob {
	now := time.Now().Unix()
	job := &AsyncJob{
		ID:        fmt.Sprintf("async_%d", time.Now().UnixNano()),
		Object:    "async_inference",
		Status:    AsyncStatusQueued,
		CreatedAt: now,
		UpdatedAt: now,
		Model:     model,
		Request:   request,
	}
	s.mu.Lock()
	s.jobs[job.ID] = job
	s.mu.Unlock()
	return job
}

// GetJob retrieves an async job by ID.
func (s *AsyncStore) GetJob(id string) *AsyncJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.jobs[id]
}

// ListJobs returns all async jobs with pagination.
func (s *AsyncStore) ListJobs(limit, offset int) []*AsyncJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := make([]*AsyncJob, 0, len(s.jobs))
	for _, j := range s.jobs {
		jobs = append(jobs, j)
	}
	// Simple pagination
	if offset >= len(jobs) {
		return nil
	}
	end := offset + limit
	if end > len(jobs) {
		end = len(jobs)
	}
	return jobs[offset:end]
}

// MarkProcessing marks a job as processing.
func (s *AsyncStore) MarkProcessing(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return
	}
	job.Status = AsyncStatusProcessing
	job.UpdatedAt = time.Now().Unix()
}

// CompleteJob marks a job as completed with the response.
func (s *AsyncStore) CompleteJob(id string, response map[string]any, usage map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return
	}
	now := time.Now().Unix()
	job.Status = AsyncStatusCompleted
	job.UpdatedAt = now
	job.CompletedAt = &now
	job.Response = response
	job.Usage = usage
}

// FailJob marks a job as failed with an error message.
func (s *AsyncStore) FailJob(id string, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return
	}
	now := time.Now().Unix()
	job.Status = AsyncStatusFailed
	job.UpdatedAt = now
	job.CompletedAt = &now
	job.Error = &errMsg
}

// Cleanup removes old completed/failed jobs older than the given duration.
func (s *AsyncStore) Cleanup(maxAge time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-maxAge).Unix()
	for id, job := range s.jobs {
		if job.CreatedAt < cutoff && (job.Status == AsyncStatusCompleted || job.Status == AsyncStatusFailed) {
			delete(s.jobs, id)
		}
	}
}
