package handler

import (
	"encoding/json"
	"testing"

	"github.com/kunish/wheel/apps/worker/internal/relay"
)

func TestBuildRerankPayload_RewritesModel(t *testing.T) {
	req := relay.RerankRequest{
		Model:     "frontend-model",
		Query:     "hello",
		Documents: []string{"a", "b"},
		TopN:      1,
	}

	body, err := buildRerankPayload(req, "provider-model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload relay.RerankRequest
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}

	if payload.Model != "provider-model" {
		t.Fatalf("expected rewritten model, got %q", payload.Model)
	}
	if payload.Query != req.Query {
		t.Fatalf("query changed unexpectedly: %q", payload.Query)
	}
	if len(payload.Documents) != len(req.Documents) {
		t.Fatalf("documents changed unexpectedly: %v", payload.Documents)
	}
}
