package handler

import (
	"encoding/json"
	"testing"
)

func decodeJSONBody(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to unmarshal json body: %v body=%s", err, string(body))
	}
	return payload
}

func assertStableOpenAIErrorEnvelope(t *testing.T, body []byte, wantType, wantMessage string) {
	t.Helper()

	payload := decodeJSONBody(t, body)
	errObj, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected top-level error object, got %#v", payload)
	}
	if got := errObj["type"]; got != wantType {
		t.Fatalf("error.type = %#v, want %q", got, wantType)
	}
	if got := errObj["message"]; got != wantMessage {
		t.Fatalf("error.message = %#v, want %q", got, wantMessage)
	}
	if _, ok := errObj["param"]; !ok {
		t.Fatalf("expected error.param key, got %#v", errObj)
	}
	if errObj["param"] != nil {
		t.Fatalf("expected error.param null, got %#v", errObj["param"])
	}
	if _, ok := errObj["code"]; !ok {
		t.Fatalf("expected error.code key, got %#v", errObj)
	}
	if errObj["code"] != nil {
		t.Fatalf("expected error.code null, got %#v", errObj["code"])
	}
}
