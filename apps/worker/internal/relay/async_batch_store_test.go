package relay

import "testing"

func TestAsyncStore_CreateJobStoresApiKeyID(t *testing.T) {
	store := NewAsyncStore()
	job := store.CreateJob("gpt-4.1", 42, map[string]any{"model": "gpt-4.1"})

	if job == nil {
		t.Fatal("expected job to be created")
	}
	if job.ApiKeyID != 42 {
		t.Fatalf("expected api key id 42, got %d", job.ApiKeyID)
	}
}

func TestBatchStore_CreateJobStoresApiKeyID(t *testing.T) {
	store := NewBatchStore()
	job := store.CreateJob([]BatchRequest{{CustomID: "c1", URL: "/v1/chat/completions", Body: map[string]any{"model": "gpt-4.1"}}}, "gpt-4.1", 7)

	if job == nil {
		t.Fatal("expected batch job to be created")
	}
	if job.ApiKeyID != 7 {
		t.Fatalf("expected api key id 7, got %d", job.ApiKeyID)
	}
}

func TestBatchStore_UpdateJobDoesNotOverrideCancelledStatus(t *testing.T) {
	store := NewBatchStore()
	job := store.CreateJob([]BatchRequest{{CustomID: "c1", URL: "/v1/chat/completions", Body: map[string]any{"model": "gpt-4.1"}}}, "gpt-4.1", 9)

	if ok := store.CancelJob(job.ID); !ok {
		t.Fatal("expected cancel to succeed")
	}

	store.UpdateJob(job.ID, BatchStatusProcessing, []BatchResponse{{
		CustomID: "c1",
		Response: &BatchResult{StatusCode: 200, Body: map[string]any{"ok": true}},
	}})

	got := store.GetJob(job.ID)
	if got == nil {
		t.Fatal("expected batch job to exist")
	}
	if got.Status != BatchStatusCancelled {
		t.Fatalf("expected status to remain cancelled, got %s", got.Status)
	}
	if got.RequestCounts.Completed != 1 || got.RequestCounts.Failed != 0 {
		t.Fatalf("expected request counts to update while cancelled, got %+v", got.RequestCounts)
	}
}
